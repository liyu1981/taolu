// Package commands implements taolu slash-command installation for agent tools
// (opencode, Claude Desktop, VS Code) and MCP config merging.
package commands

// CommandNames lists the command files that get installed.
var CommandNames = []string{"taolu", "taolu-list", "taolu-apply", "taolu-author", "taolu-save"}

// Commands maps command names to their full .md file content.
var Commands = map[string]string{
	"taolu":        taoluCmd,
	"taolu-list":   taoluListCmd,
	"taolu-apply":  taoluApplyCmd,
	"taolu-author": taoluAuthorCmd,
	"taolu-save":   taoluSaveCmd,
}

const taoluCmd = `---
description: "Interact with the taolu practice vault"
agent: build
---
You are a taolu assistant. The user invoked /taolu with:
$ARGUMENTS

The taolu MCP server provides these tools:
- taolu_list: search and list taolus by query, tag, or group
- taolu_get: read a taolu's SKILL.md and ACTION.md at a version
- taolu_apply: apply, install, or enforce a taolu into the project
- taolu_save: save a new taolu to the vault
- taolu_history: list version history of a taolu
- taolu_diff: diff between two versions
- taolu_delete: archive a taolu
- taolu_restore: restore an archived taolu
- taolu_rename: rename a taolu
- taolu_info: show vault metadata
- taolu_export: export full taolu content with all assets
- taolu_install_commands: install slash commands for an agent tool
- taolu_list_archived: list archived taolus

If the vault is not initialized, tell the user to run "taolu init" in their terminal.

Interpret the user's intent and use the appropriate tool.
If the request is ambiguous, list available taolus first with taolu_list.
`

const taoluListCmd = `---
description: "List available taolus in the vault"
agent: build
---
List all taolus in the vault.
Optional filter: $ARGUMENTS

Use the taolu_list tool. Format the output as a readable table showing
group, name, version, action mode, and description.
`

const taoluApplyCmd = `---
description: "Apply a taolu to this project"
agent: build
---
Apply the taolu: $ARGUMENTS

Use the taolu_apply tool with mode "enforce" to install the skill and add
a reference to AGENTS.md. Get user approval before writing any files.
If the taolu name is not specified, list available taolus first.
`

const taoluAuthorCmd = `---
description: "Author a new taolu from project conventions"
agent: build
---
Author a new taolu. Scope: $ARGUMENTS

Follow the taolu-authoring guide:
1. Confirm the taolu's scope before surveying the project
2. Confirm the action mode (apply, install, or enforce)
3. Survey the project: README, AGENTS.md, module structure, tooling
4. Extract durable conventions
5. Write SKILL.md with YAML frontmatter
6. Collect any asset files the skill or action references (paths relative to files/)
7. Write ACTION.md with the confirmed mode
8. Review briefly and get explicit approval before saving (for assets, list the files/ paths, not their contents)
`

const taoluSaveCmd = `---
description: "Save a taolu to the vault"
agent: build
---
Save a taolu to the vault: $ARGUMENTS

Use the taolu_save tool. Ensure:
- Name is a valid slug (lowercase alphanumeric with single hyphens)
- Group is specified (e.g. backend, frontend, workflows, meta)
- SKILL.md has valid frontmatter (name, description)
- ACTION.md has a valid mode (apply, install, or enforce)
- Files the skill or action references are passed as files/ assets and saved
  together with skill and action in this same call
- Get user approval before saving; confirm briefly and list asset paths only,
  not their contents
`
