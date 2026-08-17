# Taolu Web Interface — Browse the Vault in a Browser (Design Note)

Design note for a browser-based UI that sits next to the MCP server and lets a
human browse the taolu vault: system status/setup, the taolus in the Fossil DB,
and each taolu's versions, file content, and diffs. It builds on the existing
MCP server (`cmd/taolu/main.go`), the vault data layer (`pkg/vault`), and the
HTTP transport introduced for the MCP server.

---

## 1. Problem

The taolu vault is currently only reachable through MCP tools. That is great for
agents but poor for humans:

- **No at-a-glance status.** Is the server up? Which vault is it using? How many
  taolus, groups, versions? A human must run `taolu_info` by hand.
- **No browsing.** Finding a taolu means recalling its name and calling
  `taolu_list` / `taolu_get` through an agent. There is no way to scan the whole
  library visually, grouped and searchable.
- **No version / diff UI.** Understanding *why* a taolu changed — comparing two
  versions, reading history — is text-only output in a terminal. Humans want a
  timeline, side-by-side content, and highlighted diffs.

Goal: a lightweight read-mostly web UI, served from the same binary as the MCP
server, that opens in a browser for inspection without touching the MCP surface.

## 2. Goals / non-goals

Goals:

- Serve a single-page app from the taolu Go binary, on a second HTTP port just
  above the MCP port (MCP `:8264` → UI `:8265`), so one process runs both.
- Status / setup view: show server identity + version, vault path, project-code,
  taolu count, groups, and the seeded `taolu-authoring` version.
- Browse view: list taolus (active + archived), grouped, filterable by
  query/tag/group/mode; click into a taolu.
- Taolu detail: latest content (SKILL.md / ACTION.md / files), version timeline,
  pick two versions and view a unified diff, and view raw file content at any
  version.
- Read-mostly. The first cut is read-only; mutating actions are out of scope (see
  non-goals).

Non-goals (first cut):

- **Writing / mutating from the UI** — no save, rename, archive, restore, or
  apply. Those stay agent/MCP-only for now (see Open Questions for a later phase).
- **AuthN / multi-user** — binds to `127.0.0.1` by default, same trust boundary as
  the MCP server.
- **Full-text search engine** — just substring filtering like `taolu_list`.
- **Live sync / websockets** — plain HTTP fetch; refresh to see new saves.
- **Running the UI as a separate deployment** — it ships embedded in the binary.

## 3. Architecture overview

```
taolu binary
├─ MCP HTTP server  :8264  (Streamable HTTP, existing)
├─ Web HTTP server  :8265  (new)  ── serves:
│     ├─ GET /api/status          → system + vault status (JSON)
│     ├─ GET /api/taolus          → list, filters, groups, mode
│     ├─ GET /api/taolus/{name}   → latest bundle (skill/action/files meta)
│     ├─ GET /api/taolus/{name}/history   → versions (label, uuid, date, user, msg)
│     ├─ GET /api/taolus/{name}/content?version=… → raw files at a version
│     ├─ GET /api/taolus/{name}/diff?a=…&b=…    → unified diff
│     └─ GET /                    → embedded SPA (index.html + assets)
└─ vault data layer: pkg/vault (reused as-is)
```

The two HTTP servers share one process and one vault repo. Each API call opens
the vault read-only (`vault.OpenVault`), performs the query with the existing
`pkg/vault` functions, and closes it — mirroring how the MCP tool handlers already
work. Because the vault is read-only from the UI, there is no write-lock
contention with the MCP server.

### 3.1 Port selection

MCP port defaults to `8264` (`TAOLU_PORT`). The UI port is **MCP port + 1**
(`8265` by default), overridable via `TAOLU_WEB_PORT`. If `TAOLU_WEB_PORT=0`, the
UI server is disabled entirely (stdio mode and headless runs). Both listen on
`TAOLU_HOST`.

## 4. Backend: HTTP handlers (Go)

New package `pkg/web` with a `Handler` that mounts the API and the embedded SPA.
It reuses `pkg/vault` directly — **no new data-access code is written** beyond
thin JSON marshalling, because every capability already exists:

