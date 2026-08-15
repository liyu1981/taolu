package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	libfossil "github.com/danmestas/go-libfossil"
)

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

// InstallPractice materializes a skill as a SKILL.md in the target project.
func InstallPractice(r *libfossil.Repo, repoPath, name, version, target, format string, force bool) (string, error) {
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

	pin := fmt.Sprintf("%s %s\n", repoPath, ShortUUID(uuid))
	label := ""
	if hist, err := SkillHistory(r, path); err == nil {
		for _, v := range hist {
			if v.UUID == uuid {
				label = v.Label
				break
			}
		}
	}
	if label != "" {
		pin = fmt.Sprintf("%s %s\n", repoPath, label)
	}
	if err := os.WriteFile(filepath.Join(dir, ".vault-version"), []byte(pin), 0o644); err != nil {
		return "", err
	}
	return filepath.Join(rel, name), nil
}

// ShortUUID returns the first 12 characters of a UUID.
func ShortUUID(uuid string) string {
	if len(uuid) > 12 {
		return uuid[:12]
	}
	return uuid
}
