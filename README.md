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

The binary is an MCP server that speaks newline-delimited JSON-RPC over stdio.

## Configuration

The vault defaults to `~/.taolu/vault.fossil`. Override it:

- Per call: pass the `path` argument to any tool.
- Globally: set `TAOLU_REPO=/path/to/vault.fossil`.

Run `taolu_init` once to create the vault; it also seeds the built-in
`taolu-authoring` guide under the `meta` group, which teaches agents how to
summarize projects and author new taolus. Opening a pre-v1 vault migrates any
legacy `practices/` tree to `taolus/` automatically.

## Registering with an MCP client

### opencode (`opencode.json`)

```json
{
  "mcp": {
    "taolu": {
      "type": "local",
      "command": ["go", "run", "./cmd/taolu"],
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
      "args": ["run", "./cmd/taolu"]
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
| `taolu_get` | Read a taolu's `SKILL.md` + `ACTION.md` at a version | `name` | `version`, `path` |
| `taolu_list` | List/filter taolus (shows action mode) | — | `query`, `tag`, `group`, `include`, `path` |
| `taolu_history` | List a taolu's versions (oldest first) | `name` | `path` |
| `taolu_diff` | Unified diff between two versions (skill + action together) | `name`, `version_b` | `version_a`, `path` |
| `taolu_apply` | Apply a taolu per its action: apply / install / enforce | `name` | `version`, `target`, `format`, `action`, `force`, `path` |
| `taolu_export` | Export raw taolu content | `name` | `version`, `path` |

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
- End-to-end behavior is verified with scripted JSON-RPC clients speaking the
  stdio protocol (initialize → tools/list → tools/call) covering init,
  migration, save, list, get, history, diff, apply (all three modes + pins +
  AGENTS.md idempotency), export, and error paths.

Note: `go mod tidy` cannot fully complete because an upstream dependency of go-libfossil references an invalid pseudo-version (`github.com/ncruces/go-sqlite3-wasm`). `go mod tidy -e` updates the module file correctly, and the build is unaffected.

## License

MIT
