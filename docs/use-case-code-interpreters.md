---
sidebar_position: 9
---

# Use Case: Code Interpreters

Advanced AI products like ChatGPT Advanced Data Analysis or Devin rely on Code Interpreters. They give the LLM an interactive Jupyter-like environment where it can write Python, execute it, see the error, and fix it.

Building your own Code Interpreter requires significant infrastructure. AgentSandbox gives you an instant, secure Code Interpreter API out of the box.

## Persistent Sessions

Unlike serverless functions (which die immediately), AgentSandbox sessions are persistent for the duration of the `ttl`. 

If your LLM installs a package in step 1, it will still be there in step 2.

### Step 1: Install Dependencies
```bash
curl -X POST https://api.agentsandbox.com/v1/sessions/YOUR_SESSION_ID/actions \
  -H "Authorization: Bearer sb_live_..." \
  -H "Content-Type: application/json" \
  -d '{"command": "pip install pandas matplotlib"}'
```

### Step 2: Run Analysis
```bash
curl -X POST https://api.agentsandbox.com/v1/sessions/YOUR_SESSION_ID/actions \
  -H "Authorization: Bearer sb_live_..." \
  -H "Content-Type: application/json" \
  -d '{"command": "python script.py"}'
```

The agent has a persistent `/workspace` directory where it can save CSVs, generate PNG charts, and read files across multiple turns of the conversation. 

Once the user closes the chat, you simply delete the session, and the entire filesystem is wiped securely.
