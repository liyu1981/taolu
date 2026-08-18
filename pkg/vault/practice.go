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

	// taoluFilesDir is the reserved subdirectory holding a taolu's support
	// files (snippets, templates, complete components). Every asset lives
	// under it; nothing else may sit in the taolu directory.
	taoluFilesDir = "files"

	// maxAssetBytes caps a single support file, and maxAssetTotalBytes caps the
	// whole files/ bundle of one version, so taolu_get/export/save stay usable
	// over MCP text frames.
	maxAssetBytes      = 1 << 20
	maxAssetTotalBytes = 8 << 20

	// archivedMarker is a marker file committed in a taolu directory that
	// flags the taolu as archived. Archived taolus are hidden from normal
	// listings and refused by consuming tools until restored.
	archivedMarker = ".archived"
	// originMarker records the previous directory path (under taolus/) of a
	// renamed taolu, so version history continues across renames.
	originMarker = "origin"
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

// DomainPrefix is the well-known domain for the local Fossil vault.
const DomainPrefix = "@local"

// TaoluRef is a fully qualified reference to a taolu: @domain/group/name.
type TaoluRef struct {
	Domain string // "@local", "@liyu1981", etc.
	Group  string // "frontend", "backend", etc.
	Name   string // "local-first-webapp", etc.
}

// String returns the full taolu reference as "@domain/group/name".
func (r TaoluRef) String() string {
	return r.Domain + "/" + r.Group + "/" + r.Name
}

// Path returns the vault path for this taolu reference.
func (r TaoluRef) Path() string {
	return filepath.Join(taoluRoot, r.Domain, r.Group, r.Name, "SKILL.md")
}

// ActionPath returns the ACTION.md vault path for this taolu reference.
func (r TaoluRef) ActionPath() string {
	return filepath.Join(taoluRoot, r.Domain, r.Group, r.Name, "ACTION.md")
}

// Dir returns the taolu directory path (without SKILL.md).
func (r TaoluRef) Dir() string {
	return filepath.Join(taoluRoot, r.Domain, r.Group, r.Name)
}

// AssetPath returns the vault path of an asset relative to the files/ directory.
func (r TaoluRef) AssetPath(rel string) string {
	return filepath.Join(taoluRoot, r.Domain, r.Group, r.Name, taoluFilesDir, filepath.Clean(rel))
}

// ParseTaoluRef parses a full taolu reference "@domain/group/name" or "group/name".
// For "group/name" format, domain defaults to empty (resolved later).
func ParseTaoluRef(ref string) (TaoluRef, error) {
	parts := strings.Split(ref, "/")
	switch len(parts) {
	case 3:
		domain, group, name := parts[0], parts[1], parts[2]
		if !strings.HasPrefix(domain, "@") {
			return TaoluRef{}, fmt.Errorf("domain must start with @: %q", domain)
		}
		if !ValidSlug(group) {
			return TaoluRef{}, fmt.Errorf("invalid group %q: must be 1-64 lowercase alphanumeric with single hyphens", group)
		}
		if !ValidSlug(name) {
			return TaoluRef{}, fmt.Errorf("invalid name %q: must be 1-64 lowercase alphanumeric with single hyphens", name)
		}
		return TaoluRef{Domain: domain, Group: group, Name: name}, nil
	case 2:
		group, name := parts[0], parts[1]
		if !ValidSlug(group) {
			return TaoluRef{}, fmt.Errorf("invalid group %q: must be 1-64 lowercase alphanumeric with single hyphens", group)
		}
		if !ValidSlug(name) {
			return TaoluRef{}, fmt.Errorf("invalid name %q: must be 1-64 lowercase alphanumeric with single hyphens", name)
		}
		return TaoluRef{Group: group, Name: name}, nil
	default:
		return TaoluRef{}, fmt.Errorf("invalid taolu reference %q: expected @domain/group/name or group/name", ref)
	}
}

// ParseTaoluPath parses a vault path "taolus/@domain/group/name/SKILL.md" into a TaoluRef.
func ParseTaoluPath(p string) (TaoluRef, bool) {
	if filepath.Base(p) != "SKILL.md" {
		return TaoluRef{}, false
	}
	if !strings.HasPrefix(p, taoluRoot+string(filepath.Separator)) {
		return TaoluRef{}, false
	}
	parts := strings.Split(p, string(filepath.Separator))
	// 3-layer format: taolus/@domain/group/name/SKILL.md -> 5 parts
	if len(parts) == 5 && strings.HasPrefix(parts[1], "@") {
		domain := parts[1]
		group := parts[2]
		name := parts[3]
		if domain == "" || group == "" || name == "" {
			return TaoluRef{}, false
		}
		return TaoluRef{Domain: domain, Group: group, Name: name}, true
	}
	// Legacy 2-layer format: taolus/group/name/SKILL.md -> 4 parts
	if len(parts) == 4 && !strings.HasPrefix(parts[1], "@") {
		group := parts[1]
		name := parts[2]
		if group == "" || name == "" {
			return TaoluRef{}, false
		}
		return TaoluRef{Domain: DomainPrefix, Group: group, Name: name}, true
	}
	return TaoluRef{}, false
}

