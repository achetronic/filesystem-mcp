package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	//
	"github.com/mark3labs/mcp-go/mcp"
)

func (tm *ToolsManager) HandleDiff(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	pathA, ok := args["path_a"].(string)
	if !ok || pathA == "" {
		return toolError("path_a parameter is required"), nil
	}

	if err := sanitizePath(pathA); err != nil {
		return toolError(err.Error()), nil
	}

	pathB, ok := args["path_b"].(string)
	if !ok || pathB == "" {
		return toolError("path_b parameter is required"), nil
	}

	if err := sanitizePath(pathB); err != nil {
		return toolError(err.Error()), nil
	}

	absPathA, err := filepath.Abs(pathA)
	if err != nil {
		return toolError(fmt.Sprintf("invalid path_a: %s", err.Error())), nil
	}

	absPathB, err := filepath.Abs(pathB)
	if err != nil {
		return toolError(fmt.Sprintf("invalid path_b: %s", err.Error())), nil
	}

	if err := tm.dependencies.RBAC.Check("diff", []string{absPathA, absPathB}, nil); err != nil {
		return toolError(err.Error()), nil
	}

	linesA, err := readLines(absPathA)
	if err != nil {
		return toolError(fmt.Sprintf("failed to read %s: %s", absPathA, err.Error())), nil
	}

	linesB, err := readLines(absPathB)
	if err != nil {
		return toolError(fmt.Sprintf("failed to read %s: %s", absPathB, err.Error())), nil
	}

	startA := 0
	if v, ok := args["start_a"].(float64); ok {
		startA = int(v)
	}
	endA := len(linesA)
	if v, ok := args["end_a"].(float64); ok {
		endA = int(v)
	}

	startB := 0
	if v, ok := args["start_b"].(float64); ok {
		startB = int(v)
	}
	endB := len(linesB)
	if v, ok := args["end_b"].(float64); ok {
		endB = int(v)
	}

	if startA < 0 {
		startA = 0
	}
	if endA > len(linesA) {
		endA = len(linesA)
	}
	if startB < 0 {
		startB = 0
	}
	if endB > len(linesB) {
		endB = len(linesB)
	}

	sliceA := linesA[startA:endA]
	sliceB := linesB[startB:endB]

	diff := computeDiff(absPathA, absPathB, sliceA, sliceB, startA, startB)

	if diff == "" {
		return toolSuccess("Files are identical (in the specified ranges)"), nil
	}

	return toolSuccess(diff), nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
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
	return lines, scanner.Err()
}

func computeDiff(pathA, pathB string, linesA, linesB []string, offsetA, offsetB int) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "--- %s\n", pathA)
	fmt.Fprintf(&sb, "+++ %s\n", pathB)

	m := len(linesA)
	n := len(linesB)

	table := make([][]int, m+1)
	for i := range table {
		table[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if linesA[i-1] == linesB[j-1] {
				table[i][j] = table[i-1][j-1] + 1
			} else if table[i-1][j] >= table[i][j-1] {
				table[i][j] = table[i-1][j]
			} else {
				table[i][j] = table[i][j-1]
			}
		}
	}

	type diffLine struct {
		op   byte
		text string
		numA int
		numB int
	}

	var diffs []diffLine
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && linesA[i-1] == linesB[j-1] {
			diffs = append(diffs, diffLine{' ', linesA[i-1], offsetA + i - 1, offsetB + j - 1})
			i--
			j--
		} else if j > 0 && (i == 0 || table[i][j-1] >= table[i-1][j]) {
			diffs = append(diffs, diffLine{'+', linesB[j-1], -1, offsetB + j - 1})
			j--
		} else if i > 0 {
			diffs = append(diffs, diffLine{'-', linesA[i-1], offsetA + i - 1, -1})
			i--
		}
	}

	for left, right := 0, len(diffs)-1; left < right; left, right = left+1, right-1 {
		diffs[left], diffs[right] = diffs[right], diffs[left]
	}

	hasChanges := false
	for _, d := range diffs {
		if d.op != ' ' {
			hasChanges = true
			break
		}
	}

	if !hasChanges {
		return ""
	}

	for _, d := range diffs {
		fmt.Fprintf(&sb, "%c %s\n", d.op, d.text)
	}

	return sb.String()
}
