package vault

import (
	"strings"
	"testing"

	libfossil "github.com/danmestas/go-libfossil"
)

// testSkill is a minimal valid SKILL.md with %s as the name placeholder.
const testSkill = `---
name: %s
description: test taolu for round-trips
license: MIT
compatibility: opencode
metadata:
  tags: "test,mutation"
---

## Purpose
Exercise vault operations.

## Conventions
Do the thing.
`

// testAction is a minimal valid ACTION.md.
const testAction = `---
mode: apply
---
Apply this taolu in tests only.
`

func skillContent(name string) string {
	return strings.ReplaceAll(testSkill, "%s", name)
}

func skillContentV2(name string) string {
	return strings.ReplaceAll(skillContent(name), "Do the thing.", "Do the refined thing.")
}

func skillContentV3(name string) string {
	return strings.ReplaceAll(skillContentV2(name), "Do the refined thing.", "Do the extra thing.")
}

// newTestVault creates a fresh seeded vault under a temp directory.
func newTestVault(t *testing.T) *libfossil.Repo {
	t.Helper()
	r, _, err := EnsureVault(t.TempDir()+"/vault.fossil", "tester")
	if err != nil {
		t.Fatalf("EnsureVault: %v", err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

// saveTestTaolu saves name under group as its first version and expects the
// returned label to be v1.
func saveTestTaolu(t *testing.T, r *libfossil.Repo, group, name string) {
	t.Helper()
	label, _, _, err := SaveTaolu(r, group, name, skillContent(name), testAction, "seed "+name, "tester", "")
	if err != nil {
		t.Fatalf("SaveTaolu(%s): %v", name, err)
	}
	if label != "v1" {
		t.Fatalf("SaveTaolu(%s): label = %s, want v1", name, label)
	}
}

// saveTaoluContent saves the given skill/action content and fails the test on
// error, returning the version label.
func saveTaoluContent(t *testing.T, r *libfossil.Repo, group, name, skill, action, message string) string {
	t.Helper()
	label, _, _, err := SaveTaolu(r, group, name, skill, action, message, "tester", "")
	if err != nil {
		t.Fatalf("SaveTaolu(%s) content: %v", name, err)
	}
	return label
}

// mustNotArchived is a small assert helper naming the taolu in messages.
func mustFindSkill(t *testing.T, r *libfossil.Repo, name string) string {
	t.Helper()
	sp, err := FindSkillPath(r, name)
	if err != nil {
		t.Fatalf("FindSkillPath(%s): %v", name, err)
	}
	if sp == "" {
		t.Fatalf("FindSkillPath(%s) = \"\", want present", name)
	}
	return sp
}
