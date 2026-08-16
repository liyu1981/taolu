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

// ApplyResult describes the outcome of applying a taolu.
type ApplyResult struct {
	Mode   string
	Skill  string
	Action string
	Assets []Asset
	Rel    string
	Label  string
	Pinned bool
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

// ApplyTaolu executes a taolu at a version. It reads the taolu's ACTION.md to
// determine the mode and dispatches:
//
//   - apply:   returns the content; nothing is written.
//   - install: writes SKILL.md + a .taolu-version pin into the format target.
//   - enforce: installs, then appends/replaces a compliance reference line in
//     AGENTS.md so every agent loads it.
//
// modeOverride wins over the stored action when non-empty.
func ApplyTaolu(r *libfossil.Repo, repoPath, name, version, target, format, modeOverride string, force bool) (*ApplyResult, error) {
	sp, err := FindSkillPath(r, name)
	if err != nil {
		return nil, err
	}
	if sp == "" {
		return nil, fmt.Errorf("taolu %q not found in vault", name)
	}
	uuid, vpath, err := resolveSkillVersion(r, sp, version)
	if err != nil {
		return nil, err
	}
	label := versionLabel(r, sp, uuid)
	skillData, err := r.ReadFileAt(uuid, vpath)
	if err != nil {
		return nil, err
	}
	actionData, err := r.ReadFileAt(uuid, filepath.Join(filepath.Dir(vpath), "ACTION.md"))
	if err != nil {
		return nil, fmt.Errorf("taolu %q has no ACTION.md", name)
	}
	assets, err := readAssetsAt(r, uuid, filepath.Dir(vpath))
	if err != nil {
		return nil, err
	}
	am, _, err := splitActionFrontmatter(string(actionData))
	if err != nil {
		return nil, err
	}

	mode := am.Mode
	if mode == "" {
		mode = ModeInstall
	}
	if modeOverride != "" {
		if !ValidActionMode(modeOverride) {
			return nil, fmt.Errorf("invalid action %q: must be apply, install, or enforce", modeOverride)
		}
		mode = modeOverride
	}

	res := &ApplyResult{Mode: mode, Skill: string(skillData), Action: string(actionData), Assets: assets, Label: label}
	if mode == ModeApply {
		return res, nil
	}

	if format == "" {
		format = am.Detail["format"]
	}
	if format == "" {
		format = "opencode"
	}
	rel, err := installSkill(r, repoPath, name, uuid, skillData, label, assets, target, format, force)
	if err != nil {
		return nil, err
	}
	res.Rel = rel
	res.Pinned = true
	if mode == ModeEnforce {
		if err := enforceAGENTS(target, name, label, filepath.Join(rel, "SKILL.md")); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// installSkill materializes a taolu's SKILL.md, files/ assets, and a
// .taolu-version pin into the format target, returning the relative installed
// directory. Assets are written preserving their relative paths under files/.
func installSkill(r *libfossil.Repo, repoPath, name, uuid string, content []byte, label string, assets []Asset, target, format string, force bool) (string, error) {
	rel := installTargets[format]
	if rel == "" {
		return "", fmt.Errorf("unknown format %q (expected opencode, claude, or agents)", format)
	}
	dir, err := safeJoin(target, filepath.Join(rel, name))
	if err != nil {
		return "", err
	}
	if !force {
		for _, a := range assets {
			dest := filepath.Join(dir, a.Path)
			if _, err := os.Stat(dest); err == nil {
				return "", fmt.Errorf("%s already exists (pass force=true to overwrite)", dest)
			}
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	skillFile := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillFile); err == nil && !force {
		return "", fmt.Errorf("%s already exists (pass force=true to overwrite)", skillFile)
	}
	if err := os.WriteFile(skillFile, content, 0o644); err != nil {
		return "", err
	}
	for _, a := range assets {
		dest, err := safeJoin(dir, a.Path)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dest, []byte(a.Content), 0o644); err != nil {
			return "", err
		}
	}

	pinVersion := label
	if pinVersion == "" {
		pinVersion = ShortUUID(uuid)
	}
	pin := fmt.Sprintf("%s %s\n", repoPath, pinVersion)
	if err := os.WriteFile(filepath.Join(dir, ".taolu-version"), []byte(pin), 0o644); err != nil {
		return "", err
	}
	return filepath.Join(rel, name), nil
}

// enforceAGENTS appends (or updates) an idempotent compliance reference for a
// taolu in the target project's AGENTS.md. Only the marker line and the
// reference line that follows it are ever touched; nothing else in the file.
func enforceAGENTS(target, name, label, relSkill string) error {
	if label == "" {
		label = "tip"
	}
	marker := fmt.Sprintf("<!-- taolu-enforce:%s -->", name)
	refLine := fmt.Sprintf("- Follow the taolu %s (%s) in %s", name, label, relSkill)
	p, err := safeJoin(target, "AGENTS.md")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var lines []string
	if len(data) > 0 {
		lines = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
	}
	idx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == marker {
			idx = i
			break
		}
	}
	block := []string{marker, refLine}
	if idx >= 0 {
		newLines := append([]string{}, lines[:idx]...)
		newLines = append(newLines, block...)
		if idx+2 <= len(lines) {
			newLines = append(newLines, lines[idx+2:]...)
		}
		lines = newLines
	} else {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, block...)
	}
	return os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// ShortUUID returns the first 12 characters of a UUID.
func ShortUUID(uuid string) string {
	if len(uuid) > 12 {
		return uuid[:12]
	}
	return uuid
}
