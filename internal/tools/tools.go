package tools

import (
	"mcp-forge/internal/globals"
	"mcp-forge/internal/middlewares"
	"mcp-forge/internal/rbac"
	"mcp-forge/internal/state"

	//
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolsManagerDependencies struct {
	AppCtx *globals.ApplicationContext

	McpServer   *server.MCPServer
	Middlewares []middlewares.ToolMiddleware
	RBAC        *rbac.Engine
	Undo        *state.UndoStore
	Scratch     *state.ScratchStore
	Processes   *state.ProcessStore
}

type ToolsManager struct {
	dependencies ToolsManagerDependencies
	toolPrefix   string
}

func NewToolsManager(deps ToolsManagerDependencies) *ToolsManager {
	return &ToolsManager{
		dependencies: deps,
		toolPrefix:   deps.AppCtx.ToolPrefix,
	}
}

func (tm *ToolsManager) toolName(base string) string {
	return tm.toolPrefix + base
}

func (tm *ToolsManager) AddTools() {

	// system_info
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("system_info"),
		mcp.WithDescription("Get system information: OS, architecture, hostname, user, working directory, environment variables"),
	), tm.HandleSystemInfo)

	// ls
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("ls"),
		mcp.WithDescription("List directory contents with optional depth, glob pattern filter, and hidden file inclusion. Use depth=1 for flat listing, depth>1 for tree view"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute or relative directory path to list. Must be a single concrete path — shell expansions like {a,b} are not supported"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Maximum depth to traverse (default: 1)"),
		),
		mcp.WithString("pattern",
			mcp.Description("Glob pattern to filter results (e.g. '*.go', '*.yaml')"),
		),
		mcp.WithBoolean("include_hidden",
			mcp.Description("Include hidden files and directories (default: false)"),
		),
	), tm.HandleLs)

	// read_file
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("read_file"),
		mcp.WithDescription("Read a file's contents. Supports reading specific line ranges to save tokens. Without ranges, reads the entire file"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute or relative file path to read. Must be a single concrete path — shell expansions like {a,b} are not supported"),
		),
		mcp.WithArray("ranges",
			mcp.Description("Array of {offset, limit} objects for partial reads. offset is 0-based line number, limit is number of lines"),
		),
	), tm.HandleReadFile)

	// write_file
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("write_file"),
		mcp.WithDescription("Create or overwrite a file. Automatically creates parent directories. Saves undo state before writing. Call this tool once per file — do not combine multiple files into one call"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute or relative file path to write. Must be a single concrete path — shell expansions like {a,b} are not supported"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Content to write to the file"),
		),
	), tm.HandleWriteFile)

	// edit_file
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("edit_file"),
		mcp.WithDescription("Apply one or more find-and-replace edits to a file. Edits are applied sequentially. Each edit must match exactly. Saves undo state before any edits. Reports which edits succeeded and which failed"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute or relative file path to edit. Must be a single concrete path — shell expansions like {a,b} are not supported"),
		),
		mcp.WithArray("edits",
			mcp.Required(),
			mcp.Description("Array of {old_text, new_text, replace_all} objects. old_text must match exactly. new_text is the replacement. replace_all (optional, default false) replaces all occurrences"),
		),
	), tm.HandleEditFile)

	// search
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("search"),
		mcp.WithDescription("Search for text patterns in files recursively. Returns matching file paths, line numbers, and content with configurable context"),
		mcp.WithString("pattern",
			mcp.Required(),
			mcp.Description("Search pattern (regex by default, or literal if literal=true)"),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Directory or file path to search in. Must be a single concrete path — shell expansions like {a,b} are not supported"),
		),
		mcp.WithString("include",
			mcp.Description("Glob pattern for files to include (e.g. '*.go')"),
		),
		mcp.WithString("exclude",
			mcp.Description("Glob pattern for files to exclude (e.g. '*.test')"),
		),
		mcp.WithBoolean("literal",
			mcp.Description("Treat pattern as literal text instead of regex (default: false)"),
		),
		mcp.WithNumber("context_lines",
			mcp.Description("Number of context lines before and after each match (default: 0)"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of matches to return (default: 100)"),
		),
	), tm.HandleSearch)

	// diff
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("diff"),
		mcp.WithDescription("Compare two files or sections of files. Returns unified diff format. Supports line ranges to compare specific sections"),
		mcp.WithString("path_a",
			mcp.Required(),
			mcp.Description("First file path. Must be a single concrete path — shell expansions like {a,b} are not supported"),
		),
		mcp.WithString("path_b",
			mcp.Required(),
			mcp.Description("Second file path. Must be a single concrete path — shell expansions like {a,b} are not supported"),
		),
		mcp.WithNumber("start_a",
			mcp.Description("Start line for first file (0-based, optional)"),
		),
		mcp.WithNumber("end_a",
			mcp.Description("End line for first file (exclusive, optional)"),
		),
		mcp.WithNumber("start_b",
			mcp.Description("Start line for second file (0-based, optional)"),
		),
		mcp.WithNumber("end_b",
			mcp.Description("End line for second file (exclusive, optional)"),
		),
	), tm.HandleDiff)

	// exec
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("exec"),
		mcp.WithDescription("Execute a shell command. Can run in foreground (returns output) or background (returns process ID). WARNING: grants full shell access — RBAC exec permission bypasses filesystem restrictions"),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("Shell command to execute"),
		),
		mcp.WithString("workdir",
			mcp.Description("Working directory for the command. Must be a single concrete path — shell expansions like {a,b} are not supported"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds for foreground commands (default: 30)"),
		),
		mcp.WithObject("env",
			mcp.Description("Environment variables as key-value pairs"),
		),
		mcp.WithBoolean("background",
			mcp.Description("Run in background and return process ID (default: false)"),
		),
	), tm.HandleExec)

	// process_status
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("process_status"),
		mcp.WithDescription("Get status and output of background processes. Without an ID, lists all background processes"),
		mcp.WithString("id",
			mcp.Description("Process ID to get status for (omit to list all)"),
		),
	), tm.HandleProcessStatus)

	// process_kill
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("process_kill"),
		mcp.WithDescription("Kill a background process"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Process ID to kill"),
		),
	), tm.HandleProcessKill)

	// undo
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("undo"),
		mcp.WithDescription("Undo the last write_file or edit_file operation on a specific file path. Restores the file to its state before the last modification"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("File path to undo changes for. Must be a single concrete path — shell expansions like {a,b} are not supported"),
		),
	), tm.HandleUndo)

	// scratch
	tm.dependencies.McpServer.AddTool(mcp.NewTool(tm.toolName("scratch"),
		mcp.WithDescription("In-memory key-value store for temporary data. Use to save snippets, plans, or intermediate results between tool calls without retransmitting them"),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform: 'set', 'get', 'delete', or 'list'"),
		),
		mcp.WithString("key",
			mcp.Description("Key name (required for set, get, delete)"),
		),
		mcp.WithString("value",
			mcp.Description("Value to store (required for set)"),
		),
	), tm.HandleScratch)
}
