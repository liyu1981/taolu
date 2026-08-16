package vault

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	libfossil "github.com/danmestas/go-libfossil"
)

const taoluAuthoringSkill = `---
name: taolu-authoring
description: How to summarize a project's conventions into a taolu (a SKILL.md plus an ACTION.md that says what to do with it) and how to save, apply, upgrade, and roll back versioned taolus.
license: MIT
compatibility: opencode
metadata:
  tags: "taolu,authoring,skill,meta"
  source: "taolu"
---

## What a taolu is

A **taolu** is a skill plus an action. It is the unit of knowledge in the
vault: a reusable convention, saved under a **group** (a domain folder), and
versioned in the Fossil database.

- The **skill** is a SKILL.md document describing the convention.
- The **action** is an ACTION.md document telling the agent what to do after
  obtaining the skill. Skill and action are one taolu: they are saved,
  versioned, read, and diffed together.

### Action modes

- **apply**: use the skill once on the current project; do not install it.
- **install**: install the skill into the local repo as a real skill file with
  a version pin.
- **enforce**: install the skill and add a reference to it in the local
  AGENTS.md so every agent follows it.

## How to summarize a project into a taolu

### 1. Confirm the taolu's scope

Establish the scope **before** surveying the project. If the user did not
specify it, ask and confirm before summarizing:

- What should this taolu cover? One component (e.g. go-api-server), a
  horizontal area (e.g. backend conventions), or the whole project?
- What should stay out of scope?

Do not invent a scope and start drafting; confirm it first.

### 2. Confirm the action (strict gate)

The action mode is a hard gate: do not proceed until the user has explicitly
chosen **apply**, **install**, or **enforce**. Never guess, never default
silently, and never proceed with an unstated mode. If the user has not
specified one, stop and ask:

- **apply**: use once on the current project, nothing saved.
- **install**: make the skill available in the project's skill store.
- **enforce**: a project-wide convention every agent must follow (installs and
  adds an AGENTS.md reference).

Restate the confirmed mode back to the user before moving on. If the answer is
ambiguous, ask again until the choice is explicit. Do not infer the mode from
context.

### 3. Survey the project

Read the project's context before drafting anything:

- README, docs, AGENTS.md, and any existing skill/config conventions.
- Repository layout and module structure.
- Tooling and configs (build, lint, test, CI, secrets, package manifests).
- How code is organized and how features are implemented.

### 4. Extract the conventions

Focus on durable, actionable conventions, not trivia:

- Architecture and layering.
- Project structure and naming.
- Error handling and logging.
- Testing strategy and fixtures.
- Configuration, secrets, and environment handling.
- Build, release, and CI workflow.

### 5. Write the SKILL.md draft

Every taolu's skill is a SKILL.md document with YAML frontmatter:

    ---
    name: <skill-name>
    description: <1-1024 chars, what the agent uses to pick this taolu>
    license: MIT
    compatibility: opencode
    metadata:
      tags: "comma,separated,search,terms"
      source: "<original repo if any>"
      stack: "<primary tech stack>"
    ---

    ## Purpose
    When to use this taolu.

    ## Conventions
    The concrete, actionable instructions.

- The name must match the save name.
- Keep the description specific enough that an agent can choose correctly.
- Use concrete, actionable instructions. Prefer small, focused taolus.

### 6. Write the ACTION.md

Use the action mode confirmed in step 2:

    ---
    mode: install        # apply | install | enforce
    detail:
      format: opencode   # install/enforce only: opencode | claude | agents
    ---

- Choose **apply** for a one-off: read the skill, do the work, nothing saved.
- Choose **install** to make the skill available in the project's skill store.
- Choose **enforce** for a project-wide convention that every agent must
  follow (installs and adds an AGENTS.md reference).

### 7. Review and get explicit approval before sending out

Before saving or applying the taolu, get explicit approval:

- Present the taolu for review. If the taolu is long, provide a brief of it
  (name, group, action mode, one-line description, and a short summary of the
  conventions it captures) and offer the option to review it in full length.
- Ask for approval through the interactive UI when one is available — for
  example, opencode's ask tool or any elicitation prompt the client exposes.
  Fall back to a plain text prompt only when no UI mechanism exists.
- Treat approval as a hard gate: do not call taolu_save or taolu_apply until
  the user has explicitly approved. Silence or an absent reply is **not**
  approval.

## How to save / update

- taolu_save with the name, a group (e.g. backend, frontend, workflows, meta),
  the full SKILL.md content, and the ACTION.md content.
- The server validates both, commits them together, and labels the new version
  (v1, v2, ...). Provide a concise message describing the change.
- Saving again with the same name creates a new version; nothing is lost.
- Always review and get explicit approval before saving (see step 7 above);
  when updating an existing taolu, show what changed.

## How to apply / upgrade / roll back

- taolu_list to find taolus; taolu_get to read one (skill + action).
- taolu_apply dispatches on the action mode: apply returns the content for a
  one-shot use; install/enforce write the skill and a .taolu-version pin.
- To upgrade, read the pin, check taolu_history and taolu_diff, then apply the
  newer version (or an older one to roll back). Pins make the upgrade explicit.

## Quality checklist

A good taolu is:

- Specific: it captures real conventions of a real project, not generic advice.
- Actionable: an agent can follow it without guessing.
- Small: one topic per taolu.
- Findable: a clear description and metadata tags.
- Versioned: save improvements as new versions, never by overwriting silently.
`

