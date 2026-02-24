package tools

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func sanitizePath(path string) error {
	openIdx := strings.Index(path, "{")
	if openIdx == -1 {
		return nil
	}
	closeIdx := strings.Index(path[openIdx:], "}")
	if closeIdx == -1 {
		return fmt.Errorf("path contains an unclosed brace expansion pattern: %s", path)
	}
	inner := path[openIdx+1 : openIdx+closeIdx]
	if strings.Contains(inner, ",") {
		return fmt.Errorf("path contains a shell brace expansion pattern: %s — expand it into individual paths before calling this tool", path)
	}
	return nil
}

func toolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: msg,
			},
		},
		IsError: true,
	}
}

func toolSuccess(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: msg,
			},
		},
	}
}
