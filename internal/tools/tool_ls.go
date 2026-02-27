package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

type lsEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Type    string    `json:"type"`
	Size    int64     `json:"size,omitempty"`
	Mode    string    `json:"mode"`
	ModTime string    `json:"mod_time"`
	Children []lsEntry `json:"children,omitempty"`
}

func (tm *ToolsManager) HandleLs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	if err := tm.dependencies.RBAC.Check("ls", []string{absPath}, jwtPayloadFromCtx(ctx)); err != nil {
		return toolError(err.Error()), nil
	}

	depth := 1
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}

	pattern := ""
	if p, ok := args["pattern"].(string); ok {
		pattern = p
	}

	includeHidden := false
	if h, ok := args["include_hidden"].(bool); ok {
		includeHidden = h
	}

	entries, err := listDir(absPath, depth, 0, pattern, includeHidden)
	if err != nil {
		return toolError(fmt.Sprintf("failed to list directory: %s", err.Error())), nil
	}

	jsonBytes, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to marshal results: %s", err.Error())), nil
	}

	return toolSuccess(string(jsonBytes)), nil
}

func listDir(dirPath string, maxDepth int, currentDepth int, pattern string, includeHidden bool) ([]lsEntry, error) {
	if currentDepth >= maxDepth {
		return nil, nil
	}

	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var entries []lsEntry
	for _, de := range dirEntries {
		name := de.Name()

		if !includeHidden && strings.HasPrefix(name, ".") {
			continue
		}

		if pattern != "" {
			matched, _ := filepath.Match(pattern, name)
			if !matched && !de.IsDir() {
				continue
			}
		}

		fullPath := filepath.Join(dirPath, name)
		info, err := de.Info()
		if err != nil {
			continue
		}

		entry := lsEntry{
			Name:    name,
			Path:    fullPath,
			Type:    "file",
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		}

		if de.IsDir() {
			entry.Type = "directory"
			entry.Size = 0

			children, err := listDir(fullPath, maxDepth, currentDepth+1, pattern, includeHidden)
			if err == nil && len(children) > 0 {
				entry.Children = children
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
