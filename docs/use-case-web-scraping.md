---
sidebar_position: 10
---

# Use Case: Autonomous Web Scraping

Giving an AI agent access to a real web browser enables powerful use cases like autonomous web scraping, QA testing, and RPA (Robotic Process Automation). However, running full Chrome browsers is extremely memory intensive and fraught with security risks.

## AgentSandbox Browser Runtime

AgentSandbox provides a specialized `browser` backend that comes pre-configured with Xvfb (virtual framebuffer) and Chromium.

When an agent requests to use Puppeteer, Playwright, or Selenium, the sandbox is already prepared to render the headless (or headed) browser securely.

### Security Boundaries

When scraping the web autonomously, the agent might visit malicious sites that exploit browser zero-days.

By wrapping the browser inside AgentSandbox's `gVisor` or `Firecracker` boundaries, even if the Chrome renderer process is compromised by a malicious website, the attacker is trapped inside the microVM and cannot access your host infrastructure or internal AWS metadata.

### Example Network Policy
You can restrict the web scraper to only visit specific domains to ensure the agent doesn't wander:

```yaml
version: "1.0"
policy:
  network:
    allow_egress: true
    allowed_domains:
      - "wikipedia.org"
      - "ycombinator.com"
```
