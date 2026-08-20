# Plan: Taolu Fork

## Status: ✅ CONFIRMED — IMPLEMENTING (option A)

## Overview

Add a `taolu_fork` feature: clone a single taolu (its `SKILL.md`, `ACTION.md`,
and `files/` assets, plus its full version history) into a **new name** within
the same vault, while **keeping the original**. The fork records provenance —
who the original taolu is and from which version (commit) it forked — so the
relationship is knowable but the two taolus evolve independently afterwards.

This is deliberately **not** Fossil's internal fork concept (`DetectForks`,
`FindCommonAncestor`, branch divergence) and not the existing `origin` rename
marker. It is a git-style fork: a copy with a remembered upstream.

## Scope (confirmed with user)

- **Unit forked:** a single taolu (not the whole vault repo).
- **Origin tracking:** a `.fork` marker file committed in the forked taolu's
  directory (source ref + fork version UUID), plus optional vault config for
  the upstream identity.

## Why this matters / current behavior

Today a taolu is the unit of knowledge: one `SKILL.md` + `ACTION.md` + optional
`files/`, versioned `v1..vN`, all stored under `taolus/<domain>/<group>/<name>/`
in a single Fossil repo. There is no way to take an existing taolu, base a new
one on it under a different name, and remember that lineage. `rename` moves a
taolu and continues its history; it cannot leave a copy behind.

## Key design decision: `.fork` vs `origin`

The existing `origin` marker makes `SkillHistory` **merge** history across
renames (one taolu that moved). A fork is a copy that keeps both sides. For
option (A) — show the full copied lineage in `taolu_history` — `SkillHistory`
must **follow** the `.fork` marker exactly like it follows `origin`: it walks
back to the source's path, reconstructs the source's versions (v1..vN), then
the fork's own check-ins appear after them (vN+1…). The fork then versions
**independently** — its own saves append new versions on top of the copied
lineage, and the source taolu is untouched.

The difference from `origin` is semantic, not mechanical:
- `origin` means "this taolu WAS X and moved here" — saving overwrites X's path.
- `.fork` means "this taolu was COPIED from X" — X still exists; the fork just
  borrows the upstream history for display and then diverges on its own.

## Data model

New reserved marker name in `pkg/vault/practice.go`:

```go
// forkMarker records the source taolu and version a fork was created from,
// so provenance is knowable while the fork evolves independently.
forkMarker = ".fork"
```

`.fork` marker content (one line, whitespace-trimmed), JSON for forward
compatibility:

```json
{"source":"@local/backend/go-api-server","version":"v3","source_uuid":"<uuid>"}
```

`source` is the full taolu ref (`@domain/group/name`) of the original; `version`
is the vN label forked from; `source_uuid` is the check-in UUID of that version.

Vault config keys (via `repo.Config`/`SetConfig`, mirroring `user-domain`):

- `fork-upstream` — optional display identity of the upstream vault/repo
  (free-form, e.g. a path or URL). Set once when a vault is itself a fork.
- `fork-source-commit` — the upstream tip commit UUID at fork time (vault-level).

## New / changed files

### `pkg/vault/practice.go`
- Add `forkMarker` constant.
- Add `forkMarker` to `reservedAssetNames` so an asset cannot collide with it.

### `pkg/vault/fork.go` (new)
- `type ForkInfo struct { Source TaoluRef; Version string; SourceUUID string }`
- `ParseForkMarker(content string) (ForkInfo, error)`
- `ReadForkInfo(r *libfossil.Repo, ref TaoluRef) (*ForkInfo, error)`
- `ForkTaolu(r *libfossil.Repo, src TaoluRef, newName, newGroup string, message, user string) (ForkInfo, error)`
  - Resolve source ref (use `ParseTaoluRefWithConfig`), verify it exists and is
    not archived, resolve tip version + UUID.
  - Refuse if the target name already exists in the target domain/group.
  - Read the source's current bundle (`ReadTaoluBundleByRef`).
  - Rewrite SKILL.md frontmatter `name` to `newName` (reuse
    `renameSkillFrontmatter`).
  - Commit the full tree: carry all unrelated taolus forward, add the new
    taolu's `SKILL.md`/`ACTION.md`/`files/` at the new path, and add the `.fork`
    marker. Re-tag the copied upstream check-ins under the new name so
    `taolu_history <new_name>` shows the full copied lineage, then tag the
    fork's own next version (vN+1, where N is the copied history length).
