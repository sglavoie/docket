package tui

import "testing"

// Set resets targeted TaskIDs so stale data from a prior bulk
// operation can't leak into the next single-target undo.
func TestUndoContextSetClearsStaleTargets(t *testing.T) {
	u := UndoContext{
		Action:  "Bulk Status Change",
		Context: "5 tasks",
		TaskIDs: []string{"t1", "t2", "t3", "t4", "t5"},
	}

	u.Set("Set Priority", "P1: lone task")

	if u.Action != "Set Priority" {
		t.Errorf("Action: want %q, got %q", "Set Priority", u.Action)
	}
	if u.Context != "P1: lone task" {
		t.Errorf("Context: want %q, got %q", "P1: lone task", u.Context)
	}
	if u.TaskIDs != nil {
		t.Errorf("TaskIDs should be cleared, got %v", u.TaskIDs)
	}
}

func TestUndoContextClearResetsAllFields(t *testing.T) {
	u := UndoContext{
		Action:  "Delete Task",
		Context: "#1 foo",
		TaskIDs: []string{"t1"},
	}
	u.Clear()
	if u.Action != "" || u.Context != "" {
		t.Errorf("expected empty strings, got %q / %q", u.Action, u.Context)
	}
	if u.TaskIDs != nil {
		t.Errorf("expected nil task ids, got %v", u.TaskIDs)
	}
}
