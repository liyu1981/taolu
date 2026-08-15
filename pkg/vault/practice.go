// Package vault implements the versioned practice library backed by a Fossil
// repository: skill storage, versioning, install, and seeding.
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
	practiceRoot = "practices"
	// SeedName is the built-in practice-authoring skill.
	SeedName  = "practice-authoring"
	seedGroup = "meta"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidSlug reports whether s is a valid skill slug: 1-64 lowercase
// alphanumeric characters with single hyphen separators.
func ValidSlug(s string) bool {
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

// ValidateContent checks slug rules and required frontmatter for a skill save.
func ValidateContent(name, content string) error {
	if !ValidSlug(name) {
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

func skillGroup(path string) string {
	group, _, ok := parseSkillPath(path)
	if !ok {
		return "general"
	}
	return group
}
