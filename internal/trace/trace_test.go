// Package trace provides tests for TraceEvent, Trace, and RunLogger.
//
// The RunLogger tests create temporary directories, run logger operations,
// and verify that the correct files are created with valid content.
// All temp directories are cleaned up after each test.
package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Legacy TraceEvent and Trace tests (from Phase 0)
// ============================================================================

func TestNewTraceEvent(t *testing.T) {
	event := NewTraceEvent(EventTypeActionStart, "starting action")

	if event.Type != EventTypeActionStart {
		t.Errorf("expected type %s, got %s", EventTypeActionStart, event.Type)
	}
	if event.Message != "starting action" {
		t.Errorf("expected message 'starting action', got %s", event.Message)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
	if event.Data != nil {
		t.Error("expected nil data")
	}
}

func TestNewTraceEventWithData(t *testing.T) {
	data := map[string]interface{}{
		"exit_code": 0,
		"output":    "hello",
	}
	event := NewTraceEventWithData(EventTypeOutput, "command output", data)

	if event.Data == nil {
		t.Fatal("expected data to be set")
	}
	if event.Data["exit_code"] != 0 {
		t.Errorf("expected exit_code=0, got %v", event.Data["exit_code"])
	}
}

func TestTrace_Add(t *testing.T) {
	tr := NewTrace("action-123")

	if tr.ActionID != "action-123" {
		t.Errorf("expected action ID 'action-123', got %s", tr.ActionID)
	}
	if tr.Len() != 0 {
		t.Errorf("expected empty trace, got %d events", tr.Len())
	}

	tr.Add(NewTraceEvent(EventTypeActionStart, "start"))
	tr.Add(NewTraceEvent(EventTypeActionEnd, "end"))

	if tr.Len() != 2 {
		t.Errorf("expected 2 events, got %d", tr.Len())
	}
	if tr.Events[0].Type != EventTypeActionStart {
		t.Errorf("expected first event to be action_start, got %s", tr.Events[0].Type)
	}
	if tr.Events[1].Type != EventTypeActionEnd {
		t.Errorf("expected second event to be action_end, got %s", tr.Events[1].Type)
	}
}

// ============================================================================
// RunLogger tests (Phase 2)
// ============================================================================

// TestNewRunLogger_CreatesDirectory verifies that NewRunLogger creates
// the correct directory structure: <baseDir>/.agentsandbox/runs/<run_id>/
func TestNewRunLogger_CreatesDirectory(t *testing.T) {
	// Create a temporary directory for this test.
	// t.TempDir() automatically cleans up when the test finishes.
	tmpDir := t.TempDir()

	logger, err := NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}
	defer logger.Close()

	// Verify the run directory was created.
	if _, err := os.Stat(logger.RunDir); os.IsNotExist(err) {
		t.Fatalf("run directory was not created: %s", logger.RunDir)
	}

	// Verify the run ID starts with "run_".
	if !strings.HasPrefix(logger.RunID, "run_") {
		t.Errorf("expected run ID to start with 'run_', got %s", logger.RunID)
	}

	// Verify the directory is inside .agentsandbox/runs/.
	expectedParent := filepath.Join(tmpDir, ".agentsandbox", "runs")
	if !strings.HasPrefix(logger.RunDir, expectedParent) {
		t.Errorf("expected run dir under %s, got %s", expectedParent, logger.RunDir)
	}
}

// TestNewRunLogger_CreatesTraceFile verifies that trace.jsonl is created
// and contains the initial "run.created" event.
func TestNewRunLogger_CreatesTraceFile(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}
	defer logger.Close()

	// Verify trace.jsonl exists.
	tracePath := filepath.Join(logger.RunDir, "trace.jsonl")
	if _, err := os.Stat(tracePath); os.IsNotExist(err) {
		t.Fatalf("trace.jsonl was not created at %s", tracePath)
	}

	// Read and parse the first line — it should be the "run.created" event.
	content, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("failed to read trace.jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 1 {
		t.Fatal("trace.jsonl is empty, expected at least the run.created event")
	}

	var event TraceEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("first line of trace.jsonl is not valid JSON: %v", err)
	}

	if event.Type != EventTypeRunCreated {
		t.Errorf("expected first event type %s, got %s", EventTypeRunCreated, event.Type)
	}
	if event.Data["run_id"] != logger.RunID {
		t.Errorf("expected run_id %s in event data, got %v", logger.RunID, event.Data["run_id"])
	}
}

