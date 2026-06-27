package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/actions"
	"github.com/ritikraj2425/agentsandbox/internal/policy"
	"github.com/ritikraj2425/agentsandbox/internal/trace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// ============================================================================
// Tests for RunCommand (the real execution engine)
// ============================================================================

func TestRunCommand_EchoHello(t *testing.T) {
	r := NewLocalRunner("/tmp")
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "echo hello",
	})

	obs := r.RunCommand(context.Background(), action, nil)

	if obs.Status != protocol.ObsStatusCompleted {
		t.Errorf("expected status completed, got %s (error: %s)", obs.Status, obs.Error)
	}
	if obs.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", obs.ExitCode)
	}
	if obs.StdoutSummary != "hello\n" {
		t.Errorf("expected stdout 'hello\\n', got %q", obs.StdoutSummary)
	}
	if obs.DurationMs < 0 {
		t.Errorf("expected non-negative duration, got %d", obs.DurationMs)
	}
	if obs.Command != "echo hello" {
		t.Errorf("expected command 'echo hello', got %s", obs.Command)
	}
}

func TestRunCommand_InvalidCommand(t *testing.T) {
	r := NewLocalRunner("/tmp")
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "invalid-command-xyz-12345",
	})

	obs := r.RunCommand(context.Background(), action, nil)

	if obs.Status != protocol.ObsStatusFailed {
		t.Errorf("expected status failed, got %s", obs.Status)
	}
	// Exit code 127 = "command not found" on most Unix shells.
	if obs.ExitCode != 127 {
		t.Errorf("expected exit code 127, got %d", obs.ExitCode)
	}
	if obs.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRunCommand_NonZeroExit(t *testing.T) {
	r := NewLocalRunner("/tmp")
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "exit 42",
	})

	obs := r.RunCommand(context.Background(), action, nil)

	if obs.Status != protocol.ObsStatusFailed {
		t.Errorf("expected status failed, got %s", obs.Status)
	}
	if obs.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", obs.ExitCode)
	}
}

func TestRunCommand_CapturesStderr(t *testing.T) {
	r := NewLocalRunner("/tmp")
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "echo error_msg >&2",
	})

	obs := r.RunCommand(context.Background(), action, nil)

	if obs.Status != protocol.ObsStatusCompleted {
		t.Errorf("expected status completed, got %s", obs.Status)
	}
	if obs.StderrSummary != "error_msg\n" {
		t.Errorf("expected stderr 'error_msg\\n', got %q", obs.StderrSummary)
	}
}

func TestRunCommand_Timeout(t *testing.T) {
	r := NewLocalRunner("/tmp")
	// Set a very short timeout so the test doesn't take long.
	r.Timeout = 100 * time.Millisecond

	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "sleep 10",
	})

	obs := r.RunCommand(context.Background(), action, nil)

	if obs.Status != protocol.ObsStatusTimeout {
		t.Errorf("expected status timeout, got %s (error: %s)", obs.Status, obs.Error)
	}
}

func TestRunCommand_NoCommand(t *testing.T) {
	r := NewLocalRunner("/tmp")
	action := protocol.NewAction(protocol.ActionTypeShellRun, nil)

	obs := r.RunCommand(context.Background(), action, nil)

	if obs.Status != protocol.ObsStatusFailed {
		t.Errorf("expected status failed, got %s", obs.Status)
	}
	if obs.Error != "no command specified in action parameters" {
		t.Errorf("unexpected error: %s", obs.Error)
	}
}

func TestRunCommand_WorkingDirectory(t *testing.T) {
	// Create a runner in /tmp and run pwd to verify.
	r := NewLocalRunner("/tmp")
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "pwd",
	})

	obs := r.RunCommand(context.Background(), action, nil)

	if obs.Status != protocol.ObsStatusCompleted {
		t.Errorf("expected completed, got %s", obs.Status)
	}
	// On macOS, /tmp is a symlink to /private/tmp.
	if obs.StdoutSummary != "/tmp\n" && obs.StdoutSummary != "/private/tmp\n" {
		t.Errorf("expected working directory /tmp, got %q", obs.StdoutSummary)
	}
}

// ============================================================================
// Tests for RunCommand with RunLogger (Phase 2)
// ============================================================================

