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
	if sp != "taolus/meta/taolu-authoring/SKILL.md" {
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
