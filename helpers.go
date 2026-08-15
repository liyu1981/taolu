package main

import (
	_ "github.com/danmestas/go-libfossil/db/driver/modernc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func shortUUID(uuid string) string {
	if len(uuid) > 12 {
		return uuid[:12]
	}
	return uuid
}
