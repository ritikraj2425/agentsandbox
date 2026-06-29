// Package trace provides structured event logging for agent action execution.
//
// Every action produces a trace of events that can be inspected, replayed,
// and used for debugging or auditing.
//
// Phase 2 adds the RunLogger — a struct that manages a run directory
// and writes trace events, stdout/stderr logs, and a final report to disk.
//
// The run directory structure:
//
//	.agentsandbox/runs/<run_id>/
//	├── trace.jsonl     # One JSON object per line (append-only event log)
//	├── stdout.log      # Raw stdout captured from the process
//	├── stderr.log      # Raw stderr captured from the process
//	└── report.json     # The full Observation as pretty-printed JSON
//
// Why JSONL (JSON Lines) for trace.jsonl?
// Each line is a complete, independent JSON object. This format:
//   - Can be appended to without reading the whole file (no closing bracket).
//   - Can be streamed line-by-line (each line is parse-able on its own).
//   - Is the standard format for observability tools (Datadog, Elastic, etc.).
//   - Can be processed with simple Unix tools: `cat trace.jsonl | jq .`
//
// Compare with a JSON array: you'd need to remove the closing "]", add a comma,
// append the new object, and add "]" back. Much more fragile.
package trace

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EventType categorizes trace events.
//
// Each event type maps to a specific moment in the action lifecycle.
// When you read a trace.jsonl file, the event types tell you the story
// of what happened in chronological order.
type EventType string

const (
	// EventTypeRunCreated is logged when a new run directory is initialized.
	// This is always the FIRST event in any trace.
	EventTypeRunCreated EventType = "run.created"

	// EventTypeActionReceived is logged when the CLI receives a command.
	// At this point the command has been parsed but NOT yet executed.
	EventTypeActionReceived EventType = "action.received"

	// EventTypeProcessStarted is logged when os/exec starts the process.
	// Between "action.received" and "process.started", the policy engine
	// (Phase 3) will run — so a gap here indicates policy evaluation time.
	EventTypeProcessStarted EventType = "process.started"

	// EventTypeProcessFinished is logged when the process exits.
	// The data field will contain exit_code and duration_ms.
	EventTypeProcessFinished EventType = "process.finished"

	// --- Legacy event types (kept for backward compatibility with Phase 0/1) ---

	// EventTypeActionStart marks the beginning of action execution (legacy).
	EventTypeActionStart EventType = "action_start"
	// EventTypeActionEnd marks the end of action execution (legacy).
	EventTypeActionEnd EventType = "action_end"
	// EventTypePolicyCheck records the evaluation of a command against a policy.
	EventTypePolicyCheck EventType = "policy.check"

	// EventTypeApprovalRequested records when the engine paused to ask a human.
	EventTypeApprovalRequested EventType = "approval.requested"

	// EventTypeApprovalDecision records the human's response to an approval request.
	EventTypeApprovalDecision EventType = "approval.decision"

	// EventTypeHumanInteraction records scoped end-user browser handoff events.
	EventTypeHumanInteraction EventType = "human.interaction"

	// EventTypeProcessStarted records the exact moment the OS process launched.
	EventTypeOutput EventType = "output"
	// EventTypeError records an error during execution (legacy).
	EventTypeError EventType = "error"
	// EventTypeFSDiff records a filesystem change (legacy).
	EventTypeFSDiff EventType = "fs_diff"
)

// TraceEvent represents a single event in the execution trace of an action.
//
// This is the core data structure written to trace.jsonl. Each line in
// the file is one JSON-serialized TraceEvent.
//
// Design decisions:
//   - Timestamp is always UTC for consistency across time zones.
//   - Data is a flexible map for event-specific details (exit codes,
//     command strings, file paths, etc.).
//   - Message is human-readable for quick scanning in logs.
type TraceEvent struct {
	// Timestamp is when the event occurred (always UTC).
	Timestamp time.Time `json:"timestamp"`

	// Type categorizes the event (e.g., "run.created", "process.finished").
	Type EventType `json:"type"`

	// Message is a human-readable description of the event.
	Message string `json:"message"`

	// Data holds optional structured data associated with the event.
	// For example: {"exit_code": 0, "duration_ms": 42}
	Data map[string]interface{} `json:"data,omitempty"`
}

