package web

import "github.com/yli/taolu/pkg/version"

const (
	// serverName mirrors the MCP server identity so the status view reports
	// the same build the user is talking to.
	serverName = "taolu"
)

// serverVersion returns the build version from the shared version package.
func serverVersion() string {
	return version.Version
}
