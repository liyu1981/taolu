# Agent Vault — Versioned Practice Library MCP Server

## 1. The idea (original)

Create an MCP server for agents able to:

1. Summarize a project's practices and save them to a Fossil DB with versions.
2. Provide, from the Fossil DB, a practice by name to the current project.

**Use cases**

- **Case 1 (push):** a project has a specific tech stack. From the project we ask to summarize the architecture conventions with a proper name (in skill format).
- **Case 2 (pull):** when starting a new project we ask the agent to query our past agent-vault for practices, so it installs local skills with the ability to specify versions.

---

## 2. What we are building

A **vault MCP server**. Fossil SCM is the *underlying database and technology* — it is **not exposed** to agents. The entire public MCP surface is the `vault_*` tool set below.

- A **skill** is a named, versioned skill document stored inside a Fossil repository ("the vault"), grouped under a required **practice** (a domain folder).
- Each save = a commit = a new immutable version, identified by UUID **and** a semantic label (`v1`, `v2`, …).
- Skills are **installed into a project as real skill files** (opencode / Claude / agents format) with an explicit version pin.
- The vault ships with a built-in **practice-authoring guide** (itself a skill under the `meta` practice), so the vault is self-documenting and bootstrapping: an agent can query for it, install it, and follow it to author more skills.

Division of labor:

- **The agent (LLM)** does the *analysis* (reads the project, drafts the skill) using its normal file/search tools, guided by the authoring guide.
- **The MCP server** does the *storage, retrieval, history, diff, tagging, and installation* of skills.
- This keeps the server simple, client-agnostic, and reusable from any MCP host.

---

## 3. Core concepts

| Concept | Definition |
| --- | --- |
| **Vault** | A Fossil repository (`.fossil` file) that stores skills. Default `~/.agent-vault/vault.fossil`; overridable per call. |
| **Practice** | A **required** grouping/domain folder in the vault (e.g. `backend`, `frontend`, `workflows`, `meta`). Organizational only — never appears in installed skill names. |
| **Skill** | A named, versioned skill document: markdown with YAML frontmatter, shaped like a `SKILL.md`. Lives at `practices/<practice>/<name>/SKILL.md`. |
| **Name (slug)** | The unique id of a skill = the skill name. Globally unique in the vault. Also the install target name. Must be a valid skill slug. |
| **Version** | An immutable snapshot of a skill, identified by its check-in UUID prefix **or** a semantic label (`v1`, `v2`, …). `tip` = newest. |
| **Meta-practice / authoring guide** | The built-in skill `practice-authoring`, seeded under the `meta` practice at `vault_init`. Versioned and installable like any skill. |
| **Install target** | A directory in the current project (e.g. `.opencode/skills/`) where the skill is materialized as `SKILL.md`. |
| **Version pin** | A record of which vault version a project has installed, so upgrades are explicit. |

---

## 4. Storage & data model

### 4.1 Vault layout

```
<vault.fossil>
└─ practices/
   ├─ meta/                      ← seeded at init
   │  └─ practice-authoring/
   │     └─ SKILL.md
   ├─ backend/
   │  ├─ go-api-server/
   │  │  └─ SKILL.md
   │  └─ python-fastapi/
   │     └─ SKILL.md
   ├─ frontend/
   │  └─ react-frontend/
   │     └─ SKILL.md
   └─ workflows/
      ├─ git-release/
      │  └─ SKILL.md
      └─ pr-review/
         └─ SKILL.md
```

- Each skill is a directory: `practices/<practice>/<name>/SKILL.md`. The directory can later hold support files (templates, examples, assets) alongside `SKILL.md`.
- `practice` is a required folder slug; `name` is the skill name and is **globally unique** in the vault.
- Committing only the changed file with `CommitOpts{ParentID: tip}` gives full-tree Fossil semantics automatically (untouched files are carried forward — verified behavior of the current implementation).

### 4.2 Skill file format

The content is a `SKILL.md`-compatible document, so it installs as-is:

```markdown
---
name: go-api-server
description: Conventions for building Go HTTP API services: layout, error handling, logging, migrations.
license: MIT
compatibility: opencode
metadata:
  tags: "go,http,rest,layered"
  source: "github.com/acme/payments"
  stack: "go"
---

## Purpose
...

## Architecture conventions
...
```

- `name` (required): must equal the skill slug / file name.
- `description` (required, 1–1024 chars): what the agent uses to pick the skill.
- `metadata.*` (optional, string map): free-form search keys (`tags`, `source`, `stack`, ...). OpenCode ignores unknown keys, so extra vault fields are safe.
- Body: the actual skill/instructions.

### 4.3 Naming rules

- `name` (the skill name) is validated as: `^[a-z0-9]+(-[a-z0-9]+)*$`, length 1–64. This is exactly the rule opencode enforces for skill directory names, so an installed skill loads without friction.
- `practice` follows the same slug rule and is **required** on save.
- Full vault path: `practices/<practice>/<name>/SKILL.md` (a directory per skill, leaving room for support files). Category never appears in the installed skill name — skills are flat on disk, so install always targets `.opencode/skills/<name>/SKILL.md`.

### 4.4 The built-in authoring guide (`practice-authoring`)

Seeded into every vault at `vault_init` under the `meta` practice as the initial commit. It is a normal, versioned skill and installs via `vault_practice_install` like any other. Content outline:

- **What the vault is** — practices, skills, versions, install targets, pins.
- **How to summarize a project into a skill**
  - Survey the project: read README/AGENTS/docs, layout, configs, tooling.
  - Extract conventions: architecture & layering, error handling, logging, testing, project structure, naming, config/secrets, CI.
  - Write the `SKILL.md` draft (frontmatter rules, slug rules, good-description guidance).
- **How to save / update** — `vault_practice_save`, message conventions, version labels.
- **How to install / upgrade / roll back** — `vault_practice_install`, version pin, diff-before-upgrade.
- **Quality checklist** — a skill should be specific, actionable, small, and tagged so it can be found.

Because the guide is itself a skill, the vault can be taught to author new skills without any code changes.

---

## 5. Versioning model

- **Primary id:** check-in UUID. Short prefix (12 hex chars, e.g. `a1b2c3d4e5f6`) is the human-facing version string.
- **Semantic labels (from day one):** every save is tagged with a label. Default labels auto-increment per skill (`v1`, `v2`, …); the caller may pass an explicit label. Labels are stored as Fossil tags named `<name>-<label>` (repo tags are global, so labels are namespaced per skill).
- **Latest:** `tip` resolves to the newest check-in.
- **Per-skill history:** derived by scanning the vault timeline newest→oldest and keeping check-ins where the skill file's `blob.uuid` changed (a check-in that only touched other files is skipped). This also yields the label sequence (`v1` = oldest, `vN` = newest).
- **Resolution:** a `version` argument accepts either a UUID prefix (resolved via the blob table) or a label like `v2` (resolved via the history sequence / tagxref).
- **Rollback:** read/install any past version via its UUID prefix or label; content is immutable and content-addressed.

---

## 6. MCP tool set

**Public surface = `vault_*` tools only.** The `fossil_*` tools and the demo tools (`hello`, `echo`, `add`, `current_time`) are retired from the MCP registration; Fossil calls move to internal helpers. `libfossil` remains the engine — it is just no longer a public API.

All `vault_*` tools take an optional `path` (vault repo; default from config). Optional args use `omitempty` (the SDK marks non-`omitempty` fields as required).

