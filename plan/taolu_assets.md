# Taolu Assets — Code Snippets and Multi-File Components (Design Note)

Design note for extending the taolu model so a taolu can carry code — from a
few inline snippets to a complete multi-file component (e.g. a React component
with `.tsx`, `.css`, `.test.tsx`, `.stories.tsx`). It builds on taolu v1
(`plan/taolu_v1.md`): the taolu remains `SKILL.md` + `ACTION.md` as one
versioned unit; this note adds a third, optional part to that unit.

---

## 1. Problem

Today a taolu is exactly two documents, `SKILL.md` + `ACTION.md`, saved and
versioned as one unit (`SaveTaolu`, `pkg/vault/save.go`). The only way to carry
code is to inline it as fenced blocks in the markdown bodies. That covers small
snippets, but not:

- **Complete components** — a React component spans several files with
  references between them (`import "./Button.css"`, tests, stories). Inlining
  them all into one markdown document mangles the layout and breaks tooling
  that expects real files.
- **Reusable templates/boilerplate** — a scaffolding taolu that wants to emit
  `Dockerfile`, `tsconfig.json`, `vitest.config.ts` etc. verbatim.
- **Round-tripping** — a taolu summarized from a project should be installable
  into a new project and yield the same tree, not a wall of fenced text.

The v1 design already foresaw this: "Extra support files (templates, examples,
assets) may live beside them later; they are carried forward by commits and
ignored by the taolu parsing" (`plan/taolu_v1.md:97`). The parser already
ignores non-`SKILL.md`/`ACTION.md` files in a taolu directory
(`parseSkillPath`, `pkg/vault/practice.go:165`), but **no tool can write them**:
`taolu_save` takes only `skill` + `action` strings and commits exactly two files
(`pkg/vault/save.go:42`). This note closes that gap.

## 2. Goals / non-goals

Goals:

- A taolu can carry a complete multi-file artifact (component, template tree,
  fixture set) alongside its skill and action.
- The artifact round-trips: save → get/export → apply/install reproduces the
  same files, versioned as one unit with `SKILL.md` + `ACTION.md`.
- Inline snippets keep working unchanged; small taolus are not forced to bundle
  files.
- Existing taolus without artifacts are untouched; all current tools keep
  working.

Non-goals:

- **Executing** anything server-side. Assets are data, stored and copied
  verbatim, never run (unchanged security posture).
- **Dependency resolution** between taolus, or patching/generating assets from
  variables inside the vault. Asset content is stored verbatim; the action prose
  tells the agent whether and how to adapt it.
- **Binary blob storage** in the first cut. Assets are text files; large
  binaries stay out of scope (see Section 8, open questions).

## 3. Design overview

Three tiers of "code in a taolu", lowest to highest fidelity:

| tier | carrier | round-trip |
| --- | --- | --- |
| 1. Inline snippet | fenced block in `SKILL.md`/`ACTION.md` body | works today, no change |
| 2. Single reusable file | one asset under `files/` | new — save/get/install handles it |
| 3. Multi-file component | asset tree under `files/` | new — install writes the whole tree |

Tier 1 is unchanged. Tiers 2–3 are the same mechanism: a **`files/` bundle**
inside the taolu directory. Each asset is a text file addressed by a validated
relative path.

### 3.1 Storage layout

```
<vault.fossil>
└─ taolus/
   ├─ meta/
   │  └─ taolu-authoring/
   │     ├─ SKILL.md
   │     └─ ACTION.md
   └─ frontend/
      └─ button/
         ├─ SKILL.md          ← describes the component + how to use it
         ├─ ACTION.md         ← mode: install
         └─ files/            ← the component tree (optional)
            ├─ Button.tsx
            ├─ Button.css
            ├─ Button.test.tsx
            └─ Button.stories.tsx
```

All files under `files/` are relative to that directory; subdirectories are
allowed. `files/` is a reserved name: the two canonical documents live at the
taolu root, every asset lives under `files/`, and nothing else may be placed in
the taolu directory.

### 3.2 Why a reserved `files/` subdirectory (and not arbitrary side-by-side files)

