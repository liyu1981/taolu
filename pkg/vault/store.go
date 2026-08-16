package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	libfossil "github.com/danmestas/go-libfossil"
	_ "github.com/danmestas/go-libfossil/db/driver/modernc"
)

// TaoluInfo is a listing entry for a taolu.
type TaoluInfo struct {
	Name          string
	Group         string
	Mode          string
	Description   string
	Tags          string
	LatestVersion string
	LatestUUID    string
}

// PracticeVersion is one version of a taolu.
type PracticeVersion struct {
	Label   string
	UUID    string
	Date    time.Time
	User    string
	Message string
}

// OpenVault opens the vault repo at path, or fails with a hint to run taolu_init.
func OpenVault(path string) (*libfossil.Repo, string, error) {
	p, err := VaultPath(path)
	if err != nil {
		return nil, "", err
	}
	r, err := libfossil.Open(p)
	if err != nil {
		return nil, "", fmt.Errorf("vault not initialized at %s (run taolu_init first): %w", p, err)
	}
	return r, p, nil
}

// VaultPath resolves the requested path, falling back to TAOLU_REPO and
// then to ~/.taolu/vault.fossil.
func VaultPath(arg string) (string, error) {
	if arg != "" {
		return filepath.Clean(arg), nil
	}
	if env := os.Getenv("TAOLU_REPO"); env != "" {
		return filepath.Clean(env), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".taolu", "vault.fossil"), nil
}

// FindSkillPath returns the vault path of the taolu's SKILL.md, or "" if not present.
func FindSkillPath(r *libfossil.Repo, name string) (string, error) {
	rid, err := r.ResolveVersion("tip")
	if errors.Is(err, libfossil.ErrVersionNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	files, err := r.ListFiles(rid)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if !strings.HasPrefix(f.Name, taoluRoot+string(filepath.Separator)) {
			continue
		}
		if filepath.Base(f.Name) != "SKILL.md" {
			continue
		}
		if filepath.Base(filepath.Dir(f.Name)) == name {
			return f.Name, nil
		}
	}
	return "", nil
}

