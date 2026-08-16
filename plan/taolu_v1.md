# Taolu v1 — Versioned Practice Library with Actions (Detailed Plan)

This document evolves the original idea in `plan/init_idea.md` into the next
stage. It is a detailed plan: what changes, how the data model and tool surface
look, and how we get there in phases. The original v1 idea is preserved in
Section 1; everything after it combines that idea with the implementation we
already have (Fossil-backed vault, `vault_*` MCP tools, versioning, install +
pin) and the decisions recorded in Section 12.

---

## 1. The idea (from taolu_v1.md, original)

- A **taolu** is a *skill + action items* for the agent:
  1. The **skill** is the usual agent skill (a `SKILL.md` document).
  2. The **action** tells the agent what to do after obtaining the skill:
     - a. **apply** the skill once to this project;
     - b. **install** the skill in the local repo;
     - c. **install** the skill in the local repo **and enforce** compliance to
       it in the local `AGENTS.md`.
- Our MCP server is a **cross-project storage** for taolus:
  1. A taolu has a **group** category, lives in its skill folder, and carries an
     **action**.
  2. It is saved in the Fossil database with **versioning**.
- The MCP server also accepts **list** and **query**, and offers
  **instructions** for agents on how to create a taolu and apply it.

## 2. Where we are today

Current implementation (pre-v1) already delivers:

- A Fossil-backed vault at `~/.taolu/vault.fossil` (overridable per-call or via
  `TAOLU_REPO`), storage root `practices/<group>/<name>/SKILL.md`.
- `vault_*` MCP toolset: `vault_init`, `vault_info`, `vault_practice_save`,
  `_get`, `_list`, `_history`, `_diff`, `_install`, `_export`.
- Immutable versioning per skill: auto `vN` labels, UUID prefixes, `tip`; tags
  named `<name>-<label>`.
- Install materializes `SKILL.md` into `.opencode|.claude|.agents/skills/<name>/`
  plus a `.vault-version` pin; always requires explicit user approval.
- Built-in `practice-authoring` seed skill under the `meta` practice group.
- Slug + frontmatter validation (`name`, `description`, `license`,
  `compatibility`, `metadata`); support files can sit beside `SKILL.md`.

v1 keeps all of this machinery and builds the taolu concept on top of it.

## 3. Concept change: skill → taolu

| Aspect | Today (practice) | v1 (taolu) |
| --- | --- | --- |
| Unit of knowledge | a skill (`SKILL.md`) | skill + action (`SKILL.md` + `ACTION.md`) |
| Storage root | `practices/` | `taolus/` |
| Tool prefix | `vault_practice_*` | `taolu_*` |
| What the agent does | installs and/or reads | follows the action: apply, install, or enforce |
| Authoring guide | `practice-authoring` | `taolu-authoring` |
| Version pin | `.vault-version` | `.taolu-version` |

The **action is first-class**: it is stored, versioned, listed, and returned
alongside the skill, and the server has a dedicated apply tool that dispatches
on it.

## 4. Data model

### 4.1 Storage layout

```
<vault.fossil>
└─ taolus/
   ├─ meta/                      ← seeded at init
   │  └─ taolu-authoring/
   │     ├─ SKILL.md
   │     └─ ACTION.md
   ├─ backend/
   │  └─ go-api-server/
   │     ├─ SKILL.md
   │     └─ ACTION.md
   ├─ frontend/
   │  └─ react-frontend/
   │     ├─ SKILL.md
   │     └─ ACTION.md
   └─ workflows/
      └─ git-release/
         ├─ SKILL.md
         └─ ACTION.md
```

Each taolu is a directory `taolus/<group>/<name>/` containing:

- **`SKILL.md`** — the skill document (unchanged format: markdown with YAML
  frontmatter `name`, `description`, `license`, `compatibility`, `metadata`).
- **`ACTION.md`** — the action specification (new, required on save).

**`SKILL.md` + `ACTION.md` are one taolu — a single unit.** They are saved
together, versioned together, read together, and diffed together. There is no
per-file versioning: a taolu version is the pair, and a check-in that changes
either file is a new version of the taolu.

