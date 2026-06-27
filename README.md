# AgentSandbox 🛡️

**Open-source Go runtime for safely running AI agent actions.**

AI agents (like coding assistants and research bots) need to perform real actions — run shell commands, write files, make network requests. AgentSandbox gives you a safe, observable way to let them do that.

## What It Does

AgentSandbox sits between an AI agent and your operating system. When an agent wants to do something (write a file, run a command, fetch a URL), AgentSandbox:

1. **Checks the action against a policy** — Is this action allowed? Should it be blocked?
2. **Runs the action in a controlled environment** — From a simple local runner to full container isolation.
3. **Records everything** — Every action, its inputs, outputs, and filesystem changes are captured in a structured trace.

```
┌─────────────┐     ┌───────────────────────────────────────┐     ┌────────────┐
│             │     │           AgentSandbox                │     │            │
│   AI Agent  │────▶│  Policy ──▶ Runner ──▶ Trace/FSDiff   │────▶│  Your OS   │
│             │     │                                       │     │            │
└─────────────┘     └───────────────────────────────────────┘     └────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21 or later

### Build & Run

```bash
# Clone the repository
git clone https://github.com/ritikraj2425/agentsandbox.git
cd agentsandbox

# Run the CLI
go run ./cmd/agentsandbox --help

# Run tests
go test ./...
```

### Example: Run an action with a policy

```bash
go run ./cmd/agentsandbox run --action write_file --policy examples/policy.yaml
```

## Project Structure

```
agentsandbox/
├── cmd/agentsandbox/       # CLI entrypoint
├── internal/
│   ├── actions/            # Action type definitions
│   ├── policy/             # Policy engine (allow/deny rules)
│   ├── runner/             # Runtime backends (local, docker, etc.)
│   ├── trace/              # Execution tracing and event logging
│   └── fsdiff/             # Filesystem change detection
├── pkg/                    # Public SDK and protocol definitions
├── runtimes/               # Runtime backend implementations
├── policies/               # Built-in policy templates
├── examples/               # Example configurations and agents
├── testdata/               # Test fixtures and benchmarks
├── web/                    # Dashboard and replay viewer
├── docs/                   # Architecture and design documentation
└── scripts/                # Development and setup scripts
```

## Core Concepts

### Actions

An **Action** is something an AI agent wants to do. Every action has a type (`shell`, `file_write`, `file_read`, `network`, `custom`), parameters, and a lifecycle status.

```go
action := actions.NewAction(actions.ActionTypeShell, "list-files", map[string]interface{}{
    "command": "ls -la",
})
```

### Policies

A **Policy** is a set of rules that determine which actions are allowed or denied. Policies use a first-match-wins evaluation strategy.

```yaml
name: coding-safe
default_effect: deny
rules:
  - action: file_read
    effect: allow
  - action: shell
    effect: allow
  - action: network
    effect: deny
```

### Traces

A **Trace** is a structured log of everything that happened during action execution — policy checks, outputs, errors, and filesystem changes. Traces enable replay, debugging, and auditing.

### FSDiff

An **FSDiff** captures the filesystem changes (files created, modified, or deleted) produced by an action, enabling rollback and auditing.

## Runtime Backends

| Backend      | Isolation Level | Status      |
|-------------|----------------|-------------|
| Local       | None           | ✅ Available |
| Docker      | Container      | 🔜 Planned  |
| gVisor      | Kernel-level   | 🔜 Planned  |
| Firecracker | MicroVM        | 🔜 Planned  |
| Browser     | Tab sandbox    | 🔜 Planned  |

## Development

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Build the CLI binary
go build -o bin/agentsandbox ./cmd/agentsandbox
```

## Roadmap

- [x] Core types: Action, Policy, TraceEvent, FSDiff
- [x] Local runner with policy evaluation
- [x] CLI with `run` command
- [ ] YAML policy file loading
- [ ] Real shell command execution in local runner
- [ ] Docker runtime backend
- [ ] Approval workflows (human-in-the-loop)
- [ ] Web dashboard for trace replay
- [ ] gVisor and Firecracker backends
- [ ] Agent protocol SDK

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please open an issue first to discuss what you'd like to change.
# agentsandbox
