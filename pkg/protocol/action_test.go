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

func TestActionExecutionRequest_ToAction_Structured(t *testing.T) {
	req := ActionExecutionRequest{
		Type: ActionTypeShellRun,
		Parameters: map[string]interface{}{
			"command": "echo structured",
		},
		ClientActionID: "client_123",
		Command:        "echo legacy",
	}

	action, err := req.ToAction()
	if err != nil {
		t.Fatalf("expected valid action, got error: %v", err)
	}
	if action.ID != "client_123" {
		t.Fatalf("expected client action id, got %s", action.ID)
	}
	if action.Type != ActionTypeShellRun {
		t.Fatalf("expected shell.run, got %s", action.Type)
	}
	if action.Command() != "echo structured" {
		t.Fatalf("expected structured command, got %q", action.Command())
	}
}

func TestActionExecutionRequest_ToAction_LegacyCommand(t *testing.T) {
	req := ActionExecutionRequest{Command: "browser.goto https://example.com"}

	action, err := req.ToAction()
	if err != nil {
		t.Fatalf("expected legacy command to parse, got error: %v", err)
	}
	if action.Type != ActionTypeBrowserGoto {
		t.Fatalf("expected browser.goto, got %s", action.Type)
	}
	if action.URL() != "https://example.com" {
		t.Fatalf("expected parsed URL, got %q", action.URL())
	}
}

func TestValidateAction_ValidStructuredActions(t *testing.T) {
	tests := []struct {
		name       string
		actionType ActionType
		params     map[string]interface{}
	}{
		{"shell run", ActionTypeShellRun, map[string]interface{}{"command": "pwd"}},
		{"file read", ActionTypeFileRead, map[string]interface{}{"path": "README.md"}},
		{"file write", ActionTypeFileWrite, map[string]interface{}{"path": "a.txt", "content": "hello"}},
		{"file patch", ActionTypeFilePatch, map[string]interface{}{"path": "a.txt", "patch": "@@"}},
		{"browser goto", ActionTypeBrowserGoto, map[string]interface{}{"url": "https://example.com"}},
		{"browser click selector", ActionTypeBrowserClick, map[string]interface{}{"selector": "#submit"}},
		{"browser click coordinates", ActionTypeBrowserClick, map[string]interface{}{"x": 10, "y": 20}},
		{"browser type", ActionTypeBrowserType, map[string]interface{}{"text": "hello"}},
		{"browser press", ActionTypeBrowserPress, map[string]interface{}{"key": "Enter"}},
		{"browser wait selector", ActionTypeBrowserWaitFor, map[string]interface{}{"selector": "#ready"}},
		{"browser wait timeout", ActionTypeBrowserWaitFor, map[string]interface{}{"timeout_ms": 1000}},
		{"browser screenshot", ActionTypeBrowserScreenshot, map[string]interface{}{"full_page": true}},
		{"browser evaluate", ActionTypeBrowserEvaluate, map[string]interface{}{"expression": "document.title"}},
		{"browser assert", ActionTypeBrowserAssert, map[string]interface{}{"type": "text", "expected": "Done"}},
		{"task done", ActionTypeTaskDone, map[string]interface{}{"summary": "complete"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateAction(tt.actionType, tt.params); err != nil {
				t.Fatalf("expected valid action, got error: %v", err)
			}
		})
	}
}

func TestValidateAction_InvalidStructuredActions(t *testing.T) {
	tests := []struct {
		name       string
		actionType ActionType
		params     map[string]interface{}
		code       string
	}{
		{"shell missing command", ActionTypeShellRun, nil, "invalid_action_parameters"},
		{"file read path wrong type", ActionTypeFileRead, map[string]interface{}{"path": 1}, "invalid_action_parameters"},
		{"file write missing content", ActionTypeFileWrite, map[string]interface{}{"path": "a.txt"}, "invalid_action_parameters"},
		{"file patch missing patch", ActionTypeFilePatch, map[string]interface{}{"path": "a.txt"}, "invalid_action_parameters"},
		{"browser goto missing url", ActionTypeBrowserGoto, nil, "invalid_action_parameters"},
		{"browser click incomplete coordinates", ActionTypeBrowserClick, map[string]interface{}{"x": 1}, "invalid_action_parameters"},
		{"browser type missing text", ActionTypeBrowserType, nil, "invalid_action_parameters"},
		{"browser press missing key", ActionTypeBrowserPress, nil, "invalid_action_parameters"},
		{"browser wait missing condition", ActionTypeBrowserWaitFor, nil, "invalid_action_parameters"},
		{"browser screenshot full_page wrong type", ActionTypeBrowserScreenshot, map[string]interface{}{"full_page": "yes"}, "invalid_action_parameters"},
		{"browser evaluate missing expression", ActionTypeBrowserEvaluate, nil, "invalid_action_parameters"},
		{"browser assert missing expected", ActionTypeBrowserAssert, map[string]interface{}{"type": "text"}, "invalid_action_parameters"},
		{"task done summary wrong type", ActionTypeTaskDone, map[string]interface{}{"summary": 42}, "invalid_action_parameters"},
		{"unknown type", ActionType("unknown.action"), nil, "unsupported_action_type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAction(tt.actionType, tt.params)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if err.Code != tt.code {
				t.Fatalf("expected code %s, got %s", tt.code, err.Code)
			}
		})
	}
}
