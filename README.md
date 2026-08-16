# taolu

An MCP server for a **versioned practice library** ("the vault"). Agents can save skills that capture a project's conventions, then find and install them (pinned to a version) into any project.

Fossil SCM is the underlying storage engine via [go-libfossil](https://github.com/danmestas/go-libfossil) — pure-Go, no CGo, no `fossil` binary. It is **not exposed** to agents: the entire public MCP surface is the `vault_*` tools below.

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

Run `vault_init` once to create the vault; it also seeds the built-in
`practice-authoring` skill under the `meta` practice, which teaches agents how
to summarize projects and author new skills.

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
| `vault_init` | Create/open the vault and seed `practice-authoring` | — | `path`, `user` |
| `vault_info` | Show vault path, project-code, skills, practices | — | `path` |
| `vault_practice_save` | Save a skill as a new versioned check-in | `name`, `practice`, `content` | `version_label`, `message`, `user`, `path` |
| `vault_practice_get` | Read a skill's content at a version | `name` | `version`, `path` |
| `vault_practice_list` | List/filter skills | — | `query`, `tag`, `practice`, `path` |
| `vault_practice_history` | List a skill's versions (oldest first) | `name` | `path` |
| `vault_practice_diff` | Unified diff between two versions | `name`, `version_b` | `version_a`, `path` |
| `vault_practice_install` | Install a skill as `SKILL.md` with a version pin | `name` | `version`, `target`, `format`, `force`, `path` |
| `vault_practice_export` | Export raw skill content | `name` | `version`, `path` |

### Details

- **Skills** live at `practices/<practice>/<name>/SKILL.md` in the vault. `practice` (e.g. `backend`, `frontend`, `workflows`, `meta`) is an organizational folder; `name` is the skill slug, globally unique, and the install target name. Each skill is a directory so it can hold support files (templates, examples, assets) next to `SKILL.md` in the future.
- **Content** is a `SKILL.md` with YAML frontmatter (`name`, `description`, optional `license`/`compatibility`/`metadata`). Names must be lowercase alphanumeric with single hyphens.
- **Versions** are immutable. Each save is tagged `v1`, `v2`, … (or a custom `version_label`). The `version` argument accepts a label or a UUID prefix.
- **Install** writes `.opencode/skills/<name>/SKILL.md` (or `.claude/skills` / `.agents/skills` via `format`) plus a `.vault-version` pin recording the installed version. It **always requires explicit user approval**, and refuses to overwrite without `force`.

## Example flow

```text
vault_init
vault_practice_save   name=go-api-server practice=backend content=<SKILL.md>
vault_practice_save   name=go-api-server practice=backend content=<v2> message="add pkg/ layout"
vault_practice_list   query=go
vault_practice_history name=go-api-server
vault_practice_install name=go-api-server version=v2 target=./my-new-project
```

## Security

- Skills are **data**: agent-authored content stored verbatim and never executed server-side. Installed skills are instructions the agent explicitly chose to load.
- `vault_practice_install` writes into the project and always prompts for approval.

## Development

- `go build ./...` and `go vet ./...` must pass.
- End-to-end behavior is verified with scripted JSON-RPC clients speaking the stdio protocol (initialize → tools/list → tools/call) covering init, save, list, get, history, diff, install (all formats + pins), export, and error paths.

Note: `go mod tidy` cannot fully complete because an upstream dependency of go-libfossil references an invalid pseudo-version (`github.com/ncruces/go-sqlite3-wasm`). `go mod tidy -e` updates the module file correctly, and the build is unaffected.

## License

MIT