- **Unambiguous parsing.** `parseSkillPath` keeps working; anything under
  `files/` is an asset, anything else that isn't `SKILL.md`/`ACTION.md` is
  rejected on save rather than silently carried. No guessing about which stray
  file belongs to the taolu.
- **Safe install.** Install copies `files/*` into the target skill directory
  relative to `SKILL.md`, preserving the tree. A reserved root makes the copy
  rule a one-liner and removes collision risk with the two markdown documents.
- **Clean validation.** Paths are validated against a known prefix and reserved
  names once, at save.

## 4. Data model

### 4.1 The taolu unit

A taolu version is now the **file set** of its directory:

```
SKILL.md  +  ACTION.md  +  files/** (optional)
```

These change, version, and diff together. A check-in that changes any one of
them is a new version of the taolu. `SkillHistory` today keys off one file's
blob change (`pkg/vault/store.go`); it must be extended to the whole file set of
the directory (newest → oldest scan, "did this file change" test over
`SKILL.md`, `ACTION.md`, and every `files/**` entry).

### 4.2 Asset path rules (validated at save)

For each asset path `p`:

- `p` is relative, non-empty, and resolves under `files/` — no leading `/`, no
  `..`, no absolute paths (`safeJoin` already enforces the no-escape rule at
  install; the same rule applies at save).
- `p` must not collide with reserved names: `SKILL.md`, `ACTION.md`,
  `.taolu-version`, `.archived`, `origin`, and `files` itself.
- Paths are case-sensitive and must be unique within a version (no
  `Button.tsx` and `Button.TSX` in one save).
- Content is text; a per-file or total size cap is set (see Section 8).

## 5. Tool surface

### 5.1 `taolu_save` — accept assets

New optional argument `files`: an array of `{path, content}` pairs, where
`path` is the asset's path **relative to `files/`** (e.g. `Button.tsx`,
`components/Button.tsx`). Example:

```json
{
  "name": "button",
  "group": "frontend",
  "skill": "...",
  "action": "...",
  "files": [
    { "path": "Button.tsx", "content": "..." },
    { "path": "Button.css", "content": "..." }
  ]
}
```

- Omitted or empty → behavior identical to today.
- `SaveTaolu` commits `SKILL.md`, `ACTION.md`, and every asset in **one**
  check-in and applies one tag (`<name>-<label>`), preserving the unit.
- Validation runs over the whole set before commit: slugs, both frontmatters,
  asset paths (Section 4.2), and a size cap.

### 5.2 `taolu_get` / `taolu_export` — read assets

- `taolu_get` returns `SKILL.md` + `ACTION.md` as today and appends an **asset
  manifest** when the taolu has assets: one line per file (`files/Button.tsx`),
  so the agent knows a bundle exists without dumping every byte.
- `taolu_export` returns the full bundle: the pair plus every asset with
  content, delimited (e.g. `## FILE files/Button.tsx` blocks) so the agent can
  reproduce the tree verbatim.
- Reading at a version reads all three parts at the same check-in UUID.

### 5.3 `taolu_apply` — materialize assets

Dispatch on mode, as today, extended:

- **`apply`** — returns `SKILL.md` + `ACTION.md` + the asset manifest (same as
  `taolu_get`); nothing is written. The agent reproduces the tree from the
  manifest, or uses `taolu_export` for full content.
- **`install`** — writes `SKILL.md` + `.taolu-version` pin as today, **and**
  copies every `files/**` asset into the target skill directory preserving
  relative paths. Refuses to overwrite without `force`, per-file.
- **`enforce`** — install as above, then the AGENTS.md reference line
  (unchanged; it points at `SKILL.md`, the whole directory is the skill).

Installed layout for tier 3:

```
.opencode/skills/button/
├─ SKILL.md
├─ .taolu-version
├─ Button.tsx
├─ Button.css
├─ Button.test.tsx
└─ Button.stories.tsx
```

`SKILL.md` references assets by their relative path (`see Button.tsx`); because
the tree is preserved, the references hold after install and for skill runtimes
that resolve relative paths.

## 6. Authoring flow (summarizing a practice to a taolu)

The `taolu-authoring` seed guide gains a section on carrying code:

1. Survey the component/artifact as part of step 3 of the existing flow.
2. Decide the tier: inline snippet for a few lines, `files/` bundle when the
   artifact is multi-file or meant to be installed verbatim.