- `VaultForkInfo(r *libfossil.Repo) (upstream, commit string, err error)` —
  reads the `fork-upstream` / `fork-source-commit` config keys.

### `pkg/vault/store.go`
- `SkillHistory` and `skillHistorySegment` follow the `.fork` marker in
  addition to `origin`, so a fork's `taolu_history` shows the copied upstream
  lineage followed by its own independent versions. Add `forkPathToSkill` (a
  3-layer-ref → SKILL.md-path helper) and read the `.fork` JSON in
  `skillHistorySegment`.

### `pkg/tools/tools.go`
- New MCP tool `taolu_fork`:
  - `name` (source taolu ref, required), `new_name` (required),
    `new_group` (optional; defaults to source group), `message`, `user`, `path`.
  - Returns source ref, fork version, new ref, and the new taolu's history.
- New MCP tool `taolu_fork_info` (or fold into `taolu_get`/`taolu_history`):
  - `name` (required), `path` — returns the fork provenance (source, version,
    source_uuid) when present, else "not a fork".
- Extend `taolu_list` `TaoluInfo` with an `IsFork` flag / fork source so
  listings can show provenance.

### `cmd/taolu/main.go` + `cmd/taolu/fork.go` (new)
- Add `taolu fork` CLI subcommand and a `taolu fork-info` subcommand, matching
  the existing `serve`/`init`/`migrate` command pattern.

### `pkg/web/handlers.go` (+ types)
- Surface fork provenance in `TaoluDetail` and `Status` (read-only web UI).

### `pkg/web/` frontend (optional, read-only)
- Show "forked from" in the taolu detail view if present.

### Tests
- `pkg/vault/fork_test.go`: fork copies content, records marker, keeps original
  intact, rejects collisions, refuses archived sources, frontmatter rewritten,
  fork history independent after the copy, `SkillHistory` unaffected.
- Tools-level verification per the README's scripted JSON-RPC client pattern.

## Behavior / semantics

1. `ForkTaolu` copies the source's current `SKILL.md`/`ACTION.md`/`files/`.
2. Frontmatter `name` is rewritten to `new_name` (source is untouched).
3. The upstream versions are re-tagged under the new name so
   `taolu_history <new_name>` shows the full copied lineage.
4. A `.fork` marker records `{source, version, source_uuid}`.
5. Future `taolu_save` on the fork bumps **its own** independent vN sequence
   continuing from the copied history (option A): copy had v1..v3 → fork's
   first independent save is v4; the copied versions v1..v3 remain shown in
   history as read-only provenance.
6. The source taolu is unchanged; both coexist.
7. `taolu_delete`/archive and `taolu_rename` work normally on either side and
   do not disturb the other (the `.fork` marker is a provenance note, not a
   hard link).

## Version-numbering decision — CONFIRMED: option (A)

- **(A) [chosen]** Fork history = copied upstream versions (v1..vN) then
  independent vN+1, vN+2… — richest provenance; `taolu_history` on the fork
  tells the whole story, matching the existing "history continues across
  renames" spirit. The fork's first independent save is vN+1.

## Example flow

```
taolu_save   name=go-api-server group=backend skill=... action=...   # v1..v3
taolu_fork   name=@local/backend/go-api-server new_name=go-api-fork
# -> @local/backend/go-api-fork  forked from go-api-server v3
taolu_history name=go-api-fork     # v1..v3 (copied), then v4+ independent
taolu_get    name=go-api-fork      # SKILL.md frontmatter name=go-api-fork
taolu_list                          # both entries; fork flagged
```

## Risks & mitigations

- **History merge confusion**: `.fork` is followed by `SkillHistory` for
  lineage display, but it must never be conflated with `origin` (which
  implies the taolu moved and saving overwrites the old path). A fork keeps
  the source alive and diverges. Mitigation: comment + test asserting fork
  history is source-lineage + independent saves, and the source is untouched.
- **Name collisions**: fork into an existing ref is refused (mirrors rename).
- **Archived source**: forking an archived taolu is refused (mirrors apply/save).
- **Marker drift**: `.fork` JSON kept minimal; missing/invalid marker treated as
  "not a fork" rather than an error, so old data never breaks reads.

## Success criteria

- Fork copies content + history under a new name, source untouched.
- Provenance (source ref + version + uuid) readable via a tool and the web UI.
- Fork versions independently afterwards.
- No regression: `go build ./...`, `go vet ./...`, full test suite pass; README
  and plan updated.
