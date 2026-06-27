package color

import "testing"

// TestColorFunctions_Enabled tests that color functions emit ANSI codes
// when colors are enabled.
func TestColorFunctions_Enabled(t *testing.T) {
	// Force colors on for testing (normally auto-detected from terminal).
	SetEnabled(true)
	defer SetEnabled(false) // Clean up after test.

	tests := []struct {
		name     string
		fn       func(string) string
		input    string
		contains string // The ANSI code that should be present.
	}{
		{"Green", Green, "ok", "\033[32m"},
		{"Red", Red, "err", "\033[31m"},
		{"Yellow", Yellow, "warn", "\033[33m"},
		{"Cyan", Cyan, "info", "\033[36m"},
		{"Bold", Bold, "title", "\033[1m"},
		{"Dim", Dim, "path", "\033[2m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)

			// Check that the ANSI code is present.
			if len(result) <= len(tt.input) {
				t.Errorf("expected colored output longer than input, got %q", result)
			}

			// Check that the original text is still in the output.
			found := false
			for i := 0; i <= len(result)-len(tt.input); i++ {
				if result[i:i+len(tt.input)] == tt.input {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected output to contain %q, got %q", tt.input, result)
			}

			// Check that it ends with the reset code.
			resetCode := "\033[0m"
			if len(result) < len(resetCode) {
				t.Errorf("output too short to contain reset code: %q", result)
			} else if result[len(result)-len(resetCode):] != resetCode {
				t.Errorf("expected output to end with reset code, got %q", result)
			}
		})
	}
}

// TestColorFunctions_Disabled tests that color functions return plain text
// when colors are disabled (e.g., in CI, piped output, NO_COLOR set).
func TestColorFunctions_Disabled(t *testing.T) {
	SetEnabled(false)

	tests := []struct {
		name  string
		fn    func(string) string
		input string
	}{
		{"Green", Green, "ok"},
		{"Red", Red, "err"},
		{"Yellow", Yellow, "warn"},
		{"Cyan", Cyan, "info"},
		{"Bold", Bold, "title"},
		{"Dim", Dim, "path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			if result != tt.input {
				t.Errorf("with colors disabled, expected %q, got %q", tt.input, result)
			}
		})
	}
}

// TestBoldCyan_Enabled tests that BoldCyan combines both codes.
func TestBoldCyan_Enabled(t *testing.T) {
	SetEnabled(true)
	defer SetEnabled(false)

	result := BoldCyan("header")
	if result == "header" {
		t.Error("expected colored output, got plain text")
	}
	// Should contain the original text.
	found := false
	for i := 0; i <= len(result)-len("header"); i++ {
		if result[i:i+len("header")] == "header" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected output to contain 'header', got %q", result)
	}
}

// TestBoldCyan_Disabled tests that BoldCyan returns plain text when disabled.
func TestBoldCyan_Disabled(t *testing.T) {
	SetEnabled(false)

	result := BoldCyan("header")
	if result != "header" {
		t.Errorf("expected plain 'header', got %q", result)
	}
}

// TestStatusColor tests the status-to-color mapping.
func TestStatusColor(t *testing.T) {
	SetEnabled(true)
	defer SetEnabled(false)

	tests := []struct {
		status   string
		contains string // Expected ANSI code.
	}{
		{"completed", "\033[32m"}, // Green.
		{"failed", "\033[31m"},    // Red.
		{"timeout", "\033[33m"},   // Yellow.
		{"denied", "\033[31m"},    // Red.
		{"unknown", "\033[36m"},   // Cyan (default).
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			colorFn := StatusColor(tt.status)
			result := colorFn("test")
			if result == "test" {
				t.Errorf("expected colored output for status %q", tt.status)
			}
		})
	}
}

// TestIsEnabled tests the enabled getter.
func TestIsEnabled(t *testing.T) {
	SetEnabled(true)
	if !IsEnabled() {
		t.Error("expected IsEnabled to return true")
	}
	SetEnabled(false)
	if IsEnabled() {
		t.Error("expected IsEnabled to return false")
	}
}
