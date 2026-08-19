package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"charm.land/huh/v2"

	"github.com/yli/taolu/pkg/commands"
)

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	tool := fs.String("tool", "", "agent tool: opencode, claude, vscode, or all (non-interactive)")
	scope := fs.String("scope", "", "local or global (non-interactive, default: global)")
	transport := fs.String("transport", "", "http or stdio (non-interactive, default: http)")
	localDir := fs.String("local", "", "project directory for local scope (non-interactive)")
	force := fs.Bool("force", false, "overwrite existing command files")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: taolu install [options]

Install taolu slash commands for an agent tool.

Without flags, launches an interactive TUI to select options.

Options:
  --tool <tool>       agent tool: opencode, claude, vscode, or all
  --scope <scope>     local or global (default: global)
  --transport <mode>  http or stdio (default: http)
  --local <dir>       project directory for local scope
  --force             overwrite existing command files
  -h, --help          show this help message

Examples:
  taolu install                           Interactive TUI
  taolu install --tool opencode           Install for opencode (global, HTTP)
  taolu install --tool all --scope local  Install for all tools locally`)
	}
	fs.Parse(args)

	if *tool != "" {
		installNonInteractive(*tool, *scope, *transport, *localDir, *force)
		return
	}

	installTUI(*force)
}

func installNonInteractive(tool, scope, transport, localDir string, force bool) {
	if scope == "" {
		scope = "global"
	}
	if transport == "" {
		transport = commands.TransportHTTP
	}

	tools := resolveTools(tool)
	for _, t := range tools {
		opts := commands.InstallOptions{
			Tool:      t,
			Scope:     scope,
			Transport: transport,
			Target:    localDir,
			Force:     force,
		}
		written, err := commands.Install(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error installing for %s: %v\n", t, err)
			os.Exit(1)
		}
		for _, f := range written {
			fmt.Printf("  wrote %s\n", f)
		}
	}
}

func resolveTools(tool string) []string {
	if tool == "all" {
		return []string{"opencode", "claude", "vscode"}
	}
	return []string{tool}
}

func installTUI(force bool) {
	overwrite := force
	var (
		selectedTools []string
		scopeLocal    bool
		useHTTP       bool
		confirmed     bool
	)

	toolOptions := make([]huh.Option[string], len(commands.Tools))
	for i, t := range commands.Tools {
		toolOptions[i] = huh.NewOption(t.Label, t.ID)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select agent tools to install for").
				Description("Choose one or more agent tools.").
				Options(toolOptions...).
				Validate(func(s []string) error {
					if len(s) == 0 {
						return errors.New("select at least one tool")
					}
					return nil
				}).
				Value(&selectedTools),
		),
		huh.NewGroup(
			huh.NewSelect[bool]().
				Title("Install scope").
				Description("Global installs to your user config. Local installs to the current project.").
				Options(
					huh.NewOption("Global (all projects)", false),
					huh.NewOption("Local (this project only)", true),
				).
				Value(&scopeLocal),
		),
		huh.NewGroup(
			huh.NewSelect[bool]().
				Title("MCP connection mode").
				Description("HTTP requires 'taolu serve' running separately. Stdio is self-contained.").
				Options(
					huh.NewOption("HTTP server (recommended for global)", true),
					huh.NewOption("Stdio (agent starts server automatically)", false),
				).
				Value(&useHTTP),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Update existing command files?").
				Description("Existing commands keep the old templates. Choose update to overwrite them with the latest ones.").
				Affirmative("Update existing").
				Negative("Keep existing").
				Value(&overwrite),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Install now?").
				Description("This will write command files and merge MCP config.").
				Affirmative("Install").
				Negative("Cancel").
				Value(&confirmed),
		),
	)

	if err := form.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if !confirmed {
		fmt.Println("Cancelled.")
		return
	}

	scope := "global"
	if scopeLocal {
		scope = "local"
	}
	transport := commands.TransportHTTP
	if !useHTTP {
		transport = commands.TransportStdio
	}

	for _, toolID := range selectedTools {
		opts := commands.InstallOptions{
			Tool:      toolID,
			Scope:     scope,
			Transport: transport,
			Force:     overwrite,
		}
		written, err := commands.Install(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error installing for %s: %v\n", toolID, err)
			os.Exit(1)
		}
		t, _ := commands.FindTool(toolID)
		fmt.Printf("\n%s:\n", t.Label)
		for _, f := range written {
			fmt.Printf("  wrote %s\n", f)
		}
	}

	fmt.Println("\nDone. Start the server with: taolu serve")
}
