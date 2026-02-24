package tools

import (
	"context"
	"fmt"
	"path/filepath"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

func (tm *ToolsManager) HandleUndo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	path, ok := args["path"].(string)
	if !ok || path == "" {
		return toolError("path parameter is required"), nil
	}

	if err := sanitizePath(path); err != nil {
		return toolError(err.Error()), nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return toolError(fmt.Sprintf("invalid path: %s", err.Error())), nil
	}

	if err := tm.dependencies.RBAC.Check("undo", []string{absPath}, nil); err != nil {
		return toolError(err.Error()), nil
	}

	if err := tm.dependencies.Undo.Restore(absPath); err != nil {
		return toolError(err.Error()), nil
	}

	return toolSuccess(fmt.Sprintf("Restored %s to previous state", absPath)), nil
}