Extra support files (templates, examples, assets) may live beside them later;
they are carried forward by commits and ignored by the taolu parsing.

### 4.2 `ACTION.md` — the action specification

YAML frontmatter followed by optional prose:

```yaml
---
mode: install          # required: apply | install | enforce
detail:                # optional map
  format: opencode     # install/enforce only: opencode | claude | agents
---
<optional prose: instructions the agent follows when applying>
```

**Modes and their semantics:**

| mode | what the agent does | server side |
| --- | --- | --- |
| `apply` | applies the skill once to the current project, then moves on | returns `SKILL.md` + `ACTION.md`; writes nothing |
| `install` | materializes the skill into the local repo | writes `SKILL.md` + `.taolu-version` pin into the format target |
| `enforce` | installs **and** locks the skill in for every future agent | install as above, plus append a compliance reference line to `AGENTS.md` |

An explicit `action` argument on `taolu_apply` overrides the stored mode for a
one-off run.

### 4.3 Validation

- `name` and `group` keep the current slug rule (`^[a-z0-9]+(-[a-z0-9]+)*$`,
  1–64 chars).
- `SKILL.md` frontmatter validates as today (`name` matches, `description`
  required 1–1024 chars).
- `ACTION.md` is **required** on save and its `mode` must be one of
  `apply`, `install`, `enforce`. When `mode` is `install`/`enforce`,
  `detail.format` (if given) must be `opencode`, `claude`, or `agents`.
- Existing skills saved before v1 have no `ACTION.md`; they default to
  `mode: install` (their current behavior). See Section 9.

## 5. Tool surface: `vault_*` → `taolu_*`

All tools are renamed to the new prefix; storage/path semantics are unchanged.
The `path` argument still defaults to `TAOLU_REPO` or `~/.taolu/vault.fossil`.

| Today | v1 | change |
| --- | --- | --- |
| `vault_init` | `taolu_init` | seeds `taolu-authoring` (skill + action) |
| `vault_info` | `taolu_info` | lists taolu modes |
| `vault_practice_save` | `taolu_save` | takes `skill` + `action` content |
| `vault_practice_get` | `taolu_get` | returns both files |
| `vault_practice_list` | `taolu_list` | shows action mode |
| `vault_practice_history` | `taolu_history` | versions span the whole taolu |
| `vault_practice_diff` | `taolu_diff` | diffs `SKILL.md` + `ACTION.md` together |
| `vault_practice_install` | `taolu_apply` | dispatches on the action |
| `vault_practice_export` | `taolu_export` | returns the whole taolu |

### 5.1 `taolu_apply` — the "use a taolu" entry point

Arguments: `name`, `version` (default latest), `target` (default cwd),
`format` (default from `ACTION.md`, else `opencode`), `action` (optional
override), `force`.

Flow:

1. Read the taolu at the requested version (`SKILL.md` + `ACTION.md`).
2. Resolve the effective mode: explicit `action` arg wins, else `ACTION.md`.
3. Dispatch:
   - **`apply`** — return `SKILL.md` and `ACTION.md` so the agent can perform
     the one-shot application. Nothing is written; no approval is needed.
   - **`install`** — write `SKILL.md` into `.opencode|.claude|.agents/skills/<name>/`
     and a `.taolu-version` pin. Requires explicit user approval; refuses to
     overwrite without `force`.
   - **`enforce`** — install as above, then append/replace the compliance
     reference line in `AGENTS.md`. Requires approval; idempotent (see 7.3).
4. Return a summary: what was done, where, and the pinned version.

### 5.2 Other tools

- `taolu_save(name, group, skill, action, version_label?, message?, user?, path?)`
  — validates both documents, commits both files in one check-in, tags the
  version. `skill` and `action` are both required.
- `taolu_get(name, version?, path?)` — returns `SKILL.md` and `ACTION.md`
  together, clearly delimited, so the agent sees the action it must follow.
- `taolu_list(query?, tag?, group?, path?)` — each row: `group/name`, action
  `mode`, latest version, description. Mode is filterable via `query` and shown
  so agents can pick taolus by what they will do.
