package tools

import (
	"context"
	"encoding/json"
	"fmt"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

func (tm *ToolsManager) HandleScratch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	action, ok := args["action"].(string)
	if !ok || action == "" {
		return toolError("action parameter is required (set, get, delete, list)"), nil
	}

	switch action {
	case "set":
		key, ok := args["key"].(string)
		if !ok || key == "" {
			return toolError("key parameter is required for set"), nil
		}
		value, ok := args["value"].(string)
		if !ok {
			return toolError("value parameter is required for set"), nil
		}
		tm.dependencies.Scratch.Set(key, value)
		return toolSuccess(fmt.Sprintf("Stored key %q", key)), nil

	case "get":
		key, ok := args["key"].(string)
		if !ok || key == "" {
			return toolError("key parameter is required for get"), nil
		}
		value, err := tm.dependencies.Scratch.Get(key)
		if err != nil {
			return toolError(err.Error()), nil
		}
		return toolSuccess(value), nil

	case "delete":
		key, ok := args["key"].(string)
		if !ok || key == "" {
			return toolError("key parameter is required for delete"), nil
		}
		tm.dependencies.Scratch.Delete(key)
		return toolSuccess(fmt.Sprintf("Deleted key %q", key)), nil

	case "list":
		data := tm.dependencies.Scratch.List()
		jsonBytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return toolError(fmt.Sprintf("failed to marshal scratch data: %s", err.Error())), nil
		}
		return toolSuccess(string(jsonBytes)), nil

	default:
		return toolError(fmt.Sprintf("unknown action %q (valid: set, get, delete, list)", action)), nil
	}
}
