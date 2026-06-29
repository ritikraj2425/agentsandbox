---
sidebar_position: 3
---

# Architecture Overview

AgentSandbox is broken down into three core components:

1. **The API Gateway** (Go Server)
2. **The Session Manager** (State & Limits)
3. **The Runtime Backend** (Docker / gVisor / Firecracker)

## 1. The API Gateway

The API Gateway is a lightweight, multi-tenant HTTP server built in Go. It acts as the front door for all agent requests. 

When you send a request to the Gateway, it:
* Validates your API key against the PostgreSQL database.
* Rate limits requests using a token bucket algorithm to prevent abuse.
* Forwards actions to the assigned runtime.

## 2. The Session Manager

The Session Manager is responsible for the lifecycle of sandboxes.
When an agent starts a session, the Session Manager tracks its `TTL` (Time To Live). If the agent enters an infinite loop and stops responding, the Session Manager's `CleanupLoop` will automatically reap the session, destroy the container, and free up server resources.

## 3. The Runtime Backend

This is where the actual code execution happens. AgentSandbox utilizes a pluggable architecture:

* **Local**: No isolation. Runs commands directly on the host. (Only for testing).
* **Docker**: Medium isolation. Wraps execution in a Linux container. Protects against accidental filesystem modifications, but vulnerable to kernel exploits.
* **gVisor**: High isolation. A user-space kernel written in Go that intercepts all syscalls from the container. Used by Google Cloud Run.
* **Firecracker**: Maximum isolation. MicroVMs used by AWS Lambda. Provides hardware-level virtualization with sub-second boot times.