// ListTaolu enumerates all taolus at the vault tip.
func ListTaolu(r *libfossil.Repo) ([]TaoluInfo, error) {
	rid, err := r.ResolveVersion("tip")
	if errors.Is(err, libfossil.ErrVersionNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files, err := r.ListFiles(rid)
	if err != nil {
		return nil, err
	}
	var out []TaoluInfo
	for _, f := range files {
		group, name, ok := parseSkillPath(f.Name)
		if !ok {
			continue
		}
		content, err := r.ReadFileAt("tip", f.Name)
		if err != nil {
			continue
		}
		meta, _, err := splitFrontmatter(string(content))
		if err != nil {
			continue
		}
		mode := ModeInstall
		if actionData, err := r.ReadFileAt("tip", filepath.Join(filepath.Dir(f.Name), "ACTION.md")); err == nil {
			if am, _, err := splitActionFrontmatter(string(actionData)); err == nil && ValidActionMode(am.Mode) {
				mode = am.Mode
			}
		}
		hist, err := SkillHistory(r, f.Name)
		if err != nil {
			return nil, err
		}
		info := TaoluInfo{
			Name:        name,
			Group:       group,
			Mode:        mode,
			Description: meta.Description,
			Tags:        meta.Metadata["tags"],
		}
		if len(hist) > 0 {
			info.LatestVersion = hist[len(hist)-1].Label
			info.LatestUUID = hist[len(hist)-1].UUID
		} else {
			info.LatestVersion = "v1"
		}
		out = append(out, info)
	}
	return out, nil
}

// UniqueGroups returns the distinct practice groups in taolus, in first-seen order.
func UniqueGroups(skills []TaoluInfo) []string {
	seen := map[string]bool{}
	var groups []string
	for _, s := range skills {
		if !seen[s.Group] {
			seen[s.Group] = true
			groups = append(groups, s.Group)
		}
	}
	return groups
}

// ReadTaoluAtVersion returns the SKILL.md and ACTION.md content of a taolu at
// a version. The pair is always read at the same check-in.
func ReadTaoluAtVersion(r *libfossil.Repo, name, version string) (skill, action string, err error) {
	path, err := FindSkillPath(r, name)
	if err != nil {
		return "", "", err
	}
	if path == "" {
		return "", "", fmt.Errorf("taolu %q not found in vault", name)
	}
	uuid, err := resolveSkillVersion(r, path, version)
	if err != nil {
		return "", "", err
	}
	skillData, err := r.ReadFileAt(uuid, path)
	if err != nil {
		return "", "", err
	}
	actionData, err := r.ReadFileAt(uuid, filepath.Join(filepath.Dir(path), "ACTION.md"))
	if err != nil {
		return "", "", fmt.Errorf("taolu %q has no ACTION.md at version %q", name, version)
	}
	return string(skillData), string(actionData), nil
}

// versionLabel returns the semantic label (vN) of a taolu version, or "".
func versionLabel(r *libfossil.Repo, path, uuid string) string {
	hist, err := SkillHistory(r, path)
	if err != nil {
		return ""
	}
	for _, v := range hist {
		if v.UUID == uuid {
			return v.Label
		}
	}
	return ""
}

// SkillHistory returns the versions of a taolu, oldest first, labeled v1..vN.
// A version is recorded when either SKILL.md or its sibling ACTION.md changes:
// the taolu is one unit.
func SkillHistory(r *libfossil.Repo, path string) ([]PracticeVersion, error) {
	action := filepath.Join(filepath.Dir(path), "ACTION.md")
	paths := []string{path, action}
	entries, err := r.Timeline(libfossil.TimelineOpts{})
	if err != nil {
		return nil, err
	}
	var rev []PracticeVersion
	lastUUIDs := map[string]string{}
	for _, e := range entries {
		if e.Kind != libfossil.EventKindCheckin {
			continue
		}
		files, err := r.ListFiles(e.RID)
		if err != nil {
			return nil, err
		}
		uuids := map[string]string{}
		for _, f := range files {
			uuids[f.Name] = f.UUID
		}
		changed := false
		for _, p := range paths {
			u := uuids[p]
			if u == "" {
				continue
			}
			if lastUUIDs[p] != u {
				changed = true
			}
			lastUUIDs[p] = u
		}
		if !changed {
			continue
		}
		rev = append(rev, PracticeVersion{
			UUID:    e.UUID,
			Date:    e.Time,
			User:    e.User,
			Message: e.Comment,
		})
	}
	out := make([]PracticeVersion, len(rev))
	for i := range rev {
		out[i] = rev[len(rev)-1-i]
		out[i].Label = fmt.Sprintf("v%d", i+1)
	}
	return out, nil
}

// resolveSkillVersion resolves a version string (empty/"tip", a label like
// "v2", or a UUID prefix) to the full check-in UUID of that version.
func resolveSkillVersion(r *libfossil.Repo, path, version string) (string, error) {
	if version == "" || version == "tip" {
		rid, err := r.ResolveVersion("tip")
		if err != nil {
			return "", fmt.Errorf("resolve version %q: %w", version, err)
		}
		return r.UUIDFromRID(rid)
	}
	if hist, err := SkillHistory(r, path); err == nil {
		for _, v := range hist {
			if v.Label == version {
				return v.UUID, nil
			}
		}
	}
	rid, err := r.ResolveVersion(version)
	if err != nil {
		return "", fmt.Errorf("resolve version %q: %w", version, err)
	}
	return r.UUIDFromRID(rid)
}

func resolveParentTip(r *libfossil.Repo) (int64, error) {
	rid, err := r.ResolveVersion("tip")
	if errors.Is(err, libfossil.ErrVersionNotFound) {
		return 0, nil
	}
	return rid, err
}

// ResolveDiffVersions resolves the two ends of a diff for a taolu. An empty
// versionA resolves to the version before versionB.
func ResolveDiffVersions(r *libfossil.Repo, path, versionA, versionB string) (string, string, error) {
	uuidB, err := resolveSkillVersion(r, path, versionB)
	if err != nil {
		return "", "", err
	}
	if versionA != "" {
		uuidA, err := resolveSkillVersion(r, path, versionA)
		if err != nil {
			return "", "", err
		}
		return uuidA, uuidB, nil
	}
	hist, err := SkillHistory(r, path)
	if err != nil {
		return "", "", err
	}
	for i, v := range hist {
		if v.UUID == uuidB {
			if i == 0 {
				return "", "", fmt.Errorf("no previous version of %s to diff against", path)
			}
			return hist[i-1].UUID, uuidB, nil
		}
	}
	return "", "", fmt.Errorf("version %s not found in history of %s", versionB, path)
}
