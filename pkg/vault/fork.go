package vault

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	libfossil "github.com/danmestas/go-libfossil"
)

// ForkInfo is the provenance recorded by a taolu fork: the source taolu, the
// version it was forked from, and that version's check-in UUID.
type ForkInfo struct {
	Source     TaoluRef `json:"source"`
	Version    string   `json:"version"`
	SourceUUID string   `json:"source_uuid"`
}

// forkMarkerContent marshals a ForkInfo into the .fork marker file content.
func forkMarkerContent(f ForkInfo) string {
	data, _ := json.Marshal(f)
	return string(data)
}

// ParseForkInfo parses .fork marker content into a ForkInfo. Empty or
// malformed content yields ok=false (treated as "not a fork"), never an error,
// so stale data never breaks reads.
func ParseForkInfo(content string) (ForkInfo, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return ForkInfo{}, false
	}
	var f ForkInfo
	if err := json.Unmarshal([]byte(content), &f); err != nil {
		return ForkInfo{}, false
	}
	if f.Source.Domain == "" || f.Source.Group == "" || f.Source.Name == "" {
		return ForkInfo{}, false
	}
	return f, true
}

// ReadForkInfo reads the .fork provenance marker of a taolu at tip, or nil when
// the taolu is not a fork.
func ReadForkInfo(r *libfossil.Repo, ref TaoluRef) (*ForkInfo, error) {
	data, err := r.ReadFileAt("tip", filepath.Join(ref.Dir(), forkMarker))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	f, ok := ParseForkInfo(string(data))
	if !ok {
		return nil, nil
	}
	return &f, nil
}

// forkPathToSkill converts a .fork source ref "@domain/group/name" to the
// SKILL.md path of that taolu, or "" if empty/malformed.
func forkPathToSkill(source string) string {
	if source == "" {
		return ""
	}
	parts := strings.Split(source, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "@") {
		return ""
	}
	return filepath.Join(taoluRoot, parts[0], parts[1], parts[2], "SKILL.md")
}

// VaultForkInfo returns the vault-level fork provenance from config: an
// upstream identity string and the source commit the vault forked from.
func VaultForkInfo(r *libfossil.Repo) (upstream, sourceCommit string, err error) {
	upstream, err = r.Config("fork-upstream")
	if err != nil {
		return "", "", err
	}
	sourceCommit, err = r.Config("fork-source-commit")
	if err != nil {
		return "", "", err
	}
	return upstream, sourceCommit, nil
}

// SetVaultForkInfo records the vault-level fork provenance in config.
func SetVaultForkInfo(r *libfossil.Repo, upstream, sourceCommit string) error {
	if err := r.SetConfig("fork-upstream", upstream); err != nil {
		return err
	}
	return r.SetConfig("fork-source-commit", sourceCommit)
}

// ForkTaolu copies a taolu (SKILL.md, ACTION.md, and files/ assets) into a new
// name under the same domain/group, records a .fork provenance marker, and
// lets SkillHistory follow the fork so the new taolu's history shows the copied
// upstream lineage followed by its own independent versions. The source taolu
// is left untouched.
func ForkTaolu(r *libfossil.Repo, source TaoluRef, newName, newGroup, message, user string) (ForkInfo, error) {
	if !ValidSlug(newName) {
		return ForkInfo{}, fmt.Errorf("invalid new name %q: must be 1-64 lowercase alphanumeric with single hyphen separators", newName)
	}
	if newGroup == "" {
		newGroup = source.Group
	} else if !ValidSlug(newGroup) {
		return ForkInfo{}, fmt.Errorf("invalid group %q: must be 1-64 lowercase alphanumeric with single hyphens", newGroup)
	}
	if source.Name == SeedName {
		return ForkInfo{}, fmt.Errorf("refusing to fork the built-in %q guide", SeedName)
	}

	srcPath, err := FindSkillPathByRefResolved(r, source)
	if err != nil {
		return ForkInfo{}, err
	}
	if srcPath == "" {
		return ForkInfo{}, fmt.Errorf("taolu %q not found in vault", source.String())
	}
	archived, err := IsArchived(r, srcPath)
	if err != nil {
		return ForkInfo{}, err
	}
	if archived {
		return ForkInfo{}, fmt.Errorf("taolu %q is archived and must not be forked; restore it first", source.String())
	}
	targetRef := TaoluRef{Domain: source.Domain, Group: newGroup, Name: newName}
	if existing, err := FindSkillPathByRefResolved(r, targetRef); err != nil {
		return ForkInfo{}, err
	} else if existing != "" {
		return ForkInfo{}, fmt.Errorf("taolu %q already exists; refusing to fork onto it", targetRef.String())
	}

	skill, action, assets, err := ReadTaoluBundleByRef(r, source, "")
	if err != nil {
		return ForkInfo{}, err
	}
	renamed, err := renameSkillFrontmatter(skill, newName)
	if err != nil {
		return ForkInfo{}, err
	}

	// Fork at the source's tip version: resolve its UUID for provenance.
	hist, err := SkillHistory(r, srcPath)
	if err != nil {
		return ForkInfo{}, err
	}
	forkInfo := ForkInfo{
		Source:     source,
		Version:    "tip",
		SourceUUID: "",
	}
	if len(hist) > 0 {
		last := hist[len(hist)-1]
		forkInfo.Version = last.Label
		forkInfo.SourceUUID = last.UUID
	}

	tree, err := tipTree(r)
	if err != nil {
		return ForkInfo{}, err
	}
	files := make([]libfossil.FileToCommit, 0, len(tree)+len(assets)+3)
	files = append(files, tree...)
	files = append(files,
		libfossil.FileToCommit{Name: targetRef.Path(), Content: []byte(renamed)},
		libfossil.FileToCommit{Name: targetRef.ActionPath(), Content: []byte(action)},
		libfossil.FileToCommit{Name: filepath.Join(targetRef.Dir(), forkMarker), Content: []byte(forkMarkerContent(forkInfo))},
	)
	for _, a := range assets {
		files = append(files, libfossil.FileToCommit{
			Name:    targetRef.AssetPath(a.Path),
			Content: []byte(a.Content),
		})
	}

	if user == "" {
		user = "admin"
	}
	if message == "" {
		message = fmt.Sprintf("fork taolu %s to %s", source.String(), newName)
	}
	rid, _, err := commitFullTree(r, files, message, user)
	if err != nil {
		return ForkInfo{}, err
	}

	// The fork's own first version continues the copied lineage: if the source
	// had N versions, the fork commit is vN+1.
	nextLabel := fmt.Sprintf("v%d", len(hist)+1)
	if _, err := r.Tag(libfossil.TagOpts{
		Name:     newName + "-" + nextLabel,
		TargetID: rid,
		User:     user,
	}); err != nil {
		return ForkInfo{}, fmt.Errorf("tag version: %w", err)
	}
	return forkInfo, nil
}
