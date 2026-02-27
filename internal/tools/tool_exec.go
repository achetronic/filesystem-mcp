package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

func (tm *ToolsManager) HandleExec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	command, ok := args["command"].(string)
	if !ok || command == "" {
		return toolError("command parameter is required"), nil
	}

	workdir := ""
	if v, ok := args["workdir"].(string); ok && v != "" {
		if err := sanitizePath(v); err != nil {
			return toolError(err.Error()), nil
		}
		absWorkdir, err := filepath.Abs(v)
		if err != nil {
			return toolError(fmt.Sprintf("invalid workdir: %s", err.Error())), nil
		}
		workdir = absWorkdir
	}

	if err := tm.dependencies.RBAC.Check("exec", []string{resolveExecPath(workdir)}, jwtPayloadFromCtx(ctx)); err != nil {
		return toolError(err.Error()), nil
	}

	timeout := 30 * time.Second
	if v, ok := args["timeout"].(float64); ok && v > 0 {
		timeout = time.Duration(v) * time.Second
	}

	var env []string
	if v, ok := args["env"].(map[string]interface{}); ok {
		env = os.Environ()
		for key, val := range v {
			env = append(env, fmt.Sprintf("%s=%v", key, val))
		}
	}

	background := false
	if v, ok := args["background"].(bool); ok {
		background = v
	}

	if background {
		id, err := tm.dependencies.Processes.Start(command, workdir, env)
		if err != nil {
			return toolError(fmt.Sprintf("failed to start background process: %s", err.Error())), nil
		}

		result := map[string]string{
			"id":      id,
			"status":  "running",
			"command": command,
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		return toolSuccess(string(jsonBytes)), nil
	}

	stdout, stderr, exitCode, err := tm.dependencies.Processes.Exec(command, workdir, env, timeout)
	if err != nil {
		return toolError(fmt.Sprintf("command failed: %s\nstdout: %s\nstderr: %s", err.Error(), stdout, stderr)), nil
	}

	result := map[string]interface{}{
		"exit_code": exitCode,
		"stdout":    stdout,
		"stderr":    stderr,
	}
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")

	if exitCode != 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(jsonBytes),
				},
			},
			IsError: true,
		}, nil
	}

	return toolSuccess(string(jsonBytes)), nil
}

func resolveExecPath(workdir string) string {
	if workdir != "" {
		return workdir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "/"
	}
	return cwd
}
