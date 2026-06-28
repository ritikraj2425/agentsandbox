# Runtime Backends in AgentSandbox

AgentSandbox abstracts command execution through a pluggable `Runtime` interface, allowing it to support multiple execution backends with varying levels of isolation, security, and performance.

---

## 1. Pluggable Architecture

Runtimes are decoupled from command execution policies and CLI parsing logic. Every backend registers itself using Go's `init()` function with a global registry database.

```go
type Runtime interface {
    Name() string
    Run(ctx context.Context, action protocol.Action) (protocol.Observation, error)
}
```

Currently, two runtime backends are implemented: **Local** and **Docker**.

---

## 2. Local Backend (`local`)

The Local backend executes commands directly on the host operating system. It does not provide virtualization or sandboxing boundaries.

### Implementation Mechanics
- Invokes `/bin/sh -c "<cmd>"` on macOS/Linux and `cmd.exe /C "<cmd>"` on Windows.
- Runs directly inside the working directory (`os.Getwd()`) from which the CLI was invoked.
- Captures standard streams using in-memory byte buffers (`bytes.Buffer`).

### Trade-offs
* **Pros**:
  - Extremely fast execution (sub-millisecond startup overhead).
  - No external software dependencies (uses Go's standard `os/exec` library).
* **Cons**:
  - **Zero Security Isolation**: A malicious command can wipe the host filesystem, access personal environment variables, or spawn rogue daemons.
  - State Pollution: Temp files, local caches, and environmental changes pollute the host environment.

---

## 3. Docker Backend (`docker`)

The Docker backend wraps command execution in an isolated Linux container, providing physical isolation of process namespaces, network configurations, and filesystem spaces.

```
       HOST SYSTEM (macOS/Linux)                   DOCKER CONTAINER (Linux VM/Kernel namespaces)
 ┌───────────────────────────────────┐            ┌──────────────────────────────────────────────┐
 │ Workspace directory:              │            │ Workspace mount point:                       │
 │ `/Users/ritikraj/agentsandbox`   │ ◄─────────► │ `/workspace`                                 │
 │ (Contains go.mod, cmd/, etc.)     │  [Mount]   │ (Set as the active working directory)        │
 ├───────────────────────────────────┤            ├──────────────────────────────────────────────┤
 │ Host Process Space                │            │ Container Process Namespace (Isolated PID)   │
 │ - Main CLI binary (agentsandbox)  │            │ - shell `/bin/sh`                            │
 │ - Docker daemon (dockerd)         │            │ - command (e.g., `go test ./...`)            │
 │ - User processes (IDE, Shell)     │            │   (Cannot see host processes or files outside│
 └───────────────────────────────────┘            │    of `/workspace`)                          │
                                                  └──────────────────────────────────────────────┘
```

### Detailed Lifecycle & Mechanics

#### A. Mount Mapping & Synced State
During the runtime configuration, AgentSandbox runs:
```bash
docker run --rm -v [HostWorkspace]:/workspace -w /workspace [Image] /bin/sh -c "[Cmd]"
```
- **Bind Mounting (`-v`)**: The absolute path of the host working directory is bind-mounted directly to `/workspace` inside the container. 
- **Two-way Synced Filesystem**: Files generated or modified in the container's `/workspace` are written directly back to the host filesystem. This is crucial for developers who want the sandboxed agent to write code and update source repositories. Files outside the workspace (like your host system settings or SSH directory) are completely hidden.

#### B. Ephemeral Lifecycle (`--rm`)
To avoid container sprawl (leaving hundreds of stopped container processes on the host), the Docker daemon is configured with the `--rm` flag.
1. The Docker engine allocates container namespaces and starts the command.
2. Output streams are piped back to the host and buffered.
3. The moment the command exits (or hits the runtime timeout limit), the Docker engine automatically destroys the container, releasing all write-layers and network resources.
4. Only the cached base image (e.g., `alpine:latest` or `golang:1.24`) is retained on disk.

#### C. Isolation Boundaries
- **Process Space**: Process isolation is enforced using Linux PID namespaces. A process inside the container cannot see or kill processes running on the host.
- **Network**: The container is isolated on a virtual bridge. Network capabilities can be fully shut down in the future (e.g., `--net none`) to prevent exfiltration.

### Trade-offs
* **Pros**:
  - High degree of process and environment isolation.
  - Multi-platform: Run Linux-only commands on macOS seamlessly.
  - Clean state: Toolchains (e.g., Go, Node.js) do not need to be installed on the host.
* **Cons**:
  - Boot overhead (requires spinning up the container, typically takes 500ms to 1s).
  - Requires the Docker daemon to be running on the host system.
  - File permissions: Files created inside the container as the default `root` user may end up owned by `root` on the host filesystem.
