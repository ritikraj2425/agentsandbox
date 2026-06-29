---
sidebar_position: 2
---

# Getting Started

This guide will show you how to get AgentSandbox running locally on your machine, or how to connect to the Cloud API.

## Local Installation (Open Source)

AgentSandbox is built in pure Go. It requires zero external dependencies to run locally (though Docker is highly recommended for actual isolation).

### Prerequisites
1. Go 1.24+ installed on your system.
2. Docker Desktop (if using the Docker backend).

### Setup
```bash
git clone https://github.com/agentsandbox/agentsandbox.git
cd agentsandbox
go build ./cmd/agentsandbox
```

### Running your first command
You can test the sandbox immediately using the CLI:
```bash
./agentsandbox run "echo Hello, World!" --backend docker
```

## Cloud Usage (Subscription)

If you are using our managed cloud offering, you don't need to run any infrastructure.

1. Go to the [Dashboard](https://app.agentsandbox.com) and create an account.
2. Generate an API Key.
3. Make an HTTP request to our API Gateway:

```bash
curl -X POST https://api.agentsandbox.com/v1/sessions \
  -H "Authorization: Bearer sb_live_your_api_key_here" \
  -H "Content-Type: application/json" \
  -d '{"backend": "docker"}'
```