// TestRunLogger_LogEvent verifies that LogEvent appends events to trace.jsonl.
func TestRunLogger_LogEvent(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}

	// Log additional events.
	logger.LogEvent(EventTypeActionReceived, "Received command", map[string]interface{}{
		"command": "echo hello",
	})
	logger.LogEvent(EventTypeProcessStarted, "Process started", nil)
	logger.LogEvent(EventTypeProcessFinished, "Process finished", map[string]interface{}{
		"exit_code":   0,
		"duration_ms": 42,
	})

	logger.Close()

	// Read trace.jsonl and verify we have 4 events total
	// (1 run.created + 3 manually logged).
	tracePath := filepath.Join(logger.RunDir, "trace.jsonl")
	content, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("failed to read trace.jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 event lines in trace.jsonl, got %d", len(lines))
	}

	// Verify each line is valid JSON.
	expectedTypes := []EventType{
		EventTypeRunCreated,
		EventTypeActionReceived,
		EventTypeProcessStarted,
		EventTypeProcessFinished,
	}
	for i, line := range lines {
		var event TraceEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
			continue
		}
		if event.Type != expectedTypes[i] {
			t.Errorf("line %d: expected type %s, got %s", i, expectedTypes[i], event.Type)
		}
	}
}

// TestRunLogger_WriteStdout verifies that stdout is written to stdout.log.
func TestRunLogger_WriteStdout(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}
	defer logger.Close()

	logger.WriteStdout("hello world\n")

	content, err := os.ReadFile(filepath.Join(logger.RunDir, "stdout.log"))
	if err != nil {
		t.Fatalf("failed to read stdout.log: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", string(content))
	}
}

// TestRunLogger_WriteStderr verifies that stderr is written to stderr.log.
func TestRunLogger_WriteStderr(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}
	defer logger.Close()

	logger.WriteStderr("error: something went wrong\n")

	content, err := os.ReadFile(filepath.Join(logger.RunDir, "stderr.log"))
	if err != nil {
		t.Fatalf("failed to read stderr.log: %v", err)
	}
	if string(content) != "error: something went wrong\n" {
		t.Errorf("expected error message, got %q", string(content))
	}
}

// TestRunLogger_WriteReport verifies that report.json contains valid,
// pretty-printed JSON.
func TestRunLogger_WriteReport(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}
	defer logger.Close()

	// Write a mock observation-like struct.
	report := map[string]interface{}{
		"action_id":      "act_test123",
		"status":         "completed",
		"exit_code":      0,
		"duration_ms":    42,
		"stdout_summary": "hello\n",
		"stderr_summary": "",
	}
	logger.WriteReport(report)

	content, err := os.ReadFile(filepath.Join(logger.RunDir, "report.json"))
	if err != nil {
		t.Fatalf("failed to read report.json: %v", err)
	}

	// Verify it's valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("report.json is not valid JSON: %v", err)
	}

	// Verify key fields.
	if parsed["action_id"] != "act_test123" {
		t.Errorf("expected action_id 'act_test123', got %v", parsed["action_id"])
	}
	if parsed["status"] != "completed" {
		t.Errorf("expected status 'completed', got %v", parsed["status"])
	}

	// Verify it's pretty-printed (indented).
	if !strings.Contains(string(content), "\n  ") {
		t.Error("expected pretty-printed JSON with indentation")
	}
}

// TestRunLogger_NilSafe verifies that a nil RunLogger does not panic.
// This is important because we pass nil when logging is not desired.
func TestRunLogger_NilSafe(t *testing.T) {
	var logger *RunLogger

	// None of these should panic.
	logger.LogEvent(EventTypeRunCreated, "test", nil)
	logger.WriteStdout("test")
	logger.WriteStderr("test")
	logger.WriteReport(nil)
	logger.Close()
}

// TestRunLogger_EmptyContent verifies that empty stdout/stderr creates
// the log files but with empty content.
func TestRunLogger_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewRunLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewRunLogger failed: %v", err)
	}
	defer logger.Close()

	logger.WriteStdout("")
	logger.WriteStderr("")

	// Files should exist but be empty.
	stdoutContent, err := os.ReadFile(filepath.Join(logger.RunDir, "stdout.log"))
	if err != nil {
		t.Fatalf("stdout.log should exist: %v", err)
	}
	if len(stdoutContent) != 0 {
		t.Errorf("expected empty stdout.log, got %d bytes", len(stdoutContent))
	}

	stderrContent, err := os.ReadFile(filepath.Join(logger.RunDir, "stderr.log"))
	if err != nil {
		t.Fatalf("stderr.log should exist: %v", err)
	}
	if len(stderrContent) != 0 {
		t.Errorf("expected empty stderr.log, got %d bytes", len(stderrContent))
	}
}
