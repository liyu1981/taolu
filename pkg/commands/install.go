package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Tool describes an agent tool's config layout.
type Tool struct {
	ID          string // "opencode", "claude", "vscode"
	Label       string // human-readable
	CommandsDir string // relative path for command files under config root
	ConfigFile  string // relative path for MCP config under config root
	MCPKey      string // top-level JSON key for MCP servers
}

// Tools is the list of supported agent tools.
var Tools = []Tool{
	{
		ID:          "opencode",
		Label:       "OpenCode",
		CommandsDir: "commands",
		ConfigFile:  "opencode.json",
		MCPKey:      "mcp",
	},
	{
		ID:          "claude",
		Label:       "Claude Desktop",
		CommandsDir: "commands",
		ConfigFile:  "claude_desktop_config.json",
		MCPKey:      "mcpServers",
	},
	{
		ID:          "vscode",
		Label:       "VS Code",
		CommandsDir: "commands",
		ConfigFile:  "mcp.json",
		MCPKey:      "servers",
	},
}

// Transport mode for the MCP connection.
const (
	TransportHTTP  = "http"
	TransportStdio = "stdio"
)

// DefaultPort is the default taolu MCP server port.
const DefaultPort = 8264

// InstallOptions configures a install run.
type InstallOptions struct {
	Tool      string // tool ID: "opencode", "claude", "vscode", or "all"
	Scope     string // "local" or "global"
	Transport string // "http" or "stdio"
	Target    string // project root for local scope, ignored for global
	Force     bool   // overwrite existing command files
	Port      int    // MCP server port (default DefaultPort)
	RepoPath  string // vault path for stdio mode
}

// FindTool returns the Tool for the given ID.
func FindTool(id string) (*Tool, error) {
	for i := range Tools {
		if Tools[i].ID == id {
			return &Tools[i], nil
		}
	}
	return nil, fmt.Errorf("unknown tool %q (supported: opencode, claude, vscode)", id)
}

// globalConfigDir returns the tool-specific global config directory.
func globalConfigDir(t *Tool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	switch t.ID {
	case "opencode":
		return filepath.Join(home, ".config", "opencode"), nil
	case "claude":
		return filepath.Join(home, ".config", "Claude"), nil
	case "vscode":
		return filepath.Join(home, ".config", "Code", "User"), nil
	}
	return "", fmt.Errorf("no global config directory for tool %q", t.ID)
}

// localConfigDir returns the project-root-relative config directory for a tool.
func localConfigDir(t *Tool, projectRoot string) string {
	switch t.ID {
	case "opencode":
		return filepath.Join(projectRoot, ".opencode")
	case "claude":
		return filepath.Join(projectRoot, ".claude")
	case "vscode":
		return filepath.Join(projectRoot, ".vscode")
	}
	return filepath.Join(projectRoot, "."+t.ID)
}

// Install writes command files and merges MCP config for the given tool.
// It returns the list of files written or modified.
func Install(opts InstallOptions) ([]string, error) {
	if opts.Port == 0 {
		opts.Port = DefaultPort
	}
	if opts.Transport == "" {
		opts.Transport = TransportHTTP
	}

	t, err := FindTool(opts.Tool)
	if err != nil {
		return nil, err
	}

	// Resolve base directory.
	var base string
	if opts.Scope == "local" {
		target := opts.Target
		if target == "" {
			target, err = os.Getwd()
			if err != nil {
				return nil, err
			}
		}
		base = localConfigDir(t, target)
	} else {
		base, err = globalConfigDir(t)
		if err != nil {
			return nil, err
		}
	}

	var written []string

	// Step 1: Write command files.
	cmdDir := filepath.Join(base, t.CommandsDir)
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		return nil, fmt.Errorf("create commands dir: %w", err)
	}
	for _, name := range CommandNames {
		dest := filepath.Join(cmdDir, name+".md")
		if !opts.Force {
			if _, err := os.Stat(dest); err == nil {
				continue // skip existing
			}
		}
		content := Commands[name]
		if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
		written = append(written, dest)
	}

	// Step 2: Merge MCP config.
	configPath := filepath.Join(base, t.ConfigFile)
	serverConfig := buildMCPEntry(t, opts.Transport, opts.Port, opts.RepoPath)
	configWritten, err := mergeMCPConfig(configPath, t.MCPKey, "taolu", serverConfig)
	if err != nil {
		return nil, fmt.Errorf("merge MCP config: %w", err)
	}
	if configWritten {
		written = append(written, configPath)
	}

	return written, nil
}

// buildMCPEntry creates the MCP server config entry for the given tool and transport.
func buildMCPEntry(t *Tool, transport string, port int, repoPath string) any {
	if t.ID == "opencode" {
		return buildOpenCodeEntry(transport, port, repoPath)
	}
	if t.ID == "claude" {
		return buildClaudeEntry(repoPath)
	}
	if t.ID == "vscode" {
		return buildVSCodeEntry(port)
	}
	return nil
}

func buildOpenCodeEntry(transport string, port int, repoPath string) map[string]any {
	if transport == TransportStdio {
		cmd := []string{"taolu", "serve", "--with=stdio"}
		entry := map[string]any{
			"type":    "local",
			"command": cmd,
			"enabled": true,
		}
		if repoPath != "" {
			entry["environment"] = map[string]string{
				"TAOLU_REPO": repoPath,
			}
		}
		return entry
	}
	return map[string]any{
		"type":    "remote",
		"url":     fmt.Sprintf("http://127.0.0.1:%d", port),
		"enabled": true,
	}
}

func buildClaudeEntry(repoPath string) map[string]any {
	entry := map[string]any{
		"command": "taolu",
		"args":    []string{"serve", "--with=stdio"},
	}
	if repoPath != "" {
		entry["env"] = map[string]string{
			"TAOLU_REPO": repoPath,
		}
	}
	return entry
}

func buildVSCodeEntry(port int) map[string]any {
	return map[string]any{
		"type": "http",
		"url":  fmt.Sprintf("http://127.0.0.1:%d", port),
	}
}

// mergeMCPConfig reads an existing JSON config file (or starts empty), ensures
// the MCP key exists, sets the server entry, and writes back. It returns true
// if the file was modified.
func mergeMCPConfig(path, mcpKey, serverName string, serverConfig any) (bool, error) {
	var root map[string]any

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return false, fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	if root == nil {
		root = make(map[string]any)
	}

	// Ensure the MCP key exists as an object.
	mcp, _ := root[mcpKey].(map[string]any)
	if mcp == nil {
		mcp = make(map[string]any)
		root[mcpKey] = mcp
	}

	// Check if already configured identically.
	existing, _ := json.Marshal(mcp[serverName])
	proposed, _ := json.Marshal(serverConfig)
	if string(existing) == string(proposed) {
		return false, nil // no change needed
	}

	mcp[serverName] = serverConfig

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal config: %w", err)
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create config dir: %w", err)
	}

	// Preserve trailing newline if original had one.
	suffix := "\n"
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		suffix = ""
	}

	if err := os.WriteFile(path, append(out, []byte(suffix)...), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}

	return true, nil
}
