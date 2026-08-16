# taolu

An MCP server for a **versioned taolu library** ("the vault"). A **taolu** is a
skill plus an action: a `SKILL.md` that captures a project's conventions paired
with an `ACTION.md` that tells the agent what to do with it (apply once, install,
or install and enforce). Agents can save taolus, then find and apply them
(pinned to a version) into any project.

Fossil SCM is the underlying storage engine via [go-libfossil](https://github.com/danmestas/go-libfossil) — pure-Go, no CGo, no `fossil` binary. It is **not exposed** to agents: the entire public MCP surface is the `taolu_*` tools below.

## Requirements

- Go 1.26+ (the server's `go.mod` declares `go 1.26.0`; `GOTOOLCHAIN=auto` will download it if needed)
- Any MCP-compatible client (opencode, Claude Desktop, Cursor, VS Code, etc.)

## Build

```sh
go build -o taolu ./cmd/taolu
```

## Running

By default the binary runs as a **shared HTTP MCP server** (Streamable HTTP),
so multiple opencode instances connect to one process — the vault is touched by
a single process and there is no SQLite lock contention.

```sh
./taolu                     # listens on http://127.0.0.1:8264
./taolu --stdio             # instead: single-process stdio mode
```

On startup the server **creates and seeds the default vault if it does not
exist** (creating `~/.taolu/vault.fossil`, seeding the `taolu-authoring` guide,
and migrating any legacy `practices/` tree), so it is usable immediately. The
`taolu_init` tool is only needed to initialize or re-inspect a non-default vault
path.

## Configuration

The vault defaults to `~/.taolu/vault.fossil`. Override it:

- Per call: pass the `path` argument to any tool.
- Globally: set `TAOLU_REPO=/path/to/vault.fossil`.

Server binding:

- `TAOLU_HOST` (default `127.0.0.1`) and `TAOLU_PORT` (default `8264`)
  configure the HTTP listen address.

Run `taolu_init` only for a non-default vault path. Opening a pre-v1 vault migrates any
legacy `practices/` tree to `taolus/` automatically.

## Registering with an MCP client

### opencode — shared HTTP server (recommended, multi-instance safe)

Start the server once, then point every opencode instance at it:

```json
{
  "mcp": {
    "taolu": {
      "type": "remote",
      "url": "http://127.0.0.1:8264",
      "enabled": true
    }
  }
}
```

### opencode — local stdio (single instance)

```json
{
  "mcp": {
    "taolu": {
      "type": "local",
      "command": ["go", "run", "./cmd/taolu", "--stdio"],
      "environment": {
        "TAOLU_REPO": "/home/you/.taolu/vault.fossil"
      }
    }
  }
}
```

### Claude Desktop (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "taolu": {
      "command": "go",
      "args": ["run", "./cmd/taolu", "--stdio"]
    }
  }
}
```

To avoid recompiling on every launch, build the binary once and point `command` at `./taolu`.

## Tools

All tools take an optional `path` (vault repo; defaults to `TAOLU_REPO` or `~/.taolu/vault.fossil`).

| Tool | Purpose | Required | Optional |
| --- | --- | --- | --- |
| `taolu_init` | Create/open the vault, migrate legacy `practices/`, seed `taolu-authoring` | — | `path`, `user` |
| `taolu_info` | Show vault path, project-code, taolus, groups | — | `path` |
| `taolu_save` | Save a taolu (`SKILL.md` + `ACTION.md`) as a new versioned check-in | `name`, `group`, `skill`, `action` | `version_label`, `message`, `user`, `path` |
| `taolu_get` | Read a taolu's `SKILL.md` + `ACTION.md` at a version; warns when the taolu is archived | `name` | `version`, `path` |
| `taolu_list` | List/filter active taolus (shows action mode); archived taolus are hidden | — | `query`, `tag`, `group`, `include`, `path` |
| `taolu_list_archived` | List/filter archived taolus, which are hidden from `taolu_list` and must not be used until restored | — | `query`, `tag`, `group`, `path` |
| `taolu_history` | List a taolu's versions (oldest first, continues across renames) | `name` | `path` |
| `taolu_diff` | Unified diff between two versions (skill + action together) | `name`, `version_b` | `version_a`, `path` |
| `taolu_apply` | Apply a taolu per its action: apply / install / enforce; refuses archived taolus | `name` | `version`, `target`, `format`, `action`, `force`, `path` |
| `taolu_export` | Export raw taolu content; warns when archived | `name` | `version`, `path` |
| `taolu_rename` | Rename a taolu (optionally into another group); rewrites the SKILL.md name and continues versioning at the next vN | `name`, `new_name` | `new_group`, `message`, `user`, `path` |
| `taolu_delete` | Archive a taolu (commits an `.archived` marker); source tree is kept, reversible via `taolu_restore` | `name` | `message`, `user`, `path` |
| `taolu_restore` | Restore an archived taolu back into normal listings and use | `name` | `message`, `user`, `path` |

### Details

- **Taolus** live at `taolus/<group>/<name>/` in the vault, as `SKILL.md` plus
  `ACTION.md`. `group` (e.g. `backend`, `frontend`, `workflows`, `meta`) is an
  organizational folder; `name` is the taolu slug, globally unique. Skill and
  action are **one unit**: they are saved, versioned, read, and diffed
  together.
- **`SKILL.md`** has YAML frontmatter (`name`, `description`, optional
  `license`/`compatibility`/`metadata`). **`ACTION.md`** has `mode`
  (`apply`, `install`, or `enforce`) plus optional `detail.format`
  (`opencode` | `claude` | `agents`). Names must be lowercase alphanumeric with
  single hyphens.
- **Versions** are immutable. Each save is tagged `v1`, `v2`, … (or a custom
  `version_label`). The `version` argument accepts a label or a UUID prefix.
- **Rename** moves `SKILL.md`, `ACTION.md`, and support files to the new path
  (optionally another group), rewrites the frontmatter `name`, and writes an
  `origin` marker so version history **continues** under the new name instead
  of restarting at `v1`. Older versions stay readable through the new name.
- **Archive** (`taolu_delete`) is not a source-tree delete: Fossil is an SCM, so
  nothing is ever erased. It commits an `.archived` marker into the taolu's
  directory, which hides the taolu from `taolu_list`, makes `taolu_get`/
  `taolu_export` warn, and makes `taolu_save`/`taolu_apply` refuse. Use
  `taolu_list_archived` to see archived taolus and `taolu_restore` to bring one
  back. The built-in `taolu-authoring` guide can be neither archived nor renamed.
- **Apply** dispatches on the action: `apply` returns the content for a one-shot
  use (nothing written); `install` writes `.opencode/skills/<name>/SKILL.md`
  (or `.claude/skills` / `.agents/skills` via `format`) plus a `.taolu-version`
  pin; `enforce` does the same and appends a single idempotent compliance
  reference to the project's `AGENTS.md`. Install and enforce **always require
  explicit user approval**, and refuse to overwrite without `force`.

## Example flow

```text
taolu_init
taolu_save   name=go-api-server group=backend skill=<SKILL.md> action=<ACTION.md>
taolu_save   name=go-api-server group=backend skill=<v2> action=<v2> message="add pkg/ layout"
taolu_list   query=go
taolu_history name=go-api-server
taolu_apply  name=go-api-server version=v2 target=./my-new-project
```

## Security

- Taolus are **data**: agent-authored content stored verbatim and never executed
  server-side. Actions are instructions the agent explicitly chose to follow.
- `taolu_apply` in `install`/`enforce` modes writes into the project and always
  prompts for approval; `AGENTS.md` is never touched beyond the single
  reference line.

## Development

- `go build ./...` and `go vet ./...` must pass.
- End-to-end behavior is verified with scripted JSON-RPC clients: over HTTP
  (initialize → tools/list → tools/call against the Streamable HTTP endpoint)
  and over stdio, covering init, migration, save, list, get, history, diff,
  apply (all three modes + pins + AGENTS.md idempotency), export, and error
  paths.

Note: `go mod tidy` cannot fully complete because an upstream dependency of go-libfossil references an invalid pseudo-version (`github.com/ncruces/go-sqlite3-wasm`). `go mod tidy -e` updates the module file correctly, and the build is unaffected.

## License

MIT