// TestRunCommand_WithLogger verifies that when a RunLogger is provided,
// the runner writes process events to trace.jsonl and output to log files.
func TestRunCommand_WithLogger(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a RunLogger that writes to a temp directory.
	logger, err := trace.NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}

	r := NewLocalRunner("/tmp")
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "echo logged_output",
	})

	obs := r.RunCommand(context.Background(), action, logger)
	logger.Close()

	if obs.Status != protocol.ObsStatusCompleted {
		t.Errorf("expected completed, got %s", obs.Status)
	}

	// Verify stdout.log was written.
	stdoutContent, err := os.ReadFile(filepath.Join(logger.RunDir, "stdout.log"))
	if err != nil {
		t.Fatalf("stdout.log not found: %v", err)
	}
	if string(stdoutContent) != "logged_output\n" {
		t.Errorf("expected 'logged_output\\n' in stdout.log, got %q", string(stdoutContent))
	}

	// Verify trace.jsonl contains process events.
	traceContent, err := os.ReadFile(filepath.Join(logger.RunDir, "trace.jsonl"))
	if err != nil {
		t.Fatalf("trace.jsonl not found: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(traceContent)), "\n")
	// Should have: run.created, process.started, process.finished (at minimum 3).
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 trace events, got %d", len(lines))
	}

	// Verify the last event is process.finished.
	var lastEvent trace.TraceEvent
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &lastEvent); err != nil {
		t.Fatalf("last trace line is not valid JSON: %v", err)
	}
	if lastEvent.Type != trace.EventTypeProcessFinished {
		t.Errorf("expected last event type %s, got %s", trace.EventTypeProcessFinished, lastEvent.Type)
	}
}

// TestRunCommand_WithLogger_FailedCommand verifies that failed commands
// are properly logged to the trace.
func TestRunCommand_WithLogger_FailedCommand(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := trace.NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}

	r := NewLocalRunner("/tmp")
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "exit 1",
	})

	obs := r.RunCommand(context.Background(), action, logger)
	logger.Close()

	if obs.Status != protocol.ObsStatusFailed {
		t.Errorf("expected failed, got %s", obs.Status)
	}

	// Verify trace.jsonl records the failure.
	traceContent, err := os.ReadFile(filepath.Join(logger.RunDir, "trace.jsonl"))
	if err != nil {
		t.Fatalf("trace.jsonl not found: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(traceContent)), "\n")
	var lastEvent trace.TraceEvent
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &lastEvent); err != nil {
		t.Fatalf("last line not valid JSON: %v", err)
	}

	// Verify the exit code is recorded in the event data.
	if exitCode, ok := lastEvent.Data["exit_code"].(float64); !ok || int(exitCode) != 1 {
		t.Errorf("expected exit_code=1 in trace data, got %v", lastEvent.Data["exit_code"])
	}
}

// ============================================================================
// Tests for backward-compatible Run() method
// ============================================================================

func TestLocalRunner_Run_NoPolicy(t *testing.T) {
	r := NewLocalRunner("/tmp")
	action := actions.NewAction(actions.ActionTypeShell, "test-cmd", map[string]interface{}{
		"command": "echo hello",
	})

	events, err := r.Run(action, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if action.Status != actions.ActionStatusComplete {
		t.Errorf("expected status complete, got %s", action.Status)
	}
}

func TestLocalRunner_Run_WithAllowPolicy(t *testing.T) {
	r := NewLocalRunner("/tmp")
	pol := &policy.Policy{
		Name:    "test-allow",
		Version: "1",
		Rules: []policy.Rule{
			{
				Action: "shell",
				Effect: policy.EffectAllow,
			},
		},
	}
	action := actions.NewAction(actions.ActionTypeShell, "test-cmd", map[string]interface{}{
		"command": "echo hello",
	})

	events, err := r.Run(action, pol)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events (start, policy, end), got %d", len(events))
	}
	if action.Status != actions.ActionStatusComplete {
		t.Errorf("expected status complete, got %s", action.Status)
	}
}

func TestLocalRunner_Run_WithDenyPolicy(t *testing.T) {
	r := NewLocalRunner("/tmp")
	pol := &policy.Policy{
		Name:          "test-deny",
		Version:       "1",
		DefaultEffect: policy.EffectDeny,
	}
	action := actions.NewAction(actions.ActionTypeShell, "denied-cmd", nil)

	events, err := r.Run(action, pol)
	if err == nil {
		t.Fatal("expected error for denied action")
	}
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if action.Status != actions.ActionStatusFailed {
		t.Errorf("expected status failed, got %s", action.Status)
	}
}