- `taolu_history(name, path?)` — versions, oldest first, unchanged shape.
- `taolu_diff(name, version_b, version_a?, path?)` — unified diff of the whole
  taolu between two versions: `SKILL.md` and `ACTION.md` are diffed together
  and the output shows both, so the agent sees the skill and its action change
  as one.
- `taolu_export(name, version?, path?)` — raw content of the whole taolu
  (`SKILL.md` + `ACTION.md`).
- `taolu_init` / `taolu_info` — as today, with the authoring seed and mode-aware
  summaries.

## 6. Versioning: the taolu is one unit

Version semantics are unchanged (immutable check-ins, `vN` labels, UUID
prefixes, `tip`), but a **taolu version** is the `SKILL.md` + `ACTION.md` pair.

- **Save** commits `SKILL.md` and `ACTION.md` in a single check-in and applies
  one tag (`<name>-<label>`); the pair is always changed together, never one
  without the other.
- **History** (currently `SkillHistory` keys off one file's blob change) must
  record a version when **either** `SKILL.md` or `ACTION.md` blob changes — the
  pair changes as a unit. The timeline scan keeps the newest → oldest order and
  dedupes unchanged files; we extend the "did this file change" test to the
  two-file set.
- **Resolve/read** operate on the same check-in UUID for both files, so the
  skill and its action are always read at the same version.
- **Diff** is for the whole taolu: `SKILL.md` and `ACTION.md` diffs are
  rendered together so the agent reviews the taolu, not a single file.

## 7. Install & enforce details

### 7.1 Format targets (unchanged)

| format | target directory |
| --- | --- |
| `opencode` | `.opencode/skills/<name>/` |
| `claude` | `.claude/skills/<name>/` |
| `agents` | `.agents/skills/<name>/` |

### 7.2 Version pin: `.taolu-version`

Renamed from `.vault-version`. One line, unchanged format:
`<vault-path> <version-label|uuid>`. Written beside `SKILL.md` on every
install/enforce; supports the explicit upgrade/rollback flow.

### 7.3 `AGENTS.md` compliance reference (enforce)

`taolu_apply` in `enforce` mode appends a single idempotent reference line to
the target project's `AGENTS.md`:

```text
- Follow the taolu <name> (v<label>) in .opencode/skills/<name>/SKILL.md.
```

- **Idempotent:** before appending, scan `AGENTS.md` for an existing reference
  to this taolu; replace it with the updated line rather than duplicating.
- **Upgrade/rollback:** re-applying `enforce` at a new (or old) version updates
  the line, keeping pin and reference in sync.
- **Revoke:** removing the line (or uninstalling the skill) opts the project
  out; the plan of record is the `.taolu-version` pin, the reference line only
  makes compliance load automatically for every agent.
- **Scope:** only the reference line is touched; `AGENTS.md` is otherwise
  never edited by the server.

## 8. Authoring guide: `taolu-authoring`

`practice-authoring` becomes `taolu-authoring` (under `meta`), itself a taolu
with `mode: apply`. It keeps the existing content (what the vault is, how to
survey a project, how to write `SKILL.md`, save/upgrade/rollback flows, quality
checklist) and adds:

- What an **action** is and the three modes with when to pick each
  (`apply` for one-off, `install` for a reusable practice, `enforce` for a
  project-wide convention every agent must follow).
- How to write `ACTION.md` (mode, optional `detail.format`, prose).
- How to **apply** a taolu via `taolu_apply` and what each mode does on the
  server and for the agent.

Because the guide is itself a taolu, the vault stays self-documenting and
bootstrapping: an agent can query it, apply it, and author more taolus without
code changes.

## 9. Migration & back-compat

v1 is a breaking evolution. Plan for it:

- **Storage root** `practices/` → `taolus/`. New vaults use `taolus/` from the
  start; existing vaults are migrated by a one-time `taolu_init` upgrade step
  that commits a rename of the skill tree (Fossil handles this as normal
  check-ins; history is preserved).
- **Legacy skills without `ACTION.md`** default to `mode: install` (their exact
  pre-v1 behavior), so nothing silently changes meaning.
- **Pins** `.vault-version` are read for compatibility and written as
  `.taolu-version` going forward.
- **Tool names** are a client-side breaking change; the README config examples
  and any client registrations must be updated in the same change.

## 10. Security & permissions

- Taolu content is **data**: skill and action are stored verbatim and never
  executed server-side. Actions are instructions the agent explicitly chose to
  follow.
- `taolu_apply` in `install`/`enforce` modes writes into the project and
  **always requires explicit user approval**; target paths are validated as
  today (no escape from the base directory) and existing files are not
  overwritten without `force`.
- `AGENTS.md` edits are limited to the single idempotent reference line.
- Vault paths are resolved to real paths before open/commit (unchanged).

## 11. Implementation phases

| Phase | Scope | Deliverables / acceptance |
| --- | --- | --- |
| **P0 — data model + rename** | storage root `taolus/`, `ACTION.md`, tool rename | `taolu_save` validates and commits `SKILL.md` + `ACTION.md` together; `taolu_get/list/history/diff/export` work across the taolu as one unit; `vault_*`/`practice` names removed from the surface. *Accept:* save → get → history round-trip with two files changed together; invalid `mode` rejected; list shows modes. |
| **P1 — apply / install / enforce** | `taolu_apply`, pin rename, AGENTS.md reference | dispatch on `mode`; install/enforce write `SKILL.md` + `.taolu-version`; enforce appends/replaces the reference line idempotently. *Accept:* apply returns content without writing; install/enforce approve → write → pin; enforce line is single and updates on re-apply; revoke works. |
| **P2 — authoring + UX** | `taolu-authoring` rewrite, search polish, upgrade flow | guide covers actions and modes; `taolu_list` filter by mode; upgrade = read pin → diff → re-apply. *Accept:* end-to-end author → save → new-project apply (all three modes) → upgrade → rollback in a demo. |
| **P3 — team sharing** | sync (future, unchanged) | vault clone/sync via libfossil HTTP transport. *Accept:* two machines sync the same vault. |

## 12. Decisions (resolved / proposed)

1. **Concept name** — `practice` is renamed to **taolu** = skill + action. ✅
2. **Tool surface** — all `vault_practice_*` tools become `taolu_*`, plus a new
   `taolu_apply`. Breaking change, adopted for consistency. ✅
3. **Action storage** — a dedicated **`ACTION.md`** file beside `SKILL.md`,
   versioned together; keeps the skill file pristine and makes actions a
   first-class artifact. ✅
4. **Action modes** — `apply`, `install`, `enforce` (install + AGENTS.md
   reference), with an optional `action` override on `taolu_apply`. ✅
5. **Enforce representation** — a single idempotent **reference line** in
   `AGENTS.md`, never a copied section, so upgrades/rollbacks stay in sync. ✅
6. **Versioning** — the taolu is one unit: `SKILL.md` and `ACTION.md` change
   together, version together, and diff together; a check-in that changes
   either file is a new version of the taolu. ✅
7. **Legacy** — skills without `ACTION.md` default to `install`; storage root
   migrates `practices/` → `taolus/` via `taolu_init`. Proposed.
8. **Seed** — `practice-authoring` becomes `taolu-authoring` under `meta`, with
   `mode: apply`. Proposed.
9. **Pin rename** — `.vault-version` → `.taolu-version`; legacy pins still read. Proposed.

## 13. Verification

- `go build ./... && go vet ./...` must pass.
- Scripted JSON-RPC clients (initialize → tools/list → tools/call) cover:
  init (incl. migration of a legacy vault), save (both files as one taolu, bad
  `mode` rejected), get, list (mode shown), history, diff (skill + action
  together), apply in all three modes, pin contents, AGENTS.md idempotency
  (append + update + revoke), export, and error paths (unknown format, escape
  attempts, missing approval).
- A scratch project is installed to and uninstalled from to prove enforce mode
  leaves a single, correct reference line.
