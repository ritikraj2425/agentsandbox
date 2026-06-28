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

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		expectType ActionType
		validate   func(t *testing.T, a Action)
	}{
		{
			name:       "goto space",
			command:    "browser.goto https://example.com",
			expectType: ActionTypeBrowserGoto,
			validate: func(t *testing.T, a Action) {
				if a.URL() != "https://example.com" {
					t.Errorf("expected url https://example.com, got %s", a.URL())
				}
			},
		},
		{
			name:       "goto function",
			command:    "browser.goto(\"https://example.com\")",
			expectType: ActionTypeBrowserGoto,
			validate: func(t *testing.T, a Action) {
				if a.URL() != "https://example.com" {
					t.Errorf("expected url https://example.com, got %s", a.URL())
				}
			},
		},
		{
			name:       "screenshot",
			command:    "browser.screenshot",
			expectType: ActionTypeBrowserScreenshot,
		},
		{
			name:       "click selector",
			command:    "browser.click(\"#submit-btn\")",
			expectType: ActionTypeBrowserClick,
			validate: func(t *testing.T, a Action) {
				if a.Selector() != "#submit-btn" {
					t.Errorf("expected selector #submit-btn, got %s", a.Selector())
				}
			},
		},
		{
			name:       "click coordinates",
			command:    "browser.click(100, 200)",
			expectType: ActionTypeBrowserClick,
			validate: func(t *testing.T, a Action) {
				x, y, ok := a.Coordinates()
				if !ok || x != 100 || y != 200 {
					t.Errorf("expected coordinates 100, 200, got %.0f, %.0f, %t", x, y, ok)
				}
			},
		},
		{
			name:       "click coordinates space",
			command:    "browser.click 150 250",
			expectType: ActionTypeBrowserClick,
			validate: func(t *testing.T, a Action) {
				x, y, ok := a.Coordinates()
				if !ok || x != 150 || y != 250 {
					t.Errorf("expected coordinates 150, 250, got %.0f, %.0f, %t", x, y, ok)
				}
			},
		},
		{
			name:       "type",
			command:    "browser.type(\"some text to type\")",
			expectType: ActionTypeBrowserType,
			validate: func(t *testing.T, a Action) {
				if a.Text() != "some text to type" {
					t.Errorf("expected text, got %s", a.Text())
				}
			},
		},
		{
			name:       "shell command",
			command:    "go test ./...",
			expectType: ActionTypeShellRun,
			validate: func(t *testing.T, a Action) {
				if a.Command() != "go test ./..." {
					t.Errorf("expected command 'go test ./...', got %s", a.Command())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := ParseCommand(tt.command)
			if a.Type != tt.expectType {
				t.Fatalf("expected type %s, got %s", tt.expectType, a.Type)
			}
			if tt.validate != nil {
				tt.validate(t, a)
			}
		})
	}
}
