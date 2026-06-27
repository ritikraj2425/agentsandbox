// Package actions defines the core Action type representing a single
// operation an AI agent requests to perform.
package actions

import "time"

// ActionType categorizes agent actions.
type ActionType string

const (
	// ActionTypeShell represents a shell/command execution.
	ActionTypeShell ActionType = "shell"
	// ActionTypeFileWrite represents a file write operation.
	ActionTypeFileWrite ActionType = "file_write"
	// ActionTypeFileRead represents a file read operation.
	ActionTypeFileRead ActionType = "file_read"
	// ActionTypeNetworkRequest represents an outbound network request.
	ActionTypeNetworkRequest ActionType = "network"
	// ActionTypeCustom represents a user-defined custom action.
	ActionTypeCustom ActionType = "custom"
)

// ActionStatus tracks the lifecycle of an action.
type ActionStatus string

const (
	ActionStatusPending  ActionStatus = "pending"
	ActionStatusApproved ActionStatus = "approved"
	ActionStatusDenied   ActionStatus = "denied"
	ActionStatusRunning  ActionStatus = "running"
	ActionStatusComplete ActionStatus = "complete"
	ActionStatusFailed   ActionStatus = "failed"
)

// Action represents a single operation an AI agent wants to perform.
// It captures what the agent wants to do, any parameters, and the
// lifecycle state of the request.
type Action struct {
	// ID is a unique identifier for this action.
	ID string `json:"id" yaml:"id"`

	// Type categorizes the action (shell, file_write, etc.).
	Type ActionType `json:"type" yaml:"type"`

	// Name is a human-readable name for the action.
	Name string `json:"name" yaml:"name"`

	// Description explains what the action does.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Parameters holds action-specific configuration.
	Parameters map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`

	// Status tracks the current lifecycle state.
	Status ActionStatus `json:"status" yaml:"status"`

	// CreatedAt is when the action was requested.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`

	// CompletedAt is when the action finished (success or failure).
	CompletedAt *time.Time `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`

	// Error holds any error message if the action failed.
	Error string `json:"error,omitempty" yaml:"error,omitempty"`

	// AgentID identifies which agent requested this action.
	AgentID string `json:"agent_id,omitempty" yaml:"agent_id,omitempty"`
}

// NewAction creates a new Action with the given type, name, and parameters.
// It initializes the action in a pending state.
func NewAction(actionType ActionType, name string, params map[string]interface{}) *Action {
	return &Action{
		Type:       actionType,
		Name:       name,
		Parameters: params,
		Status:     ActionStatusPending,
		CreatedAt:  time.Now(),
	}
}

// IsPending returns true if the action is waiting for approval.
func (a *Action) IsPending() bool {
	return a.Status == ActionStatusPending
}

// IsTerminal returns true if the action has reached a final state.
func (a *Action) IsTerminal() bool {
	return a.Status == ActionStatusComplete || a.Status == ActionStatusFailed || a.Status == ActionStatusDenied
}

// Complete marks the action as successfully completed.
func (a *Action) Complete() {
	now := time.Now()
	a.Status = ActionStatusComplete
	a.CompletedAt = &now
}

// Fail marks the action as failed with the given error.
func (a *Action) Fail(err string) {
	now := time.Now()
	a.Status = ActionStatusFailed
	a.CompletedAt = &now
	a.Error = err
}
