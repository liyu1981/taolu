package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func versionLabelOrTip(v string) string {
	if v == "" {
		return "tip"
	}
	return v
}