const taoluAuthoringAction = `---
mode: apply
---
Apply this taolu by reading the accompanying skill and following it whenever
you summarize a project or author, save, or apply taolus.
`

// EnsureAuthoringGuide seeds the taolu-authoring guide if it is missing.
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
			{Name: skillPath(seedGroup, SeedName), Content: []byte(taoluAuthoringSkill)},
			{Name: actionPath(seedGroup, SeedName), Content: []byte(taoluAuthoringAction)},
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

// MigrateLegacy migrates a pre-v1 vault's practices/<group>/<name>/ tree to the
// taolus/ root, adding a default ACTION.md (mode install) to skills that lack
// one, and tags each migrated taolu v1. It is a no-op when there is no legacy
// tree.
func MigrateLegacy(r *libfossil.Repo, user string) error {
	rid, err := r.ResolveVersion("tip")
	if errors.Is(err, libfossil.ErrVersionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	files, err := r.ListFiles(rid)
	if err != nil {
		return err
	}
	legacyPrefix := "practices" + string(filepath.Separator)
	var toCommit []libfossil.FileToCommit
	dirs := map[string]bool{}
	for _, f := range files {
		if !strings.HasPrefix(f.Name, legacyPrefix) {
			continue
		}
		base := filepath.Base(f.Name)
		if base != "SKILL.md" && base != "ACTION.md" {
			continue
		}
		data, err := r.ReadFile(rid, f.Name)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(f.Name, "practices")
		toCommit = append(toCommit, libfossil.FileToCommit{
			Name:    "taolus" + rel,
			Content: data,
		})
		if base == "SKILL.md" {
			dirs[filepath.Dir(f.Name)] = true
		}
	}
	if len(toCommit) == 0 {
		return nil
	}
	for d := range dirs {
		if fileExistsAt(files, filepath.Join(d, "ACTION.md")) {
			continue
		}
		rel := strings.TrimPrefix(d, "practices")
		toCommit = append(toCommit, libfossil.FileToCommit{
			Name:    "taolus" + rel + string(filepath.Separator) + "ACTION.md",
			Content: []byte(defaultActionInstall),
		})
	}
	if user == "" {
		user = "admin"
	}
	parent, err := resolveParentTip(r)
	if err != nil {
		return err
	}
	newRID, _, err := r.Commit(libfossil.CommitOpts{
		Files:    toCommit,
		Comment:  "migrate legacy practices/ tree to taolus/",
		User:     user,
		ParentID: parent,
	})
	if err != nil {
		return err
	}
	for d := range dirs {
		name := filepath.Base(d)
		if !ValidSlug(name) {
			continue
		}
		if _, err := r.Tag(libfossil.TagOpts{Name: name + "-v1", TargetID: newRID, User: user}); err != nil {
			return err
		}
	}
	return nil
}

func fileExistsAt(files []libfossil.FileEntry, name string) bool {
	for _, f := range files {
		if f.Name == name {
			return true
		}
	}
	return false
}
