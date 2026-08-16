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

// PracticeVersion is one version of a taolu. Path is the SKILL.md path the
// version was recorded under, which can differ from the current path when the
// taolu has been renamed.
type PracticeVersion struct {
	Label   string
	UUID    string
	Date    time.Time
	User    string
	Message string
	Path    string
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

// EnsureVault opens the vault at path (or the default), creating and seeding it
// if missing. It is idempotent: opening an existing vault only re-seeds the
// authoring guide if absent and migrates any legacy practices/ tree. The caller
// must Close the returned repo.
func EnsureVault(path, user string) (*libfossil.Repo, string, error) {
	p, err := VaultPath(path)
	if err != nil {
		return nil, "", err
	}
	var r *libfossil.Repo
	if _, statErr := os.Stat(p); statErr == nil {
		r, err = libfossil.Open(p)
	} else {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, "", err
		}
		r, err = libfossil.Create(p, libfossil.CreateOpts{User: user})
	}
	if err != nil {
		return nil, "", err
	}
	if err := EnsureAuthoringGuide(r, user); err != nil {
		r.Close()
		return nil, "", err
	}
	if err := MigrateLegacy(r, user); err != nil {
		r.Close()
		return nil, "", err
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

// IsArchived reports whether the taolu at skillPath is archived (has an
// .archived marker file at tip).
func IsArchived(r *libfossil.Repo, skillPath string) (bool, error) {
	_, err := r.ReadFileAt("tip", filepath.Join(filepath.Dir(skillPath), archivedMarker))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, libfossil.ErrFileNotFound) {
		return false, nil
	}
	return false, err
}

// ListTaolu enumerates all non-archived taolus at the vault tip.
func ListTaolu(r *libfossil.Repo) ([]TaoluInfo, error) {
	return listTaolu(r, false)
}

// ListArchivedTaolu enumerates the taolus archived at the vault tip.
func ListArchivedTaolu(r *libfossil.Repo) ([]TaoluInfo, error) {
	return listTaolu(r, true)
}

func listTaolu(r *libfossil.Repo, wantArchived bool) ([]TaoluInfo, error) {
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
		archived, err := IsArchived(r, f.Name)
		if err != nil {
			return nil, err
		}
		if archived != wantArchived {
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
// a version. The pair is always read at the same check-in, and reads use the
// path the taolu had at that version so history stays readable across renames.
func ReadTaoluAtVersion(r *libfossil.Repo, name, version string) (skill, action string, err error) {
	path, err := FindSkillPath(r, name)
	if err != nil {
		return "", "", err
	}
	if path == "" {
		return "", "", fmt.Errorf("taolu %q not found in vault", name)
	}
	uuid, vpath, err := resolveSkillVersion(r, path, version)
	if err != nil {
		return "", "", err
	}
	skillData, err := r.ReadFileAt(uuid, vpath)
	if err != nil {
		return "", "", err
	}
	actionData, err := r.ReadFileAt(uuid, filepath.Join(filepath.Dir(vpath), "ACTION.md"))
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
// the taolu is one unit. Following the origin marker, history continues across
// renames: a taolu renamed from an older path keeps that path's versions under
// the same v1..vN sequence.
func SkillHistory(r *libfossil.Repo, path string) ([]PracticeVersion, error) {
	var combined []PracticeVersion
	seen := map[string]bool{}
	var walk func(p string) error
	walk = func(p string) error {
		if p == "" || seen[p] {
			return nil
		}
		seen[p] = true
		hist, origin, err := skillHistorySegment(r, p)
		if err != nil {
			return err
		}
		if err := walk(originPathToSkill(origin)); err != nil {
			return err
		}
		combined = append(combined, hist...)
		return nil
	}
	if err := walk(path); err != nil {
		return nil, err
	}
	for i := range combined {
		combined[i].Label = fmt.Sprintf("v%d", i+1)
	}
	return combined, nil
}

// skillHistorySegment computes the versions recorded while SKILL.md lived at
// path (oldest first) and the origin directory the taolu was renamed from, if
// any. Each version carries the path it was recorded under.
func skillHistorySegment(r *libfossil.Repo, path string) ([]PracticeVersion, string, error) {
	action := filepath.Join(filepath.Dir(path), "ACTION.md")
	paths := []string{path, action}
	entries, err := r.Timeline(libfossil.TimelineOpts{})
	if err != nil {
		return nil, "", err
	}
	var rev []PracticeVersion
	lastUUIDs := map[string]string{}
	newestUUID := ""
	for _, e := range entries {
		if e.Kind != libfossil.EventKindCheckin {
			continue
		}
		files, err := r.ListFiles(e.RID)
		if err != nil {
			return nil, "", err
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
			if newestUUID == "" {
				newestUUID = e.UUID
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
			Path:    path,
		})
	}
	out := make([]PracticeVersion, len(rev))
	for i := range rev {
		out[i] = rev[len(rev)-1-i]
	}
	origin := ""
	if newestUUID != "" {
		if data, err := r.ReadFileAt(newestUUID, filepath.Join(filepath.Dir(path), originMarker)); err == nil {
			origin = strings.TrimSpace(string(data))
		}
	}
	return out, origin, nil
}

// originPathToSkill converts an origin directory under taolus/ (e.g.
// "workflows/go-lint") to the SKILL.md path of its taolu, or "" if empty.
func originPathToSkill(origin string) string {
	if origin == "" {
		return ""
	}
	return filepath.Join(taoluRoot, origin, "SKILL.md")
}

// resolveSkillVersion resolves a version string (empty/"tip", a label like
// "v2", or a UUID prefix) to the full check-in UUID and the SKILL.md path the
// taolu used at that version.
func resolveSkillVersion(r *libfossil.Repo, path, version string) (uuid, vpath string, err error) {
	if version == "" || version == "tip" {
		rid, err := r.ResolveVersion("tip")
		if err != nil {
			return "", "", fmt.Errorf("resolve version %q: %w", version, err)
		}
		u, err := r.UUIDFromRID(rid)
		if err != nil {
			return "", "", err
		}
		return u, path, nil
	}
	if hist, err := SkillHistory(r, path); err == nil {
		for _, v := range hist {
			if v.Label == version {
				return v.UUID, v.Path, nil
			}
			if strings.HasPrefix(v.UUID, version) {
				return v.UUID, v.Path, nil
			}
		}
	}
	rid, err := r.ResolveVersion(version)
	if err != nil {
		return "", "", fmt.Errorf("resolve version %q: %w", version, err)
	}
	u, err := r.UUIDFromRID(rid)
	if err != nil {
		return "", "", err
	}
	return u, path, nil
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
	uuidB, _, err := resolveSkillVersion(r, path, versionB)
	if err != nil {
		return "", "", err
	}
	if versionA != "" {
		uuidA, _, err := resolveSkillVersion(r, path, versionA)
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
