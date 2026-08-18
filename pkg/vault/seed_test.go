package vault

import (
	"path/filepath"
	"testing"

	libfossil "github.com/danmestas/go-libfossil"
)

func TestEnsureAuthoringGuideSeeds(t *testing.T) {
	p := filepath.Join(t.TempDir(), "vault.fossil")
	r, err := libfossil.Create(p, libfossil.CreateOpts{User: "tester"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer r.Close()

	if err := EnsureAuthoringGuide(r, "tester"); err != nil {
		t.Fatalf("EnsureAuthoringGuide: %v", err)
	}
	sp := mustFindSkill(t, r, SeedName)
	// New 3-layer format with @local domain
	if sp != "taolus/@local/meta/taolu-authoring/SKILL.md" {
		t.Errorf("seed path = %q", sp)
	}

	// Idempotent: a second call must not add a second version.
	if err := EnsureAuthoringGuide(r, "tester"); err != nil {
		t.Fatalf("EnsureAuthoringGuide (2nd): %v", err)
	}
	hist, err := SkillHistory(r, sp)
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("seed history = %d, want 1", len(hist))
	}
}

func TestEnsureAuthoringGuideUser(t *testing.T) {
	p := filepath.Join(t.TempDir(), "vault.fossil")
	r, err := libfossil.Create(p, libfossil.CreateOpts{User: "tester"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer r.Close()
	if err := EnsureAuthoringGuide(r, ""); err != nil {
		t.Fatalf("EnsureAuthoringGuide: %v", err)
	}
	hist, err := SkillHistory(r, mustFindSkill(t, r, SeedName))
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if hist[0].User != "admin" {
		t.Errorf("seed user = %q, want admin", hist[0].User)
	}
}

func TestEnsureAuthoringGuideUpgrades(t *testing.T) {
	p := filepath.Join(t.TempDir(), "vault.fossil")
	r, err := libfossil.Create(p, libfossil.CreateOpts{User: "tester"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer r.Close()
	if err := EnsureAuthoringGuide(r, "tester"); err != nil {
		t.Fatalf("EnsureAuthoringGuide seed: %v", err)
	}
	sp := mustFindSkill(t, r, SeedName)

	// Simulate an outdated bundled guide by replacing the content.
	stale := "---\nname: taolu-authoring\ndescription: stale bundled guide\n---\nold content\n"
	if _, _, _, err := SaveTaolu(r, seedGroup, SeedName, stale, taoluAuthoringAction, nil, "simulate stale seed", "tester", ""); err != nil {
		t.Fatalf("stale save: %v", err)
	}

	// The next ensure upgrades to the current bundled content as a new version.
	if err := EnsureAuthoringGuide(r, "tester"); err != nil {
		t.Fatalf("EnsureAuthoringGuide upgrade: %v", err)
	}
	hist, err := SkillHistory(r, sp)
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if len(hist) != 3 {
		t.Fatalf("history length = %d, want 3 (seed, stale, upgrade)", len(hist))
	}
	if hist[2].Message != "upgrade taolu-authoring to match the bundled seed" {
		t.Errorf("upgrade message = %q", hist[2].Message)
	}
	skill, _, err := ReadTaoluAtVersion(r, SeedName, "")
	if err != nil {
		t.Fatalf("read tip: %v", err)
	}
	if skill != taoluAuthoringSkill {
		t.Error("tip content does not match the bundled seed")
	}
	// A further ensure with unchanged content is a no-op: no new version.
	if err := EnsureAuthoringGuide(r, "tester"); err != nil {
		t.Fatalf("EnsureAuthoringGuide repeat: %v", err)
	}
	if hist, err := SkillHistory(r, sp); err != nil {
		t.Fatalf("SkillHistory: %v", err)
	} else if len(hist) != 3 {
		t.Fatalf("history length = %d after no-op ensure, want 3", len(hist))
	}
}
