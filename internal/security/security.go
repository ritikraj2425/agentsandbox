// Package security provides automated tests for sandbox isolation and vulnerability checks.
package security

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/color"
	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// RunSuite runs a suite of security exploit tests against a given backend.
func RunSuite(backendName string, rt runtime.Runtime, workDir string) error {
	fmt.Printf("\n%s\n", color.BoldCyan("━━━ AgentSandbox Security Suite ━━━"))
	fmt.Printf("Testing backend: %s\n\n", color.Bold(backendName))

	tests := []struct {
		name        string
		command     string
		expectError bool
		expectOut   string
	}{
		{
			name:        "File Read Escapes (Shadow)",
			command:     "cat /etc/shadow",
			expectError: true,
			expectOut:   "", // shouldn't print shadow contents
		},
		{
			name:        "File Read Escapes (Root)",
			command:     "ls -la /root",
			expectError: true,
			expectOut:   "",
		},
		{
			name:        "Network Leaks (DNS)",
			command:     "nslookup example.com",
			expectError: true,
			expectOut:   "",
		},
		{
			name:        "Network Leaks (Ping)",
			command:     "ping -c 1 8.8.8.8",
			expectError: true,
			expectOut:   "",
		},
		{
			name:        "Prompt Injection script (malicious.sh)",
			command:     "/bin/sh testdata/prompt-injection/malicious.sh",
			expectError: true,
			expectOut:   "[-] FAILED", // Expecting the script to print failures
		},
		{
			name:        "Network script (leak.sh)",
			command:     "/bin/sh testdata/prompt-injection/leak.sh",
			expectError: true,
			expectOut:   "[-] FAILED",
		},
	}

	passCount := 0

	for _, test := range tests {
		fmt.Printf("Running test: %s... ", color.Cyan(test.name))

		action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
			"command": test.command,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		obs, err := rt.Run(ctx, action)
		cancel()

		passed := false
		
		// If it's a script that outputs explicitly
		if test.expectOut != "" {
			if strings.Contains(obs.StdoutSummary, test.expectOut) || strings.Contains(obs.StderrSummary, test.expectOut) {
				passed = true
			} else if obs.ExitCode != 0 {
				passed = true // If it failed to run entirely, it's also a pass for isolation
			}
		} else {
			// standard commands
			if test.expectError {
				if err != nil || obs.ExitCode != 0 {
					passed = true
				}
			} else {
				if err == nil && obs.ExitCode == 0 {
					passed = true
				}
			}
		}

		if passed {
			fmt.Printf("[%s]\n", color.Green("PASS"))
			passCount++
		} else {
			fmt.Printf("[%s]\n", color.Red("FAIL (Exploit Succeeded!)"))
			fmt.Printf("  Stdout: %s\n  Stderr: %s\n", strings.TrimSpace(obs.StdoutSummary), strings.TrimSpace(obs.StderrSummary))
		}
	}

	fmt.Printf("\n%s\n", color.BoldCyan("━━━ Summary ━━━"))
	fmt.Printf("Tests passed: %d/%d\n", passCount, len(tests))

	if passCount < len(tests) {
		return fmt.Errorf("security suite failed %d tests", len(tests)-passCount)
	}

	return nil
}
