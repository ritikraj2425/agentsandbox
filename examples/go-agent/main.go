// Example go-agent demonstrates using the AgentSandbox Go SDK to programmatically
// create sessions, execute commands, and manage sandbox lifecycles.
//
// Prerequisites:
//
//	Start the gateway in a separate terminal:
//	  go run ./cmd/agentsandbox serve --port 8080 --auth-key secret
//
// Run this example:
//
//	go run ./examples/go-agent/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ritikraj2425/agentsandbox/pkg/sdk"
)

func main() {
	// ── Configuration ───────────────────────────────────────────────────
	// Read the gateway URL and auth key from environment variables,
	// falling back to sensible defaults for local development.
	gatewayURL := envOrDefault("SANDBOX_URL", "http://localhost:8080")
	authKey := envOrDefault("SANDBOX_AUTH_KEY", "secret")

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║       AgentSandbox Go SDK — Example Agent       ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Printf("  Gateway: %s\n\n", gatewayURL)

	// ── Step 1: Create the SDK Client ───────────────────────────────────
	sandbox := sdk.NewClient(gatewayURL, authKey)

	// ── Step 2: Create a Session ────────────────────────────────────────
	fmt.Println("▸ Creating sandbox session (backend: local)...")
	session, err := sandbox.CreateSession(sdk.SessionConfig{
		Backend: "local",
	})
	if err != nil {
		log.Fatalf("  ✗ Failed to create session: %v", err)
	}
	defer session.Destroy()
	fmt.Printf("  ✓ Session created: %s (expires: %s)\n\n", session.ID, session.ExpiresAt.Format("15:04:05"))

	// ── Step 3: Execute Commands ────────────────────────────────────────
	commands := []string{
		"echo 'Hello from the Go SDK!'",
		"date",
		"uname -a",
		"ls -la",
	}

	for i, cmd := range commands {
		fmt.Printf("▸ [%d/%d] Running: %s\n", i+1, len(commands), cmd)

		obs, err := session.Run(cmd)
		if err != nil {
			fmt.Printf("  ✗ Error: %v\n\n", err)
			continue
		}

		fmt.Printf("  Status:    %s\n", obs.Status)
		fmt.Printf("  Exit Code: %d\n", obs.ExitCode)
		fmt.Printf("  Duration:  %dms\n", obs.DurationMs)

		if obs.StdoutSummary != "" {
			// Show first 3 lines of output for brevity
			lines := strings.SplitN(obs.StdoutSummary, "\n", 4)
			for _, line := range lines[:min(len(lines), 3)] {
				fmt.Printf("  │ %s\n", line)
			}
			if len(lines) > 3 {
				fmt.Printf("  │ ... (%d more lines)\n", len(lines)-3)
			}
		}
		fmt.Println()
	}

	// ── Step 4: Destroy the Session ─────────────────────────────────────
	fmt.Println("▸ Destroying session...")
	if err := session.Destroy(); err != nil {
		log.Fatalf("  ✗ Failed to destroy session: %v", err)
	}
	fmt.Println("  ✓ Session destroyed successfully")
	fmt.Println("\nDone!")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
