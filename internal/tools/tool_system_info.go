package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"runtime"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

func (tm *ToolsManager) HandleSystemInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	currentUser, _ := user.Current()
	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()

	info := map[string]interface{}{
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"hostname": hostname,
		"user":     currentUser.Username,
		"home":     currentUser.HomeDir,
		"cwd":      cwd,
		"shell":    os.Getenv("SHELL"),
		"path":     os.Getenv("PATH"),
	}

	jsonBytes, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to marshal system info: %s", err.Error())), nil
	}

	return toolSuccess(string(jsonBytes)), nil
}
