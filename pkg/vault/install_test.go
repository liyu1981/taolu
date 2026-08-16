package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyMode(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	target := t.TempDir()

	res, err := ApplyTaolu(r, r.Path(), "go-lint", "", target, "", "", false)
	if err != nil {
		t.Fatalf("ApplyTaolu: %v", err)
	}
	if res.Mode != ModeApply {
		t.Errorf("mode = %q", res.Mode)
	}
	if res.Pinned {
		t.Error("apply mode should not pin")
	}
	if res.Rel != "" {
		t.Errorf("rel = %q, want empty", res.Rel)
	}
	if !strings.Contains(res.Skill, "name: go-lint") || !strings.Contains(res.Action, "mode: apply") {
		t.Errorf("content missing:\n%s\n%s", res.Skill, res.Action)
	}
	if _, err := os.Stat(filepath.Join(target, ".opencode")); !os.IsNotExist(err) {
		t.Errorf("apply mode must not write to target: %v", err)
	}
}

func TestApplyModeOverride(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	res, err := ApplyTaolu(r, r.Path(), "go-lint", "", t.TempDir(), "", ModeEnforce, true)
	if err != nil {
		t.Fatalf("ApplyTaolu override: %v", err)
	}
	if res.Mode != ModeEnforce {
		t.Errorf("mode = %q, want enforce", res.Mode)
	}
	if _, err := ApplyTaolu(r, r.Path(), "go-lint", "", t.TempDir(), "", "bogus", false); err == nil {
		t.Error("invalid mode override accepted")
	}
}

func TestInstallMode(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV2("go-lint"), testAction, "v2")
	target := t.TempDir()

	res, err := ApplyTaolu(r, r.Path(), "go-lint", "v1", target, "", "", false)
	if err != nil {
		t.Fatalf("ApplyTaolu: %v", err)
	}
	if res.Mode != ModeApply {
		t.Fatalf("stored action is apply; mode = %q", res.Mode)
	}

	res, err = ApplyTaolu(r, r.Path(), "go-lint", "v1", target, "", ModeInstall, false)
	if err != nil {
		t.Fatalf("ApplyTaolu install: %v", err)
	}
	if !res.Pinned || res.Rel != filepath.Join(".opencode", "skills", "go-lint") {
		t.Fatalf("pinned=%v rel=%q", res.Pinned, res.Rel)
	}

	skillPath := filepath.Join(target, ".opencode", "skills", "go-lint", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read installed SKILL.md: %v", err)
	}
	if !strings.Contains(string(data), "Do the thing.") {
		t.Errorf("installed wrong version:\n%s", data)
	}
	if strings.Contains(string(data), "refined") {
		t.Errorf("installed v2 instead of v1:\n%s", data)
	}

	pinPath := filepath.Join(target, ".opencode", "skills", "go-lint", ".taolu-version")
	pin, err := os.ReadFile(pinPath)
	if err != nil {
		t.Fatalf("read pin: %v", err)
	}
	wantPin := r.Path() + " v1\n"
	if string(pin) != wantPin {
		t.Errorf("pin = %q, want %q", string(pin), wantPin)
	}
}

func TestInstallFormats(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	formats := map[string]string{
		"opencode": filepath.Join(".opencode", "skills"),
		"claude":   filepath.Join(".claude", "skills"),
		"agents":   filepath.Join(".agents", "skills"),
	}
	for format, rel := range formats {
		target := t.TempDir()
		res, err := ApplyTaolu(r, r.Path(), "go-lint", "", target, format, ModeInstall, false)
		if err != nil {
			t.Fatalf("install %s: %v", format, err)
		}
		if res.Rel != filepath.Join(rel, "go-lint") {
			t.Errorf("rel = %q, want %q", res.Rel, filepath.Join(rel, "go-lint"))
		}
		if _, err := os.Stat(filepath.Join(target, rel, "go-lint", "SKILL.md")); err != nil {
			t.Errorf("SKILL.md missing for %s: %v", format, err)
		}
	}

	if _, err := ApplyTaolu(r, r.Path(), "go-lint", "", t.TempDir(), "vim", ModeInstall, false); err == nil {
		t.Error("unknown format accepted")
	}
}

