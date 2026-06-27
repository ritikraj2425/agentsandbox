// Package color provides minimal, zero-dependency ANSI color support
// for terminal output.
//
// Why build our own instead of using github.com/fatih/color?
// We committed to keeping AgentSandbox dependency-free (stdlib only).
// importing an external package would add unnecessary complexity
// to our go.mod for very little benefit.
//
// How ANSI colors work:
// Terminals interpret special "escape sequences" embedded in text.
// The sequence starts with ESC[ (written as \033[ or \x1b[) followed
// by a color code and the letter "m". For example:
//
//	\033[32m  = switch to green text
//	\033[0m   = reset to default (MUST be added after colored text)
//
// If you forget the reset code, ALL subsequent text stays colored.
//
// The NO_COLOR standard (https://no-color.org/):
// Many CLI tools respect the NO_COLOR environment variable.
// When set (to any value), tools should not emit color codes.
// This is important for CI/CD systems, log files, and accessibility.
package color

import (
	"fmt"
	"os"
)

// ANSI escape code constants.
// These are the "instructions" we send to the terminal to change text appearance.
//
// The format is: \033[<code>m
//   - \033 is the ESC character (ASCII 27)
//   - [ opens the "control sequence"
//   - <code> is the color/style number
//   - m terminates the sequence
const (
	reset  = "\033[0m" // Reset all styles back to terminal default.
	bold   = "\033[1m" // Bold/bright text.
	dim    = "\033[2m" // Dimmed/faint text (for secondary information).
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
)

// enabled controls whether color codes are emitted.
// It is set once at package initialization and never changes.
//
// Why a package-level variable instead of a function?
// Color functions are called hundreds of times during a single run.
// Checking the environment on every call would be wasteful.
// Checking once at startup and caching the result is the standard approach.
var enabled bool

// init runs automatically when the package is first imported.
// It determines whether the terminal supports colors.
//
// Colors are DISABLED when:
//  1. The NO_COLOR environment variable is set (any value, even "").
//     This follows the no-color.org standard respected by 400+ CLI tools.
//  2. stdout is not a terminal (e.g., output is piped to a file or
//     another program like "agentsandbox run 'echo hi' | grep hello").
//     We detect this by checking if stdout is a character device (terminal).
func init() {
	// Check NO_COLOR first — it takes priority over everything.
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		enabled = false
		return
	}

	// Check if stdout is a real terminal (not a pipe or file redirect).
	// os.Stdout.Stat() returns file info. Mode() & os.ModeCharDevice
	// is non-zero only for actual terminal devices.
	info, err := os.Stdout.Stat()
	if err != nil {
		enabled = false
		return
	}
	enabled = (info.Mode() & os.ModeCharDevice) != 0
}

// SetEnabled overrides the auto-detected color setting.
// This is primarily used in tests to force colors on or off.
func SetEnabled(on bool) {
	enabled = on
}

// IsEnabled returns whether color output is currently active.
func IsEnabled() bool {
	return enabled
}

// wrap surrounds text with the given ANSI code and a reset sequence.
// If colors are disabled, it returns the text unchanged.
//
// The pattern is always: <start code> + text + <reset code>
// For example: "\033[32m" + "PASSED" + "\033[0m" = green "PASSED"
func wrap(code, text string) string {
	if !enabled {
		return text
	}
	return fmt.Sprintf("%s%s%s", code, text, reset)
}

// --- Public color functions ---
// Each function wraps text in a specific ANSI color.
// When colors are disabled (NO_COLOR set, or piped output),
// they return the text completely unchanged.

// Green colors text green. Use for success messages.
// Example: "Status: completed" → "Status: \033[32mcompleted\033[0m"
func Green(text string) string {
	return wrap(green, text)
}

// Red colors text red. Use for errors and failures.
// Example: "Status: failed" → "Status: \033[31mfailed\033[0m"
func Red(text string) string {
	return wrap(red, text)
}

// Yellow colors text yellow. Use for warnings (timeouts, approvals).
func Yellow(text string) string {
	return wrap(yellow, text)
}

// Cyan colors text cyan. Use for informational labels and headers.
func Cyan(text string) string {
	return wrap(cyan, text)
}

// Bold makes text bold/bright. Use for section headers and emphasis.
func Bold(text string) string {
	return wrap(bold, text)
}

// Dim makes text dimmed/faint. Use for secondary info like file paths.
func Dim(text string) string {
	return wrap(dim, text)
}

// BoldCyan combines bold and cyan for section headers like "━━━ AgentSandbox Run ━━━".
func BoldCyan(text string) string {
	if !enabled {
		return text
	}
	return fmt.Sprintf("%s%s%s%s", bold, cyan, text, reset)
}

// StatusColor returns the appropriate color function for an observation status.
// This centralizes the status→color mapping so every part of the codebase
// uses consistent colors for the same status.
func StatusColor(status string) func(string) string {
	switch status {
	case "completed":
		return Green
	case "failed":
		return Red
	case "timeout":
		return Yellow
	case "denied":
		return Red
	case "approval_required":
		return Yellow
	default:
		return Cyan
	}
}
