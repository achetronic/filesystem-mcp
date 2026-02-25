# TODO

## Sandbox `exec` with bubblewrap

The `exec` tool is currently an open `sh -c`. RBAC only validates the `workdir` (where), not the command (what). An LLM could execute anything: delete files, exfiltrate data, install packages, etc.

### Goal

Integrate [bubblewrap](https://github.com/containers/bubblewrap) (`bwrap`) as a configurable sandbox for commands executed by `exec`.

### Implementation plan

- [ ] **Add config in `api/config_types.go`** — new `sandbox` section:
  ```yaml
  sandbox:
    enabled: true
    backend: bwrap
    bwrap:
      ro_bind:            # Paths mounted read-only inside the sandbox
        - /usr
        - /bin
        - /lib
        - /lib64
      bind: []            # Paths mounted with write access
      unshare_net: true   # Isolate network (no network access)
      unshare_pid: true   # Isolate PIDs
      die_with_parent: true
  ```

- [ ] **Modify `internal/state/processes.go`** — in `Exec()` and `Start()`, when sandbox is enabled, build the command as:
  ```go
  // Instead of:
  exec.Command("sh", "-c", command)
  // Build:
  exec.Command("bwrap", ...flags, "--", "sh", "-c", command)
  ```

- [ ] **Build bwrap flags dynamically** — helper function that takes the sandbox config and generates bwrap arguments (`--ro-bind`, `--bind`, `--unshare-net`, etc.)

- [ ] **Respect `workdir`** — ensure the exec `workdir` is mounted with `--bind` and used as `--chdir` inside the sandbox

- [ ] **Validate `bwrap` exists at startup** — if sandbox is enabled, verify the `bwrap` binary is in PATH. Fail with a clear error if missing

- [ ] **Document in example configs** — update `docs/config-http.yaml` and `docs/config-stdio.yaml` with the `sandbox` section (commented out)

- [ ] **Document in README** — section explaining sandboxing, how to install bwrap, and configuration examples

- [ ] **Tests** — at least unit tests for bwrap flag construction and `ProcessStore` integration

### Notes

- `bwrap` does not require root and is available on most distros (`apt install bubblewrap`, `dnf install bubblewrap`)
- The current Docker image is `distroless:nonroot` — evaluate whether to include `bwrap` in the image or leave it as an external dependency
- Alternatives discarded for now:
  - **seccomp**: too granular (syscalls), complex to configure correctly
  - **container-in-container**: high overhead, requires Docker/Podman runtime
  - **command allowlist/blocklist**: bypassable (symlinks, scripts, interpreters)
