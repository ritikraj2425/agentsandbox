package local

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/trace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// ─────────────────────────────────────────────────────────────────────────────
// Core execution tests
// ─────────────────────────────────────────────────────────────────────────────

func TestRuntime_Name(t *testing.T) {
	rt := New("/tmp", nil)
	if rt.Name() != "local" {
		t.Errorf("expected name 'local', got %q", rt.Name())
	}
}

func TestRuntime_EchoHello(t *testing.T) {
	rt := New("/tmp", nil)
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "echo hello",
	})

	obs, err := rt.Run(context.Background(), action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obs.Status != protocol.ObsStatusCompleted {
		t.Errorf("expected status completed, got %s (error: %s)", obs.Status, obs.Error)
	}
	if obs.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", obs.ExitCode)
	}
	if obs.StdoutSummary != "hello\n" {
		t.Errorf("expected stdout 'hello\\n', got %q", obs.StdoutSummary)
	}
	if obs.Backend != "local" {
		t.Errorf("expected backend 'local', got %q", obs.Backend)
	}
	if obs.Command != "echo hello" {
		t.Errorf("expected command 'echo hello', got %s", obs.Command)
	}
}

func TestRuntime_SessionWorkspacesAreIsolated(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	rtA := New(dirA, nil)
	rtB := New(dirB, nil)

	writeAction := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "printf secret > only-a.txt",
	})
	if obs, err := rtA.Run(context.Background(), writeAction); err != nil || obs.Status != protocol.ObsStatusCompleted {
		t.Fatalf("write in workspace A failed: status=%s err=%v", obs.Status, err)
	}

	readAction := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "test ! -e only-a.txt",
	})
	if obs, err := rtB.Run(context.Background(), readAction); err != nil || obs.Status != protocol.ObsStatusCompleted {
		t.Fatalf("workspace B should not see workspace A file: status=%s err=%v stderr=%s", obs.Status, err, obs.StderrSummary)
	}
}

func TestRuntime_InvalidCommand(t *testing.T) {
	rt := New("/tmp", nil)
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "invalid-command-xyz-12345",
	})

	obs, _ := rt.Run(context.Background(), action)

	if obs.Status != protocol.ObsStatusFailed {
		t.Errorf("expected status failed, got %s", obs.Status)
	}
	if obs.ExitCode != 127 {
		t.Errorf("expected exit code 127, got %d", obs.ExitCode)
	}
}

func TestRuntime_NonZeroExit(t *testing.T) {
	rt := New("/tmp", nil)
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

func TestRuntime_CapturesStderr(t *testing.T) {
	rt := New("/tmp", nil)
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "echo error_msg >&2",
	})

	obs, err := rt.Run(context.Background(), action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obs.StderrSummary != "error_msg\n" {
		t.Errorf("expected stderr 'error_msg\\n', got %q", obs.StderrSummary)
	}
}

func TestRuntime_Timeout(t *testing.T) {
	rt := New("/tmp", nil)
	rt.SetTimeout(100 * time.Millisecond)

	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "sleep 10",
	})

	obs, _ := rt.Run(context.Background(), action)

	if obs.Status != protocol.ObsStatusTimeout {
		t.Errorf("expected status timeout, got %s (error: %s)", obs.Status, obs.Error)
	}
}

func TestRuntime_NoCommand(t *testing.T) {
	rt := New("/tmp", nil)
	action := protocol.NewAction(protocol.ActionTypeShellRun, nil)

	obs, err := rt.Run(context.Background(), action)

	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if obs.Status != protocol.ObsStatusFailed {
		t.Errorf("expected status failed, got %s", obs.Status)
	}
}

func TestRuntime_WorkingDirectory(t *testing.T) {
	rt := New("/tmp", nil)
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "pwd",
	})

	obs, err := rt.Run(context.Background(), action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// On macOS, /tmp is a symlink to /private/tmp.
	if obs.StdoutSummary != "/tmp\n" && obs.StdoutSummary != "/private/tmp\n" {
		t.Errorf("expected working directory /tmp, got %q", obs.StdoutSummary)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Trace logger integration tests
// ─────────────────────────────────────────────────────────────────────────────

func TestRuntime_WithLogger(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := trace.NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}

	rt := New("/tmp", logger)
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "echo logged_output",
	})

	obs, runErr := rt.Run(context.Background(), action)
	logger.Close()

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
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

func TestRuntime_WithLogger_FailedCommand(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := trace.NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}

	rt := New("/tmp", logger)
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "exit 1",
	})

	obs, _ := rt.Run(context.Background(), action)
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

	if exitCode, ok := lastEvent.Data["exit_code"].(float64); !ok || int(exitCode) != 1 {
		t.Errorf("expected exit_code=1 in trace data, got %v", lastEvent.Data["exit_code"])
	}
}

func TestRuntime_BackendField(t *testing.T) {
	rt := New("/tmp", nil)
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": "echo test",
	})

	obs, err := rt.Run(context.Background(), action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Backend != "local" {
		t.Errorf("expected backend 'local', got %q", obs.Backend)
	}
}
