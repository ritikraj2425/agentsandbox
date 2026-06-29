---
sidebar_position: 1
---

# Introduction to AgentSandbox

AgentSandbox is a secure, multi-tenant environment designed specifically for running autonomous AI agents safely. 

When you give an LLM access to execute code or shell commands, you are giving it remote code execution (RCE) by design. Without a sandbox, an LLM could accidentally (or maliciously) delete your host system's files, exfiltrate sensitive data, or consume all CPU resources.

AgentSandbox solves this by providing:
* **True Isolation**: Choose between Docker, gVisor, or Firecracker microVMs.
* **Ephemeral States**: Every session starts fresh. When a session ends, the environment is destroyed completely.
* **Fine-grained Policies**: Allow or deny network access, specific file reads/writes, or restrict execution time using YAML policies.
* **Scalability**: Run thousands of concurrent agent sessions without degrading host performance.

## Why use AgentSandbox?

1. **For Researchers**: Test new Agent frameworks in an environment that guarantees they can't break out and ruin your dataset.
2. **For Companies**: Host multi-tenant platforms where thousands of users can safely let AI execute code on their behalf.
3. **For Developers**: Build applications where the AI can write, compile, and execute code dynamically.