| API | Backing vault function | Notes |
| --- | --- | --- |
| `GET /api/status` | `vault.EnsureVault`, `Repo.Config("project-code")`, `vault.ListTaolu`, `vault.UniqueGroups` | plus server name/version and the resolved vault path; also `taolu-authoring` latest version |
| `GET /api/taolus` | `vault.ListTaolu` + `vault.ListArchivedTaolu` | filters applied in handler (query/tag/group/mode/archived) |
| `GET /api/taolus/{name}` | `vault.ReadTaoluBundle` | returns skill/action raw + `files/` asset manifest + archived flag |
| `GET /api/taolus/{name}/history` | `vault.SkillHistory` | oldest-first list, labels + UUID + date + user + message |
| `GET /api/taolus/{name}/content` | `vault.ReadTaoluBundle` | `?version=` (label or UUID prefix, default tip); returns skill/action/assets **with content** |
| `GET /api/taolus/{name}/diff` | `vault.ResolveDiffVersions` + `Repo.Diff` | `?a=`/`?b=` (labels/UUID prefixes); `a` optional → previous version |

Return shapes are JSON structs defined in `pkg/web` (e.g. `Status`, `TaoluItem`,
`Version`, `DiffResult`). The diff endpoint reuses the exact file-scoped `Diff`
calls the `taolu_diff` MCP tool makes (`pkg/tools/tools.go:321-339`), so behavior
matches.

Error convention: `404` for unknown taolu/version, `400` for bad filter values,
`500` for vault open/read failures, with a `{ "error": "..." }` JSON body. The SPA
shows these inline.

### 4.1 Embedding the SPA

The frontend build output (`dist/`) is embedded into the binary with
`//go:embed`. `pkg/web` serves `index.html` at `/` and static assets under
`/assets/` with correct content types and a no-cache header for `index.html`
(so the SPA never serves a stale shell). API routes are served before the SPA
fallback.

Repo layout:

```
web/                       ← frontend source (see §5)
  src/…
  package.json, vite.config.ts, pnpm-lock.yaml
pkg/web/
  server.go                ← mux: /api/* + embedded SPA
  handlers.go              ← JSON handlers + request/response types
  embed.go                 ← //go:embed dist
  dist/                    ← frontend build output (generated, not committed)
cmd/taolu/main.go          ← start web server alongside MCP server
```

The `//go:embed` directive lives in `pkg/web/embed.go`, so `pkg/web/dist` must
exist for `go build ./...` — it is **not committed**. It is generated by the
frontend build (via pnpm) and required before compiling. The
Makefile/`serve-dev.sh` gains a `web` target that runs `pnpm install` +
`pnpm build` in `web/`, then `go build`.

## 5. Frontend: SPA (TypeScript + React + shadcn/ui + Tailwind + Vite + TanStack)

A standard Vite + React + TypeScript SPA in `web/`, using:
- **Vite** — dev server with `TAOLU_WEB_PORT` proxy to `/api`, and production
  build to `web/dist`.
- **pnpm** as the package manager (not npm); `pnpm-lock.yaml` committed.
- **React 18 + TypeScript** — strict mode, typed API client.
- **Tailwind CSS** — styling.
- **shadcn/ui** — accessible components (Table, Tabs, Select, Badge, Dialog,
  Button) built on Radix + Tailwind, so we get a polished look without hand
  rolling UI primitives.
- **TanStack Router** (routes: `/`, `/taolu`, `/taolu/:name`) and **TanStack
  Query** (server-state cache + loading/error states for the API calls).

The SPA is read-only: it fetches from the API above and renders. There is no
state mutation POST/PATCH in the first cut.

### 5.1 Feature map

**Status / setup view (`/`)** — cards:
- Server identity (`taolu`, version), bound host/port, UI port.
- Vault path (resolved), project-code.
- Counts: total taolus, archived count, distinct groups.
- `taolu-authoring` latest version (seed health).

**Browse view (`/taolu`)** — a filterable table/cards:
- Filters: text query (name/description/tags), group dropdown, action-mode
  chips (`apply`/`install`/`enforce`), archived toggle.
- Grouped listing; each row shows name, group, mode badge, latest version,
  description, tags, archived badge.
- Click → detail view.

**Detail view (`/taolu/:name`)** — tabs:
1. **Overview** — latest SKILL.md (raw text) + ACTION.md (raw text) + `files/`
   asset manifest (tree), archived warning banner.
2. **History** — timeline of versions (label, UUID, date, user, message), newest
   first; select any version.
3. **Content** — pick a version; show raw SKILL.md / ACTION.md / each asset
   (syntax-highlighted), side by side or stacked.
4. **Diff** — pick base (`a`) and target (`b`) versions from the history; render
   the per-file unified diff with `+`/`-`/context highlighted (SKILL.md /
   ACTION.md / assets).

Content is shown as **raw text**: SKILL.md / ACTION.md and assets are rendered
as plain, syntax-highlighted code (`highlight.js`/`shiki`), *not* as formatted
markdown. The diff view parses the unified diff lines returned by the API into
added/removed rows for colored rendering.

