package tools

import (
	"context"
	"encoding/json"
	"fmt"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

func (tm *ToolsManager) HandleProcessStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, _ := args["id"].(string)

	if err := tm.dependencies.RBAC.Check("process_status", []string{}, nil); err != nil {
		return toolError(err.Error()), nil
	}

	if id == "" {
		processes := tm.dependencies.Processes.List()

		type processEntry struct {
			ID        string `json:"id"`
			Command   string `json:"command"`
			WorkDir   string `json:"workdir"`
			StartedAt string `json:"started_at"`
			Done      bool   `json:"done"`
			ExitCode  int    `json:"exit_code"`
		}

		entries := make([]processEntry, 0, len(processes))
		for _, p := range processes {
			entries = append(entries, processEntry{
				ID:        p.ID,
				Command:   p.Command,
				WorkDir:   p.WorkDir,
				StartedAt: p.StartedAt.Format("2006-01-02 15:04:05"),
				Done:      p.Done,
				ExitCode:  p.ExitCode,
			})
		}

		jsonBytes, _ := json.MarshalIndent(map[string]interface{}{
			"processes": entries,
			"total":     len(entries),
		}, "", "  ")
		return toolSuccess(string(jsonBytes)), nil
	}

	info, stdout, stderr, err := tm.dependencies.Processes.Status(id)
	if err != nil {
		return toolError(err.Error()), nil
	}

	result := map[string]interface{}{
		"id":         info.ID,
		"command":    info.Command,
		"workdir":    info.WorkDir,
		"started_at": info.StartedAt.Format("2006-01-02 15:04:05"),
		"done":       info.Done,
		"exit_code":  info.ExitCode,
		"stdout":     stdout,
		"stderr":     stderr,
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return toolSuccess(string(jsonBytes)), nil
}

func (tm *ToolsManager) HandleProcessKill(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, ok := args["id"].(string)
	if !ok || id == "" {
		return toolError("id parameter is required"), nil
	}

	if err := tm.dependencies.RBAC.Check("process_kill", []string{}, nil); err != nil {
		return toolError(err.Error()), nil
	}

	if err := tm.dependencies.Processes.Kill(id, ""); err != nil {
		return toolError(fmt.Sprintf("failed to kill process: %s", err.Error())), nil
	}

	return toolSuccess(fmt.Sprintf("Process %s killed", id)), nil
}
