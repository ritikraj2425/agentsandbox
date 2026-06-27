// Package actions provides tests for the Action type.
package actions

import (
	"testing"
)

func TestNewAction(t *testing.T) {
	params := map[string]interface{}{
		"command": "echo hello",
	}
	a := NewAction(ActionTypeShell, "test-echo", params)

	if a.Type != ActionTypeShell {
		t.Errorf("expected type %s, got %s", ActionTypeShell, a.Type)
	}
	if a.Name != "test-echo" {
		t.Errorf("expected name 'test-echo', got %s", a.Name)
	}
	if a.Status != ActionStatusPending {
		t.Errorf("expected status %s, got %s", ActionStatusPending, a.Status)
	}
	if a.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if a.Parameters["command"] != "echo hello" {
		t.Errorf("expected parameter command='echo hello', got %v", a.Parameters["command"])
	}
}

func TestAction_IsPending(t *testing.T) {
	a := NewAction(ActionTypeFileWrite, "write", nil)
	if !a.IsPending() {
		t.Error("new action should be pending")
	}

	a.Status = ActionStatusRunning
	if a.IsPending() {
		t.Error("running action should not be pending")
	}
}

func TestAction_Complete(t *testing.T) {
	a := NewAction(ActionTypeFileRead, "read", nil)
	a.Complete()

	if a.Status != ActionStatusComplete {
		t.Errorf("expected status %s, got %s", ActionStatusComplete, a.Status)
	}
	if a.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if !a.IsTerminal() {
		t.Error("completed action should be terminal")
	}
}

func TestAction_Fail(t *testing.T) {
	a := NewAction(ActionTypeShell, "bad-cmd", nil)
	a.Fail("command not found")

	if a.Status != ActionStatusFailed {
		t.Errorf("expected status %s, got %s", ActionStatusFailed, a.Status)
	}
	if a.Error != "command not found" {
		t.Errorf("expected error 'command not found', got %s", a.Error)
	}
	if a.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if !a.IsTerminal() {
		t.Error("failed action should be terminal")
	}
}

func TestAction_IsTerminal(t *testing.T) {
	tests := []struct {
		status   ActionStatus
		terminal bool
	}{
		{ActionStatusPending, false},
		{ActionStatusApproved, false},
		{ActionStatusRunning, false},
		{ActionStatusComplete, true},
		{ActionStatusFailed, true},
		{ActionStatusDenied, true},
	}

	for _, tt := range tests {
		a := &Action{Status: tt.status}
		if a.IsTerminal() != tt.terminal {
			t.Errorf("status %s: expected terminal=%v, got %v", tt.status, tt.terminal, a.IsTerminal())
		}
	}
}
