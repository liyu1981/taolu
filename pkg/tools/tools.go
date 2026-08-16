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

// RegisterVaultTools registers the vault_* MCP tools on the server.
func RegisterVaultTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "vault_init",
		Description: "Create or open the practice vault (a Fossil repository) and ensure the practice-authoring guide is seeded. Returns vault path, project-code, and skill count.",
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
		projectCode, err := r.Config("project-code")
		if err != nil {
			return nil, nil, err
		}
		skills, err := vault.ListSkills(r)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("vault: %s\nproject-code: %s\nskills: %d (in %d practices)", p, projectCode, len(skills), len(vault.UniqueGroups(skills)))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vault_info",
		Description: "Show information about the practice vault: path, project-code, skill count, practice groups, and the latest practice-authoring version.",
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
		skills, err := vault.ListSkills(r)
		if err != nil {
			return nil, nil, err
		}
		groups := vault.UniqueGroups(skills)
		sort.Strings(groups)
		authoring := "not seeded"
		for _, s := range skills {
			if s.Name == vault.SeedName {
				authoring = s.LatestVersion
				break
			}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "vault: %s\nproject-code: %s\nskills: %d\npractices: %s\npractice-authoring: %s\n",
			p, projectCode, len(skills), strings.Join(groups, ", "), authoring)
		for _, s := range skills {
			fmt.Fprintf(&b, "  %s/%s  %s  %s\n", s.Group, s.Name, s.LatestVersion, s.Description)
		}
		return textResult(strings.TrimSuffix(b.String(), "\n")), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vault_practice_save",
		Description: "Save a skill to the vault as a new version. Validates the skill slug and frontmatter, commits, and tags the version (v1, v2, ... unless version_label is given).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name         string `json:"name" jsonschema:"skill name (slug, 1-64 lowercase alphanumeric with single hyphens) (required)"`
		Practice     string `json:"practice" jsonschema:"practice grouping folder, e.g. backend, frontend, workflows, meta (required)"`
		Content      string `json:"content" jsonschema:"full SKILL.md content including YAML frontmatter (required)"`
		VersionLabel string `json:"version_label,omitempty" jsonschema:"explicit version label; defaults to the next vN for this skill"`
		Message      string `json:"message,omitempty" jsonschema:"commit message describing the change"`
		User         string `json:"user,omitempty" jsonschema:"author to record; defaults to admin"`
		Path         string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" || args.Practice == "" || args.Content == "" {
			return nil, nil, errors.New("name, practice, and content are required")
		}
		if !vault.ValidSlug(args.Practice) {
			return nil, nil, fmt.Errorf("invalid practice %q: must be 1-64 lowercase alphanumeric with single hyphens", args.Practice)
		}
		if err := vault.ValidateContent(args.Name, args.Content); err != nil {
			return nil, nil, err
		}
		r, p, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		label, uuid, total, err := vault.SavePractice(r, args.Practice, args.Name, args.Content, args.Message, args.User, args.VersionLabel)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("saved %s/%s\nversion: %s (%s)\ntotal versions: %d\nvault: %s",
			args.Practice, args.Name, label, vault.ShortUUID(uuid), total, p)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vault_practice_get",
		Description: "Return the content of a skill at a given version (empty/tip, a vN label, or a UUID prefix).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name    string `json:"name" jsonschema:"skill name (required)"`
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
		content, err := vault.ReadSkillAtVersion(r, args.Name, args.Version)
		if err != nil {
			return nil, nil, err
		}
		return textResult(content), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vault_practice_list",
		Description: "List skills in the vault, optionally filtered by query, tag, or practice. Returns name, practice, latest version, and description.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Query    string `json:"query,omitempty" jsonschema:"case-insensitive substring match against name, description, and tags"`
		Tag      string `json:"tag,omitempty" jsonschema:"require this tag in the skill's metadata tags (comma-separated match)"`
		Practice string `json:"practice,omitempty" jsonschema:"only list skills under this practice group"`
		Path     string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		r, _, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		skills, err := vault.ListSkills(r)
		if err != nil {
			return nil, nil, err
		}
		q := strings.ToLower(args.Query)
		var matches []vault.SkillInfo
		for _, s := range skills {
			if args.Practice != "" && s.Group != args.Practice {
				continue
			}
			if args.Tag != "" {
				found := false
				for _, t := range strings.Split(s.Tags, ",") {
					if strings.TrimSpace(t) == args.Tag {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if q != "" {
				haystack := strings.ToLower(s.Name + " " + s.Description + " " + s.Tags)
				if !strings.Contains(haystack, q) {
					continue
				}
			}
			matches = append(matches, s)
		}
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].Group == matches[j].Group {
				return matches[i].Name < matches[j].Name
			}
			return matches[i].Group < matches[j].Group
		})
		if len(matches) == 0 {
			return textResult("no skills match"), nil, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d skill(s):\n", len(matches))
		for _, s := range matches {
			fmt.Fprintf(&b, "  %s/%s  %s  %s\n", s.Group, s.Name, s.LatestVersion, s.Description)
		}
		return textResult(strings.TrimSuffix(b.String(), "\n")), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vault_practice_history",
		Description: "List all versions of a skill, oldest first, with label, UUID, date, user, and message.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name string `json:"name" jsonschema:"skill name (required)"`
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
			return nil, nil, fmt.Errorf("skill %q not found in vault", args.Name)
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
		Name:        "vault_practice_diff",
		Description: "Unified diff of a skill between two versions. If version_a is empty, diffs against the previous version.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name     string `json:"name" jsonschema:"skill name (required)"`
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
			return nil, nil, fmt.Errorf("skill %q not found in vault", args.Name)
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
		entries, err := r.Diff(ridA, ridB, path)
		if err != nil {
			return nil, nil, err
		}
		if len(entries) == 0 {
			return textResult("no differences"), nil, nil
		}
		var b strings.Builder
		for _, e := range entries {
			fmt.Fprintf(&b, "%s\n", e.Unified)
		}
		return textResult(strings.TrimSuffix(b.String(), "\n")), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vault_practice_install",
		Description: "Install a skill as a SKILL.md in the target project with a version pin. Requires explicit user approval. Writes .opencode/skills/<name>/SKILL.md (or claude/agents) and a .vault-version pin.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name    string `json:"name" jsonschema:"skill name to install (required)"`
		Version string `json:"version,omitempty" jsonschema:"version to install (vN label or UUID prefix); defaults to latest"`
		Target  string `json:"target,omitempty" jsonschema:"project root directory; defaults to the current directory"`
		Format  string `json:"format,omitempty" jsonschema:"skill format: opencode, claude, or agents; defaults to opencode"`
		Force   bool   `json:"force,omitempty" jsonschema:"overwrite an existing SKILL.md"`
		Path    string `json:"path,omitempty" jsonschema:"vault repository path; defaults to TAOLU_REPO or ~/.taolu/vault.fossil"`
	}) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return nil, nil, errors.New("name is required")
		}
		if args.Format == "" {
			args.Format = "opencode"
		}
		r, p, err := vault.OpenVault(args.Path)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		rel, err := vault.InstallPractice(r, p, args.Name, args.Version, args.Target, args.Format, args.Force)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("installed %s to %s\nversion: %s\nvault: %s",
			args.Name, rel, versionLabelOrTip(args.Version), p)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vault_practice_export",
		Description: "Export the raw SKILL.md content of a skill at a given version (for review or copying elsewhere).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name    string `json:"name" jsonschema:"skill name (required)"`
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
		content, err := vault.ReadSkillAtVersion(r, args.Name, args.Version)
		if err != nil {
			return nil, nil, err
		}
		return textResult(content), nil, nil
	})
}
