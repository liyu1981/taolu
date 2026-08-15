package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	libfossil "github.com/danmestas/go-libfossil"
	"gopkg.in/yaml.v3"
)

const (
	practiceRoot = "practices"
	seedName     = "practice-authoring"
	seedGroup    = "meta"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validSlug(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	return slugRe.MatchString(s)
}

// practiceMeta is the YAML frontmatter of a practice file.
type practiceMeta struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
}

// splitFrontmatter splits content into frontmatter metadata and body.
func splitFrontmatter(content string) (practiceMeta, string, error) {
	var meta practiceMeta
	if !strings.HasPrefix(content, "---\n") {
		return meta, "", errors.New("content must start with a --- YAML frontmatter block")
	}
	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return meta, "", errors.New("content is missing the closing --- frontmatter delimiter")
	}
	if err := yaml.Unmarshal([]byte(rest[:end]), &meta); err != nil {
		return meta, "", fmt.Errorf("invalid frontmatter: %w", err)
	}
	body := rest[end+4:]
	body = strings.TrimPrefix(body, "\n")
	return meta, body, nil
}

// validatePracticeContent checks slug rules and required frontmatter.
func validatePracticeContent(name, content string) error {
	if !validSlug(name) {
		return fmt.Errorf("invalid skill name %q: must be 1-64 lowercase alphanumeric with single hyphen separators", name)
	}
	meta, _, err := splitFrontmatter(content)
	if err != nil {
		return err
	}
	if meta.Name != name {
		return fmt.Errorf("frontmatter name %q does not match requested name %q", meta.Name, name)
	}
	if len(meta.Description) == 0 || len(meta.Description) > 1024 {
		return errors.New("frontmatter description is required (1-1024 characters)")
	}
	return nil
}

func practicePath(group, name string) string {
	return filepath.Join(practiceRoot, group, name, "SKILL.md")
}

// parseSkillPath parses practices/<group>/<name>/SKILL.md. Returns ok=false for
// any other file (including support files inside a skill directory).
func parseSkillPath(p string) (group, name string, ok bool) {
	if filepath.Base(p) != "SKILL.md" {
		return "", "", false
	}
	name = filepath.Base(filepath.Dir(p))
	group = filepath.Base(filepath.Dir(filepath.Dir(p)))
	if group == "" || name == "" || group == "." || name == "." {
		return "", "", false
	}
	return group, name, true
}

// findSkillPath returns the vault path of the skill's SKILL.md, or "" if not present.
func findSkillPath(r *libfossil.Repo, name string) (string, error) {
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

func skillGroup(path string) string {
	group, _, ok := parseSkillPath(path)
	if !ok {
		return "general"
	}
	return group
}

// practiceVersion is one version of a skill.
type practiceVersion struct {
	Label    string
	UUID     string
	Date     time.Time
	User     string
	Message  string
}

// skillHistory returns the versions of a skill, oldest first, labeled v1..vN.
func skillHistory(r *libfossil.Repo, path string) ([]practiceVersion, error) {
	entries, err := r.Timeline(libfossil.TimelineOpts{})
	if err != nil {
		return nil, err
	}
	var rev []practiceVersion
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
		rev = append(rev, practiceVersion{
			UUID:    e.UUID,
			Date:    e.Time,
			User:    e.User,
			Message: e.Comment,
		})
		lastFileUUID = fu
	}
	out := make([]practiceVersion, len(rev))
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
	if hist, err := skillHistory(r, path); err == nil {
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

// savePractice commits content as a new version of the skill.
func savePractice(r *libfossil.Repo, group, name, content, message, user, versionLabel string) (label, uuid string, total int, err error) {
	if user == "" {
		user = "admin"
	}
	targetPath := practicePath(group, name)
	existing, err := findSkillPath(r, name)
	if err != nil {
		return "", "", 0, err
	}
	if existing != "" && existing != targetPath {
		return "", "", 0, fmt.Errorf("skill %q already exists under practice %q (path %s); refusing to save under %q",
			name, skillGroup(existing), existing, group)
	}

	hist, err := skillHistory(r, targetPath)
	if err != nil {
		return "", "", 0, err
	}
	if versionLabel == "" {
		versionLabel = fmt.Sprintf("v%d", len(hist)+1)
	}

	parent, err := resolveParentTip(r)
	if err != nil {
		return "", "", 0, err
	}
	if message == "" {
		message = fmt.Sprintf("save %s (%s)", name, versionLabel)
	}

	rid, commitUUID, err := r.Commit(libfossil.CommitOpts{
		Files: []libfossil.FileToCommit{
			{Name: targetPath, Content: []byte(content)},
		},
		Comment:  message,
		User:     user,
		ParentID: parent,
	})
	if err != nil {
		return "", "", 0, err
	}
	if _, err := r.Tag(libfossil.TagOpts{
		Name:     name + "-" + versionLabel,
		TargetID: rid,
		User:     user,
	}); err != nil {
		return "", "", 0, fmt.Errorf("tag version: %w", err)
	}
	return versionLabel, commitUUID, len(hist) + 1, nil
}

// installTargets maps skill format to the relative directory under the target root.
var installTargets = map[string]string{
	"opencode": filepath.Join(".opencode", "skills"),
	"claude":   filepath.Join(".claude", "skills"),
	"agents":   filepath.Join(".agents", "skills"),
}

func safeJoin(base, rel string) (string, error) {
	if base == "" {
		base = "."
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(abs, filepath.Clean(rel))
	relCheck, err := filepath.Rel(abs, joined)
	if err != nil {
		return "", err
	}
	if relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("target path escapes base directory %s", base)
	}
	return joined, nil
}

// installPractice materializes a skill as a SKILL.md in the target project.
func installPractice(r *libfossil.Repo, vaultPath, name, version, target, format string, force bool) (string, error) {
	path, err := findSkillPath(r, name)
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
	content, err := r.ReadFileAt(uuid, path)
	if err != nil {
		return "", err
	}
	rel := installTargets[format]
	if rel == "" {
		return "", fmt.Errorf("unknown format %q (expected opencode, claude, or agents)", format)
	}
	dir, err := safeJoin(target, filepath.Join(rel, name))
	if err != nil {
		return "", err
	}
	skillFile := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillFile); err == nil && !force {
		return "", fmt.Errorf("%s already exists (pass force=true to overwrite)", skillFile)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(skillFile, content, 0o644); err != nil {
		return "", err
	}

	pin := fmt.Sprintf("%s %s\n", vaultPath, shortUUID(uuid))
	label := ""
	if hist, err := skillHistory(r, path); err == nil {
		for _, v := range hist {
			if v.UUID == uuid {
				label = v.Label
				break
			}
		}
	}
	if label != "" {
		pin = fmt.Sprintf("%s %s\n", vaultPath, label)
	}
	if err := os.WriteFile(filepath.Join(dir, ".vault-version"), []byte(pin), 0o644); err != nil {
		return "", err
	}
	return filepath.Join(rel, name), nil
}
