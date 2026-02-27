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

type editOperation struct {
	OldText    string `json:"old_text"`
	NewText    string `json:"new_text"`
	ReplaceAll bool   `json:"replace_all"`
}

type editResult struct {
	Index   int    `json:"index"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (tm *ToolsManager) HandleEditFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	if err := tm.dependencies.RBAC.Check("edit_file", []string{absPath}, jwtPayloadFromCtx(ctx)); err != nil {
		return toolError(err.Error()), nil
	}

	rawEdits, ok := args["edits"]
	if !ok || rawEdits == nil {
		return toolError("edits parameter is required"), nil
	}

	editsJSON, err := json.Marshal(rawEdits)
	if err != nil {
		return toolError(fmt.Sprintf("invalid edits parameter: %s", err.Error())), nil
	}

	var edits []editOperation
	if err := json.Unmarshal(editsJSON, &edits); err != nil {
		return toolError(fmt.Sprintf("invalid edits format: %s", err.Error())), nil
	}

	if len(edits) == 0 {
		return toolError("edits array is empty"), nil
	}

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		return toolError(fmt.Sprintf("failed to read file: %s", err.Error())), nil
	}

	if err := tm.dependencies.Undo.Save(absPath); err != nil {
		tm.dependencies.AppCtx.Logger.Error("failed to save undo state", "path", absPath, "error", err.Error())
	}

	content := string(contentBytes)
	results := make([]editResult, len(edits))
	appliedCount := 0

	for i, edit := range edits {
		results[i] = editResult{Index: i}

		if edit.OldText == "" {
			results[i].Error = "old_text cannot be empty"
			continue
		}

		if !strings.Contains(content, edit.OldText) {
			results[i].Error = "old_text not found in file"
			continue
		}

		if !edit.ReplaceAll {
			count := strings.Count(content, edit.OldText)
			if count > 1 {
				results[i].Error = fmt.Sprintf("old_text matches %d locations; use replace_all=true or provide more context", count)
				continue
			}
		}

		if edit.ReplaceAll {
			content = strings.ReplaceAll(content, edit.OldText, edit.NewText)
		} else {
			content = strings.Replace(content, edit.OldText, edit.NewText, 1)
		}

		results[i].Success = true
		appliedCount++
	}

	if appliedCount > 0 {
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return toolError(fmt.Sprintf("failed to write file after edits: %s", err.Error())), nil
		}
	}

	jsonBytes, err := json.MarshalIndent(map[string]interface{}{
		"path":          absPath,
		"edits_applied": appliedCount,
		"edits_failed":  len(edits) - appliedCount,
		"results":       results,
	}, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to marshal results: %s", err.Error())), nil
	}

	if appliedCount < len(edits) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(jsonBytes),
				},
			},
			IsError: appliedCount == 0,
		}, nil
	}

	return toolSuccess(string(jsonBytes)), nil
}