| Tool | Purpose | Required | Optional |
| --- | --- | --- | --- |
| `vault_init` | Create/ensure the vault repo; seed `practice-authoring` if missing. Returns project-code + skill count. | — | `path`, `user` |
| `vault_practice_save` | Validate slug + frontmatter, commit the skill, tag the new version. Returns version label + UUID + total version count. | `name`, `practice`, `content` | `path`, `version_label`, `message`, `user` |
| `vault_practice_get` | Return skill content for a version. | `name` | `path`, `version` (label or UUID prefix; default latest) |
| `vault_practice_list` | List skills, optional filter by query/tag/practice. Returns name, practice, description, latest version, tags, source. | — | `path`, `query`, `tag`, `practice` |
| `vault_practice_history` | List versions of a skill (label, uuid, date, user, message). | `name` | `path` |
| `vault_practice_diff` | Unified diff of a skill between two versions. | `name`, `version_b` | `path`, `version_a` (default previous version) |
| `vault_practice_install` | Write the skill as `SKILL.md` into the target project dir with a version pin. | `name` | `path`, `version` (default latest), `target` (default cwd), `format` (`opencode`\|`claude`\|`agents`, default `opencode`), `force` |
| `vault_practice_export` | Return raw markdown of a version (review/copy path). | `name` | `path`, `version` |

Lookup tools (`get`/`history`/`diff`/`install`/`export`) are keyed by `name` only, because names are globally unique; `practice` is only needed when saving or filtering.

### Install locations per format

| format | target default | file written | pin file |
| --- | --- | --- | --- |
| `opencode` | `.opencode/skills/` | `.opencode/skills/<name>/SKILL.md` | `.opencode/skills/<name>/.vault-version` |
| `claude` | `.claude/skills/` | `.claude/skills/<name>/SKILL.md` | `.claude/skills/<name>/.vault-version` |
| `agents` | `.agents/skills/` | `.agents/skills/<name>/SKILL.md` | `.agents/skills/<name>/.vault-version` |

`.vault-version` is a one-line file: `<vault-path> <version-label|uuid>`. It enables an explicit upgrade flow (Phase 3).

---

## 7. Workflows

### 7.1 Case 1 — summarize & save (push)

1. User: *"Summarize this project's conventions and save as skill `go-api-server`."*
2. Agent loads the authoring guide if not already present:
   - `vault_practice_get(name="practice-authoring")` (or `vault_practice_install` once into the project).
3. Agent analyzes the repo with its existing tools (read/grep/glob) and drafts a `SKILL.md` following the guide (frontmatter + body).
4. Agent calls `vault_practice_save(name="go-api-server", practice="backend", content=..., message="...")`.
5. Server validates slug + frontmatter, commits `practices/backend/go-api-server.md`, auto-tags the version (`v1`, `v2`, …), returns label + UUID.
6. Subsequent saves create new versions; nothing is ever lost.

### 7.2 Case 2 — find & install (pull)

1. User (in a brand-new project): *"Install our Go API server practices."*
2. Agent calls `vault_practice_list(query="go api")` → finds `go-api-server` @ `v3` / `a1b2c3d4e5f6`.
3. Optionally `vault_practice_get` to review, or `vault_practice_history`/`diff` to compare versions.
4. Agent calls `vault_practice_install(name="go-api-server", version="v3")`.
5. Server writes `.opencode/skills/go-api-server/SKILL.md` + `.vault-version` pin.
6. The project now has the skill available natively in opencode; the pin records provenance.
7. If the project has no authoring skill yet, the agent also installs `practice-authoring` so future skills can be authored locally.

### 7.3 Upgrade / rollback

- Check the pin: read `.vault-version`.
- `vault_practice_history` + `vault_practice_diff` to see what changed.
- `vault_practice_install(name, version=<new>)` to re-install and update the pin; `version=<old>` (or a UUID prefix) to roll back.

### 7.4 Team sharing (Phase 4)

`go-libfossil` already ships clone + HTTP sync with pluggable transports. A team can host a vault on a Fossil server and sync between machines — skills (including the authoring guide) become shareable across a team with full history, for free.

---

## 8. Architecture & project layout (Go)

