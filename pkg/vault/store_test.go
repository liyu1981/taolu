package vault

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TAOLU_REPO", filepath.Join(dir, "env.fossil"))
	t.Setenv("HOME", dir)

	if got, _ := VaultPath("/explicit.fossil"); got != "/explicit.fossil" {
		t.Errorf("arg path = %q", got)
	}
	if got, _ := VaultPath(""); got != filepath.Join(dir, "env.fossil") {
		t.Errorf("env path = %q", got)
	}
	t.Setenv("TAOLU_REPO", "")
	if got, _ := VaultPath(""); got != filepath.Join(dir, ".taolu", "vault.fossil") {
		t.Errorf("home path = %q", got)
	}
}

func TestEnsureVaultCreatesAndSeeds(t *testing.T) {
	p := filepath.Join(t.TempDir(), "vault.fossil")
	r, got, err := EnsureVault(p, "tester")
	if err != nil {
		t.Fatalf("EnsureVault: %v", err)
	}
	defer r.Close()
	if got != p {
		t.Errorf("path = %q, want %q", got, p)
	}
	if err := r.Verify(); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	sp := mustFindSkill(t, r, SeedName)
	hist, err := SkillHistory(r, sp)
	if err != nil {
		t.Fatalf("SkillHistory(seed): %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("seed history = %d, want 1", len(hist))
	}
	skill, action, err := ReadTaoluAtVersion(r, SeedName, "")
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	if err := ValidateContent(SeedName, skill); err != nil {
		t.Errorf("seeded skill invalid: %v", err)
	}
	if err := ValidateAction(action); err != nil {
		t.Errorf("seeded action invalid: %v", err)
	}
}

func TestEnsureVaultIdempotent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "vault.fossil")
	r1, _, err := EnsureVault(p, "tester")
	if err != nil {
		t.Fatalf("EnsureVault 1: %v", err)
	}
	r1.Close()
	r2, _, err := EnsureVault(p, "tester")
	if err != nil {
		t.Fatalf("EnsureVault 2: %v", err)
	}
	defer r2.Close()
	// The seed guide is not duplicated by a second open.
	taolus, err := ListTaolu(r2)
	if err != nil {
		t.Fatalf("ListTaolu: %v", err)
	}
	seedCount := 0
	for _, s := range taolus {
		if s.Name == SeedName {
			seedCount++
		}
	}
	if seedCount != 1 {
		t.Fatalf("seed appears %d times, want 1", seedCount)
	}
}

func TestFindSkillPath(t *testing.T) {
	r := newTestVault(t)
	if _, err := FindSkillPath(r, "missing"); err != nil {
		t.Fatalf("FindSkillPath(missing): %v", err)
	}
	saveTestTaolu(t, r, "workflows", "go-lint")
	// New 3-layer format with @local domain
	if sp := mustFindSkill(t, r, "go-lint"); sp != "taolus/@local/workflows/go-lint/SKILL.md" {
		t.Errorf("path = %q", sp)
	}
}

func TestListTaoluAndUniqueGroups(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTestTaolu(t, r, "backend", "go-api")

	taolus, err := ListTaolu(r)
	if err != nil {
		t.Fatalf("ListTaolu: %v", err)
	}
	want := map[string]bool{SeedName: true, "go-lint": true, "go-api": true}
	if len(taolus) != len(want) {
		t.Fatalf("ListTaolu returned %d taolus, want %d", len(taolus), len(want))
	}
	for _, s := range taolus {
		if !want[s.Name] {
			t.Errorf("unexpected taolu %q", s.Name)
		}
		if s.Name == "go-lint" {
			if s.Mode != ModeApply || s.Group != "workflows" || s.Description == "" || s.LatestVersion != "v1" {
				t.Errorf("go-lint info = %+v", s)
			}
		}
	}

	groups := UniqueGroups(taolus)
	if len(groups) != 3 {
		t.Fatalf("groups = %v, want 3", groups)
	}
	seen := map[string]bool{}
	for _, g := range groups {
		if seen[g] {
			t.Errorf("duplicate group %q", g)
		}
		seen[g] = true
	}
	for _, g := range []string{"backend", "workflows", "meta"} {
		if !seen[g] {
			t.Errorf("group %q missing from %v", g, groups)
		}
	}
}

func TestReadTaoluAtVersion(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV2("go-lint"), testAction, "v2")
	hist, err := SkillHistory(r, mustFindSkill(t, r, "go-lint"))
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("history length = %d", len(hist))
	}

	tip, _, err := ReadTaoluAtVersion(r, "go-lint", "")
	if err != nil {
		t.Fatalf("read tip: %v", err)
	}
	if !strings.Contains(tip, "refined thing") {
		t.Errorf("tip should be v2 content")
	}

	v1, _, err := ReadTaoluAtVersion(r, "go-lint", "v1")
	if err != nil {
		t.Fatalf("read v1: %v", err)
	}
	if !strings.Contains(v1, "Do the thing.") || strings.Contains(v1, "refined") {
		t.Errorf("v1 content mismatch:\n%s", v1)
	}

	prefix := hist[0].UUID[:8]
	byPrefix, _, err := ReadTaoluAtVersion(r, "go-lint", prefix)
	if err != nil {
		t.Fatalf("read by uuid prefix: %v", err)
	}
	if byPrefix != v1 {
		t.Errorf("uuid-prefix read != label read")
	}

	if _, _, err := ReadTaoluAtVersion(r, "go-lint", "v99"); err == nil {
		t.Error("unknown version accepted")
	}
	if _, _, err := ReadTaoluAtVersion(r, "missing", ""); err == nil {
		t.Error("unknown taolu accepted")
	}
}

