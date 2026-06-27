// Package fsdiff provides tests for FSDiff.
package fsdiff

import (
	"testing"
)

func TestNewFSDiff(t *testing.T) {
	diff := NewFSDiff("action-1")

	if diff.ActionID != "action-1" {
		t.Errorf("expected action ID 'action-1', got %s", diff.ActionID)
	}
	if diff.HasChanges() {
		t.Error("new diff should have no changes")
	}
	if diff.Len() != 0 {
		t.Errorf("expected length 0, got %d", diff.Len())
	}
}

func TestFSDiff_AddChange(t *testing.T) {
	diff := NewFSDiff("action-2")

	diff.AddChange(FileChange{
		Path:    "/tmp/test.txt",
		Type:    ChangeTypeCreate,
		NewHash: "abc123",
		Size:    42,
	})

	if diff.Len() != 1 {
		t.Fatalf("expected 1 change, got %d", diff.Len())
	}
	if !diff.HasChanges() {
		t.Error("expected HasChanges to be true")
	}

	change := diff.Changes[0]
	if change.Path != "/tmp/test.txt" {
		t.Errorf("unexpected path: %s", change.Path)
	}
	if change.Type != ChangeTypeCreate {
		t.Errorf("expected create, got %s", change.Type)
	}
}

func TestFSDiff_MultipleChanges(t *testing.T) {
	diff := NewFSDiff("action-3")

	diff.AddChange(FileChange{Path: "/a", Type: ChangeTypeCreate, Size: 10})
	diff.AddChange(FileChange{Path: "/b", Type: ChangeTypeModify, Size: 20})
	diff.AddChange(FileChange{Path: "/c", Type: ChangeTypeDelete, Size: -1})

	if diff.Len() != 3 {
		t.Errorf("expected 3 changes, got %d", diff.Len())
	}
}
