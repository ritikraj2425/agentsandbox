package docker

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// dockerAvailable checks if Docker is running and accessible.
// Tests that require Docker are skipped when it's unavailable,
// ensuring `go test ./...` never fails due to missing Docker.
func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

func TestRuntimeDockerArgsMountSessionWorkspaceOnly(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "session", "workspace")
	rt := &Runtime{workDir: workspace, image: "alpine:latest"}

	args := rt.dockerArgs("pwd")

	foundMount := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-v" {
			foundMount = true
			expected := workspace + ":/workspace"
			if args[i+1] != expected {
				t.Fatalf("expected mount %s, got %s", expected, args[i+1])
			}
		}
	}
	if !foundMount {
		t.Fatal("expected docker args to include workspace mount")
	}
}

func TestRuntime_Name(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker is not available, skipping")
	}

	rt, err := New("/tmp", "alpine:latest", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.Name() != "docker" {
		t.Errorf("expected name 'docker', got %q", rt.Name())
	}
}

func TestRuntime_EchoHello(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker is not available, skipping")
	}

	rt, err := New("/tmp", "alpine:latest", nil)
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
		t.Errorf("expected status completed, got %s (error: %s)", obs.Status, obs.Error)
	}
	if obs.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", obs.ExitCode)
	}
	if obs.Backend != "docker" {
		t.Errorf("expected backend 'docker', got %q", obs.Backend)
	}
}

func TestRuntime_NonZeroExit(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker is not available, skipping")
	}

	rt, err := New("/tmp", "alpine:latest", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "exit 42",
	})

	obs, _ := rt.Run(context.Background(), action)

	if obs.Status != protocol.ObsStatusFailed {
		t.Errorf("expected status failed, got %s", obs.Status)
	}
	if obs.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", obs.ExitCode)
	}
}

func TestRuntime_NoCommand(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker is not available, skipping")
	}

	rt, err := New("/tmp", "alpine:latest", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action := protocol.NewAction(protocol.ActionTypeShellRun, nil)

	obs, runErr := rt.Run(context.Background(), action)
	if runErr == nil {
		t.Fatal("expected error for missing command")
	}
	if obs.Status != protocol.ObsStatusFailed {
		t.Errorf("expected status failed, got %s", obs.Status)
	}
}

func TestNew_DockerUnavailable(t *testing.T) {
	// This test validates error handling when Docker is unavailable.
	// We can only truly test this if Docker is NOT running.
	if dockerAvailable() {
		t.Skip("Docker is available, cannot test unavailable path")
	}

	_, err := New("/tmp", "alpine:latest", nil)
	if err == nil {
		t.Fatal("expected error when Docker is unavailable")
	}
}
