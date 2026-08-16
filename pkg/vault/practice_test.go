package vault

import (
	"strings"
	"testing"
)

func TestValidSlug(t *testing.T) {
	long := strings.Repeat("a", 64)
	tooLong := strings.Repeat("a", 65)
	valid := []string{"a", "go", "go-lint", "a-b-c", "1", "x1-y2", long}
	invalid := []string{
		"", "A", "Go-Lint", "-a", "a-", "a--b", "a b", "a_b", "a.b", "a/b",
		"a\nb", "é", tooLong,
	}
	for _, s := range valid {
		if !ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = true, want false", s)
		}
	}
}

func TestValidActionMode(t *testing.T) {
	for _, m := range []string{ModeApply, ModeInstall, ModeEnforce} {
		if !ValidActionMode(m) {
			t.Errorf("ValidActionMode(%q) = false, want true", m)
		}
	}
	for _, m := range []string{"", "DELETE", "apply "} {
		if ValidActionMode(m) {
			t.Errorf("ValidActionMode(%q) = true, want false", m)
		}
	}
}

func TestSplitRawFrontmatter(t *testing.T) {
	raw, body, err := splitRawFrontmatter("---\nname: x\n---\nbody text")
	if err != nil {
		t.Fatalf("splitRawFrontmatter: %v", err)
	}
	if raw != "name: x" {
		t.Errorf("raw = %q, want %q", raw, "name: x")
	}
	if body != "body text" {
		t.Errorf("body = %q, want %q", body, "body text")
	}

	if _, _, err := splitRawFrontmatter("name: x\n---\n"); err == nil {
		t.Error("missing opening --- accepted")
	}
	if _, _, err := splitRawFrontmatter("---\nname: x\n"); err == nil {
		t.Error("missing closing --- accepted")
	}
	if _, _, err := splitRawFrontmatter(""); err == nil {
		t.Error("empty content accepted")
	}
}

func TestSplitFrontmatter(t *testing.T) {
	meta, body, err := splitFrontmatter(skillContent("go-lint"))
	if err != nil {
		t.Fatalf("splitFrontmatter: %v", err)
	}
	if meta.Name != "go-lint" {
		t.Errorf("meta.Name = %q", meta.Name)
	}
	if !strings.Contains(body, "## Conventions") {
		t.Errorf("body lost: %q", body)
	}
	if meta.Metadata["tags"] != "test,mutation" {
		t.Errorf("tags = %q", meta.Metadata["tags"])
	}

	if _, _, err := splitFrontmatter("not frontmatter"); err == nil {
		t.Error("invalid frontmatter accepted")
	}
}

func TestSplitActionFrontmatter(t *testing.T) {
	meta, body, err := splitActionFrontmatter(testAction)
	if err != nil {
		t.Fatalf("splitActionFrontmatter: %v", err)
	}
	if meta.Mode != ModeApply {
		t.Errorf("mode = %q, want apply", meta.Mode)
	}
	if !strings.Contains(body, "Apply this taolu") {
		t.Errorf("body lost: %q", body)
	}
}

func TestValidateContent(t *testing.T) {
	if err := ValidateContent("go-lint", skillContent("go-lint")); err != nil {
		t.Fatalf("valid content rejected: %v", err)
	}
	if err := ValidateContent("other", skillContent("go-lint")); err == nil {
		t.Error("frontmatter name mismatch accepted")
	}
	if err := ValidateContent("Bad_Name", skillContent("Bad_Name")); err == nil {
		t.Error("invalid slug accepted")
	}
	noDesc := strings.Replace(skillContent("go-lint"), "description: test taolu for round-trips", "description:", 1)
	if err := ValidateContent("go-lint", noDesc); err == nil {
		t.Error("empty description accepted")
	}
	longDesc := strings.Replace(skillContent("go-lint"), "test taolu for round-trips", strings.Repeat("x", 1025), 1)
	if err := ValidateContent("go-lint", longDesc); err == nil {
		t.Error("1025-char description accepted")
	}
}

func TestValidateAction(t *testing.T) {
	if err := ValidateAction(testAction); err != nil {
		t.Fatalf("valid action rejected: %v", err)
	}
	if err := ValidateAction("---\nmode: bogus\n---\n"); err == nil {
		t.Error("invalid mode accepted")
	}
	if err := ValidateAction("---\n---\n"); err == nil {
		t.Error("missing mode accepted")
	}
	badFormat := "---\nmode: install\ndetail:\n  format: vim\n---\n"
	if err := ValidateAction(badFormat); err == nil {
		t.Error("invalid detail.format accepted")
	}
	goodFormat := "---\nmode: install\ndetail:\n  format: claude\n---\n"
	if err := ValidateAction(goodFormat); err != nil {
		t.Errorf("valid detail.format rejected: %v", err)
	}
}

func TestParseSkillPath(t *testing.T) {
	cases := []struct {
		path      string
		group     string
		name      string
		wantOK    bool
	}{
		{"taolus/workflows/go-lint/SKILL.md", "workflows", "go-lint", true},
		{"taolus/backend/api/SKILL.md", "backend", "api", true},
		{"taolus/workflows/go-lint/ACTION.md", "", "", false},
		{"taolus/workflows/go-lint/notes.md", "", "", false},
		{"practices/workflows/legacy/SKILL.md", "", "", false},
		{"taolus/x/y/SKILL.md", "x", "y", true},
	}
	for _, c := range cases {
		g, n, ok := parseSkillPath(c.path)
		if ok != c.wantOK {
			t.Errorf("parseSkillPath(%q) ok = %v, want %v", c.path, ok, c.wantOK)
			continue
		}
		if g != c.group || n != c.name {
			t.Errorf("parseSkillPath(%q) = (%q, %q), want (%q, %q)", c.path, g, n, c.group, c.name)
		}
	}
}

func TestSkillGroup(t *testing.T) {
	if g := skillGroup("taolus/backend/api/SKILL.md"); g != "backend" {
		t.Errorf("skillGroup = %q, want backend", g)
	}
	if g := skillGroup("practices/legacy/SKILL.md"); g != "general" {
		t.Errorf("legacy skillGroup = %q, want general", g)
	}
}

func TestRenameSkillFrontmatter(t *testing.T) {
	out, err := renameSkillFrontmatter(skillContent("go-lint"), "lint-checks")
	if err != nil {
		t.Fatalf("renameSkillFrontmatter: %v", err)
	}
	meta, body, err := splitFrontmatter(out)
	if err != nil {
		t.Fatalf("renamed content does not parse: %v", err)
	}
	if meta.Name != "lint-checks" {
		t.Errorf("name = %q, want lint-checks", meta.Name)
	}
	if meta.Description != "test taolu for round-trips" {
		t.Errorf("description changed to %q", meta.Description)
	}
	if !strings.Contains(body, "## Conventions") {
		t.Errorf("body changed:\n%s", body)
	}

	if _, err := renameSkillFrontmatter("---\ndescription: no name\n---\n", "x"); err == nil {
		t.Error("missing name field accepted")
	}
}

func TestShortUUID(t *testing.T) {
	if got := ShortUUID("0123456789abcdef"); got != "0123456789ab" {
		t.Errorf("ShortUUID(long) = %q", got)
	}
	if got := ShortUUID("abc"); got != "abc" {
		t.Errorf("ShortUUID(short) = %q", got)
	}
}
