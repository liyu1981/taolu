package vault

import (
	"fmt"

	libfossil "github.com/danmestas/go-libfossil"
)

const practiceAuthoringSeed = `---
name: practice-authoring
description: How to summarize a project's conventions and save, install, upgrade, and roll back versioned skills in the agent vault.
license: MIT
compatibility: opencode
metadata:
  tags: "vault,authoring,skill,meta"
  source: "taolu"
---

## What the vault is

The vault is a versioned library of skills. A **skill** is a SKILL.md document
stored under a **practice** (a grouping folder). Each save creates a new
immutable version labeled v1, v2, ... Skills are installed into a project as
real skill files and can be pinned to a specific version.

## How to summarize a project into a skill

### 1. Survey the project

Read the project's context before drafting anything:

- README, docs, AGENTS.md, and any existing skill/config conventions.
- Repository layout and module structure.
- Tooling and configs (build, lint, test, CI, secrets, package manifests).
- How code is organized and how features are implemented.

### 2. Extract the conventions

Focus on durable, actionable conventions, not trivia:

- Architecture and layering.
- Project structure and naming.
- Error handling and logging.
- Testing strategy and fixtures.
- Configuration, secrets, and environment handling.
- Build, release, and CI workflow.

### 3. Write the SKILL.md draft

Every skill is a SKILL.md document with YAML frontmatter:

    ---
    name: <skill-name>
    description: <1-1024 chars, what the agent uses to pick this skill>
    license: MIT
    compatibility: opencode
    metadata:
      tags: "comma,separated,search,terms"
      source: "<original repo if any>"
      stack: "<primary tech stack>"
    ---

    ## Purpose
    When to use this skill.

    ## Conventions
    The concrete, actionable instructions.

- The name must be the skill name and match the save name.
- Keep the description specific enough that an agent can choose correctly.
- Use concrete, actionable instructions. Prefer small, focused skills.

## How to save / update

- vault_practice_save with the skill name, a practice group
  (e.g. backend, frontend, workflows, meta), and the full content.
- The server validates the name and frontmatter, commits, and labels the new
  version (v1, v2, ...). Provide a concise message describing the change.
- Saving again with the same name creates a new version; nothing is lost.

## How to install / upgrade / roll back

- vault_practice_list to find skills; vault_practice_get to read one.
- vault_practice_install writes .opencode/skills/<name>/SKILL.md and a
  .vault-version pin. Install requires explicit user approval.
- To upgrade, read the pin, check vault_practice_history and
  vault_practice_diff, then re-install the newer version (or an older one to
  roll back). Pins make the upgrade explicit.

## Quality checklist

A good skill is:

- Specific: it captures real conventions of a real project, not generic advice.
- Actionable: an agent can follow it without guessing.
- Small: one topic per skill.
- Findable: a clear description and metadata tags.
- Versioned: save improvements as new versions, never by overwriting silently.
`

// EnsureAuthoringGuide seeds the practice-authoring skill if it is missing.
func EnsureAuthoringGuide(r *libfossil.Repo, user string) error {
	path, err := FindSkillPath(r, SeedName)
	if err != nil {
		return err
	}
	if path != "" {
		return nil
	}
	parent, err := resolveParentTip(r)
	if err != nil {
		return err
	}
	if user == "" {
		user = "admin"
	}
	rid, _, err := r.Commit(libfossil.CommitOpts{
		Files: []libfossil.FileToCommit{
			{Name: practicePath(seedGroup, SeedName), Content: []byte(practiceAuthoringSeed)},
		},
		Comment:  fmt.Sprintf("seed %s (v1)", SeedName),
		User:     user,
		ParentID: parent,
	})
	if err != nil {
		return fmt.Errorf("seed %s: %w", SeedName, err)
	}
	_, err = r.Tag(libfossil.TagOpts{
		Name:     SeedName + "-v1",
		TargetID: rid,
		User:     user,
	})
	return err
}
