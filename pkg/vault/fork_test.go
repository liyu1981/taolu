package vault

import (
	"strings"
	"testing"
)

func TestForkCopiesContentAndRecordsProvenance(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")

	ref := TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"}
	f, err := ForkTaolu(r, ref, "go-lint-fork", "", "", "tester")
	if err != nil {
		t.Fatalf("ForkTaolu: %v", err)
	}
	if f.Source.Name != "go-lint" || f.Source.Domain != DomainPrefix {
		t.Fatalf("ForkInfo.Source = %s, want @local/workflows/go-lint", f.Source.String())
	}
	if f.Version != "v1" {
		t.Fatalf("ForkInfo.Version = %s, want v1", f.Version)
	}

	sp := mustFindSkill(t, r, "go-lint-fork")
	skill, action, assets, err := ReadTaoluBundle(r, "go-lint-fork", "")
	if err != nil {
		t.Fatalf("ReadTaoluBundle: %v", err)
	}
	if !strings.Contains(skill, "name: go-lint-fork") {
		t.Fatalf("frontmatter name not rewritten:\n%s", skill)
	}
	if !strings.Contains(action, "mode: apply") {
		t.Fatalf("ACTION.md mangled:\n%s", action)
	}
	if len(assets) != 0 {
		t.Fatalf("fork assets = %d, want 0", len(assets))
	}
	if !strings.HasPrefix(sp, "taolus/@local/workflows/go-lint-fork/SKILL.md") {
		t.Fatalf("path = %q", sp)
	}

	// Source is untouched.
	srcSkill, _, _, err := ReadTaoluBundle(r, "go-lint", "")
	if err != nil {
		t.Fatalf("ReadTaoluBundle(source): %v", err)
	}
	if strings.Contains(srcSkill, "name: go-lint-fork") {
		t.Fatal("source taolu was mutated by fork")
	}

	// Fork marker readable on the fork.
	forkInfo, err := ReadForkInfo(r, TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint-fork"})
	if err != nil {
		t.Fatalf("ReadForkInfo: %v", err)
	}
	if forkInfo == nil {
		t.Fatal("ReadForkInfo returned nil, want provenance")
	}
	if forkInfo.Source.String() != DomainPrefix+"/workflows/go-lint" {
		t.Fatalf("fork source = %q", forkInfo.Source.String())
	}
}

func TestForkKeepsSourceIntact(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "backend", "api-server")

	ref := TaoluRef{Domain: DomainPrefix, Group: "backend", Name: "api-server"}
	if _, err := ForkTaolu(r, ref, "api-server-fork", "", "", "tester"); err != nil {
		t.Fatalf("ForkTaolu: %v", err)
	}

	sp := mustFindSkill(t, r, "api-server")
	skill, _, _, err := ReadTaoluBundle(r, "api-server", "")
	if err != nil {
		t.Fatalf("ReadTaoluBundle(source): %v", err)
	}
	if strings.Contains(skill, "name: api-server-fork") {
		t.Fatal("source taolu was mutated by fork")
	}
	if !strings.HasPrefix(sp, "taolus/@local/backend/api-server/SKILL.md") {
		t.Fatalf("source path = %q", sp)
	}
}

func TestForkHistoryShowsCopiedLineage(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV2("go-lint"), testAction, "v2 tweak")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV3("go-lint"), testAction, "v3 tweak")

	ref := TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"}
	if _, err := ForkTaolu(r, ref, "go-fork", "", "", "tester"); err != nil {
		t.Fatalf("ForkTaolu: %v", err)
	}

	hist, err := SkillHistory(r, TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-fork"}.Path())
	if err != nil {
		t.Fatalf("SkillHistory: %v", err)
	}
	if len(hist) != 4 {
		t.Fatalf("history length = %d, want 4 (v1..v3 from source + fork commit)", len(hist))
	}
	for i, v := range hist {
		if v.Label != "v"+string(rune('1'+i)) {
			t.Fatalf("hist[%d].Label = %q, want v%d", i, v.Label, i+1)
		}
	}
}