// NewTraceEvent creates a new TraceEvent with the current timestamp.
func NewTraceEvent(eventType EventType, message string) TraceEvent {
	return TraceEvent{
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Message:   message,
	}
}

// NewTraceEventWithData creates a new TraceEvent with associated data.
func NewTraceEventWithData(eventType EventType, message string, data map[string]interface{}) TraceEvent {
	return TraceEvent{
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Message:   message,
		Data:      data,
	}
}

// Trace is an ordered collection of TraceEvents for a single action execution.
// This is primarily used by the legacy Run() method in the runner.
type Trace struct {
	// ActionID links this trace to its originating action.
	ActionID string `json:"action_id"`

	// Events is the ordered list of trace events.
	Events []TraceEvent `json:"events"`
}

// NewTrace creates a new empty Trace for the given action.
func NewTrace(actionID string) *Trace {
	return &Trace{
		ActionID: actionID,
		Events:   make([]TraceEvent, 0),
	}
}

// Add appends an event to the trace.
func (t *Trace) Add(event TraceEvent) {
	t.Events = append(t.Events, event)
}

// Len returns the number of events in the trace.
func (t *Trace) Len() int {
	return len(t.Events)
}

// ============================================================================
// RunLogger — Phase 2: Persistent trace logging to disk
// ============================================================================

// RunLogger manages the run directory and writes structured logs to disk.
//
// Every time you run "agentsandbox run ..." a new RunLogger is created.
// It creates a unique directory under .agentsandbox/runs/ and writes:
//   - trace.jsonl: append-only event log (one JSON object per line)
//   - stdout.log: raw stdout from the executed process
//   - stderr.log: raw stderr from the executed process
//   - report.json: the final Observation as pretty-printed JSON
//
// Why a struct instead of standalone functions?
// The RunLogger holds state that multiple functions need:
//   - The run directory path (where to write files)
//   - The open file handle for trace.jsonl (so we can append without re-opening)
//   - The run ID (to include in log messages)
//
// Having a struct keeps all of this together and makes cleanup easy (Close()).
type RunLogger struct {
	// RunID uniquely identifies this run.
	// Format: "run_<YYYYMMDD>_<HHmmss>_<random>"
	// The timestamp prefix makes runs sortable by time in directory listings.
	RunID string

	// RunDir is the absolute path to this run's directory.
	// Example: "/Users/ritikraj/project/.agentsandbox/runs/run_20260627_090000_a1b2c3d4"
	RunDir string

	// traceFile is the open file handle for trace.jsonl.
	// We keep it open for the entire run so we can append events efficiently
	// without re-opening the file each time.
	traceFile *os.File

	// encoder is a JSON encoder that writes to traceFile.
	// Using an encoder instead of json.Marshal + file.Write is more efficient
	// because it avoids allocating intermediate byte slices.
	encoder *json.Encoder
}

// NewRunLogger creates a new run directory and initializes the trace log.
//
// NewRunLogger initializes a new TraceLogger for the specified execution context.
//
// The logger creates a unique, chronological run directory under `<baseDir>/.agentsandbox/runs/`
// to persist the action's lifecycle events, file diffs, and output streams.
//
// It emits the initial "run.created" event. The caller is responsible for invoking Close()
// to flush resources.
func NewRunLogger(baseDir string) (*RunLogger, error) {
	return newRunLogger(filepath.Join(baseDir, ".agentsandbox", "runs"))
}

// NewRunLoggerInDir creates a run logger directly under runRoot.
func NewRunLoggerInDir(runRoot string) (*RunLogger, error) {
	return newRunLogger(runRoot)
}

func newRunLogger(runRoot string) (*RunLogger, error) {
	now := time.Now()
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	runID := fmt.Sprintf("run_%s_%x", now.Format("20060102_150405"), randomBytes)

	runDir := filepath.Join(runRoot, runID)

	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create run directory %s: %w", runDir, err)
	}

	// Open trace.jsonl for writing.
	// os.O_CREATE: create the file if it doesn't exist.
	// os.O_WRONLY: open for writing only (we never read this file from code).
	// os.O_APPEND: all writes go to the end of the file (important for JSONL).
	// 0644: owner can read/write, group and others can read.
	traceFilePath := filepath.Join(runDir, "trace.jsonl")
	traceFile, err := os.OpenFile(traceFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace file %s: %w", traceFilePath, err)
	}

	logger := &RunLogger{
		RunID:     runID,
		RunDir:    runDir,
		traceFile: traceFile,
		encoder:   json.NewEncoder(traceFile),
	}

	// Log the very first event: the run has been created.
	// This timestamp marks the beginning of the run lifecycle.
	logger.LogEvent(EventTypeRunCreated, "Run initialized", map[string]interface{}{
		"run_id": runID,
	})

	return logger, nil
}

