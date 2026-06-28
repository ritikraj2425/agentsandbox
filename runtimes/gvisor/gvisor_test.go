package gvisor

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// gvisorAvailable checks if Docker is running and runsc is configured.
func gvisorAvailable() bool {
	checkCmd := exec.Command("docker", "info", "--format", "{{.Runtimes}}")
	out, err := checkCmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "runsc")
}

func TestRuntime_Name(t *testing.T) {
	if !gvisorAvailable() {
		t.Skip("gVisor (runsc) is not available, skipping")
	}

	rt, err := New("/tmp", "alpine:latest", "1.0", "512m", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.Name() != "gvisor" {
		t.Errorf("expected name 'gvisor', got %q", rt.Name())
	}
}

func TestRuntime_EchoHello(t *testing.T) {
	if !gvisorAvailable() {
		t.Skip("gVisor is not available, skipping")
	}

	rt, err := New("/tmp", "alpine:latest", "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "echo hello",
	})

	obs, runErr := rt.Run(context.Background(), action)
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	if obs.Status != protocol.ObsStatusCompleted {
		if obs.ExitCode == 127 && strings.Contains(obs.StderrSummary, "runsc") {
			t.Skip("gVisor runsc binary is registered but not functional (may need reinstall after Docker restart), skipping")
		}
		t.Errorf("expected status completed, got %s (error: %s)", obs.Status, obs.Error)
	}
	if obs.Backend != "gvisor" {
		t.Errorf("expected backend 'gvisor', got %q", obs.Backend)
	}
}

func TestNew_GVisorUnavailable(t *testing.T) {
	if gvisorAvailable() {
		t.Skip("gVisor is available, cannot test unavailable path")
	}

	_, err := New("/tmp", "alpine:latest", "", "", nil)
	if err == nil {
		t.Fatal("expected error when gVisor is unavailable")
	}
}
