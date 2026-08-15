package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	libfossil "github.com/danmestas/go-libfossil"
	_ "github.com/danmestas/go-libfossil/db/driver/modernc"
)

// SkillInfo is a listing entry for a skill.
type SkillInfo struct {
	Name          string
	Group         string
	Description   string
	Tags          string
	LatestVersion string
	LatestUUID    string
}

// PracticeVersion is one version of a skill.
type PracticeVersion struct {
	Label   string
	UUID    string
	Date    time.Time
	User    string
	Message string
}

// OpenVault opens the vault repo at path, or fails with a hint to run vault_init.
func OpenVault(path string) (*libfossil.Repo, string, error) {
	p, err := VaultPath(path)
	if err != nil {
		return nil, "", err
	}
	r, err := libfossil.Open(p)
	if err != nil {
		return nil, "", fmt.Errorf("vault not initialized at %s (run vault_init first): %w", p, err)
	}
	return r, p, nil
}

// VaultPath resolves the requested path, falling back to AGENT_VAULT_REPO and
// then to ~/.agent-vault/vault.fossil.
func VaultPath(arg string) (string, error) {
	if arg != "" {
		return filepath.Clean(arg), nil
	}
	if env := os.Getenv("AGENT_VAULT_REPO"); env != "" {
		return filepath.Clean(env), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-vault", "vault.fossil"), nil
}

// FindSkillPath returns the vault path of the skill's SKILL.md, or "" if not present.
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
		if filepath.Base(f.Name) != "SKILL.md" {
			continue
		}
		if filepath.Base(filepath.Dir(f.Name)) == name {
			return f.Name, nil
		}
	}
	return "", nil
}

// ListSkills enumerates all skills at the vault tip.
func ListSkills(r *libfossil.Repo) ([]SkillInfo, error) {
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
	var out []SkillInfo
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
		hist, err := SkillHistory(r, f.Name)
		if err != nil {
			return nil, err
		}
		info := SkillInfo{
			Name:        name,
			Group:       group,
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

// UniqueGroups returns the distinct practice groups in skills, in first-seen order.
func UniqueGroups(skills []SkillInfo) []string {
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

// ReadSkillAtVersion returns the raw content of a skill at a version.
func ReadSkillAtVersion(r *libfossil.Repo, name, version string) (string, error) {
	path, err := FindSkillPath(r, name)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("skill %q not found in vault", name)
	}
	uuid, err := resolveSkillVersion(r, path, version)
	if err != nil {
		return "", err
	}
	data, err := r.ReadFileAt(uuid, path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SkillHistory returns the versions of a skill, oldest first, labeled v1..vN.
func SkillHistory(r *libfossil.Repo, path string) ([]PracticeVersion, error) {
	entries, err := r.Timeline(libfossil.TimelineOpts{})
	if err != nil {
		return nil, err
	}
	var rev []PracticeVersion
	lastFileUUID := ""
	for _, e := range entries {
		if e.Kind != libfossil.EventKindCheckin {
			continue
		}
		files, err := r.ListFiles(e.RID)
		if err != nil {
			return nil, err
		}
		fu := ""
		for _, f := range files {
			if f.Name == path {
				fu = f.UUID
				break
			}
		}
		if fu == "" || fu == lastFileUUID {
			continue
		}
		rev = append(rev, PracticeVersion{
			UUID:    e.UUID,
			Date:    e.Time,
			User:    e.User,
			Message: e.Comment,
		})
		lastFileUUID = fu
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

// ResolveDiffVersions resolves the two ends of a diff for a skill. An empty
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
