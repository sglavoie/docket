package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestToggleMode(t *testing.T) {
	t.Run("enters multi-select mode with enter message", func(t *testing.T) {
		s := &listsurface.MultiSelectState{}
		var status string
		s.ToggleMode(&status, "Multi-select active")
		if !s.MultiSelectMode {
			t.Fatal("expected MultiSelectMode to be true")
		}
		if status != "Multi-select active" {
			t.Fatalf("expected enter message, got %q", status)
		}
	})

	t.Run("exits multi-select mode and clears selections", func(t *testing.T) {
		s := &listsurface.MultiSelectState{
			MultiSelectMode: true,
			Selected: map[taskref.TaskIdentity]bool{
				{ProjectID: "project", TaskID: "a"}: true,
				{ProjectID: "project", TaskID: "b"}: true,
			},
		}
		var status string
		s.ToggleMode(&status, "enter msg")
		if s.MultiSelectMode {
			t.Fatal("expected MultiSelectMode to be false")
		}
		if s.SelectedCount() != 0 {
			t.Fatal("expected selections to be cleared")
		}
		if status != "" {
			t.Fatalf("expected empty status on exit, got %q", status)
		}
	})
}

func TestToggleTaskIdentityAndUpdateStatus(t *testing.T) {
	noSel := "no selection"
	withSel := "%d selected"
	identity := taskref.TaskIdentity{ProjectID: "project", TaskID: "task-1"}

	t.Run("selecting a task updates status with count", func(t *testing.T) {
		s := &listsurface.MultiSelectState{MultiSelectMode: true}
		var status string
		s.ToggleTaskIdentity(identity)
		s.UpdateStatus(&status, noSel, withSel)
		if !s.IsTaskIdentitySelected(identity) {
			t.Fatal("expected task-1 to be selected")
		}
		if status != "1 selected" {
			t.Fatalf("expected '1 selected', got %q", status)
		}
	})

	t.Run("deselecting last task shows no-selection message", func(t *testing.T) {
		s := &listsurface.MultiSelectState{
			MultiSelectMode: true,
			Selected:        map[taskref.TaskIdentity]bool{identity: true},
		}
		var status string
		s.ToggleTaskIdentity(identity)
		s.UpdateStatus(&status, noSel, withSel)
		if s.IsTaskIdentitySelected(identity) {
			t.Fatal("expected task-1 to be deselected")
		}
		if status != noSel {
			t.Fatalf("expected no-selection message, got %q", status)
		}
	})
}

func TestUpdateStatus(t *testing.T) {
	noSel := "none selected"
	withSel := "%d items"

	t.Run("zero selections", func(t *testing.T) {
		s := &listsurface.MultiSelectState{}
		var status string
		s.UpdateStatus(&status, noSel, withSel)
		if status != noSel {
			t.Fatalf("expected %q, got %q", noSel, status)
		}
	})

	t.Run("multiple selections", func(t *testing.T) {
		s := &listsurface.MultiSelectState{
			Selected: map[taskref.TaskIdentity]bool{
				{ProjectID: "project", TaskID: "a"}: true,
				{ProjectID: "project", TaskID: "b"}: true,
				{ProjectID: "project", TaskID: "c"}: true,
			},
		}
		var status string
		s.UpdateStatus(&status, noSel, withSel)
		if status != "3 items" {
			t.Fatalf("expected '3 items', got %q", status)
		}
	})
}

func TestFinishBulkPreservesSelection(t *testing.T) {
	s := &listsurface.MultiSelectState{
		MultiSelectMode: true,
		Selected: map[taskref.TaskIdentity]bool{
			{ProjectID: "project", TaskID: "a"}: true,
			{ProjectID: "project", TaskID: "b"}: true,
		},
	}
	var status string
	s.FinishBulk(&status, "Changed 2 tasks")

	if !s.MultiSelectMode {
		t.Fatal("expected MultiSelectMode to remain active after FinishBulk")
	}
	if s.SelectedCount() != 2 {
		t.Fatal("expected selections to be preserved after FinishBulk")
	}
	if status != "Changed 2 tasks" {
		t.Fatalf("expected result message, got %q", status)
	}
}

func TestMultiSelectStateHasTaskIDRejectsAmbiguousIdentity(t *testing.T) {
	sharedID := "shared-task"
	state := &listsurface.MultiSelectState{
		MultiSelectMode: true,
		Selected: map[taskref.TaskIdentity]bool{
			{ProjectID: "alpha", TaskID: sharedID}: true,
			{ProjectID: "beta", TaskID: sharedID}:  true,
		},
	}
	selectedIDs := map[string]bool{sharedID: true}

	if multiSelectStateHasTaskID(state, selectedIDs) {
		t.Fatal("ID-only lookup should reject duplicate selected task identities")
	}
	if !state.MultiSelectMode || state.SelectedCount() != 2 {
		t.Fatalf("ambiguous selection changed: active=%v count=%d", state.MultiSelectMode, state.SelectedCount())
	}
}
