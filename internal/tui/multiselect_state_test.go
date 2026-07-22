package tui

import (
	"testing"
)

func TestToggleMode(t *testing.T) {
	t.Run("enters multi-select mode with enter message", func(t *testing.T) {
		s := &MultiSelectState{}
		var status string
		s.ToggleMode(&status, "Multi-select active")
		if !s.multiSelectMode {
			t.Fatal("expected multiSelectMode to be true")
		}
		if status != "Multi-select active" {
			t.Fatalf("expected enter message, got %q", status)
		}
	})

	t.Run("exits multi-select mode and clears selections", func(t *testing.T) {
		s := &MultiSelectState{
			multiSelectMode: true,
			selectedTaskIdentities: map[TaskIdentity]bool{
				{ProjectID: "project", TaskID: "a"}: true,
				{ProjectID: "project", TaskID: "b"}: true,
			},
		}
		var status string
		s.ToggleMode(&status, "enter msg")
		if s.multiSelectMode {
			t.Fatal("expected multiSelectMode to be false")
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
	identity := TaskIdentity{ProjectID: "project", TaskID: "task-1"}

	t.Run("selecting a task updates status with count", func(t *testing.T) {
		s := &MultiSelectState{multiSelectMode: true}
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
		s := &MultiSelectState{
			multiSelectMode:        true,
			selectedTaskIdentities: map[TaskIdentity]bool{identity: true},
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
		s := &MultiSelectState{}
		var status string
		s.UpdateStatus(&status, noSel, withSel)
		if status != noSel {
			t.Fatalf("expected %q, got %q", noSel, status)
		}
	})

	t.Run("multiple selections", func(t *testing.T) {
		s := &MultiSelectState{
			selectedTaskIdentities: map[TaskIdentity]bool{
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

func TestFinishBulk(t *testing.T) {
	s := &MultiSelectState{
		multiSelectMode: true,
		selectedTaskIdentities: map[TaskIdentity]bool{
			{ProjectID: "project", TaskID: "a"}: true,
			{ProjectID: "project", TaskID: "b"}: true,
		},
	}
	var status string
	s.FinishBulk(&status, "Changed 2 tasks")

	if s.multiSelectMode {
		t.Fatal("expected multiSelectMode to be false after FinishBulk")
	}
	if s.SelectedCount() != 0 {
		t.Fatal("expected selections to be cleared after FinishBulk")
	}
	if status != "Changed 2 tasks" {
		t.Fatalf("expected result message, got %q", status)
	}
}

func TestMultiSelectStateHasTaskIDRejectsAmbiguousIdentity(t *testing.T) {
	sharedID := "shared-task"
	state := &MultiSelectState{
		multiSelectMode: true,
		selectedTaskIdentities: map[TaskIdentity]bool{
			{ProjectID: "alpha", TaskID: sharedID}: true,
			{ProjectID: "beta", TaskID: sharedID}:  true,
		},
	}
	selectedIDs := map[string]bool{sharedID: true}

	if multiSelectStateHasTaskID(state, selectedIDs) {
		t.Fatal("ID-only lookup should reject duplicate selected task identities")
	}
	if clearMultiSelectStateForTaskIDs(state, selectedIDs) {
		t.Fatal("ID-only cleanup should not clear an ambiguous duplicate selection")
	}
	if !state.multiSelectMode || state.SelectedCount() != 2 {
		t.Fatalf("ambiguous selection changed: active=%v count=%d", state.multiSelectMode, state.SelectedCount())
	}
}
