package tools

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yli/taolu/pkg/commands"
	"github.com/yli/taolu/pkg/vault"
)

// RegisterTaoluTools registers the taolu_* MCP tools on the server.
func RegisterTaoluTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_info",
		Description: "Show information about the practice vault: path, project-code, taolu count, groups, and the latest taolu-authoring version.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Path string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		r, p, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		projectCode, err := r.Config("project-code")
		if err != nil {
			return nil, nil, err
		}
		taolus, err := vault.ListTaolu(r)
		if err != nil {
			return nil, nil, err
		}
		groups := vault.UniqueGroups(taolus)
		sort.Strings(groups)
		authoring := "not seeded"
		for _, t := range taolus {
			if t.Name == vault.SeedName {
				authoring = t.LatestVersion
				break
			}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "vault: %s\nproject-code: %s\ntaolus: %d\ngroups: %s\ntaolu-authoring: %s\n",
			p, projectCode, len(taolus), strings.Join(groups, ", "), authoring)
		for _, t := range taolus {
			fmt.Fprintf(&b, "  %s/%s  %s  %s  %s\n", t.Group, t.Name, t.LatestVersion, t.Mode, t.Description)
		}
		return textResult(strings.TrimSuffix(b.String(), "\n")), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_save",
		Description: "Save a taolu (SKILL.md + ACTION.md, plus optional files/ assets) to the vault as a new version. Validates slugs, both frontmatters, and asset paths, commits all files together, and tags the version (v1, v2, ... unless version_label is given).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name   string `json:"name" jsonschema:"taolu name (slug, 1-64 lowercase alphanumeric with single hyphens) (required)"`
		Group  string `json:"group" jsonschema:"group folder, e.g. backend, frontend, workflows, meta (required)"`
		Skill  string `json:"skill" jsonschema:"full SKILL.md content including YAML frontmatter (required)"`
		Action string `json:"action" jsonschema:"full ACTION.md content including YAML frontmatter (required)"`
		Files  []struct {
			Path    string `json:"path" jsonschema:"asset path relative to the files/ directory, e.g. Button.tsx or components/Button.tsx (required)"`
			Content string `json:"content" jsonschema:"asset file content (required)"`
		} `json:"files,omitempty" jsonschema:"optional files/ assets: each {path, content} pair is committed as a support file of the taolu"`
		VersionLabel string `json:"version_label,omitempty" jsonschema:"explicit version label; defaults to the next vN for this taolu"`
		Message      string `json:"message,omitempty" jsonschema:"commit message describing the change"`
		User         string `json:"user,omitempty" jsonschema:"author to record; defaults to admin"`
		Path         string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" || args.Group == "" || args.Skill == "" || args.Action == "" {
			return nil, nil, errors.New("name, group, skill, and action are required")
		}
		if !vault.ValidSlug(args.Group) {
			return nil, nil, fmt.Errorf("invalid group %q: must be 1-64 lowercase alphanumeric with single hyphens", args.Group)
		}
		if err := vault.ValidateContent(args.Name, args.Skill); err != nil {
			return nil, nil, err
		}
		if err := vault.ValidateAction(args.Action); err != nil {
			return nil, nil, err
		}
		assets := make([]vault.Asset, 0, len(args.Files))
		for _, f := range args.Files {
			assets = append(assets, vault.Asset{Path: f.Path, Content: f.Content})
		}
		r, p, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		if sp, err := vault.FindSkillPath(r, args.Name); err != nil {
			return nil, nil, err
		} else if sp != "" {
			archived, err := vault.IsArchived(r, sp)
			if err != nil {
				return nil, nil, err
			}
			if archived {
				return nil, nil, fmt.Errorf("taolu %q is archived; restore it before saving a new version", args.Name)
			}
		}
		label, uuid, total, err := vault.SaveTaolu(r, args.Group, args.Name, args.Skill, args.Action, assets, args.Message, args.User, args.VersionLabel)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("saved %s/%s\nversion: %s (%s)\ntotal versions: %d\nfiles: %d\nvault: %s",
			args.Group, args.Name, label, vault.ShortUUID(uuid), total, len(assets), p)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_get",
		Description: "Return the SKILL.md and ACTION.md of a taolu at a given version (empty/tip, a vN label, or a UUID prefix), plus a manifest of any files/ assets. The taolu is returned as one unit.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name    string `json:"name" jsonschema:"taolu name (required)"`
		Version string `json:"version,omitempty" jsonschema:"version to read (vN label or UUID prefix); defaults to latest"`
		Path    string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return nil, nil, errors.New("name is required")
		}
		r, _, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		skill, action, assets, err := vault.ReadTaoluBundle(r, args.Name, args.Version)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		sp, err := vault.FindSkillPath(r, args.Name)
		if err != nil {
			return nil, nil, err
		}
		if archived, err := vault.IsArchived(r, sp); err != nil {
			return nil, nil, err
		} else if archived {
			fmt.Fprintf(&b, "## ARCHIVED — do not use this taolu\n\n")
		}
		fmt.Fprintf(&b, "## SKILL.md\n\n%s\n\n## ACTION.md\n\n%s", skill, action)
		if len(assets) > 0 {
			fmt.Fprintf(&b, "\n\n## Files\n\n")
			for _, a := range assets {
				fmt.Fprintf(&b, "files/%s\n", a.Path)
			}
			fmt.Fprintf(&b, "\n(full asset content via taolu_export)")
		}
		return textResult(b.String()), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_list",
		Description: "List taolus in the vault, optionally filtered by query, tag, or group. Returns name, group, action mode, latest version, and description.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Query   string `json:"query,omitempty" jsonschema:"case-insensitive substring match against name, description, and tags"`
		Tag     string `json:"tag,omitempty" jsonschema:"require this tag in the taolu's metadata tags (comma-separated match)"`
		Group   string `json:"group,omitempty" jsonschema:"only list taolus under this group"`
		Path    string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
		Include string `json:"include,omitempty" jsonschema:"action modes to include, comma-separated (apply, install, enforce); empty means all"`
	}) (*mcp.CallToolResult, any, error) {
		r, _, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		taolus, err := vault.ListTaolu(r)
		if err != nil {
			return nil, nil, err
		}
		q := strings.ToLower(args.Query)
		var wanted map[string]bool
		if args.Include != "" {
			wanted = map[string]bool{}
			for _, m := range strings.Split(args.Include, ",") {
				wanted[strings.TrimSpace(m)] = true
			}
		}
		var matches []vault.TaoluInfo
		for _, t := range taolus {
			if args.Group != "" && t.Group != args.Group {
				continue
			}
			if len(wanted) > 0 && !wanted[t.Mode] {
				continue
			}
			if args.Tag != "" {
				found := false
				for _, tag := range strings.Split(t.Tags, ",") {
					if strings.TrimSpace(tag) == args.Tag {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if q != "" {
				haystack := strings.ToLower(t.Name + " " + t.Description + " " + t.Tags)
				if !strings.Contains(haystack, q) {
					continue
				}
			}
			matches = append(matches, t)
		}
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].Group == matches[j].Group {
				return matches[i].Name < matches[j].Name
			}
			return matches[i].Group < matches[j].Group
		})
		if len(matches) == 0 {
			return textResult("no taolus match"), nil, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d taolu(s):\n", len(matches))
		for _, t := range matches {
			fmt.Fprintf(&b, "  %s/%s  %s  %s  %s\n", t.Group, t.Name, t.LatestVersion, t.Mode, t.Description)
		}
		return textResult(strings.TrimSuffix(b.String(), "\n")), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_history",
		Description: "List all versions of a taolu, oldest first, with label, UUID, date, user, and message. A version spans SKILL.md and ACTION.md together.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name string `json:"name" jsonschema:"taolu name (required)"`
		Path string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return nil, nil, errors.New("name is required")
		}
		r, _, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		path, err := vault.FindSkillPath(r, args.Name)
		if err != nil {
			return nil, nil, err
		}
		if path == "" {
			return nil, nil, fmt.Errorf("taolu %q not found in vault", args.Name)
		}
		hist, err := vault.SkillHistory(r, path)
		if err != nil {
			return nil, nil, err
		}
		if len(hist) == 0 {
			return textResult("no versions"), nil, nil
		}
		var b strings.Builder
		for _, v := range hist {
			fmt.Fprintf(&b, "%s  %s  %s  %s  %s\n", v.Label, vault.ShortUUID(v.UUID),
				v.Date.Format("2006-01-02 15:04:05"), v.User, v.Message)
		}
		return textResult(strings.TrimSuffix(b.String(), "\n")), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_diff",
		Description: "Unified diff of a taolu between two versions, showing SKILL.md and ACTION.md together. If version_a is empty, diffs against the previous version.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name     string `json:"name" jsonschema:"taolu name (required)"`
		VersionB string `json:"version_b" jsonschema:"newer version (vN label or UUID prefix) (required)"`
		VersionA string `json:"version_a,omitempty" jsonschema:"older version; defaults to the version before version_b"`
		Path     string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" || args.VersionB == "" {
			return nil, nil, errors.New("name and version_b are required")
		}
		r, _, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		path, err := vault.FindSkillPath(r, args.Name)
		if err != nil {
			return nil, nil, err
		}
		if path == "" {
			return nil, nil, fmt.Errorf("taolu %q not found in vault", args.Name)
		}
		uuidA, uuidB, err := vault.ResolveDiffVersions(r, path, args.VersionA, args.VersionB)
		if err != nil {
			return nil, nil, err
		}
		ridA, err := r.ResolveVersion(uuidA)
		if err != nil {
			return nil, nil, err
		}
		ridB, err := r.ResolveVersion(uuidB)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		action := filepath.Join(filepath.Dir(path), "ACTION.md")
		for _, f := range []string{path, action} {
			entries, err := r.Diff(ridA, ridB, f)
			if err != nil {
				return nil, nil, err
			}
			if len(entries) == 0 {
				continue
			}
			fmt.Fprintf(&b, "--- %s\n", filepath.Base(f))
			for _, e := range entries {
				fmt.Fprintf(&b, "%s\n", e.Unified)
			}
		}
		if b.Len() == 0 {
			return textResult("no differences"), nil, nil
		}
		return textResult(strings.TrimSuffix(b.String(), "\n")), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_apply",
		Description: "Apply a taolu to the current project. Dispatches on the taolu's action mode (or the explicit action argument): apply returns the content for a one-shot use; install writes SKILL.md + a .taolu-version pin into the format target; enforce also appends a compliance reference to AGENTS.md. Install and enforce require explicit user approval and refuse to overwrite without force.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name    string `json:"name" jsonschema:"taolu name to apply (required)"`
		Version string `json:"version,omitempty" jsonschema:"version to apply (vN label or UUID prefix); defaults to latest"`
		Target  string `json:"target,omitempty" jsonschema:"project root directory; defaults to the current directory"`
		Format  string `json:"format,omitempty" jsonschema:"skill format: opencode, claude, or agents; defaults to the taolu's action detail, else opencode"`
		Action  string `json:"action,omitempty" jsonschema:"override the action mode: apply, install, or enforce"`
		Force   bool   `json:"force,omitempty" jsonschema:"overwrite an existing SKILL.md"`
		Path    string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return nil, nil, errors.New("name is required")
		}
		r, p, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		if sp, err := vault.FindSkillPath(r, args.Name); err != nil {
			return nil, nil, err
		} else if sp != "" {
			archived, err := vault.IsArchived(r, sp)
			if err != nil {
				return nil, nil, err
			}
			if archived {
				return nil, nil, fmt.Errorf("taolu %q is archived and must not be used; restore it first", args.Name)
			}
		}
		res, err := vault.ApplyTaolu(r, p, args.Name, args.Version, args.Target, args.Format, args.Action, args.Force)
		if err != nil {
			return nil, nil, err
		}
		switch res.Mode {
		case vault.ModeApply:
			var b strings.Builder
			fmt.Fprintf(&b, "taolu %s (mode: apply)\n\n## SKILL.md\n\n%s\n\n## ACTION.md\n\n%s", args.Name, res.Skill, res.Action)
			if len(res.Assets) > 0 {
				fmt.Fprintf(&b, "\n\n## Files\n\n")
				for _, a := range res.Assets {
					fmt.Fprintf(&b, "files/%s\n", a.Path)
				}
				fmt.Fprintf(&b, "\n(full asset content via taolu_export)")
			}
			return textResult(b.String()), nil, nil
		case vault.ModeInstall:
			return textResult(fmt.Sprintf("installed %s to %s\nversion: %s\nfiles: %d\npinned: %s/.taolu-version\nvault: %s",
				args.Name, res.Rel, versionLabelOrTip(res.Label), len(res.Assets), res.Rel, p)), nil, nil
		default: // enforce
			return textResult(fmt.Sprintf("enforced %s to %s\nversion: %s\nfiles: %d\npinned: %s/.taolu-version\nAGENTS.md: reference added\nvault: %s",
				args.Name, res.Rel, versionLabelOrTip(res.Label), len(res.Assets), res.Rel, p)), nil, nil
		}
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_export",
		Description: "Export the raw content of a taolu at a given version (for review or copying elsewhere): SKILL.md, ACTION.md, and every files/ asset with full content.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name    string `json:"name" jsonschema:"taolu name (required)"`
		Version string `json:"version,omitempty" jsonschema:"version to export (vN label or UUID prefix); defaults to latest"`
		Path    string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return nil, nil, errors.New("name is required")
		}
		r, _, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		skill, action, assets, err := vault.ReadTaoluBundle(r, args.Name, args.Version)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		sp, err := vault.FindSkillPath(r, args.Name)
		if err != nil {
			return nil, nil, err
		}
		if archived, err := vault.IsArchived(r, sp); err != nil {
			return nil, nil, err
		} else if archived {
			fmt.Fprintf(&b, "# ARCHIVED — do not use this taolu\n\n")
		}
		fmt.Fprintf(&b, "== SKILL.md ==\n%s\n== ACTION.md ==\n%s", skill, action)
		for _, a := range assets {
			fmt.Fprintf(&b, "\n== files/%s ==\n%s", a.Path, a.Content)
		}
		return textResult(b.String()), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_delete",
		Description: "Archive a taolu: commits an .archived marker into its directory, hiding it from taolu_list and refusing taolu_apply/taolu_save until it is restored. The source tree is kept; use taolu_restore to bring it back, or taolu_list_archived to see archived taolus. Refuses the built-in taolu-authoring guide.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name    string `json:"name" jsonschema:"taolu name to archive (required)"`
		Message string `json:"message,omitempty" jsonschema:"commit message; defaults to 'archive taolu <name>'"`
		User    string `json:"user,omitempty" jsonschema:"author to record; defaults to admin"`
		Path    string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return nil, nil, errors.New("name is required")
		}
		r, p, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		group, err := vault.ArchiveTaolu(r, args.Name, args.Message, args.User)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("archived %s/%s\nsource tree kept; restore with taolu_restore\nvault: %s",
			group, args.Name, p)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_restore",
		Description: "Restore an archived taolu: removes its .archived marker so it shows up in taolu_list again and can be used by taolu_apply and taolu_save.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name    string `json:"name" jsonschema:"archived taolu name to restore (required)"`
		Message string `json:"message,omitempty" jsonschema:"commit message; defaults to 'restore taolu <name>'"`
		User    string `json:"user,omitempty" jsonschema:"author to record; defaults to admin"`
		Path    string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return nil, nil, errors.New("name is required")
		}
		r, p, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		group, err := vault.RestoreTaolu(r, args.Name, args.Message, args.User)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("restored %s/%s\nvault: %s", group, args.Name, p)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_list_archived",
		Description: "List archived taolus, optionally filtered by query, tag, or group. These are hidden from taolu_list and must not be used until restored.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Query string `json:"query,omitempty" jsonschema:"case-insensitive substring match against name, description, and tags"`
		Tag   string `json:"tag,omitempty" jsonschema:"require this tag in the taolu's metadata tags (comma-separated match)"`
		Group string `json:"group,omitempty" jsonschema:"only list taolus under this group"`
		Path  string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		r, _, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		taolus, err := vault.ListArchivedTaolu(r)
		if err != nil {
			return nil, nil, err
		}
		q := strings.ToLower(args.Query)
		var matches []vault.TaoluInfo
		for _, t := range taolus {
			if args.Group != "" && t.Group != args.Group {
				continue
			}
			if args.Tag != "" {
				found := false
				for _, tag := range strings.Split(t.Tags, ",") {
					if strings.TrimSpace(tag) == args.Tag {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if q != "" {
				haystack := strings.ToLower(t.Name + " " + t.Description + " " + t.Tags)
				if !strings.Contains(haystack, q) {
					continue
				}
			}
			matches = append(matches, t)
		}
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].Group == matches[j].Group {
				return matches[i].Name < matches[j].Name
			}
			return matches[i].Group < matches[j].Group
		})
		if len(matches) == 0 {
			return textResult("no archived taolus match"), nil, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d archived taolu(s) (restore with taolu_restore):\n", len(matches))
		for _, t := range matches {
			fmt.Fprintf(&b, "  %s/%s  %s  %s  %s\n", t.Group, t.Name, t.LatestVersion, t.Mode, t.Description)
		}
		return textResult(strings.TrimSuffix(b.String(), "\n")), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_rename",
		Description: "Rename a taolu, optionally moving it to another group: rewrites the SKILL.md frontmatter name, moves SKILL.md, ACTION.md, files/ assets, and markers to the new path, and records an origin marker so version history continues under the new name instead of restarting at v1. Old history is preserved.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name     string `json:"name" jsonschema:"current taolu name (required)"`
		NewName  string `json:"new_name" jsonschema:"new taolu name (slug, 1-64 lowercase alphanumeric with single hyphens) (required)"`
		NewGroup string `json:"new_group,omitempty" jsonschema:"new group folder; defaults to the current group"`
		Message  string `json:"message,omitempty" jsonschema:"commit message; defaults to 'rename taolu <name> to <new_name>'"`
		User     string `json:"user,omitempty" jsonschema:"author to record; defaults to admin"`
		Path     string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" || args.NewName == "" {
			return nil, nil, errors.New("name and new_name are required")
		}
		r, p, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		oldGroup, newGroup, err := vault.RenameTaolu(r, args.Name, args.NewName, args.NewGroup, args.Message, args.User)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("renamed %s/%s -> %s/%s\nSKILL.md frontmatter name updated\nversion history continued\nvault: %s",
			oldGroup, args.Name, newGroup, args.NewName, p)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_install_commands",
		Description: "Install taolu slash commands for an agent tool. Creates command files (e.g. /taolu, /taolu-list, /taolu-apply) in the agent's config directory and merges the MCP server connection config.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Tool      string `json:"tool" jsonschema:"agent tool: opencode, claude, or vscode (required)"`
		Target    string `json:"target,omitempty" jsonschema:"project root directory; defaults to the current directory"`
		Scope     string `json:"scope,omitempty" jsonschema:"local or global; defaults to local"`
		Transport string `json:"transport,omitempty" jsonschema:"http or stdio; defaults to http"`
		Port      int    `json:"port,omitempty" jsonschema:"MCP server port for http mode; defaults to 8264"`
		RepoPath  string `json:"repo_path,omitempty" jsonschema:"vault path for stdio mode; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
		Force     bool   `json:"force,omitempty" jsonschema:"overwrite existing command files and MCP config"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Tool == "" {
			return nil, nil, errors.New("tool is required (opencode, claude, or vscode)")
		}
		if args.Scope == "" {
			args.Scope = "local"
		}
		if args.Transport == "" {
			args.Transport = commands.TransportHTTP
		}
		opts := commands.InstallOptions{
			Tool:      args.Tool,
			Scope:     args.Scope,
			Transport: args.Transport,
			Target:    args.Target,
			Port:      args.Port,
			RepoPath:  args.RepoPath,
			Force:     args.Force,
		}
		written, err := commands.Install(opts)
		if err != nil {
			return nil, nil, err
		}
		if len(written) == 0 {
			return textResult("already up to date; no files changed"), nil, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "installed %d file(s) for %s (%s):\n", len(written), args.Tool, args.Scope)
		for _, f := range written {
			fmt.Fprintf(&b, "  %s\n", f)
		}
		return textResult(strings.TrimSuffix(b.String(), "\n")), nil, nil
	})
}
