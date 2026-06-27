// Package fsdiff provides filesystem snapshot and diffing capabilities.
//
// This package allows the runner to capture the state of a workspace before
// and after an action executes. By comparing the two snapshots, we can
// determine exactly which files the agent created, modified, or deleted.
//
// This is critical for the Observability layer, ensuring agents receive
// immediate feedback on the effects of their shell commands, and providing
// an audit trail of all filesystem mutations.
package fsdiff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ChangeType describes the kind of filesystem mutation.
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeModify ChangeType = "modify"
	ChangeTypeDelete ChangeType = "delete"
)

// FileChange represents a single file modification.
type FileChange struct {
	// Path is the relative path of the changed file from the workspace root.
	Path string `json:"path"`

	// Type indicates whether the file was created, modified, or deleted.
	Type ChangeType `json:"type"`

	// OldHash is the SHA-256 hash of the file before the change (empty for creates).
	OldHash string `json:"old_hash,omitempty"`

	// NewHash is the SHA-256 hash of the file after the change (empty for deletes).
	NewHash string `json:"new_hash,omitempty"`

	// Size is the file size in bytes after the change (-1 for deletes).
	Size int64 `json:"size"`
}

// FSDiff captures the complete set of filesystem changes produced by an action.
type FSDiff struct {
	// ActionID links this diff to the action that produced it.
	ActionID string `json:"action_id"`

	// Timestamp records when the diff was computed.
	Timestamp time.Time `json:"timestamp"`

	// Changes contains the detailed list of file mutations.
	Changes []FileChange `json:"changes"`

	// FilesAdded is a convenient list of newly created file paths.
	FilesAdded []string `json:"files_added"`

	// FilesModified is a convenient list of modified file paths.
	FilesModified []string `json:"files_modified"`

	// FilesDeleted is a convenient list of removed file paths.
	FilesDeleted []string `json:"files_deleted"`
}

// NewFSDiff initializes an empty FSDiff.
func NewFSDiff(actionID string) *FSDiff {
	return &FSDiff{
		ActionID:      actionID,
		Timestamp:     time.Now(),
		Changes:       make([]FileChange, 0),
		FilesAdded:    make([]string, 0),
		FilesModified: make([]string, 0),
		FilesDeleted:  make([]string, 0),
	}
}

// AddChange appends a file change to the diff and updates the convenience lists.
func (d *FSDiff) AddChange(change FileChange) {
	d.Changes = append(d.Changes, change)
	switch change.Type {
	case ChangeTypeCreate:
		d.FilesAdded = append(d.FilesAdded, change.Path)
	case ChangeTypeModify:
		d.FilesModified = append(d.FilesModified, change.Path)
	case ChangeTypeDelete:
		d.FilesDeleted = append(d.FilesDeleted, change.Path)
	}
}

// Len returns the total number of changes.
func (d *FSDiff) Len() int {
	return len(d.Changes)
}

// HasChanges returns true if any filesystem changes were recorded.
func (d *FSDiff) HasChanges() bool {
	return len(d.Changes) > 0
}

// ─────────────────────────────────────────────────────────────────────────────
// Snapshot and Diff Engine
// ─────────────────────────────────────────────────────────────────────────────

// FileState represents the metadata and hash of a single file at a point in time.
type FileState struct {
	Size int64
	Hash string
}

// Snapshot represents the state of a directory tree.
// The keys are relative file paths, and the values are the file states.
type Snapshot map[string]FileState

// TakeSnapshot walks the target directory and records the state of all files.
// It skips ignored directories (like .git or .agentsandbox) to improve performance
// and prevent irrelevant noise in the diffs.
func TakeSnapshot(dir string, ignores []string) (Snapshot, error) {
	snap := make(Snapshot)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If we can't access a path, skip it rather than failing the whole snapshot.
			return nil
		}

		// Calculate relative path for stable comparisons.
		relPath, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}

		// Check ignore list. If a directory is ignored, skip its entire contents.
		for _, ignore := range ignores {
			if relPath == ignore || strings.HasPrefix(relPath, ignore+string(filepath.Separator)) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// We only track files, not directories (directory changes are implied by files).
		if info.IsDir() {
			return nil
		}

		// Compute SHA-256 hash.
		hash, hashErr := hashFile(path)
		if hashErr != nil {
			// If we fail to hash (e.g., permission denied), skip the file.
			return nil
		}

		snap[relPath] = FileState{
			Size: info.Size(),
			Hash: hash,
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to take snapshot of %s: %w", dir, err)
	}

	return snap, nil
}

// Compare computes the difference between two snapshots and returns an FSDiff.
func Compare(actionID string, before, after Snapshot) *FSDiff {
	diff := NewFSDiff(actionID)

	// Detect creations and modifications.
	for path, afterState := range after {
		beforeState, exists := before[path]
		if !exists {
			diff.AddChange(FileChange{
				Path:    path,
				Type:    ChangeTypeCreate,
				NewHash: afterState.Hash,
				Size:    afterState.Size,
			})
		} else if beforeState.Hash != afterState.Hash {
			diff.AddChange(FileChange{
				Path:    path,
				Type:    ChangeTypeModify,
				OldHash: beforeState.Hash,
				NewHash: afterState.Hash,
				Size:    afterState.Size,
			})
		}
	}

	// Detect deletions.
	for path, beforeState := range before {
		if _, exists := after[path]; !exists {
			diff.AddChange(FileChange{
				Path:    path,
				Type:    ChangeTypeDelete,
				OldHash: beforeState.Hash,
				Size:    -1,
			})
		}
	}

	return diff
}

// hashFile computes the SHA-256 hash of a file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