3. For a bundle, collect each file's relative path and content; the agent
   reads them from the source project.
4. Write `SKILL.md` describing the component and how to use it, referencing
   assets by relative path; write `ACTION.md` with the confirmed mode.
5. Save with the `files` argument; get explicit approval before `taolu_save`
   / `taolu_apply` as today.
6. Install into a scratch project to prove the tree reproduces.

The guide is itself a taolu, so it can gain the assets of its own examples
once the mechanism exists.

## 7. Migration & back-compat

- Existing taolus have no `files/`; everything (save/get/apply/diff/history)
  behaves exactly as today.
- The `files` save argument is optional; old callers are unaffected.
- Rename/archive carry `files/` along with the directory (the origin marker and
  `.archived` marker live at the taolu root and already cover the whole
  directory).
- Diff: the file set is diffed together; asset diffs render alongside
  `SKILL.md`/`ACTION.md`.

## 8. Decisions (resolved during implementation)

1. **Size cap** — per-file **1 MiB** and a **total 8 MiB** cap per version
   (`maxAssetBytes` / `maxAssetTotalBytes` in `pkg/vault/practice.go`),
   validated in `ValidateAssets`.
2. **Asset removal** — the `files` argument to `taolu_save` is **authoritative**
   per version. `SaveTaolu` rewrites the tree (via `commitFullTree`, the same
   full-manifest mechanism `RestoreTaolu` uses): the taolu's previous content
   files are dropped and the new set committed in one check-in, so omitted
   assets disappear instead of being carried forward by Fossil's full-tree
   merge semantics. Markers (`.archived`, `origin`) and other taolus are
   preserved.
3. **Overwrite semantics on install** — whole-bundle all-or-nothing: a
   pre-existing SKILL.md or any asset blocks install without `force`; `force`
   overwrites the entire set.
4. **History** — a version is recorded when the taolu's **content file set**
   (SKILL.md, ACTION.md, files/**, compared by blob UUID) changes between
   consecutive check-ins; markers and stray files do not count. `SkillHistory`
   was rewritten to compare the per-check-in content set instead of a fixed
   file pair, preserving the existing dedup/rename semantics.
5. **Binary assets** — text-only for now. Base64 assets are future work.
6. **Template rendering** — verbatim; adaptation belongs in the action prose.

## 9. Open questions

- **Larger or binary assets.** The size caps are conservative; revisit with
  real component trees. Binary assets (logos, images) would need base64
  encoding or a separate storage path.
- **Diff rendering.** Asset diffs are not yet shown in `taolu_diff` output;
  history records them, but the diff tool only renders SKILL.md + ACTION.md.
  Extend `taolu_diff` to walk the file set.

## 10. Implementation phases

| Phase | Scope | Deliverables / acceptance |
| --- | --- | --- |
| **P0 — storage + save** | `files/` bundle, `files` arg on `taolu_save`, path validation, history over the file set | `taolu_save` with `files` commits SKILL.md + ACTION.md + assets in one check-in, one tag; invalid asset paths rejected; history records a version when any file changes. *Accept:* save → history round-trip with an asset tree. |
| **P1 — read + apply** | `taolu_get` manifest, `taolu_export` full bundle, `taolu_apply` materializes assets | apply returns the manifest; install/enforce write the whole tree preserving relative paths; export reproduces the bundle. *Accept:* save component → install into scratch project → tree matches source. |
| **P2 — authoring + UX** | `taolu-authoring` update, diff of assets, size caps | guide documents tier 1–3 and the `files` flow; diff shows asset changes. *Accept:* end-to-end summarize component → save → install in a demo. |

All three phases are implemented in this change.

## 11. Verification

- `go build ./... && go vet ./...` must pass.
- Scripted JSON-RPC clients cover: save with assets (one check-in, one tag),
  invalid paths rejected (`..`, reserved names, collisions), get (manifest
  present/absent), export (full bundle), apply in all three modes (assets
  materialized for install/enforce, manifest for apply), history/diff over the
  file set, and the no-assets path unchanged.
- A scratch project is installed to and the resulting tree diffed against the
  saved asset bundle to prove round-trip fidelity.
