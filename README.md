<p align="center">
  <img src="docs/img/header.svg" alt="Filesystem MCP" width="800">
</p>

<p align="center">
  <img src="https://img.shields.io/github/go-mod/go-version/achetronic/filesystem-mcp" alt="Go version">
  <img src="https://img.shields.io/github/license/achetronic/filesystem-mcp" alt="License">
</p>

A production-ready MCP (Model Context Protocol) server that gives AI agents full control over a filesystem — reading, writing, editing, searching, diffing files, executing commands, and managing processes. Built in Go with OAuth authorization and RBAC.

## Features

- 🗂️ **12 powerful tools** for filesystem operations, shell execution, and agent utilities
- 🔐 **RBAC with JWT + CEL** — restrict operations per path using glob patterns and JWT claim expressions
- ⚡ **Token-efficient by design** — partial file reads, batch edits, ranged diffs, search with context control
- 🔑 **OAuth RFC 8414 / RFC 9728 compliant** — `.well-known/oauth-protected-resource` and `.well-known/oauth-authorization-server`
- 🛡️ **JWT validation** — delegated to external proxies (Istio) or validated locally via JWKS + CEL
- 🚀 **Dual transport** — stdio for local clients, HTTP for remote (Claude Web, OpenAI, etc.)

## Tools

### Filesystem

| Tool         | Description                                                                                                                                          |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ls`         | List directory contents with depth, glob filter, hidden file toggle. depth=1 is flat, depth=N is tree                                                |
| `read_file`  | Read a file fully or specific line ranges. Accepts an array of `{offset, limit}` ranges for partial reads                                            |
| `write_file` | Create or overwrite a file. Auto-creates parent directories. Saves undo state                                                                        |
| `edit_file`  | Batch find-and-replace on a file. Accepts an array of `{old_text, new_text, replace_all}` edits applied sequentially. Reports successes and failures |
| `search`     | Recursive grep with regex or literal mode. Configurable include/exclude patterns, context lines, max results                                         |
| `diff`       | Unified diff between two files or sections. Supports line ranges on both sides                                                                       |

### Shell & Processes

| Tool             | Description                                                                            |
| ---------------- | -------------------------------------------------------------------------------------- |
| `exec`           | Execute shell commands in foreground (with timeout) or background (returns process ID) |
| `process_status` | Get output and status of a background process, or list all background processes        |
| `process_kill`   | Kill a background process                                                              |

### System & Utilities

| Tool          | Description                                                                           |
| ------------- | ------------------------------------------------------------------------------------- |
| `system_info` | OS, architecture, hostname, user, working directory, shell, PATH                      |
| `undo`        | Revert a file to its state before the last `write_file` or `edit_file`                |
| `scratch`     | In-memory key-value store for the agent to save/retrieve temporary data between calls |

## RBAC

RBAC controls which operations are allowed on which filesystem paths. Rules are evaluated in order — first match wins.

### Operation Categories

| Category | Tools                              | Notes                                                                  |
| -------- | ---------------------------------- | ---------------------------------------------------------------------- |
| `read`   | ls, read_file, search, diff        | Safe, read-only operations                                             |
| `write`  | write_file, edit_file, undo        | Modifies files                                                         |
| `exec`   | exec, process_status, process_kill | **Full shell access** — granting this bypasses filesystem restrictions |

`system_info` and `scratch` don't touch the filesystem and are always allowed.

### Configuration

```yaml
rbac:
  enabled: true
  default_policy: deny # deny | allow

  rules:
    - name: "admins"
      when:
        - 'payload.groups.exists(g, g == "admin")'
      paths: ["/**"]
      operations: [read, write, exec]

    - name: "developers"
      when:
        - 'payload.groups.exists(g, g == "developer")'
      paths: ["/home/*/projects/**", "/tmp/**"]
      operations: [read, write]

    - name: "viewers"
      when:
        - 'payload.scope.contains("read")'
      paths: ["/home/*/projects/**"]
      operations: [read]

    - name: "anonymous - no JWT"
      when: []
      paths: ["/tmp/sandbox/**"]
      operations: [read]
