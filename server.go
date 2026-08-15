package main

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "agent-vault"
	serverVersion = "0.1.0"
)

func newServer() (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	registerVaultTools(server)

	return server, nil
}

func newTransport() *mcp.StdioTransport {
	return &mcp.StdioTransport{}
}