## 6. Workflows

**Status**: open `/` → GET `/api/status` → cards render; a stale server shows the
error state inline.

**Find & inspect a taolu**: open `/taolu` → filter → click row → Overview tab
shows latest content and assets.

**Compare versions**: open `/taolu/:name` → Diff tab → pick `a` (default previous)
and `b` → GET `/api/taolus/:name/diff?a=…&b=…` → highlighted diff rendered.

## 7. Security & permissions

- Binds to `127.0.0.1` by default (`TAOLU_HOST`), same as the MCP server; no
  new network exposure.
- **Read-only** — the API exposes no mutation endpoints, so a compromised UI
  cannot alter the vault. This is the key safety property of the first cut.
- Vault paths resolved via `vault.VaultPath` (env/arg), same trusted resolution
  as MCP; no user-supplied filesystem paths in the API.
- `index.html` served no-cache to avoid stale shells; assets long-cacheable.
- Taolu content is rendered as **data** (raw text / code), never executed. Code
  blocks are syntax-highlighted only, not evaluated.

## 8. Implementation phases

| Phase | Scope | Deliverables / acceptance |
| --- | --- | --- |
| **W0 — backend API + skeleton** | `pkg/web` server, JSON handlers for status/list/detail/history/content/diff, port wiring in `cmd/taolu/main.go`, SPA fallback stub | `curl /api/status`, `/api/taolus`, `/api/taolus/{name}`, `/history`, `/content`, `/diff` return correct JSON; MCP server unaffected |
| **W1 — frontend scaffold** | Vite + React + TS + Tailwind + shadcn/ui + TanStack in `web/`; API client; `/` status view; `/taolu` browse view | status cards render; browse filters and groups work against live API |
| **W2 — detail + versions + diff** | Detail view tabs (overview/history/content/diff); version pickers; per-file unified-diff rendering; raw text + syntax highlighting | open a taolu, read any version's content, diff two versions with colored output |
| **W3 — embed & polish** | `go:embed web/dist`, Makefile/`serve-dev.sh` web build target (pnpm), README docs, error/loading/empty states | `pnpm build` then `go build ./...` produces a binary that serves the UI on `:8265`; `./serve-dev.sh build` includes web |

## 9. Decisions (proposed)

1. **Separate port = MCP + 1**, not a path on the MCP port. Keeps the MCP HTTP
   handler untouched and avoids MCP request/response parsing ambiguity.
   Overridable via `TAOLU_WEB_PORT`; `0` disables. ✅
2. **Read-only first cut.** Mutations stay agent/MCP-only; the UI is a viewer.
   This keeps the security posture simple and the first milestone small. ✅
3. **Frontend stack** — Vite + React + TS + Tailwind + shadcn/ui + TanStack
   Router & Query, embedded via `go:embed`. Matches the user's requested stack. ✅
4. **Reuse `pkg/vault` directly**, no new data layer. Every API endpoint maps to
   an existing function; the diff endpoint mirrors `taolu_diff`'s exact Diff
   calls so behavior is consistent. ✅
5. **Asset build before `go build`** — `web/dist` is *not* committed; it is
   generated by the frontend build and required before compiling the binary.
   The build step uses **pnpm** (not npm). A `web` build target in
   `serve-dev.sh` / Makefile runs `pnpm install` + `pnpm build`, then `go build`.
   `//go:embed` requires `web/dist` to exist, so the build ordering is enforced
   (see §4.1 and W3). ✅
6. **UI port env naming** — `TAOLU_WEB_PORT` (parallel to `TAOLU_PORT`). ✅
7. **No mutating actions in the UI** — it is a pure viewer; save/rename/archive/
   restore/apply stay agent/MCP-only. W4 is dropped. ✅
8. **Diff granularity** — per-file unified diffs (SKILL.md / ACTION.md / assets),
   matching `taolu_diff`; no whole-taolu combined toggle. ✅
9. **Rendering depth** — raw text only. SKILL.md/ACTION.md and assets are shown
    as plain, syntax-highlighted text; no markdown rendering. ✅
 10. **Startup modes** — the default runs both the MCP server and the web UI;
    `--mcp-only` runs just the MCP server and `--web-only` runs just the web UI.
    `TAOLU_WEB_PORT`/`--mcp-only` disable the UI; `--web-only` skips the MCP
    server. ✅

## 10. Open questions (resolved)

All four open questions are now resolved (see Decisions 7, 5, 8, 9): no UI
mutations, pnpm-driven asset build before `go build`, per-file diffs, and raw
text rendering.
