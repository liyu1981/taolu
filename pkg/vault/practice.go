// Package vault implements the versioned taolu library backed by a Fossil
// repository: taolu storage, versioning, apply/install, and seeding.
package vault

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	taoluRoot = "taolus"
	// SeedName is the built-in taolu-authoring guide.
	SeedName  = "taolu-authoring"
	seedGroup = "meta"
)

// Taolu action modes.
const (
	ModeApply   = "apply"
	ModeInstall = "install"
	ModeEnforce = "enforce"
)

// defaultActionInstall is the ACTION.md assigned to legacy skills during
// migration: they keep their pre-v1 install behavior.
const defaultActionInstall = `---
mode: install
---
`

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidSlug reports whether s is a valid slug: 1-64 lowercase alphanumeric
// characters with single hyphen separators.
func ValidSlug(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	return slugRe.MatchString(s)
}

// practiceMeta is the YAML frontmatter of a SKILL.md.
type practiceMeta struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
}

// actionMeta is the YAML frontmatter of an ACTION.md.
type actionMeta struct {
	Mode   string            `yaml:"mode"`
	Detail map[string]string `yaml:"detail"`
}

// ValidActionMode reports whether m is a supported taolu action mode.
func ValidActionMode(m string) bool {
	return m == ModeApply || m == ModeInstall || m == ModeEnforce
}

// splitRawFrontmatter splits content into the raw YAML frontmatter and body.
func splitRawFrontmatter(content string) (raw, body string, err error) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", errors.New("content must start with a --- YAML frontmatter block")
	}
	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", errors.New("content is missing the closing --- frontmatter delimiter")
	}
	return rest[:end], strings.TrimPrefix(rest[end+4:], "\n"), nil
}

// splitFrontmatter splits content into SKILL.md frontmatter metadata and body.
func splitFrontmatter(content string) (practiceMeta, string, error) {
	var meta practiceMeta
	raw, body, err := splitRawFrontmatter(content)
	if err != nil {
		return meta, "", err
	}
	if err := yaml.Unmarshal([]byte(raw), &meta); err != nil {
		return meta, "", fmt.Errorf("invalid frontmatter: %w", err)
	}
	return meta, body, nil
}

// splitActionFrontmatter splits ACTION.md content into metadata and body.
func splitActionFrontmatter(content string) (actionMeta, string, error) {
	var meta actionMeta
	raw, body, err := splitRawFrontmatter(content)
	if err != nil {
		return meta, "", err
	}
	if err := yaml.Unmarshal([]byte(raw), &meta); err != nil {
		return meta, "", fmt.Errorf("invalid action frontmatter: %w", err)
	}
	return meta, body, nil
}

// ValidateContent checks slug rules and required SKILL.md frontmatter.
func ValidateContent(name, content string) error {
	if !ValidSlug(name) {
		return fmt.Errorf("invalid taolu name %q: must be 1-64 lowercase alphanumeric with single hyphen separators", name)
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

// ValidateAction checks the ACTION.md content for a valid mode and format.
func ValidateAction(content string) error {
	meta, _, err := splitActionFrontmatter(content)
	if err != nil {
		return err
	}
	if meta.Mode == "" {
		return errors.New("action mode is required (apply, install, or enforce)")
	}
	if !ValidActionMode(meta.Mode) {
		return fmt.Errorf("invalid action mode %q: must be apply, install, or enforce", meta.Mode)
	}
	if f := meta.Detail["format"]; f != "" {
		if installTargets[f] == "" {
			return fmt.Errorf("invalid action format %q: must be opencode, claude, or agents", f)
		}
	}
	return nil
}

// skillPath returns the SKILL.md path of a taolu.
func skillPath(group, name string) string {
	return filepath.Join(taoluRoot, group, name, "SKILL.md")
}

// actionPath returns the ACTION.md path of a taolu.
func actionPath(group, name string) string {
	return filepath.Join(taoluRoot, group, name, "ACTION.md")
}

// parseSkillPath parses taolus/<group>/<name>/SKILL.md. Returns ok=false for
// any other file (including legacy practices/ paths and support files inside a
// taolu directory).
func parseSkillPath(p string) (group, name string, ok bool) {
	if filepath.Base(p) != "SKILL.md" {
		return "", "", false
	}
	if !strings.HasPrefix(p, taoluRoot+string(filepath.Separator)) {
		return "", "", false
	}
	name = filepath.Base(filepath.Dir(p))
	group = filepath.Base(filepath.Dir(filepath.Dir(p)))
	if group == "" || name == "" || group == "." || name == "." {
		return "", "", false
	}
	return group, name, true
}

func skillGroup(path string) string {
	group, _, ok := parseSkillPath(path)
	if !ok {
		return "general"
	}
	return group
}