// ResolveTaoluRef resolves a taolu reference, filling in the domain if omitted.
// If ref.Domain is empty, it uses userDomain if set, otherwise DomainPrefix.
func ResolveTaoluRef(ref TaoluRef, userDomain string) TaoluRef {
	if ref.Domain != "" {
		return ref
	}
	domain := DomainPrefix
	if userDomain != "" {
		domain = userDomain
	}
	ref.Domain = domain
	return ref
}

// ValidDomain reports whether d is a valid domain: "@" followed by 1-63 lowercase
// alphanumeric characters with single hyphens.
func ValidDomain(d string) bool {
	if !strings.HasPrefix(d, "@") {
		return false
	}
	return ValidSlug(strings.TrimPrefix(d, "@"))
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

// Asset is one support file in a taolu's files/ bundle. Path is relative to
// the files/ directory (e.g. "Button.tsx", "components/Button.tsx").
type Asset struct {
	Path    string
	Content string
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

// skillPath returns the SKILL.md path of a taolu (legacy 2-layer format).
func skillPath(group, name string) string {
	return filepath.Join(taoluRoot, group, name, "SKILL.md")
}

// skillPathWithDomain returns the SKILL.md path of a taolu with domain (3-layer format).
func skillPathWithDomain(domain, group, name string) string {
	return filepath.Join(taoluRoot, domain, group, name, "SKILL.md")
}

// skillPath returns the SKILL.md path of a taolu with domain (3-layer format).
// If domain is empty, it uses DomainPrefix.
func skillPathDomain(domain, group, name string) string {
	if domain == "" {
		domain = DomainPrefix
	}
	return filepath.Join(taoluRoot, domain, group, name, "SKILL.md")
}

// actionPath returns the ACTION.md path of a taolu (legacy 2-layer format).
func actionPath(group, name string) string {
	return filepath.Join(taoluRoot, group, name, "ACTION.md")
}

// actionPathWithDomain returns the ACTION.md path of a taolu with domain (3-layer format).
func actionPathWithDomain(domain, group, name string) string {
	return filepath.Join(taoluRoot, domain, group, name, "ACTION.md")
}

// actionPath returns the ACTION.md path of a taolu with domain (3-layer format).
// If domain is empty, it uses DomainPrefix.
func actionPathDomain(domain, group, name string) string {
	if domain == "" {
		domain = DomainPrefix
	}
	return filepath.Join(taoluRoot, domain, group, name, "ACTION.md")
}

// assetPath returns the vault path of an asset relative to the files/
// directory of a taolu.
func assetPath(group, name, rel string) string {
	return filepath.Join(taoluRoot, group, name, taoluFilesDir, filepath.Clean(rel))
}

// reservedAssetNames are paths that cannot be used as assets because they
// collide with the taolu's canonical documents, markers, or the bundle root.
var reservedAssetNames = map[string]bool{
	"SKILL.md": true, "ACTION.md": true, ".archived": true,
	"origin": true, ".taolu-version": true, taoluFilesDir: true,
}

// ValidateAssets checks each asset's path and size. Paths must be relative to
// the files/ bundle root: no absolute paths, no ".", ".." or empty segments,
// no reserved names, and no duplicates.
func ValidateAssets(assets []Asset) error {
	seen := map[string]bool{}
	var total int64
	for _, a := range assets {
		if a.Path == "" {
			return errors.New("asset path is required")
		}
		if filepath.IsAbs(a.Path) {
			return fmt.Errorf("invalid asset path %q: must be relative to the files/ directory", a.Path)
		}
		if strings.ContainsAny(a.Path, "\\") {
			return fmt.Errorf("invalid asset path %q: backslash is not allowed", a.Path)
		}
		clean := filepath.Clean(a.Path)
		if clean != a.Path {
			return fmt.Errorf("invalid asset path %q: must already be clean (no '.' or '..' segments)", a.Path)
		}
		if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			return fmt.Errorf("invalid asset path %q: must not escape the files/ directory", a.Path)
		}
		for _, seg := range strings.Split(clean, string(filepath.Separator)) {
			if seg == "" || seg == "." || seg == ".." {
				return fmt.Errorf("invalid asset path %q: empty or dot segments are not allowed", a.Path)
			}
		}
		if reservedAssetNames[clean] || strings.HasPrefix(clean, taoluFilesDir+string(filepath.Separator)) {
			return fmt.Errorf("invalid asset path %q: reserved name", a.Path)
		}
		if seen[clean] {
			return fmt.Errorf("duplicate asset path %q", a.Path)
		}
		seen[clean] = true
		n := int64(len(a.Content))
		total += n
		if n > maxAssetBytes {
			return fmt.Errorf("asset %q is %d bytes, exceeds per-file limit of %d", a.Path, n, maxAssetBytes)
		}
		if total > maxAssetTotalBytes {
			return fmt.Errorf("asset bundle exceeds total limit of %d bytes", maxAssetTotalBytes)
		}
	}
	return nil
}

// parseSkillPath parses taolus/<group>/<name>/SKILL.md or taolus/@domain/<group>/<name>/SKILL.md.
// Returns ok=false for any other file. For backward compatibility, legacy 2-layer
// paths are treated as @local domain.
func parseSkillPath(p string) (group, name string, ok bool) {
	ref, ok := ParseTaoluPath(p)
	if !ok {
		return "", "", false
	}
	return ref.Group, ref.Name, true
}

// parseSkillPathFull parses taolus/<group>/<name>/SKILL.md or taolus/@domain/<group>/<name>/SKILL.md
// and returns the full TaoluRef. This is the preferred function for new code.
func parseSkillPathFull(p string) (TaoluRef, bool) {
	return ParseTaoluPath(p)
}

// parseAssetPath parses an asset path taolus/<group>/<name>/files/<rel...> or
// taolus/@domain/<group>/<name>/files/<rel...>. Returns ok=false for any other file.
func parseAssetPath(p string) (group, name, rel string, ok bool) {
	if !strings.HasPrefix(p, taoluRoot+string(filepath.Separator)) {
		return "", "", "", false
	}
	parts := strings.Split(p, string(filepath.Separator))
	// 3-layer format: taolus/@domain/group/name/files/rel... -> 6+ parts
	if len(parts) >= 6 && strings.HasPrefix(parts[1], "@") && parts[4] == taoluFilesDir {
		group, name = parts[2], parts[3]
		if group == "" || name == "" || group == "." || name == "." {
			return "", "", "", false
		}
		return group, name, strings.Join(parts[5:], string(filepath.Separator)), true
	}
	// Legacy 2-layer format: taolus/group/name/files/rel... -> 5+ parts
	if len(parts) >= 5 && !strings.HasPrefix(parts[1], "@") && parts[3] == taoluFilesDir {
		group, name = parts[1], parts[2]
		if group == "" || name == "" || group == "." || name == "." {
			return "", "", "", false
		}
		return group, name, strings.Join(parts[4:], string(filepath.Separator)), true
	}
	return "", "", "", false
}

// parseAssetPathFull parses an asset path and returns the full TaoluRef plus relative path.
func parseAssetPathFull(p string) (TaoluRef, string, bool) {
	if !strings.HasPrefix(p, taoluRoot+string(filepath.Separator)) {
		return TaoluRef{}, "", false
	}
	parts := strings.Split(p, string(filepath.Separator))
	// 3-layer format: taolus/@domain/group/name/files/rel... -> 6+ parts
	if len(parts) >= 6 && strings.HasPrefix(parts[1], "@") && parts[4] == taoluFilesDir {
		group, name := parts[2], parts[3]
		if group == "" || name == "" || group == "." || name == "." {
			return TaoluRef{}, "", false
		}
		rel := strings.Join(parts[5:], string(filepath.Separator))
		return TaoluRef{Domain: parts[1], Group: group, Name: name}, rel, true
	}
	// Legacy 2-layer format: taolus/group/name/files/rel... -> 5+ parts
	if len(parts) >= 5 && !strings.HasPrefix(parts[1], "@") && parts[3] == taoluFilesDir {
		group, name := parts[1], parts[2]
		if group == "" || name == "" || group == "." || name == "." {
			return TaoluRef{}, "", false
		}
		rel := strings.Join(parts[4:], string(filepath.Separator))
		return TaoluRef{Domain: DomainPrefix, Group: group, Name: name}, rel, true
	}
	return TaoluRef{}, "", false
}

func skillGroup(path string) string {
	ref, ok := ParseTaoluPath(path)
	if !ok {
		return "general"
	}
	return ref.Group
}

// skillDomain returns the domain of a taolu from its SKILL.md path.
func skillDomain(path string) string {
	ref, ok := ParseTaoluPath(path)
	if !ok {
		return DomainPrefix
	}
	return ref.Domain
}
