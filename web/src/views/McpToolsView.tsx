import { useState, useMemo } from "react";
import { Link } from "@tanstack/react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CodeBlock } from "@/components/code-block";
import { File, Terminal } from "lucide-react";
import { cn } from "@/lib/utils";

interface Param {
  name: string;
  type: string;
  required: boolean;
  description: string;
  fields?: Param[];
}

interface ToolDef {
  name: string;
  description: string;
  params: Param[];
  example?: string;
}

const tools: ToolDef[] = [
  {
    name: "taolu_info",
    description:
      "Show information about the practice vault: path, project-code, taolu count, groups, domains, and the latest taolu-authoring version.",
    params: [
      { name: "path", type: "string", required: false, description: "Vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil" },
    ],
  },
  {
    name: "taolu_list",
    description: "List taolus in the vault, optionally filtered by query, tag, group, or domain. Returns name, group, domain, action mode, latest version, and description.",
    params: [
      { name: "query", type: "string", required: false, description: "Case-insensitive substring match against name, description, and tags" },
      { name: "tag", type: "string", required: false, description: "Require this tag in the taolu's metadata tags" },
      { name: "group", type: "string", required: false, description: "Only list taolus under this group" },
      { name: "domain", type: "string", required: false, description: "Only list taolus under this domain (e.g. @local)" },
      { name: "include", type: "string", required: false, description: "Action modes to include, comma-separated (apply, install, enforce)" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_get",
    description: "Return the SKILL.md and ACTION.md of a taolu at a given version, plus a manifest of any files/ assets.",
    params: [
      { name: "name", type: "string", required: true, description: "Taolu name" },
      { name: "version", type: "string", required: false, description: "Version to read (vN label or UUID prefix); defaults to latest" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_save",
    description: "Save a taolu (SKILL.md + ACTION.md, plus optional files/ assets) to the vault as a new version. Validates slugs, both frontmatters, and asset paths, commits all files together, and tags the version.",
    params: [
      { name: "name", type: "string", required: true, description: "Taolu name (slug, 1-64 lowercase alphanumeric with single hyphens)" },
      { name: "group", type: "string", required: true, description: "Group folder (e.g. backend, frontend, workflows, meta)" },
      { name: "domain", type: "string", required: false, description: "Domain folder (e.g. @local); defaults to @local or user-domain" },
      { name: "skill", type: "string", required: true, description: "Full SKILL.md content including YAML frontmatter" },
      { name: "action", type: "string", required: true, description: "Full ACTION.md content including YAML frontmatter" },
      { name: "files", type: "array", required: false, description: "Support files to attach", fields: [
        { name: "path", type: "string", required: true, description: "Asset path relative to the files/ directory (e.g. Button.tsx)" },
        { name: "file_path", type: "string", required: true, description: "Absolute or workspace-relative path to a local file to read and attach" },
      ] },
      { name: "version_label", type: "string", required: false, description: "Explicit version label; defaults to the next vN" },
      { name: "message", type: "string", required: false, description: "Commit message describing the change" },
      { name: "user", type: "string", required: false, description: "Author to record; defaults to admin" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
    example: `{
  "name": "serve-dev",
  "group": "workflows",
  "skill": "---\\nname: serve-dev\\ndescription: Dev server script\\n---\\n...",
  "action": "---\\nmode: apply\\n---\\n",
  "files": [
    { "path": "serve-dev.sh", "file_path": "./serve-dev.sh" }
  ]
}`,
  },
  {
    name: "taolu_apply",
    description: "Apply a taolu to the current project. Dispatches on the taolu's action mode: apply returns content for one-shot use; install writes SKILL.md + pin; enforce also adds a reference to AGENTS.md.",
    params: [
      { name: "name", type: "string", required: true, description: "Taolu name to apply" },
      { name: "version", type: "string", required: false, description: "Version to apply (vN label or UUID prefix); defaults to latest" },
      { name: "target", type: "string", required: false, description: "Project root directory; defaults to the current directory" },
      { name: "format", type: "string", required: false, description: "Skill format: opencode, claude, or agents" },
      { name: "action", type: "string", required: false, description: "Override the action mode: apply, install, or enforce" },
      { name: "force", type: "boolean", required: false, description: "Overwrite an existing SKILL.md" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_export",
    description: "Export the raw content of a taolu at a given version: SKILL.md, ACTION.md, and every files/ asset with full content.",
    params: [
      { name: "name", type: "string", required: true, description: "Taolu name" },
      { name: "version", type: "string", required: false, description: "Version to export (vN label or UUID prefix); defaults to latest" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_history",
    description: "List all versions of a taolu, oldest first, with label, UUID, date, user, and message.",
    params: [
      { name: "name", type: "string", required: true, description: "Taolu name" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_diff",
    description: "Unified diff of a taolu between two versions, showing SKILL.md and ACTION.md together.",
    params: [
      { name: "name", type: "string", required: true, description: "Taolu name" },
      { name: "version_b", type: "string", required: true, description: "Newer version (vN label or UUID prefix)" },
      { name: "version_a", type: "string", required: false, description: "Older version; defaults to the version before version_b" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_delete",
    description: "Archive a taolu: commits an .archived marker, hiding it from taolu_list and refusing taolu_apply/taolu_save until restored.",
    params: [
      { name: "name", type: "string", required: true, description: "Taolu name to archive" },
      { name: "message", type: "string", required: false, description: "Commit message; defaults to 'archive taolu <name>'" },
      { name: "user", type: "string", required: false, description: "Author to record; defaults to admin" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_restore",
    description: "Restore an archived taolu: removes its .archived marker so it shows up in taolu_list again.",
    params: [
      { name: "name", type: "string", required: true, description: "Archived taolu name to restore" },
      { name: "message", type: "string", required: false, description: "Commit message; defaults to 'restore taolu <name>'" },
      { name: "user", type: "string", required: false, description: "Author to record; defaults to admin" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_rename",
    description: "Rename a taolu, optionally moving it to another group. Rewrites frontmatter, moves files, and records an origin marker so history continues.",
    params: [
      { name: "name", type: "string", required: true, description: "Current taolu name" },
      { name: "new_name", type: "string", required: true, description: "New taolu name (slug)" },
      { name: "new_group", type: "string", required: false, description: "New group folder; defaults to the current group" },
      { name: "message", type: "string", required: false, description: "Commit message" },
      { name: "user", type: "string", required: false, description: "Author to record; defaults to admin" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_fork",
    description: "Fork a taolu: clone its SKILL.md, ACTION.md, and files/ assets into a new name, recording a .fork provenance marker.",
    params: [
      { name: "name", type: "string", required: true, description: "Source taolu reference to fork (e.g. @local/backend/go-api-server)" },
      { name: "new_name", type: "string", required: true, description: "New taolu name (slug)" },
      { name: "new_group", type: "string", required: false, description: "New group folder; defaults to the source group" },
      { name: "message", type: "string", required: false, description: "Commit message" },
      { name: "user", type: "string", required: false, description: "Author to record; defaults to admin" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_fork_info",
    description: "Show fork provenance for a taolu: the source taolu and version it was forked from, or a note that it is not a fork.",
    params: [
      { name: "name", type: "string", required: true, description: "Taolu reference" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_install_commands",
    description: "Install taolu slash commands for an agent tool. Creates command files and merges the MCP server connection config.",
    params: [
      { name: "tool", type: "string", required: true, description: "Agent tool: opencode, claude, or vscode" },
      { name: "target", type: "string", required: false, description: "Project root directory; defaults to the current directory" },
      { name: "scope", type: "string", required: false, description: "Local or global; defaults to local" },
      { name: "transport", type: "string", required: false, description: "HTTP or stdio; defaults to http" },
      { name: "port", type: "number", required: false, description: "MCP server port for HTTP mode; defaults to 8264" },
      { name: "force", type: "boolean", required: false, description: "Overwrite existing command files and MCP config" },
    ],
  },
  {
    name: "taolu_config",
    description: "Get or set vault configuration, including user domain and domain aliases.",
    params: [
      { name: "action", type: "string", required: true, description: "Action: get or set" },
      { name: "key", type: "string", required: true, description: "Config key: user-domain, domain-aliases" },
      { name: "value", type: "string", required: false, description: "Value to set (required for set action)" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
  {
    name: "taolu_list_archived",
    description: "List archived taolus, optionally filtered by query, tag, or group. Hidden from taolu_list until restored.",
    params: [
      { name: "query", type: "string", required: false, description: "Case-insensitive substring match against name, description, and tags" },
      { name: "tag", type: "string", required: false, description: "Require this tag in the taolu's metadata tags" },
      { name: "group", type: "string", required: false, description: "Only list taolus under this group" },
      { name: "path", type: "string", required: false, description: "Vault repository path" },
    ],
  },
];

interface CommandDef {
  name: string;
  description: string;
  agent: string;
  prompt: string;
}

const commands: CommandDef[] = [
  {
    name: "/taolu",
    description: "Route a taolu request to the right vault tool and carry it out",
    agent: "build",
    prompt: `Input: free-form request in $ARGUMENTS (a question, a taolu name, or an action to perform).
Output: the result of the chosen tool, summarized for the user.

Guidelines:
- Interpret the intent; when ambiguous, run taolu_list first and confirm.
- If the vault is missing, tell the user to run "taolu init" in their terminal.
- Use exact names taken from listings; never guess a taolu name.
- Prefer read-only tools (list/get/history/diff) unless a change is requested.

Goal: the user's request is resolved with the correct tool in minimal steps, with no unconfirmed changes to the vault.`,
  },
  {
    name: "/taolu-list",
    description: "Show which practices exist in the vault so the user can pick one",
    agent: "build",
    prompt: `Input: optional filter in $ARGUMENTS (query text, tag, group, or domain).
Output: a readable table of matches: group/name, version, mode, description.

Guidelines:
- Use taolu_list; map the filter words to its query/tag/group/domain params.
- Apply filters exactly as given; do not silently broaden them.
- When nothing matches, say so and suggest taolu_list_archived for archived items.
- Do not truncate results without saying how many were hidden.

Goal: the user can identify the right taolu at a glance, including its latest version and action mode.`,
  },
  {
    name: "/taolu-apply",
    description: "Install a practice into this project via taolu_apply",
    agent: "build",
    prompt: `Input: a taolu name/ref in $ARGUMENTS, optionally with a version or format (e.g. "@local/workflows/go-lint v2").
Output: confirmation of what was written where (skill path, pin file, AGENTS.md reference), or returned content for apply mode.

Guidelines:
- If the name is ambiguous or missing, list candidates with taolu_list and confirm before applying.
- Get explicit user approval before writing any files.
- Never overwrite an existing SKILL.md unless the user explicitly asks for force.
- After install/enforce, report the pinned version and installed path.

Goal: the intended practice is applied at the intended version and format, with no files written or overwritten beyond what the user approved.`,
  },
  {
    name: "/taolu-author",
    description: "Draft a new taolu that captures the durable conventions of this project",
    agent: "build",
    prompt: `Input: scope/topic in $ARGUMENTS (what the practice should cover).
Output: drafted SKILL.md + ACTION.md plus proposed files/ asset paths, presented for review — not saved.

Guidelines:
- Follow the taolu-authoring guide: confirm scope and action mode before surveying.
- Survey README, AGENTS.md, module structure, and tooling before writing.
- Capture durable conventions only; exclude one-off details and secrets.
- Keep SKILL.md concise; put reusable code in files/ assets referenced by path.
- During review, list asset paths only (never their full contents).

Goal: the user approves a draft that accurately reflects real project conventions and is ready to save unchanged.`,
  },
  {
    name: "/taolu-save",
    description: "Commit an approved draft to the vault as a new version via taolu_save",
    agent: "build",
    prompt: `Input: approved SKILL.md and ACTION.md content, plus local paths for any files/ assets.
Output: saved ref (@domain/group/name), version label, total versions, and asset count.

Guidelines:
- Validate before calling: slug name, SKILL.md frontmatter (name, description), ACTION.md mode (apply/install/enforce).
- Attach every file the skill references via file_path; the server reads them.
- Get explicit approval first; confirm asset paths only, not their contents.
- Refuse to save over an archived taolu; restore it first.

Goal: a clean new version exists in the vault containing the approved content with every referenced asset attached.`,
  },
];

function ParamTable({ params }: { params: Param[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border/40 text-left text-xs text-muted-foreground">
            <th className="pb-2 pr-4 font-medium">Parameter</th>
            <th className="pb-2 pr-4 font-medium">Type</th>
            <th className="pb-2 font-medium">Description</th>
          </tr>
        </thead>
        <tbody>
          {params.map((p) => (
            <>
              <tr key={p.name} className="border-b border-border/20">
                <td className="py-2 pr-4 font-mono text-xs font-medium">
                  {p.name}
                  {p.required && <span className="ml-1 text-destructive">*</span>}
                </td>
                <td className="py-2 pr-4">
                  <Badge variant="outline" className="font-mono text-[10px]">
                    {p.type}{p.fields ? `‹${p.fields.map((f) => f.name).join(", ")}›` : ""}
                  </Badge>
                </td>
                <td className="py-2 text-xs text-muted-foreground">{p.description}</td>
              </tr>
              {p.fields?.map((f) => (
                <tr key={`${p.name}.${f.name}`} className="border-b border-border/20 last:border-0">
                  <td className="py-1.5 pl-6 pr-4 font-mono text-xs text-muted-foreground">
                    .{f.name}
                    {f.required && <span className="ml-1 text-destructive">*</span>}
                  </td>
                  <td className="py-1.5 pr-4">
                    <Badge variant="outline" className="font-mono text-[10px] border-dashed">
                      {f.type}
                    </Badge>
                  </td>
                  <td className="py-1.5 text-xs text-muted-foreground">{f.description}</td>
                </tr>
              ))}
            </>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default function McpToolsView() {
  return (
    <div className="space-y-4">
      <div>
        <Link to="/status" className="text-sm text-muted-foreground hover:text-foreground">
          ← Status
        </Link>
        <h1 className="text-2xl font-semibold tracking-tight mt-1">Agent interface</h1>
        <p className="text-sm text-muted-foreground">
          {tools.length} MCP tools and {commands.length} slash commands exposed to agents.
        </p>
      </div>

      <Tabs defaultValue="tools">
        <TabsList>
          <TabsTrigger value="tools">Tools</TabsTrigger>
          <TabsTrigger value="commands">Commands</TabsTrigger>
        </TabsList>

        <TabsContent value="tools">
          <ToolsBrowser />
        </TabsContent>
        <TabsContent value="commands">
          <CommandsBrowser />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function ToolsBrowser() {
  const [selected, setSelected] = useState(tools[0].name);

  const tool = useMemo(() => tools.find((t) => t.name === selected) ?? tools[0], [selected]);

  return (
    <div className="flex gap-3 min-h-[500px] mt-3">
      {/* Left panel: tool list */}
      <div className="w-56 shrink-0">
        <div className="rounded-xl glass-control bg-clip-padding p-1 overflow-auto">
          <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground border-b border-border/30 mb-1">
            Tools
          </div>
          {tools.map((t) => (
            <button
              key={t.name}
              onClick={() => setSelected(t.name)}
              className={cn(
                "flex items-center gap-1.5 w-full text-left px-2 py-1 text-xs rounded-md transition-colors",
                t.name === selected
                  ? "bg-accent text-foreground font-medium"
                  : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
              )}
            >
              <File className="w-3.5 h-3.5 shrink-0" />
              <span className="truncate">{t.name}</span>
            </button>
          ))}
        </div>
      </div>

      {/* Right panel: tool detail */}
      <div className="flex-1 min-w-0">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="font-mono text-lg">{tool.name}</CardTitle>
            <p className="text-sm text-muted-foreground">{tool.description}</p>
            <div className="flex items-center gap-2 mt-1">
              <Badge variant="secondary" className="font-mono text-[10px]">
                {tool.params.filter((p) => p.required).length} required
              </Badge>
              <Badge variant="outline" className="font-mono text-[10px]">
                {tool.params.length} total
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <ParamTable params={tool.params} />

            {tool.example && (
              <div>
                <p className="text-xs font-medium text-muted-foreground mb-2">Example</p>
                <CodeBlock content={tool.example} filename="example.json" className="text-xs" />
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function CommandsBrowser() {
  const [selected, setSelected] = useState(commands[0].name);

  const command = useMemo(
    () => commands.find((c) => c.name === selected) ?? commands[0],
    [selected],
  );

  return (
    <div className="flex gap-3 min-h-[500px] mt-3">
      {/* Left panel: command list */}
      <div className="w-56 shrink-0">
        <div className="rounded-xl glass-control bg-clip-padding p-1 overflow-auto">
          <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground border-b border-border/30 mb-1">
            Commands
          </div>
          {commands.map((c) => (
            <button
              key={c.name}
              onClick={() => setSelected(c.name)}
              className={cn(
                "flex items-center gap-1.5 w-full text-left px-2 py-1 text-xs rounded-md transition-colors",
                c.name === selected
                  ? "bg-accent text-foreground font-medium"
                  : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
              )}
            >
              <Terminal className="w-3.5 h-3.5 shrink-0" />
              <span className="truncate">{c.name}</span>
            </button>
          ))}
        </div>
      </div>

      {/* Right panel: command detail */}
      <div className="flex-1 min-w-0">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="font-mono text-lg">{command.name}</CardTitle>
            <p className="text-sm text-muted-foreground">{command.description}</p>
            <div className="flex items-center gap-2 mt-1">
              <Badge variant="secondary" className="font-mono text-[10px]">
                agent: {command.agent}
              </Badge>
            </div>
          </CardHeader>
          <CardContent>
            <CodeBlock content={command.prompt} filename={`${command.name.slice(1)}.md`} className="text-xs" />
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