func TestInstallRefusesOverwriteWithoutForce(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	target := t.TempDir()
	if _, err := ApplyTaolu(r, r.Path(), "go-lint", "", target, "", ModeInstall, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := ApplyTaolu(r, r.Path(), "go-lint", "", target, "", ModeInstall, false); err == nil {
		t.Error("overwrite without force succeeded")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := ApplyTaolu(r, r.Path(), "go-lint", "", target, "", ModeInstall, true); err != nil {
		t.Errorf("forced overwrite failed: %v", err)
	}
}

func TestEnforceWritesAGENTS(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV2("go-lint"), testAction, "v2")
	target := t.TempDir()

	if _, err := ApplyTaolu(r, r.Path(), "go-lint", "v1", target, "", ModeEnforce, false); err != nil {
		t.Fatalf("enforce v1: %v", err)
	}
	agents := filepath.Join(target, "AGENTS.md")
	data, err := os.ReadFile(agents)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "<!-- taolu-enforce:go-lint -->") {
		t.Errorf("marker missing:\n%s", content)
	}
	if !strings.Contains(content, "go-lint (v1) in .opencode/skills/go-lint/SKILL.md") {
		t.Errorf("reference line missing:\n%s", content)
	}
	if strings.Count(content, "taolu-enforce:go-lint") != 1 {
		t.Errorf("marker should appear once:\n%s", content)
	}

	// Re-applying a newer version updates the reference, not duplicates it.
	if _, err := ApplyTaolu(r, r.Path(), "go-lint", "v2", target, "", ModeEnforce, true); err != nil {
		t.Fatalf("enforce v2: %v", err)
	}
	data, err = os.ReadFile(agents)
	if err != nil {
		t.Fatalf("re-read AGENTS.md: %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "go-lint (v2) in .opencode/skills/go-lint/SKILL.md") {
		t.Errorf("reference not updated to v2:\n%s", content)
	}
	if strings.Contains(content, "(v1) in") {
		t.Errorf("stale v1 reference remains:\n%s", content)
	}
	if strings.Count(content, "taolu-enforce:go-lint") != 1 {
		t.Errorf("marker duplicated after update:\n%s", content)
	}
}

func TestEnforcePreservesExistingAGENTS(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	target := t.TempDir()
	agents := filepath.Join(target, "AGENTS.md")
	os.WriteFile(agents, []byte("# Project\n\nSome notes.\n"), 0o644)

	if _, err := ApplyTaolu(r, r.Path(), "go-lint", "", target, "", ModeEnforce, false); err != nil {
		t.Fatalf("enforce: %v", err)
	}
	data, _ := os.ReadFile(agents)
	content := string(data)
	for _, want := range []string{"# Project", "Some notes.", "<!-- taolu-enforce:go-lint -->"} {
		if !strings.Contains(content, want) {
			t.Errorf("existing content lost: %q\n%s", want, content)
		}
	}
}

func TestApplyTaoluNotFound(t *testing.T) {
	r := newTestVault(t)
	if _, err := ApplyTaolu(r, r.Path(), "missing", "", t.TempDir(), "", "", false); err == nil {
		t.Error("applying unknown taolu succeeded")
	}
}

func TestSafeJoin(t *testing.T) {
	base := t.TempDir()
	joined, err := safeJoin(base, "skills/go")
	if err != nil {
		t.Fatalf("safeJoin: %v", err)
	}
	if joined != filepath.Join(base, "skills", "go") {
		t.Errorf("joined = %q", joined)
	}

	for _, rel := range []string{"..", "../../etc", "a/../../../etc"} {
		if _, err := safeJoin(base, rel); err == nil {
			t.Errorf("safeJoin(%q) escaped base", rel)
		}
	}

	if _, err := safeJoin(base, ".."); err == nil {
		t.Error("relative .. not rejected")
	}
}

func TestShortUUIDRoundTrip(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	hist, err := SkillHistory(r, mustFindSkill(t, r, "go-lint"))
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	short := ShortUUID(hist[0].UUID)
	if len(short) != 12 {
		t.Errorf("short = %q (len %d)", short, len(short))
	}
	if !strings.HasPrefix(hist[0].UUID, short) {
		t.Errorf("short %q not a prefix of %q", short, hist[0].UUID)
	}
}

func TestInstallMaterializesAssets(t *testing.T) {
	r := newTestVault(t)
	saveTaoluAssets(t, r, "frontend", "button", skillContent("button"), testAction, buttonAssets(), "component")
	target := t.TempDir()

	res, err := ApplyTaolu(r, r.Path(), "button", "", target, "", ModeInstall, false)
	if err != nil {
		t.Fatalf("ApplyTaolu: %v", err)
	}
	if len(res.Assets) != 3 {
		t.Fatalf("result assets = %d, want 3", len(res.Assets))
	}
	root := filepath.Join(target, ".opencode", "skills", "button")
	for _, a := range buttonAssets() {
		data, err := os.ReadFile(filepath.Join(root, a.Path))
		if err != nil {
			t.Fatalf("asset %s not materialized: %v", a.Path, err)
		}
		if string(data) != a.Content {
			t.Errorf("asset %s content = %q, want %q", a.Path, string(data), a.Content)
		}
	}
}

func TestInstallRefusesOverwriteOfAsset(t *testing.T) {
	r := newTestVault(t)
	saveTaoluAssets(t, r, "frontend", "button", skillContent("button"), testAction, buttonAssets(), "component")
	target := t.TempDir()

	if _, err := ApplyTaolu(r, r.Path(), "button", "", target, "", ModeInstall, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// A pre-existing asset must block reinstall without force, like SKILL.md.
	os.WriteFile(filepath.Join(target, ".opencode", "skills", "button", "Button.css"), []byte("edited"), 0o644)
	if _, err := ApplyTaolu(r, r.Path(), "button", "", target, "", ModeInstall, false); err == nil {
		t.Error("overwrite of asset without force succeeded")
	}
	if _, err := ApplyTaolu(r, r.Path(), "button", "", target, "", ModeInstall, true); err != nil {
		t.Errorf("forced overwrite failed: %v", err)
	}
}
