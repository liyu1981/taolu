package vault

import (
	"strings"
	"testing"
)

func TestArchiveAndRestore(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")

	group, err := ArchiveTaolu(r, "go-lint", "", "tester")
	if err != nil {
		t.Fatalf("ArchiveTaolu: %v", err)
	}
	if group != "workflows" {
		t.Fatalf("group = %q, want workflows", group)
	}

	sp, err := FindSkillPath(r, "go-lint")
	if err != nil || sp == "" {
		t.Fatalf("FindSkillPath(go-lint) = %q, %v; source tree must be kept", sp, err)
	}
	if archived, err := IsArchived(r, sp); err != nil || !archived {
		t.Fatalf("IsArchived = %v, %v; want true", archived, err)
	}

	taolus, err := ListTaolu(r)
	if err != nil {
		t.Fatalf("ListTaolu: %v", err)
	}
	for _, s := range taolus {
		if s.Name == "go-lint" {
			t.Fatal("archived taolu still returned by ListTaolu")
		}
	}
	archivedTaolus, err := ListArchivedTaolu(r)
	if err != nil {
		t.Fatalf("ListArchivedTaolu: %v", err)
	}
	if len(archivedTaolus) != 1 || archivedTaolus[0].Name != "go-lint" {
		t.Fatalf("ListArchivedTaolu = %+v, want [go-lint]", archivedTaolus)
	}

	if _, err := ArchiveTaolu(r, "go-lint", "", "tester"); err == nil {
		t.Fatal("double archive succeeded, want error")
	}

	group, err = RestoreTaolu(r, "go-lint", "", "tester")
	if err != nil {
		t.Fatalf("RestoreTaolu: %v", err)
	}
	if group != "workflows" {
		t.Fatalf("restored group = %q", group)
	}
	if archived, err := IsArchived(r, sp); err != nil || archived {
		t.Fatalf("IsArchived after restore = %v, %v; want false", archived, err)
	}
	taolus, err = ListTaolu(r)
	if err != nil {
		t.Fatalf("ListTaolu after restore: %v", err)
	}
	found := false
	for _, s := range taolus {
		if s.Name == "go-lint" {
			found = true
		}
	}
	if !found {
		t.Fatal("restored taolu missing from ListTaolu")
	}
}

func TestArchiveProtectsSeed(t *testing.T) {
	r := newTestVault(t)
	if _, err := ArchiveTaolu(r, SeedName, "", "tester"); err == nil {
		t.Fatalf("ArchiveTaolu(%s) succeeded, want refusal", SeedName)
	}
	if _, err := ArchiveTaolu(r, "missing", "", "tester"); err == nil {
		t.Fatal("ArchiveTaolu(missing) succeeded, want not-found error")
	}
	if _, err := RestoreTaolu(r, "missing", "", "tester"); err == nil {
		t.Fatal("RestoreTaolu(missing) succeeded, want not-found error")
	}
}

func TestRenameContinuesVersioning(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV2("go-lint"), testAction, "v2 tweak")

	oldGroup, newGroup, err := RenameTaolu(r, "go-lint", "lint-checks", "", "", "tester")
	if err != nil {
		t.Fatalf("RenameTaolu: %v", err)
	}
	if oldGroup != "workflows" || newGroup != "workflows" {
		t.Fatalf("groups = %s/%s, want workflows/workflows", oldGroup, newGroup)
	}

	if p, err := FindSkillPath(r, "go-lint"); err != nil || p != "" {
		t.Fatalf("old name still resolves: %q, %v", p, err)
	}
	sp := mustFindSkill(t, r, "lint-checks")

	hist, err := SkillHistory(r, sp)
	if err != nil {
		t.Fatalf("SkillHistory(lint-checks): %v", err)
	}
	if len(hist) != 3 {
		t.Fatalf("history length = %d, want 3 (v1, v2, rename)", len(hist))
	}
	for i, v := range hist {
		if v.Label != "v"+string(rune('1'+i)) {
			t.Fatalf("hist[%d].Label = %q, want v%d", i, v.Label, i+1)
		}
		if i < 2 && !strings.HasPrefix(v.Path, "taolus/workflows/go-lint/") {
			t.Fatalf("hist[%d] path = %q, want old name path", i, v.Path)
		}
		if i == 2 && !strings.HasPrefix(v.Path, "taolus/workflows/lint-checks/") {
			t.Fatalf("hist[%d] path = %q, want new name path", i, v.Path)
		}
	}

	skill, action, err := ReadTaoluAtVersion(r, "lint-checks", "")
	if err != nil {
		t.Fatalf("ReadTaoluAtVersion tip: %v", err)
	}
	if !strings.Contains(skill, "name: lint-checks") || strings.Contains(skill, "name: go-lint") {
		t.Fatalf("frontmatter name not updated:\n%s", skill)
	}
	if !strings.Contains(action, "mode: apply") {
		t.Fatalf("ACTION.md mangled:\n%s", action)
	}

	oldSkill, _, err := ReadTaoluAtVersion(r, "lint-checks", "v1")
	if err != nil {
		t.Fatalf("ReadTaoluAtVersion v1 (pre-rename): %v", err)
	}
	if !strings.Contains(oldSkill, "name: go-lint") {
		t.Fatalf("v1 should be the old-name content:\n%s", oldSkill)
	}
	if _, _, err := ReadTaoluAtVersion(r, "lint-checks", "v2"); err != nil {
		t.Fatalf("ReadTaoluAtVersion v2: %v", err)
	}
}