// LogEvent appends a structured event to trace.jsonl.
//
// Each call writes exactly one JSON line to the file. The event is
// immediately flushed (JSON encoder writes through to the file).
//
// Parameters:
//   - eventType: what happened (e.g., EventTypeProcessStarted)
//   - message: human-readable description
//   - data: optional key-value pairs with structured details
//
// If data is nil, the "data" field is omitted from the JSON output
// (thanks to the `omitempty` tag on TraceEvent.Data).
func (l *RunLogger) LogEvent(eventType EventType, message string, data map[string]interface{}) {
	if l == nil || l.encoder == nil {
		return // Safety: no-op if logger is nil (allows optional logging).
	}

	event := TraceEvent{
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Message:   message,
		Data:      data,
	}

	// encoder.Encode writes JSON + newline to the underlying file.
	// Each call produces exactly one line in the JSONL file.
	// We intentionally ignore the error here because trace logging
	// should never crash the main execution — it's observability,
	// not critical path.
	_ = l.encoder.Encode(event)
}

// WriteStdout writes the raw process stdout to stdout.log.
//
// We write the FULL stdout here (not truncated). The truncated version
// goes into the Observation for the agent; the full version is preserved
// on disk for human debugging.
func (l *RunLogger) WriteStdout(content string) {
	if l == nil {
		return
	}
	path := filepath.Join(l.RunDir, "stdout.log")
	// os.WriteFile creates the file if it doesn't exist, or overwrites it.
	// 0644 = owner read/write, others read-only.
	_ = os.WriteFile(path, []byte(content), 0644)
}

// WriteStderr writes the raw process stderr to stderr.log.
func (l *RunLogger) WriteStderr(content string) {
	if l == nil {
		return
	}
	path := filepath.Join(l.RunDir, "stderr.log")
	_ = os.WriteFile(path, []byte(content), 0644)
}

// WriteReport persists the detailed Observation summary to report.json.
//
// Serializing the final observation provides an easily auditable artifact
// documenting the action's execution context, exit status, and truncated outputs.
func (l *RunLogger) WriteReport(value interface{}) {
	if l == nil {
		return
	}
	path := filepath.Join(l.RunDir, "report.json")

	// json.MarshalIndent produces pretty-printed JSON with 2-space indentation.
	// The "" prefix means no extra prefix before each line.
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return // Silently skip if marshaling fails (should never happen).
	}

	// Append a newline at the end for clean cat/less output.
	data = append(data, '\n')

	_ = os.WriteFile(path, data, 0644)
}

// WriteDiff writes the filesystem diff report to file-diff.json.
//
// Providing a detailed diff report allows developers to audit exactly
// which files an agent action mutated during execution.
func (l *RunLogger) WriteDiff(value interface{}) {
	if l == nil {
		return
	}
	path := filepath.Join(l.RunDir, "file-diff.json")

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return
	}

	data = append(data, '\n')
	_ = os.WriteFile(path, data, 0644)
}

// Close flushes and closes the trace.jsonl file handle.
//
// IMPORTANT: Always call Close() when the run is finished.
// In Go, unclosed file handles are cleaned up by the garbage collector
// eventually, but it's bad practice to rely on that because:
//   - Data might not be flushed to disk (buffered writes could be lost).
//   - The OS has a limit on open file handles per process.
//   - It's a sign of sloppy resource management.
//
// The standard Go pattern is:
//
//	logger, err := trace.NewRunLogger(dir)
//	if err != nil { ... }
//	defer logger.Close()  // <-- always pair creation with defer Close()
func (l *RunLogger) Close() {
	if l == nil || l.traceFile == nil {
		return
	}
	_ = l.traceFile.Close()
}
