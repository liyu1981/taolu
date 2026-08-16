package vault

import (
	"strings"
	"testing"
)

func TestSaveTaoluVersioning(t *testing.T) {
	r := newTestVault(t)
	label, uuid, total, err := SaveTaolu(r, "workflows", "go-lint",
		skillContent("go-lint"), testAction, nil, "first version", "tester", "")
	if err != nil {
		t.Fatalf("SaveTaolu v1: %v", err)
	}
	if label != "v1" || total != 1 {
		t.Fatalf("v1: label=%s total=%d, want v1/1", label, total)
	}
	if len(uuid) == 0 {
		t.Error("v1 uuid is empty")
	}

	label, _, total, err = SaveTaolu(r, "workflows", "go-lint",
		skillContentV2("go-lint"), testAction, nil, "second version", "tester", "")
	if err != nil {
		t.Fatalf("SaveTaolu v2: %v", err)
	}
	if label != "v2" || total != 2 {
		t.Fatalf("v2: label=%s total=%d, want v2/2", label, total)
	}

	hist, err := SkillHistory(r, mustFindSkill(t, r, "go-lint"))
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("history length = %d, want 2", len(hist))
	}
	if hist[0].Label != "v1" || hist[1].Label != "v2" {
		t.Fatalf("labels = %s, %s", hist[0].Label, hist[1].Label)
	}
	if hist[0].Message != "first version" || hist[1].Message != "second version" {
		t.Errorf("messages = %q, %q", hist[0].Message, hist[1].Message)
	}
	if hist[0].User != "tester" {
		t.Errorf("user = %q", hist[0].User)
	}
}

func TestSaveTaoluCustomLabel(t *testing.T) {
	r := newTestVault(t)
	label, _, total, err := SaveTaolu(r, "workflows", "go-lint",
		skillContent("go-lint"), testAction, nil, "beta", "tester", "beta")
	if err != nil {
		t.Fatalf("SaveTaolu: %v", err)
	}
	if label != "beta" || total != 1 {
		t.Fatalf("label=%s total=%d, want beta/1", label, total)
	}
	// The custom label is returned and the version is readable at tip; the
	// default v1..vN sequence still applies for lookups.
	skill, _, err := ReadTaoluAtVersion(r, "go-lint", "")
	if err != nil {
		t.Fatalf("read tip: %v", err)
	}
	if !strings.Contains(skill, "name: go-lint") {
		t.Errorf("skill mismatch:\n%s", skill)
	}
	if _, _, err := ReadTaoluAtVersion(r, "go-lint", "v1"); err != nil {
		t.Errorf("read by v1: %v", err)
	}
}

func TestSaveTaoluIdenticalContentDeduplicates(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	// Identical content: the blob UUID does not change, so no version is
	// recorded even though a check-in is created.
	saveTaoluContent(t, r, "workflows", "go-lint", skillContent("go-lint"), testAction, "no-op")
	hist, err := SkillHistory(r, mustFindSkill(t, r, "go-lint"))
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("history length = %d, want 1 (identical content records no version)", len(hist))
	}
}

func TestSaveTaoluGroupConflict(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	_, _, _, err := SaveTaolu(r, "backend", "go-lint", skillContent("go-lint"), testAction, nil, "", "tester", "")
	if err == nil {
		t.Fatal("saving same name under a different group succeeded, want conflict error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSaveTaoluDefaultUserAndMessage(t *testing.T) {
	r := newTestVault(t)
	label, _, _, err := SaveTaolu(r, "workflows", "go-lint", skillContent("go-lint"), testAction, nil, "", "", "")
	if err != nil {
		t.Fatalf("SaveTaolu: %v", err)
	}
	hist, err := SkillHistory(r, mustFindSkill(t, r, "go-lint"))
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if label != "v1" || hist[0].Message == "" || hist[0].User != "admin" {
		t.Fatalf("defaults not applied: label=%s msg=%q user=%q", label, hist[0].Message, hist[0].User)
	}
}

func TestSaveTaoluWithAssetsRoundTrip(t *testing.T) {
	r := newTestVault(t)
	assets := buttonAssets()
	label := saveTaoluAssets(t, r, "frontend", "button", skillContent("button"), testAction, assets, "first component")

	if label != "v1" {
		t.Fatalf("label = %s, want v1", label)
	}
	skill, action, got, err := ReadTaoluBundle(r, "button", "")
	if err != nil {
		t.Fatalf("ReadTaoluBundle: %v", err)
	}
	if !strings.Contains(skill, "name: button") || !strings.Contains(action, "mode: apply") {
		t.Errorf("skill/action mismatch:\n%s\n%s", skill, action)
	}
	if len(got) != len(assets) {
		t.Fatalf("assets = %d, want %d", len(got), len(assets))
	}
	// Assets are sorted by path on read.
	if got[0].Path != "Button.css" || got[1].Path != "Button.tsx" || got[2].Path != "components/Icon.tsx" {
		t.Errorf("asset order = %v", got)
	}
	byPath := map[string]Asset{}
	for _, a := range assets {
		byPath[a.Path] = a
	}
	for _, g := range got {
		want := byPath[g.Path]
		if g.Content != want.Content {
			t.Errorf("asset %s content = %q, want %q", g.Path, g.Content, want.Content)
		}
	}
}
