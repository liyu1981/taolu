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
Route a taolu request to the right vault tool and carry it out.

**Input**: free-form request in $ARGUMENTS (a question, a taolu name, or an
action to perform).
**Output**: the result of the chosen tool, summarized for the user.

Guidelines:
- Interpret the intent; when ambiguous, run taolu_list first and confirm.
- If the vault is missing, tell the user to run "taolu init" in their terminal.
- Use exact names taken from listings; never guess a taolu name.
- Prefer read-only tools (list/get/history/diff) unless a change is requested.

Goal: the user's request is resolved with the correct tool in minimal steps,
with no unconfirmed changes to the vault.
`

const taoluListCmd = `---
description: "List available taolus in the vault"
agent: build
---
Show which practices exist in the vault so the user can pick one.

**Input**: optional filter in $ARGUMENTS (query text, tag, group, or domain).
**Output**: a readable table of matches: group/name, version, mode, description.

Guidelines:
- Use taolu_list; map the filter words to its query/tag/group/domain params.
- Apply filters exactly as given; do not silently broaden them.
- When nothing matches, say so and suggest taolu_list_archived for archived
  items.
- Do not truncate results without saying how many were hidden.

Goal: the user can identify the right taolu at a glance, including its latest
version and action mode.
`

const taoluApplyCmd = `---
description: "Apply a taolu to this project"
agent: build
---
Install a practice into this project via taolu_apply.

**Input**: a taolu name/ref in $ARGUMENTS, optionally with a version or format
(e.g. "@local/workflows/go-lint v2").
**Output**: confirmation of what was written where (skill path, pin file,
AGENTS.md reference), or returned content for apply mode.

Guidelines:
- If the name is ambiguous or missing, list candidates with taolu_list and
  confirm before applying.
- Get explicit user approval before writing any files.
- Never overwrite an existing SKILL.md unless the user explicitly asks for
  force.
- After install/enforce, report the pinned version and installed path.

Goal: the intended practice is applied at the intended version and format,
with no files written or overwritten beyond what the user approved.
`

const taoluAuthorCmd = `---
description: "Author a new taolu from project conventions"
agent: build
---
Draft a new taolu that captures the durable conventions of this project.

**Input**: scope/topic in $ARGUMENTS (what the practice should cover).
**Output**: drafted SKILL.md + ACTION.md plus proposed files/ asset paths,
presented for review — not saved.

Guidelines:
- Follow the taolu-authoring guide: confirm scope and action mode before
  surveying.
- Survey README, AGENTS.md, module structure, and tooling before writing.
- Capture durable conventions only; exclude one-off details and secrets.
- Keep SKILL.md concise; put reusable code in files/ assets referenced by
  path.
- During review, list asset paths only (never their full contents).

Goal: the user approves a draft that accurately reflects real project
conventions and is ready to save unchanged.
`

const taoluSaveCmd = `---
description: "Save a taolu to the vault"
agent: build
---
Commit an approved draft to the vault as a new version via taolu_save.

**Input**: approved SKILL.md and ACTION.md content, plus local paths for any
files/ assets.
**Output**: saved ref (@domain/group/name), version label, total versions, and
asset count.

Guidelines:
- Validate before calling: slug name, SKILL.md frontmatter (name,
  description), ACTION.md mode (apply/install/enforce).
- Attach every file the skill references via file_path; the server reads them.
- Get explicit approval first; confirm asset paths only, not their contents.
- Refuse to save over an archived taolu; restore it first.

Goal: a clean new version exists in the vault containing the approved content
with every referenced asset attached.
`
