// Package protocol defines the standard Action and Observation types
// that form the communication protocol between AI agents and AgentSandbox.
//
// This is a PUBLIC package (under pkg/). External developers and AI agent
// frameworks import these types to send Actions to the sandbox and receive
// Observations back.
//
// Why this exists:
// Every AI model speaks differently. OpenAI returns tool_calls, Anthropic
// returns tool_use blocks, local models return raw JSON. The Action struct
// is the single normalized format that all of them get converted into
// before the sandbox processes them. This means the rest of the codebase
// (policy engine, runners, trace system) never needs to know which AI
// model is being used.
package protocol

import (
	"crypto/rand"
	"fmt"
	"time"
)

// ActionType categorizes what kind of operation the agent wants to perform.
// Each type maps to a different execution path inside the sandbox.
type ActionType string

const (
	// ActionTypeShellRun executes a shell command in the sandbox environment.
	// This is the most common action type for coding agents.
	// Parameters: "command" (string) — the shell command to execute.
	ActionTypeShellRun ActionType = "shell.run"

	// ActionTypeFileRead reads the contents of a file.
	// Parameters: "path" (string) — relative path from workspace root.
	ActionTypeFileRead ActionType = "file.read"

	// ActionTypeFileWrite writes content to a file (create or overwrite).
	// Parameters: "path" (string), "content" (string).
	ActionTypeFileWrite ActionType = "file.write"

	// ActionTypeFilePatch applies a partial edit to a file.
	// Parameters: "path" (string), "patch" (string) — unified diff format.
	ActionTypeFilePatch ActionType = "file.patch"

	// ActionTypeBrowserGoto navigates a browser to a URL.
	// Parameters: "url" (string).
	ActionTypeBrowserGoto ActionType = "browser.goto"

	// ActionTypeBrowserClick clicks at coordinates or a CSS selector.
	// Parameters: "x" (int), "y" (int) or "selector" (string).
	ActionTypeBrowserClick ActionType = "browser.click"

	// ActionTypeBrowserType types text into the focused element.
	// Parameters: "text" (string).
	ActionTypeBrowserType ActionType = "browser.type"

	// ActionTypeBrowserScreenshot captures a screenshot of the current page.
	// No parameters required.
	ActionTypeBrowserScreenshot ActionType = "browser.screenshot"

	// ActionTypeTaskDone signals the agent considers the task complete.
	// Parameters: "summary" (string) — what the agent accomplished.
	ActionTypeTaskDone ActionType = "task.done"
)

// Action represents a single operation an AI agent wants to perform.
//
// This is the INPUT to the sandbox. The agent proposes an Action,
// the sandbox normalizes it, checks policies, executes it, and returns
// an Observation.
//
// The struct is intentionally simple: a type string and a parameters map.
// This keeps the protocol extensible — new action types can be added
// without changing the struct definition.
type Action struct {
	// ID uniquely identifies this action within a session.
	// Generated automatically by NewAction if left empty.
	ID string `json:"id"`

	// Type categorizes the action (e.g., "shell.run", "file.write").
	// The runner and policy engine both use this to decide how to handle it.
	Type ActionType `json:"type"`

	// Parameters holds action-specific data as key-value pairs.
	// For "shell.run": {"command": "go test ./..."}
	// For "file.write": {"path": "main.go", "content": "package main\n..."}
	// Using map[string]interface{} keeps the protocol flexible for any action type.
	Parameters map[string]interface{} `json:"parameters,omitempty"`

	// CreatedAt records when the agent proposed this action.
	CreatedAt time.Time `json:"created_at"`
}

// NewAction creates a new Action with a unique ID and the current timestamp.
//
// actionType: what kind of operation (e.g., ActionTypeShellRun).
// params: action-specific data (e.g., {"command": "echo hello"}).
//
// The ID is generated using crypto/rand for uniqueness across sessions.
// We use "act_" prefix so IDs are human-readable in logs and traces.
func NewAction(actionType ActionType, params map[string]interface{}) Action {
	return Action{
		ID:         generateID("act"),
		Type:       actionType,
		Parameters: params,
		CreatedAt:  time.Now(),
	}
}

// Command is a convenience method that extracts the "command" parameter.
// Returns empty string if the parameter doesn't exist or isn't a string.
// This avoids repetitive type assertions throughout the codebase.
func (a Action) Command() string {
	if cmd, ok := a.Parameters["command"].(string); ok {
		return cmd
	}
	return ""
}

// generateID creates a short random ID with the given prefix.
// Format: "<prefix>_<8 random hex chars>" (e.g., "act_a1b2c3d4").
//
// Why crypto/rand instead of math/rand?
// math/rand is pseudo-random and predictable if the seed is known.
// For action IDs that appear in traces and logs, we want true randomness
// to avoid collisions across concurrent sessions.
func generateID(prefix string) string {
	b := make([]byte, 4)
	// crypto/rand.Read never returns an error on supported platforms.
	// If it did (broken OS entropy), we'd have bigger problems.
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%x", prefix, b)
}