func TestForkHistoryIndependenceAfterFork(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV2("go-lint"), testAction, "source v2")
	ref := TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"}
	if _, err := ForkTaolu(r, ref, "go-fork", "", "", "tester"); err != nil {
		t.Fatalf("ForkTaolu: %v", err)
	}

	saveTaoluContent(t, r, "workflows", "go-fork", skillContentV2("go-fork"), testAction, "fork v2")
	saveTaoluContent(t, r, "workflows", "go-lint", skillContentV3("go-lint"), testAction, "source v3")

	histFork, err := SkillHistory(r, TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-fork"}.Path())
	if err != nil {
		t.Fatalf("SkillHistory(fork): %v", err)
	}
	histSrc, err := SkillHistory(r, TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"}.Path())
	if err != nil {
		t.Fatalf("SkillHistory(source): %v", err)
	}
	// Fork lineage: v1+v2 copied from source, v3 fork commit, v4 independent save = 4.
	if len(histFork) != 4 {
		t.Fatalf("fork history = %d versions, want 4 (v1..v2 from source + v3 fork commit + v4 save)", len(histFork))
	}
	// Source: v1 + v2 + v3 = 3.
	if len(histSrc) != 3 {
		t.Fatalf("source history = %d versions, want 3", len(histSrc))
	}
	// The fork and source have different content at their latest versions.
	forkSkill, _, _, err := ReadTaoluBundle(r, "go-fork", "")
	if err != nil {
		t.Fatalf("ReadTaoluBundle(fork): %v", err)
	}
	srcSkill, _, _, err := ReadTaoluBundle(r, "go-lint", "")
	if err != nil {
		t.Fatalf("ReadTaoluBundle(source): %v", err)
	}
	if !strings.Contains(forkSkill, "name: go-fork") {
		t.Fatalf("fork frontmatter = %s", forkSkill)
	}
	if !strings.Contains(srcSkill, "name: go-lint") {
		t.Fatalf("source frontmatter = %s", srcSkill)
	}
	// The fork's history entry for the copied versions points to the source path.
	wantSrcPath := TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"}.Path()
	if histFork[0].Path != wantSrcPath {
		t.Fatalf("fork hist[0].Path = %q, want source path", histFork[0].Path)
	}
}

func TestForkAssetsCopied(t *testing.T) {
	r := newTestVault(t)
	saveTaoluAssets(t, r, "frontend", "button", skillContent("button"), testAction, buttonAssets(), "component")
	ref := TaoluRef{Domain: DomainPrefix, Group: "frontend", Name: "button"}
	if _, err := ForkTaolu(r, ref, "button-fork", "", "", "tester"); err != nil {
		t.Fatalf("ForkTaolu: %v", err)
	}
	_, _, assets, err := ReadTaoluBundle(r, "button-fork", "")
	if err != nil {
		t.Fatalf("ReadTaoluBundle: %v", err)
	}
	if len(assets) != len(buttonAssets()) {
		t.Fatalf("fork assets = %d, want %d", len(assets), len(buttonAssets()))
	}
}

func TestForkIntoNewGroup(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	ref := TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"}
	if _, err := ForkTaolu(r, ref, "go-fork", "backend", "", "tester"); err != nil {
		t.Fatalf("ForkTaolu into new group: %v", err)
	}
	sp := mustFindSkill(t, r, "go-fork")
	if !strings.HasPrefix(sp, "taolus/@local/backend/go-fork/SKILL.md") {
		t.Fatalf("path = %q, want taolus/@local/backend/go-fork/...", sp)
	}
}

func TestForkGuards(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	saveTestTaolu(t, r, "workflows", "go-test")

	ref := TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"}
	cases := []struct {
		name, newName, newGroup string
	}{
		{SeedName, "forked-seed", ""},
		{"go-lint", "Go-Lint", ""},
		{"go-lint", "go-test", ""},
		{"go-lint", "go-lint", ""},
		{"missing", "x", ""},
	}
	for _, c := range cases {
		var srcRef TaoluRef
		if c.name == SeedName {
			srcRef = TaoluRef{Domain: DomainPrefix, Group: "meta", Name: c.name}
		} else if c.name == "missing" {
			srcRef = TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: c.name}
		} else {
			srcRef = ref
		}
		if _, err := ForkTaolu(r, srcRef, c.newName, c.newGroup, "", "tester"); err == nil {
			t.Fatalf("ForkTaolu(%s -> %s) succeeded, want error", c.name, c.newName)
		}
	}
}

func TestForkRefusesArchivedSource(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	if _, err := ArchiveTaolu(r, "go-lint", "", "tester"); err != nil {
		t.Fatalf("ArchiveTaolu: %v", err)
	}
	ref := TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"}
	if _, err := ForkTaolu(r, ref, "go-fork", "", "", "tester"); err == nil {
		t.Fatal("ForkTaolu of archived taolu succeeded, want error")
	}
}

func TestParseForkInfo(t *testing.T) {
	_, ok := ParseForkInfo("")
	if ok {
		t.Fatal("ParseForkInfo('') ok, want false")
	}
	_, ok = ParseForkInfo("{invalid")
	if ok {
		t.Fatal("ParseForkInfo(invalid) ok, want false")
	}
	f, ok := ParseForkInfo(`{"source":{"Domain":"@local","Group":"g","Name":"n"},"version":"v3","source_uuid":"abc"}`)
	if !ok {
		t.Fatal("ParseForkInfo valid: ok=false, want true")
	}
	if f.Source.String() != "@local/g/n" || f.Version != "v3" || f.SourceUUID != "abc" {
		t.Fatalf("ParseForkInfo = %+v", f)
	}
}

func TestForkInfoNotPresent(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	f, err := ReadForkInfo(r, TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"})
	if err != nil {
		t.Fatalf("ReadForkInfo: %v", err)
	}
	if f != nil {
		t.Fatalf("ReadForkInfo = %+v, want nil", f)
	}
}

func TestListTaoluShowsForkSource(t *testing.T) {
	r := newTestVault(t)
	saveTestTaolu(t, r, "workflows", "go-lint")
	ref := TaoluRef{Domain: DomainPrefix, Group: "workflows", Name: "go-lint"}
	if _, err := ForkTaolu(r, ref, "go-fork", "", "", "tester"); err != nil {
		t.Fatalf("ForkTaolu: %v", err)
	}
	taolus, err := ListTaolu(r)
	if err != nil {
		t.Fatalf("ListTaolu: %v", err)
	}
	var forkTaolu, srcTaolu *TaoluInfo
	for i := range taolus {
		if taolus[i].Name == "go-fork" {
			forkTaolu = &taolus[i]
		}
		if taolus[i].Name == "go-lint" {
			srcTaolu = &taolus[i]
		}
	}
	if forkTaolu == nil {
		t.Fatal("go-fork not in listing")
	}
	if forkTaolu.ForkSource != DomainPrefix+"/workflows/go-lint" {
		t.Fatalf("ForkSource = %q, want @local/workflows/go-lint", forkTaolu.ForkSource)
	}
	if srcTaolu != nil && srcTaolu.ForkSource != "" {
		t.Fatalf("source ForkSource = %q, want empty", srcTaolu.ForkSource)
	}
}