func TestRenameMovesGroup(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	if _, _, err := RenameTaolu(r, "go-lint", "lint-checks", "backend", "", "tester"); err != nil {
		t.Fatalf("RenameTaolu into new group: %v", err)
	}
	sp := mustFindSkill(t, r, "lint-checks")
	if !strings.HasPrefix(sp, "taolus/backend/") {
		t.Fatalf("path = %q, want taolus/backend/...", sp)
	}
}

func TestRenamePreservesArchiveStatus(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	if _, err := ArchiveTaolu(r, "go-lint", "", "tester"); err != nil {
		t.Fatalf("ArchiveTaolu: %v", err)
	}
	if _, _, err := RenameTaolu(r, "go-lint", "lint-checks", "", "", "tester"); err != nil {
		t.Fatalf("RenameTaolu: %v", err)
	}
	sp := mustFindSkill(t, r, "lint-checks")
	if archived, err := IsArchived(r, sp); err != nil || !archived {
		t.Fatalf("renamed taolu archive status = %v, %v; want true", archived, err)
	}
}

func TestRenameGuards(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTestTaolu(t, r, "backend", "go-test")

	cases := []struct{ name, newName, newGroup string }{
		{SeedName, "renamed-seed", ""},
		{"go-lint", "Go-Lint", ""},
		{"go-lint", "go-lint", ""},
		{"go-lint", "go-test", ""},
		{"missing", "x", ""},
	}
	for _, c := range cases {
		if _, _, err := RenameTaolu(r, c.name, c.newName, c.newGroup, "", "tester"); err == nil {
			t.Fatalf("RenameTaolu(%s -> %s) succeeded, want error", c.name, c.newName)
		}
	}
}

func TestRenamePreservesAssetTree(t *testing.T) {
	r := newTestVault(t)
	saveTaoluAssets(t, r, "frontend", "button", skillContent("button"), testAction, buttonAssets(), "component")
	if _, _, err := RenameTaolu(r, "button", "button-v2", "", "", "tester"); err != nil {
		t.Fatalf("RenameTaolu: %v", err)
	}

	sp := mustFindSkill(t, r, "button-v2")
	if !strings.HasPrefix(sp, "taolus/frontend/button-v2/") {
		t.Fatalf("path = %q", sp)
	}
	_, _, assets, err := ReadTaoluBundle(r, "button-v2", "")
	if err != nil {
		t.Fatalf("ReadTaoluBundle: %v", err)
	}
	if len(assets) != len(buttonAssets()) {
		t.Fatalf("renamed assets = %d, want %d", len(assets), len(buttonAssets()))
	}
	// The nested components/Icon.tsx survives the rename, not flattened.
	found := false
	for _, a := range assets {
		if a.Path == "components/Icon.tsx" {
			found = true
		}
	}
	if !found {
		t.Errorf("nested asset lost in rename: %+v", assets)
	}

	// History still spans the rename (v1, rename) and old versions read assets.
	hist, err := SkillHistory(r, sp)
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("history length = %d, want 2", len(hist))
	}
	if _, _, assets, err := ReadTaoluBundle(r, "button-v2", "v1"); err != nil {
		t.Fatalf("read v1 assets: %v", err)
	} else if len(assets) != 3 {
		t.Fatalf("v1 assets = %d, want 3", len(assets))
	}
}
