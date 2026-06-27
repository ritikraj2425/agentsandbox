package protocol

import (
	"strings"
	"testing"
)

func TestNewObservation(t *testing.T) {
	obs := NewObservation("act_12345678")

	if obs.ActionID != "act_12345678" {
		t.Errorf("expected action ID 'act_12345678', got %s", obs.ActionID)
	}
	// Default exit code is -1 meaning "never ran".
	if obs.ExitCode != -1 {
		t.Errorf("expected default exit code -1, got %d", obs.ExitCode)
	}
	if obs.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestTruncateOutput_Short(t *testing.T) {
	// Short output should not be truncated.
	output := "line1\nline2\nline3"
	result := TruncateOutput(output)
	if result != output {
		t.Errorf("short output should not be modified, got %s", result)
	}
}

func TestTruncateOutput_Long(t *testing.T) {
	// Build 100 lines of output.
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "line")
	}
	output := strings.Join(lines, "\n")

	result := TruncateOutput(output)

	// Result should start with truncation notice.
	if !strings.HasPrefix(result, "[truncated to last 50 lines]") {
		t.Error("expected truncation notice prefix")
	}

	// Count the lines in the result (notice + 50 lines = 51).
	resultLines := strings.Split(result, "\n")
	if len(resultLines) != 51 {
		t.Errorf("expected 51 lines (1 notice + 50 content), got %d", len(resultLines))
	}
}

func TestTruncateOutput_Empty(t *testing.T) {
	result := TruncateOutput("")
	if result != "" {
		t.Errorf("empty output should return empty, got %q", result)
	}
}

func TestTruncateOutput_ExactlyAtLimit(t *testing.T) {
	// Exactly 50 lines should NOT be truncated.
	var lines []string
	for i := 0; i < MaxSummaryLines; i++ {
		lines = append(lines, "line")
	}
	output := strings.Join(lines, "\n")
	result := TruncateOutput(output)
	if result != output {
		t.Error("output at exactly the limit should not be truncated")
	}
}
