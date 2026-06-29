---
sidebar_position: 6
---

# Policy Language

AgentSandbox uses a YAML-based policy engine to define what an agent can and cannot do. By default, every sandbox is locked down (Default Deny). 

## Why do we need policies?
Even inside an isolated container, you may want to restrict the agent from:
* Making outgoing network requests (e.g. preventing data exfiltration).
* Reading sensitive mounted files.
* Running shell commands, restricting it to just Python execution.

## Example Policy

```yaml
version: "1.0"
policy:
  network:
    allow_egress: true
    allow_dns: true
    denied_hosts:
      - "internal-api.company.local"
      - "metadata.google.internal"
  filesystem:
    allow_read:
      - "/workspace"
      - "/usr/lib"
    allow_write:
      - "/workspace/tmp"
  execution:
    allow_shell: false
    allowed_commands:
      - "python3"
      - "node"
```

## How to Apply

When starting a session via the API, include the policy inline or reference a registered policy ID:

```json
{
  "backend": "docker",
  "policy": {
    "network": {
      "allow_egress": false
    }
  }
}
```

If the agent attempts an action that violates the policy (like `curl google.com`), the sandbox intercepts it and returns a `PolicyViolation` error to the agent, mimicking a real network failure.
