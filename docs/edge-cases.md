---
sidebar_position: 4
---

# Handling Edge Cases

AI Agents are notoriously unpredictable. When giving an LLM access to a terminal, things will eventually go wrong. AgentSandbox is built to catch these edge cases gracefully.

## 1. The Infinite Loop (CPU Exhaustion)
**Scenario**: The AI agent writes a python script `while True: pass` and runs it.
**Result**: The container's CPU spikes to 100%.
**How we handle it**: 
If using Docker or gVisor, AgentSandbox applies `cgroups` CPU limits automatically (e.g., max 1 vCPU). Furthermore, the `SessionManager` tracks the execution timeout. After 2 minutes, it will automatically send a `SIGKILL` to the container and return an `ObsStatusTimeout` observation to the gateway.

## 2. Memory Leaks (OOM Kills)
**Scenario**: The agent writes a script that allocates gigabytes of RAM.
**Result**: The host machine runs out of memory.
**How we handle it**:
We apply strict memory limits (e.g., 512MB) per session. If the agent's process exceeds this, the Linux OOM Killer immediately terminates it. AgentSandbox detects the exit code and returns a clean `OutOfMemory` error back to the API.

## 3. Prompt Injection / Privilege Escalation
**Scenario**: The user prompts the agent to "Read /etc/shadow" or "Access the docker socket".
**Result**: The agent attempts to break out of the container.
**How we handle it**:
If using the `gVisor` runtime, the container does not share the host kernel. Even if a zero-day Linux exploit is found, the agent is trapped inside the user-space Go kernel and cannot interact with the host system.

## 4. Fork Bombs
**Scenario**: The agent runs `:(){ :|:& };:`.
**Result**: The container spawns infinite processes until the system crashes.
**How we handle it**:
The runtime policy applies a `pids-limit` (e.g., max 50 processes). The fork bomb fails instantly and the session is terminated.
