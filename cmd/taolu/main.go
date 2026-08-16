// Command taolu runs the versioned practice library as an MCP server.
package main

import (
	"context"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yli/taolu/pkg/tools"
)

const (
	serverName    = "taolu"
	serverVersion = "0.1.0"
)

func main() {
	server, err := newServer()
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
		os.Exit(1)
	}
}

func newServer() (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	tools.RegisterTaoluTools(server)

	return server, nil
}
