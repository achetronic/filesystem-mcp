package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

type readRange struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type readFragment struct {
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
	Lines  []string `json:"lines"`
	Total  int      `json:"total_lines"`
}

func (tm *ToolsManager) HandleReadFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	if err := tm.dependencies.RBAC.Check("read_file", []string{absPath}, nil); err != nil {
		return toolError(err.Error()), nil
	}

	file, err := os.Open(absPath)
	if err != nil {
		return toolError(fmt.Sprintf("failed to open file: %s", err.Error())), nil
	}
	defer file.Close()

	var allLines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return toolError(fmt.Sprintf("failed to read file: %s", err.Error())), nil
	}
	totalLines := len(allLines)

	var ranges []readRange
	if rawRanges, ok := args["ranges"]; ok && rawRanges != nil {
		rangesJSON, err := json.Marshal(rawRanges)
		if err != nil {
			return toolError(fmt.Sprintf("invalid ranges parameter: %s", err.Error())), nil
		}
		if err := json.Unmarshal(rangesJSON, &ranges); err != nil {
			return toolError(fmt.Sprintf("invalid ranges format: %s", err.Error())), nil
		}
	}

	if len(ranges) == 0 {
		var sb strings.Builder
		for i, line := range allLines {
			fmt.Fprintf(&sb, "%d: %s\n", i, line)
		}
		return toolSuccess(fmt.Sprintf("(%d lines)\n%s", totalLines, sb.String())), nil
	}

	var fragments []readFragment
	for _, r := range ranges {
		offset := r.Offset
		if offset < 0 {
			offset = 0
		}
		if offset >= totalLines {
			offset = totalLines
		}

		limit := r.Limit
		if limit <= 0 {
			limit = totalLines - offset
		}
		end := offset + limit
		if end > totalLines {
			end = totalLines
		}

		fragment := readFragment{
			Offset: offset,
			Limit:  end - offset,
			Lines:  make([]string, 0, end-offset),
			Total:  totalLines,
		}

		for i := offset; i < end; i++ {
			fragment.Lines = append(fragment.Lines, fmt.Sprintf("%d: %s", i, allLines[i]))
		}

		fragments = append(fragments, fragment)
	}

	jsonBytes, err := json.MarshalIndent(fragments, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to marshal fragments: %s", err.Error())), nil
	}

	return toolSuccess(string(jsonBytes)), nil
}