- Reuse: `libfossil.Open/Create/Commit/Timeline/ReadFileAt/ResolveVersion/Diff/ListFiles/Tag`, `withRepo`, `shortUUID`, `textResult` — all moved behind internal helpers, **no longer registered as MCP tools**.
- Files:
  - `vault.go` — `registerVaultTools(server)`, tool handlers, `vaultRepo()` (resolve path from arg/env/default).
  - `practice.go` — slug validation, frontmatter parse/validate, practice-path handling (`practices/<practice>/<name>/SKILL.md`), history scan + label sequencing, version resolution (label or UUID), tagging, diff, install writer, pin read/write.
  - `vaultseed.go` — the embedded `practice-authoring` markdown + seed logic used by `vault_init`.
  - `server.go` — registers only vault tools (drop `registerTools` and `registerFossilTools`).
- Config:
  - Default vault path `~/.agent-vault/vault.fossil`; overridable via env `AGENT_VAULT_REPO` or per-call `path`.
  - `user` defaults to `admin`.
- Removal of `tools.go` (demo tools) and `fossil.go` (fossil tools) from the public surface; migrate reusable helpers into the new files.

---

## 9. Security & permissions

- `vault_practice_install` writes files into the project — it **always requires explicit user approval** (opencode `permission` rule `vault-practice-install: "ask"`). Target paths are validated (no absolute paths outside an allowed root unless `force`).
- Practices/skills are treated as untrusted instructions — they are *data*, executed only by the same agent that explicitly chose to install them. Document this in the README.
- Vault repo paths are resolved to real paths before open/commit (no traversal).
- The authoring guide is trusted content that ships with the server; user-authored skills are not trusted by default.

---

## 10. Implementation phases

| Phase | Scope | Deliverables / acceptance |
| --- | --- | --- |
| **P0 — plumbing** | Vault lifecycle + seed + surface cleanup | `vault_init` (create or open), vault path resolution from env/arg, slug + frontmatter validation, embed + seed `practice-authoring` under `meta/`, remove `fossil_*`/demo tools from registration. *Accept:* init creates a repo containing the authoring guide; bad slugs rejected; `tools/list` shows only `vault_*`. |
| **P1 — CRUD + history + tags** | Read/write core | `vault_practice_save`, `_get`, `_list`, `_history`, `_diff`; label auto-sequencing (`v1`, `v2`, …), explicit `version_label`, label/UUID resolution. *Accept:* save→get→history round-trip; labels increment correctly; diff between two versions correct; list filters by query/tag/practice. |
| **P2 — install** | Materialize into projects | `vault_practice_install` + `.vault-version` pin + format targets. *Accept:* install into a scratch project produces a loadable `SKILL.md` (verify with opencode), pin file written; re-install updates pin. |
| **P3 — UX polish** | Upgrade flow + search | Upgrade helper (read pin → diff → re-install), `vault_practice_export`, tag search improvements. *Accept:* end-to-end push → new-project pull → upgrade in a demo. |
| **P4 — team sharing** | Sync | Vault clone/sync via libfossil HTTP transport. *Accept:* two machines sync the same vault. |

---

## 11. Decisions (resolved)

1. **Vault location** — global `~/.agent-vault/vault.fossil`, overridable per call / `AGENT_VAULT_REPO`. ✅
2. **Grouping model** — the required grouping is called **practice** (`practices/<practice>/<name>/SKILL.md`, a directory per skill for future support files); the document's **name** is the skill name. Skill names are globally unique; lookups are keyed by name only. ✅
3. **Authoring guide placement** — seeded under the `meta` practice group. ✅
4. **Install approval** — `vault_practice_install` always asks for explicit user approval. ✅
5. **Version identity** — UUID prefixes **and** semantic labels from day one (auto `v1`, `v2`, … per skill; explicit `version_label` supported). ✅
6. **Authoring guide scope** — one practice: `practice-authoring`. ✅
7. **Update distribution** — authoring guide improves via normal vault commits; server re-seeds only when the vault lacks it. ✅
8. **Install format default** — `opencode`, with `format` arg for `claude`/`agents`. ✅
9. **Content ownership** — skills stored verbatim as data, never executed server-side; README warning included. ✅
