# AGENTS.md

> Guide for AI agents working in this repository.

## Project Overview

**Filesystem MCP** is a production-ready MCP server in Go that gives AI agents filesystem control — reading, writing, editing, searching, diffing files, executing shell commands, and managing background processes. Includes OAuth authorization (RFC 8414 / RFC 9728), JWT validation, and path-based RBAC with CEL expressions.

- **Go module name**: `mcp-forge`
- **Go version**: 1.24+ (see `go.mod`)
- **Primary dependency**: [`mcp-go`](https://github.com/mark3labs/mcp-go)

## Commands

| Task | Command | Notes |
|------|---------|-------|
| **Build** | `make build` | Runs `go fmt`, `go vet`, then builds to `bin/mcp-forge-{os}-{arch}` |
| **Run (HTTP)** | `make run` | Runs with `docs/config-http.yaml` |
| **Format** | `make fmt` | `go fmt ./...` |
| **Vet** | `make vet` | `go vet ./...` |
| **Lint** | `make lint` | Uses golangci-lint (auto-downloads to `bin/`) |
| **Docker build** | `make docker-build` | Builds image |
| **Docker push** | `make docker-push` | Pushes image |

There are **no tests** currently. When adding tests, use `go test ./...`.

## Project Structure

```
cmd/
  main.go                       # Entrypoint: wires config, RBAC, state, middlewares, tools, transport
api/
  config_types.go               # All YAML-mapped configuration structs including RBAC
internal/
  globals/
    globals.go                  # ApplicationContext: context, logger, parsed config
  config/
    config.go                   # YAML config reading with env var expansion
  rbac/
    rbac.go                     # RBAC engine: CEL compilation, path glob matching, operation checking
  state/
    undo.go                     # UndoStore: saves/restores file state before writes
    scratch.go                  # ScratchStore: in-memory key-value for agent temp data
    processes.go                # ProcessStore: background process management with output capture
  tools/
    tools.go                    # ToolsManager: registers all 12 MCP tools
    helpers.go                  # toolError() and toolSuccess() response helpers
    tool_system_info.go         # system_info tool
    tool_ls.go                  # ls tool (recursive, glob filter, hidden files)
    tool_read_file.go           # read_file tool (multi-range partial reads)
    tool_write_file.go          # write_file tool (auto mkdir, undo save)
    tool_edit_file.go           # edit_file tool (batch find/replace)
    tool_search.go              # search tool (regex/literal, context, include/exclude)
    tool_diff.go                # diff tool (unified diff with line ranges)
    tool_exec.go                # exec tool (foreground with timeout, background with ID)
    tool_processes.go           # process_status and process_kill tools
    tool_undo.go                # undo tool
    tool_scratch.go             # scratch tool (set/get/delete/list)
  handlers/
    handlers.go                 # HandlersManager for HTTP endpoints
    oauth_authorization_server.go
    oauth_protected_resource.go
  middlewares/
    interfaces.go               # ToolMiddleware and HttpMiddleware interfaces
    jwt_validation.go           # JWT validation HTTP middleware
    jwt_validation_utils.go     # JWKS caching, JWK-to-key conversion
    logging.go                  # Access logs middleware
    noop.go                     # No-op tool middleware
    utils.go                    # HTTP helpers (scheme detection)
docs/
  config-http.yaml              # Full config: JWT, RBAC, OAuth
  config-stdio.yaml             # Minimal config for local use
chart/                          # Helm chart (bjw-s app-template)
Dockerfile                      # Multi-stage: golang:1.24 → distroless:nonroot
```

## Architecture & Patterns

### ApplicationContext Pattern

All components receive `*globals.ApplicationContext` containing:
- `Context` (`context.Context`)
- `Logger` (`*slog.Logger` — JSON to stderr)
- `Config` (`*api.Configuration`)

Created once in `main()` and passed through dependency structs.

### Dependency Injection via Structs

Every component follows:
1. `XxxDependencies` struct with injected deps
2. `Xxx` struct with `dependencies` field
3. `NewXxx(deps XxxDependencies) *Xxx` constructor

### Shared State

Three state stores are created in `main()` and injected into `ToolsManager`:
- **UndoStore** — saves file content before writes, restores on undo
- **ScratchStore** — thread-safe key-value map for agent temp data
- **ProcessStore** — manages background processes, captures output

### RBAC Engine

- Compiled at startup from config rules
- CEL expressions precompiled for performance
- Called by every tool handler before executing: `tm.dependencies.RBAC.Check(toolName, paths, jwtPayload)`
- Three operation categories: `read`, `write`, `exec`
- `system_info` and `scratch` are always allowed (path-free)

### Tool Handler Pattern

Each tool lives in `tool_{name}.go` as a method on `ToolsManager`:
```go
func (tm *ToolsManager) HandleToolName(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
```

Use `toolSuccess(msg)` and `toolError(msg)` helpers for responses. Arguments come from `request.GetArguments()` as `map[string]interface{}` — cast with type assertions.

### Import Style

Imports use bare `//` comment separators between groups:
```go
import (
    "context"
    "fmt"

    //
    "mcp-forge/internal/globals"

    //
    "github.com/mark3labs/mcp-go/mcp"
)
```

## Adding a New Tool

1. Create `internal/tools/tool_{name}.go`
2. Add handler method on `ToolsManager`
3. Register in `AddTools()` in `tools.go` with `mcp.NewTool(...)` + `McpServer.AddTool(...)`
4. If the tool touches filesystem paths, add RBAC check:
   ```go
   if err := tm.dependencies.RBAC.Check("tool_name", []string{absPath}, nil); err != nil {
       return toolError(err.Error()), nil
   }
   ```
5. Map the tool name to an operation category in `rbac.go` `operationCategory` map, OR add to `pathFreeTools` if it doesn't touch paths

## Adding Configuration Fields

1. Add struct/field in `api/config_types.go` with `yaml:"field_name"` tags
2. Reference via `appCtx.Config.YourSection.YourField`
3. Update both `docs/config-http.yaml` and `docs/config-stdio.yaml`

## Gotchas

- **No tests exist** — add them with standard Go conventions
- **`.gitignore` ignores all dotfiles** — first line is `.*`, with exceptions for `.gitignore` and `.github`
- **Logging must go to stderr** — MCP stdio uses stdout for protocol messages. Logger is `slog.NewJSONHandler(os.Stderr, nil)`
- **`exec` bypasses RBAC filesystem restrictions** — it grants full shell access. RBAC only checks the `workdir` path. This is documented and by design
- **`goto` statements** — JWT validation middleware uses `goto` for control flow. Intentional
- **`defer` in loop** — `jwt_validation_utils.go:cacheJWKS()` has `defer resp.Body.Close()` inside an infinite loop — deferred close won't execute until function returns
- **CORS FIXMEs** — OAuth handler files have `// FIXME: TOO STRICT` on CORS headers
- **Binary name** — output is always `bin/mcp-forge-{GOOS}-{GOARCH}`
- **CEL expressions** — used in both JWT middleware (`allow_conditions`) and RBAC (`when`). Same engine, same `payload` variable
- **Environment variable expansion** — config values support `$VAR` / `${VAR}` via `os.ExpandEnv`

## CI/CD

- Triggered on GitHub release creation or manual `workflow_dispatch`
- Binaries: cross-compiled for linux/{386,amd64,arm64} and darwin/{amd64,arm64}
- Docker: multi-platform via QEMU + buildx, pushed to `ghcr.io`
- Go version read dynamically from `go.mod`

## Helm Chart

- Uses `bjw-s/app-template` v4.2.0 meta chart
- Config embedded as ConfigMap, mounted at `/data/config.yaml`
- Default: 2 replicas, 512Mi memory, Istio sidecar enabled
- Optional raw resources: ExternalSecret, HTTPRoute, EnvoyFilter, AuthorizationPolicy, RequestAuthentication
