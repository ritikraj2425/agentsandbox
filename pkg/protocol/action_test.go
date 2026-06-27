package protocol

import "testing"

func TestNewAction(t *testing.T) {
	a := NewAction(ActionTypeShellRun, map[string]interface{}{
		"command": "echo hello",
	})

	// ID should have the "act_" prefix and 8 hex characters.
	if len(a.ID) != 12 { // "act_" (4) + 8 hex chars
		t.Errorf("expected ID length 12, got %d: %s", len(a.ID), a.ID)
	}
	if a.ID[:4] != "act_" {
		t.Errorf("expected ID prefix 'act_', got %s", a.ID[:4])
	}

	if a.Type != ActionTypeShellRun {
		t.Errorf("expected type %s, got %s", ActionTypeShellRun, a.Type)
	}
	if a.Command() != "echo hello" {
		t.Errorf("expected command 'echo hello', got %s", a.Command())
	}
	if a.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestAction_Command_MissingParam(t *testing.T) {
	// Action with no "command" parameter should return empty string.
	a := NewAction(ActionTypeShellRun, nil)
	if a.Command() != "" {
		t.Errorf("expected empty command, got %s", a.Command())
	}
}

func TestAction_UniqueIDs(t *testing.T) {
	// Two actions should have different IDs.
	a1 := NewAction(ActionTypeShellRun, nil)
	a2 := NewAction(ActionTypeShellRun, nil)
	if a1.ID == a2.ID {
		t.Errorf("expected unique IDs, both got %s", a1.ID)
	}
}
