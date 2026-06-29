---
sidebar_position: 5
---

# API Reference

All endpoints are hosted at `https://api.agentsandbox.com`.

## Authentication
Every request must include an `Authorization` header with your API key.
`Authorization: Bearer sb_live_...`

---

## Create Session
Creates a new sandboxed environment.

**POST** `/v1/sessions`

### Request Body
```json
{
  "backend": "docker",
  "ttl": 3600,
  "policy": "coding-safe",
  "workspace": {
    "type": "empty"
  }
}
```

`policy` selects a bundled file from `policies/`, such as `coding-safe`,
`no-network`, `browser-safe`, or `research`. Use `policy_file` to pass an
explicit policy file path. If omitted, the gateway uses a default-deny policy.

`workspace.type` can be `empty`, `git_clone`, or `uploaded_archive`. For
`git_clone`, include `git_url`. Uploaded archive initialization is currently a
placeholder and records the requested archive in session artifacts.

### Response
```json
{
  "session_id": "abc123xyz",
  "expires_at": "2026-06-29T12:00:00Z"
}
```

---

## Execute Action
Executes a structured action inside an active session.

**POST** `/v1/sessions/:id/actions`

### Request Body
New clients should send a structured action:
```json
{
  "type": "shell.run",
  "parameters": {
    "command": "cat /etc/os-release"
  },
  "client_action_id": "optional-client-id"
}
```

The legacy `command` field is still supported:
```json
{
  "command": "cat /etc/os-release"
}
```

### Supported Action Types
| Type | Required parameters |
| --- | --- |
| `shell.run` | `command` string |
| `file.read` | `path` string |
| `file.write` | `path` string, `content` string |
| `file.patch` | `path` string, `patch` string |
| `browser.goto` | `url` string |
| `browser.click` | `selector` string, or `x` and `y` numbers |
| `browser.type` | `text` string |
| `browser.press` | `key` string |
| `browser.wait_for` | one of `selector` string, `text` string, or `timeout_ms` number |
| `browser.screenshot` | optional `full_page` boolean |
| `browser.evaluate` | `expression` string |
| `browser.assert` | `type` string and `expected` value |
| `task.done` | optional `summary` string |

### Response
```json
{
  "action_id": "act_8899",
  "status": "completed",
  "exit_code": 0,
  "stdout_summary": "PRETTY_NAME=\"Debian GNU/Linux 12 (bookworm)\"",
  "stderr_summary": "",
  "duration_ms": 45,
  "policy_decision": {
    "allowed": true,
    "effect": "allow",
    "policy_name": "coding-safe",
    "matched_rule": "shell.run",
    "reason": "allowed by policy"
  }
}
```

### Error Response
Invalid action requests return HTTP 400 with a consistent JSON envelope:
```json
{
  "error": {
    "code": "invalid_action_parameters",
    "message": "shell.run requires parameters.command string",
    "details": {
      "type": "shell.run",
      "field": "command"
    }
  }
}
```
