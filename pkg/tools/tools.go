package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	libfossil "github.com/danmestas/go-libfossil"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yli/taolu/pkg/vault"
)

// RegisterTaoluTools registers the taolu_* MCP tools on the server.
func RegisterTaoluTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_init",
		Description: "Create or open the practice vault (a Fossil repository), migrate any legacy practices/ tree, and ensure the taolu-authoring guide is seeded. Returns vault path, project-code, and taolu count.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Path string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
		User string `json:"user,omitempty" jsonschema:"user recorded for seeded commits; defaults to admin"`
	}) (*mcp.CallToolResult, any, error) {
		p, err := vault.VaultPath(args.Path)
		if err != nil {
			return nil, nil, err
		}
		var r *libfossil.Repo
		if _, statErr := os.Stat(p); statErr == nil {
			r, err = libfossil.Open(p)
		} else {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return nil, nil, err
			}
			r, err = libfossil.Create(p, libfossil.CreateOpts{User: args.User})
		}
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		if err := vault.EnsureAuthoringGuide(r, args.User); err != nil {
			return nil, nil, err
		}
		if err := vault.MigrateLegacy(r, args.User); err != nil {
			return nil, nil, err
		}
		projectCode, err := r.Config("project-code")
		if err != nil {
			return nil, nil, err
		}
		taolus, err := vault.ListTaolu(r)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("vault: %s\nproject-code: %s\ntaolus: %d (in %d groups)", p, projectCode, len(taolus), len(vault.UniqueGroups(taolus)))), nil, nil
	})

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
		Description: "Save a taolu (SKILL.md + ACTION.md) to the vault as a new version. Validates slugs and both frontmatters, commits both files together, and tags the version (v1, v2, ... unless version_label is given).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name         string `json:"name" jsonschema:"taolu name (slug, 1-64 lowercase alphanumeric with single hyphens) (required)"`
		Group        string `json:"group" jsonschema:"group folder, e.g. backend, frontend, workflows, meta (required)"`
		Skill        string `json:"skill" jsonschema:"full SKILL.md content including YAML frontmatter (required)"`
		Action       string `json:"action" jsonschema:"full ACTION.md content including YAML frontmatter (required)"`
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
		r, p, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		label, uuid, total, err := vault.SaveTaolu(r, args.Group, args.Name, args.Skill, args.Action, args.Message, args.User, args.VersionLabel)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("saved %s/%s\nversion: %s (%s)\ntotal versions: %d\nvault: %s",
			args.Group, args.Name, label, vault.ShortUUID(uuid), total, p)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_get",
		Description: "Return the SKILL.md and ACTION.md of a taolu at a given version (empty/tip, a vN label, or a UUID prefix). The pair is returned together.",
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
		skill, action, err := vault.ReadTaoluAtVersion(r, args.Name, args.Version)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("## SKILL.md\n\n%s\n\n## ACTION.md\n\n%s", skill, action)), nil, nil
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
		res, err := vault.ApplyTaolu(r, p, args.Name, args.Version, args.Target, args.Format, args.Action, args.Force)
		if err != nil {
			return nil, nil, err
		}
		switch res.Mode {
		case vault.ModeApply:
			return textResult(fmt.Sprintf("taolu %s (mode: apply)\n\n## SKILL.md\n\n%s\n\n## ACTION.md\n\n%s",
				args.Name, res.Skill, res.Action)), nil, nil
		case vault.ModeInstall:
			return textResult(fmt.Sprintf("installed %s to %s\nversion: %s\npinned: %s/.taolu-version\nvault: %s",
				args.Name, res.Rel, versionLabelOrTip(res.Label), res.Rel, p)), nil, nil
		default: // enforce
			return textResult(fmt.Sprintf("enforced %s to %s\nversion: %s\npinned: %s/.taolu-version\nAGENTS.md: reference added\nvault: %s",
				args.Name, res.Rel, versionLabelOrTip(res.Label), res.Rel, p)), nil, nil
		}
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "taolu_export",
		Description: "Export the raw SKILL.md and ACTION.md content of a taolu at a given version (for review or copying elsewhere).",
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
		skill, action, err := vault.ReadTaoluAtVersion(r, args.Name, args.Version)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("== SKILL.md ==\n%s\n== ACTION.md ==\n%s", skill, action)), nil, nil
	})
}