func TestSkillHistoryAcrossTwoRenames(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV2("go-lint"), testAction, "v2")
	if _, _, err := RenameTaolu(r, "go-lint", "lint", "", "", "tester"); err != nil {
		t.Fatalf("rename 1: %v", err)
	}
	saveTaoluContent(t, r, "workflows", "lint", skillContentV3("lint"), testAction, "v3")
	if _, _, err := RenameTaolu(r, "lint", "lint-all", "", "", "tester"); err != nil {
		t.Fatalf("rename 2: %v", err)
	}

	sp := mustFindSkill(t, r, "lint-all")
	hist, err := SkillHistory(r, sp)
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	// v1, v2 (go-lint), rename->lint (v3), save lint (v4), rename->lint-all (v5).
	if len(hist) != 5 {
		t.Fatalf("history length = %d, want 5", len(hist))
	}
	// New 3-layer format with @local domain
	paths := []string{"taolus/@local/workflows/go-lint", "taolus/@local/workflows/go-lint", "taolus/@local/workflows/lint", "taolus/@local/workflows/lint", "taolus/@local/workflows/lint-all"}
	for i, v := range hist {
		if v.Label != "v"+string(rune('1'+i)) {
			t.Fatalf("hist[%d].Label = %q, want v%d", i, v.Label, i+1)
		}
		if !strings.HasPrefix(v.Path, paths[i]) {
			t.Fatalf("hist[%d] path = %q, want prefix %q", i, v.Path, paths[i])
		}
	}

	// Old versions stay readable through the newest name.
	old, _, err := ReadTaoluAtVersion(r, "lint-all", "v1")
	if err != nil {
		t.Fatalf("read v1 through new name: %v", err)
	}
	if !strings.Contains(old, "name: go-lint") {
		t.Errorf("v1 content should carry old name:\n%s", old)
	}
	if _, _, err := ReadTaoluAtVersion(r, "lint-all", "v4"); err != nil {
		t.Fatalf("read v4 through new name: %v", err)
	}
}

func TestResolveDiffVersions(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV2("go-lint"), testAction, "v2")
	sp := mustFindSkill(t, r, "go-lint")

	a, b, err := ResolveDiffVersions(r, sp, "", "v2")
	if err != nil {
		t.Fatalf("resolve previous: %v", err)
	}
	if a == b {
		t.Error("previous and current resolved to same uuid")
	}

	a, b, err = ResolveDiffVersions(r, sp, "v1", "v2")
	if err != nil {
		t.Fatalf("resolve explicit pair: %v", err)
	}
	if a == b {
		t.Error("explicit pair resolved to same uuid")
	}

	if _, _, err := ResolveDiffVersions(r, sp, "", "v1"); err == nil {
		t.Error("diffing before v1 should fail")
	}
}

func TestVersionLabel(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV2("go-lint"), testAction, "v2")
	sp := mustFindSkill(t, r, "go-lint")
	hist, err := SkillHistory(r, sp)
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if got := versionLabel(r, sp, hist[0].UUID); got != "v1" {
		t.Errorf("versionLabel(v1 uuid) = %q", got)
	}
	if got := versionLabel(r, sp, hist[1].UUID); got != "v2" {
		t.Errorf("versionLabel(v2 uuid) = %q", got)
	}
	if got := versionLabel(r, sp, "0000000000000000000000000000000000000000"); got != "" {
		t.Errorf("versionLabel(unknown) = %q, want empty", got)
	}
}

func TestListTaoluSkipsSupportFiles(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	taolus, err := ListTaolu(r)
	if err != nil {
		t.Fatalf("ListTaolu: %v", err)
	}
	for _, s := range taolus {
		if s.Name == "SKILL" || s.Name == "go-lint/notes" {
			t.Errorf("support file listed as taolu: %q", s.Name)
		}
	}
}

func TestAssetChangeIsNewVersion(t *testing.T) {
	r := newTestVault(t)
	saveTaoluAssets(t, r, "frontend", "button", skillContent("button"), testAction, buttonAssets(), "v1")
	// Only an asset changes; SKILL.md and ACTION.md are untouched.
	saveTaoluAssets(t, r, "frontend", "button", skillContent("button"), testAction,
		[]Asset{{Path: "Button.tsx", Content: "export const Button = () => <button/>; // new\n"}}, "v2 tweak")

	hist, err := SkillHistory(r, mustFindSkill(t, r, "button"))
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("history length = %d, want 2 (asset-only change is a version)", len(hist))
	}
	_, _, assets, err := ReadTaoluBundle(r, "button", "v1")
	if err != nil {
		t.Fatalf("read v1: %v", err)
	}
	if len(assets) != 3 {
		t.Fatalf("v1 assets = %d, want 3", len(assets))
	}
	_, _, assets, err = ReadTaoluBundle(r, "button", "v2")
	if err != nil {
		t.Fatalf("read v2: %v", err)
	}
	if len(assets) != 1 || assets[0].Path != "Button.tsx" {
		t.Errorf("v2 assets = %+v, want only Button.tsx", assets)
	}
}

func TestListTaoluIgnoresAssets(t *testing.T) {
	r := newTestVault(t)
	saveTaoluAssets(t, r, "frontend", "button", skillContent("button"), testAction, buttonAssets(), "component")
	taolus, err := ListTaolu(r)
	if err != nil {
		t.Fatalf("ListTaolu: %v", err)
	}
	for _, s := range taolus {
		if s.Name == "Button.tsx" || s.Name == "files" || strings.HasPrefix(s.Name, "button/") {
			t.Errorf("asset listed as taolu: %q", s.Name)
		}
	}
}
