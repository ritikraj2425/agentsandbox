// Package fsdiff captures filesystem changes before and after an agent
// action, enabling rollback and auditing of file modifications.
package fsdiff

import "time"

// ChangeType describes the kind of filesystem change.
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeModify ChangeType = "modify"
	ChangeTypeDelete ChangeType = "delete"
)

// FileChange represents a single filesystem modification.
type FileChange struct {
	// Path is the absolute path of the changed file.
	Path string `json:"path"`

	// Type is the kind of change (create, modify, delete).
	Type ChangeType `json:"type"`

	// OldHash is the hash of the file before the change (empty for creates).
	OldHash string `json:"old_hash,omitempty"`

	// NewHash is the hash of the file after the change (empty for deletes).
	NewHash string `json:"new_hash,omitempty"`

	// Size is the file size in bytes after the change (-1 for deletes).
	Size int64 `json:"size"`
}

// FSDiff captures the set of filesystem changes produced by an action.
type FSDiff struct {
	// ActionID links this diff to the action that produced it.
	ActionID string `json:"action_id"`

	// Timestamp is when the diff was captured.
	Timestamp time.Time `json:"timestamp"`

	// Changes is the list of individual file changes.
	Changes []FileChange `json:"changes"`
}

// NewFSDiff creates a new empty FSDiff for the given action.
func NewFSDiff(actionID string) *FSDiff {
	return &FSDiff{
		ActionID:  actionID,
		Timestamp: time.Now(),
		Changes:   make([]FileChange, 0),
	}
}

// AddChange appends a file change to the diff.
func (d *FSDiff) AddChange(change FileChange) {
	d.Changes = append(d.Changes, change)
}

// Len returns the number of changes in the diff.
func (d *FSDiff) Len() int {
	return len(d.Changes)
}

// HasChanges returns true if any filesystem changes were recorded.
func (d *FSDiff) HasChanges() bool {
	return len(d.Changes) > 0
}
