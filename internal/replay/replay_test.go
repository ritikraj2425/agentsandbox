package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/trace"
)

// writeTraceJSONL writes a list of TraceEvents as JSONL to the given path.
func writeTraceJSONL(t *testing.T, path string, events []trace.TraceEvent) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create trace file: %s", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			t.Fatalf("failed to write event: %s", err)
		}
	}
}

func TestListRuns_EmptyDir(t *testing.T) {
	// ListRuns on a directory with no .agentsandbox/runs/ should return empty.
	tmpDir := t.TempDir()

	summaries, err := ListRuns(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected 0 summaries, got %d", len(summaries))
	}
}

func TestListRuns_WithRuns(t *testing.T) {
	tmpDir := t.TempDir()
	runsDir := filepath.Join(tmpDir, ".agentsandbox", "runs")

	now := time.Now().UTC()

	// Create two run directories.
	run1Dir := filepath.Join(runsDir, "run_20260627_100000_aabbccdd")
	run2Dir := filepath.Join(runsDir, "run_20260627_110000_11223344")
	os.MkdirAll(run1Dir, 0755)
	os.MkdirAll(run2Dir, 0755)

	// Write trace events for run1 (earlier run).
	events1 := []trace.TraceEvent{
		{
			Timestamp: now.Add(-2 * time.Hour),
			Type:      trace.EventTypeRunCreated,
			Message:   "Run initialized",
		},
		{
			Timestamp: now.Add(-2*time.Hour + 50*time.Millisecond),
			Type:      trace.EventTypeProcessFinished,
			Message:   "Process finished",
			Data:      map[string]interface{}{"exit_code": float64(0)},
		},
	}
	writeTraceJSONL(t, filepath.Join(run1Dir, "trace.jsonl"), events1)

	// Write trace events for run2 (later run).
	events2 := []trace.TraceEvent{
		{
			Timestamp: now.Add(-1 * time.Hour),
			Type:      trace.EventTypeRunCreated,
			Message:   "Run initialized",
		},
		{
			Timestamp: now.Add(-1*time.Hour + 100*time.Millisecond),
			Type:      trace.EventTypeProcessFinished,
			Message:   "Process finished",
			Data:      map[string]interface{}{"exit_code": float64(1)},
		},
	}
	writeTraceJSONL(t, filepath.Join(run2Dir, "trace.jsonl"), events2)

	summaries, err := ListRuns(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	// First summary should be the more recent run (run2).
	if summaries[0].ID != "run_20260627_110000_11223344" {
		t.Errorf("expected first summary to be run2, got %s", summaries[0].ID)
	}
	if summaries[0].EventCount != 2 {
		t.Errorf("expected 2 events, got %d", summaries[0].EventCount)
	}
	if summaries[0].Status != "failed" {
		t.Errorf("expected status 'failed', got %q", summaries[0].Status)
	}

	// Second summary should be run1.
	if summaries[1].ID != "run_20260627_100000_aabbccdd" {
		t.Errorf("expected second summary to be run1, got %s", summaries[1].ID)
	}
	if summaries[1].Status != "completed" {
		t.Errorf("expected status 'completed', got %q", summaries[1].Status)
	}
}

func TestLoadRun_Full(t *testing.T) {
	tmpDir := t.TempDir()
	runsDir := filepath.Join(tmpDir, ".agentsandbox", "runs")
	runID := "run_20260627_120000_deadbeef"
	runDir := filepath.Join(runsDir, runID)
	os.MkdirAll(runDir, 0755)

	now := time.Now().UTC()

	// Write trace events.
	events := []trace.TraceEvent{
		{
			Timestamp: now,
			Type:      trace.EventTypeRunCreated,
			Message:   "Run initialized",
			Data:      map[string]interface{}{"run_id": runID},
		},
		{
			Timestamp: now.Add(10 * time.Millisecond),
			Type:      trace.EventTypeActionReceived,
			Message:   "Received command",
		},
		{
			Timestamp: now.Add(200 * time.Millisecond),
			Type:      trace.EventTypeProcessFinished,
			Message:   "Process finished",
			Data:      map[string]interface{}{"exit_code": float64(0)},
		},
	}
	writeTraceJSONL(t, filepath.Join(runDir, "trace.jsonl"), events)

	// Write stdout and stderr.
	os.WriteFile(filepath.Join(runDir, "stdout.log"), []byte("hello world\n"), 0644)
	os.WriteFile(filepath.Join(runDir, "stderr.log"), []byte("some warning\n"), 0644)

	// Write report.json.
	report := map[string]interface{}{"status": "completed"}
	reportData, _ := json.Marshal(report)
	os.WriteFile(filepath.Join(runDir, "report.json"), reportData, 0644)

	// Load the run.
	run, err := LoadRun(tmpDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if run.ID != runID {
		t.Errorf("expected ID %q, got %q", runID, run.ID)
	}
	if len(run.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(run.Events))
	}
	if run.Stdout != "hello world\n" {
		t.Errorf("unexpected stdout: %q", run.Stdout)
	}
	if run.Stderr != "some warning\n" {
		t.Errorf("unexpected stderr: %q", run.Stderr)
	}
	if run.Report == nil {
		t.Error("expected report to be loaded")
	}
	if run.DurationMs <= 0 {
		t.Errorf("expected positive duration, got %d", run.DurationMs)
	}
}

func TestLoadRun_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadRun(tmpDir, "nonexistent_run")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestLoadRun_MinimalTrace(t *testing.T) {
	tmpDir := t.TempDir()
	runsDir := filepath.Join(tmpDir, ".agentsandbox", "runs")
	runID := "run_minimal"
	runDir := filepath.Join(runsDir, runID)
	os.MkdirAll(runDir, 0755)

	// Write a trace with only one event — duration should be 0.
	events := []trace.TraceEvent{
		{
			Timestamp: time.Now().UTC(),
			Type:      trace.EventTypeRunCreated,
			Message:   "Run initialized",
		},
	}
	writeTraceJSONL(t, filepath.Join(runDir, "trace.jsonl"), events)

	run, err := LoadRun(tmpDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if run.DurationMs != 0 {
		t.Errorf("expected 0 duration for single event, got %d", run.DurationMs)
	}
	if run.Stdout != "" {
		t.Errorf("expected empty stdout, got %q", run.Stdout)
	}
}
