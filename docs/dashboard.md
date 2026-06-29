---
sidebar_position: 7
---

# Using the Dashboard

If you are using the Cloud version of AgentSandbox, the **Dashboard** is your central hub.

You can access it at [app.agentsandbox.com](https://app.agentsandbox.com).

## 1. Authentication
Sign up for a free account. You will automatically be placed on the Developer tier (10 concurrent sessions).

## 2. Generating API Keys
Navigate to the **Settings** tab. Click **Generate New Key**.
A key starting with `sb_live_` will be generated.
> **Warning**: This key is only shown once. If you lose it, you must revoke it and generate a new one. The server only stores a cryptographic hash of this key.

## 3. Viewing Live Sessions
Navigate to the **Sessions** tab. Here you will see a real-time list of all agents currently executing code in your sandboxes.

You can click on any active session to view its live console output, memory usage, and CPU utilization. 

## 4. Replay Viewer
Because AgentSandbox records a deterministic event trace (`trace.jsonl`) for every session, you can view past sessions in the **Replays** tab. This acts like a DVR for your AI agent, letting you scrub backward and forward in time to see exactly what commands it ran, what files it edited, and what errors it encountered.
