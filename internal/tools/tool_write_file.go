package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

func (tm *ToolsManager) HandleWriteFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	path, ok := args["path"].(string)
	if !ok || path == "" {
		return toolError("path parameter is required"), nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return toolError("content parameter is required"), nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return toolError(fmt.Sprintf("invalid path: %s", err.Error())), nil
	}

	if err := tm.dependencies.RBAC.Check("write_file", []string{absPath}, nil); err != nil {
		return toolError(err.Error()), nil
	}

	if err := tm.dependencies.Undo.Save(absPath); err != nil {
		tm.dependencies.AppCtx.Logger.Error("failed to save undo state", "path", absPath, "error", err.Error())
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return toolError(fmt.Sprintf("failed to create parent directories: %s", err.Error())), nil
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return toolError(fmt.Sprintf("failed to write file: %s", err.Error())), nil
	}

	info, _ := os.Stat(absPath)
	return toolSuccess(fmt.Sprintf("Written %d bytes to %s", info.Size(), absPath)), nil
}