```

- **`when`** — CEL expressions evaluated against the JWT payload. All must be true (AND). Empty or missing matches requests without JWT
- **`paths`** — Glob patterns. `/**` suffix matches everything recursively
- **`operations`** — `read`, `write`, `exec`
- **`default_policy`** — `deny` (secure by default) or `allow` (open, for development)

> ⚠️ **Warning**: Granting `exec` gives the agent full shell access. Any filesystem restrictions from `paths` can be bypassed via shell commands. Only grant `exec` to trusted identities.

## Installation

### From source

Requires Go 1.24+.

```console
git clone https://github.com/achetronic/filesystem-mcp.git
cd filesystem-mcp
make build
```

Output: `bin/filesystem-mcp-{os}-{arch}`

### From release binaries

Download a prebuilt binary from the [Releases](https://github.com/achetronic/filesystem-mcp/releases) page. Binaries are available for linux/{386,amd64,arm64} and darwin/{amd64,arm64}.

### Why no Docker image?

This MCP server is designed to **control the host filesystem** — reading, writing, editing files, and executing shell commands. Running it inside a container would defeat its purpose: the agent would only see the container's filesystem, not yours. Bind-mounting the entire host filesystem into a container introduces complexity and security pitfalls that are worse than just running the binary directly.

If you need network-accessible deployment (HTTP transport), run the binary as a systemd service or behind a reverse proxy. The Dockerfile and Helm chart are kept in the repo for edge cases where containerized deployment makes sense (e.g., sandboxed CI environments with specific volume mounts), but they are **not the recommended way** to run this.

## Running

### Run locally

```console
make run
```

Default config starts an HTTP server on `:8080`. For stdio mode, modify the Makefile to use `docs/config-stdio.yaml`.

## Client Configuration

### Stdio Mode (Claude Desktop, Cursor, VSCode)

```console
make build
```

```json5
// claude_desktop_config.json
{
  mcpServers: {
    filesystem: {
      command: "/path/to/bin/filesystem-mcp-linux-amd64",
      args: ["--config", "/path/to/docs/config-stdio.yaml"],
    },
  },
}
```

### HTTP Mode (Remote clients)

```console
npm i mcp-remote && make run
```

```json5
// claude_desktop_config.json
{
  mcpServers: {
    "filesystem-remote": {
      command: "npx",
      args: [
        "mcp-remote",
        "http://localhost:8080/mcp",
        "--transport",
        "http-only",
        "--header",
        "Authorization: Bearer ${JWT}",
      ],
      env: {
        JWT: "eyJhbGciOiJSUzI1NiIsImtpZCI6...",
      },
    },
  },
}
```

## Configuration

Configuration is YAML-based, loaded via `--config` flag. Supports environment variable expansion (`$VAR` / `${VAR}`).

See example configs:

- [HTTP mode](./docs/config-http.yaml) — Full config with JWT, RBAC, OAuth endpoints
- [Stdio mode](./docs/config-stdio.yaml) — Minimal config for local use

## Documentation

- [MCP Authorization Requirements](https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization#overview)
- [RFC 9728 — OAuth Protected Resource Metadata](https://datatracker.ietf.org/doc/rfc9728/)
- [MCP Go Library](https://mcp-go.dev/getting-started)
- [mcp-remote package](https://www.npmjs.com/package/mcp-remote)
- [CEL Specification](https://github.com/google/cel-spec)

## Contributing

All contributions are welcome!

- [Open an issue](https://github.com/achetronic/filesystem-mcp/issues/new) to report bugs or request features
- [Submit a pull request](https://github.com/achetronic/filesystem-mcp/pulls) to contribute improvements

## License

Licensed under the [Apache 2.0 License](./LICENSE).
