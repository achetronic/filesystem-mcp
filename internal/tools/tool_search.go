package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

type searchMatch struct {
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Content    string   `json:"content"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

func (tm *ToolsManager) HandleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return toolError("pattern parameter is required"), nil
	}

	searchPath, ok := args["path"].(string)
	if !ok || searchPath == "" {
		return toolError("path parameter is required"), nil
	}

	if err := sanitizePath(searchPath); err != nil {
		return toolError(err.Error()), nil
	}

	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		return toolError(fmt.Sprintf("invalid path: %s", err.Error())), nil
	}

	if err := tm.dependencies.RBAC.Check("search", []string{absPath}, jwtPayloadFromCtx(ctx)); err != nil {
		return toolError(err.Error()), nil
	}

	include := ""
	if v, ok := args["include"].(string); ok {
		include = v
	}

	exclude := ""
	if v, ok := args["exclude"].(string); ok {
		exclude = v
	}

	literal := false
	if v, ok := args["literal"].(bool); ok {
		literal = v
	}

	contextLines := 0
	if v, ok := args["context_lines"].(float64); ok {
		contextLines = int(v)
	}

	maxResults := 100
	if v, ok := args["max_results"].(float64); ok && v > 0 {
		maxResults = int(v)
	}

	if literal {
		pattern = regexp.QuoteMeta(pattern)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return toolError(fmt.Sprintf("invalid regex pattern: %s", err.Error())), nil
	}

	var matches []searchMatch

	err = filepath.Walk(absPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if include != "" {
			matched, _ := filepath.Match(include, info.Name())
			if !matched {
				return nil
			}
		}

		if exclude != "" {
			matched, _ := filepath.Match(exclude, info.Name())
			if matched {
				return nil
			}
		}

		if len(matches) >= maxResults {
			return filepath.SkipAll
		}

		fileMatches, err := searchInFile(filePath, re, contextLines, maxResults-len(matches))
		if err != nil {
			return nil
		}

		matches = append(matches, fileMatches...)
		return nil
	})

	if err != nil {
		return toolError(fmt.Sprintf("search error: %s", err.Error())), nil
	}

	result := map[string]interface{}{
		"matches":       matches,
		"total_matches": len(matches),
		"truncated":     len(matches) >= maxResults,
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to marshal results: %s", err.Error())), nil
	}

	return toolSuccess(string(jsonBytes)), nil
}

func searchInFile(filePath string, re *regexp.Regexp, contextLines int, limit int) ([]searchMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	var matches []searchMatch

	for i, line := range lines {
		if len(matches) >= limit {
			break
		}

		if !re.MatchString(line) {
			continue
		}

		match := searchMatch{
			File:    filePath,
			Line:    i,
			Content: strings.TrimRight(line, "\r\n"),
		}

		if contextLines > 0 {
			start := i - contextLines
			if start < 0 {
				start = 0
			}
			for j := start; j < i; j++ {
				match.ContextBefore = append(match.ContextBefore, fmt.Sprintf("%d: %s", j, lines[j]))
			}

			end := i + contextLines + 1
			if end > len(lines) {
				end = len(lines)
			}
			for j := i + 1; j < end; j++ {
				match.ContextAfter = append(match.ContextAfter, fmt.Sprintf("%d: %s", j, lines[j]))
			}
		}

		matches = append(matches, match)
	}

	return matches, nil
}
