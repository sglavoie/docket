package tui

import "testing"

// Set resets targeted TaskIDs/ProjectIDs so stale data from a prior bulk
// operation can't leak into the next single-target undo.
func TestUndoContextSetClearsStaleTargets(t *testing.T) {
	u := UndoContext{
		Action:     "Bulk Status Change",
		Context:    "5 tasks",
		ProjectIDs: []string{"proj-a", "proj-b"},
		TaskIDs:    []string{"t1", "t2", "t3", "t4", "t5"},
		GroupID:    "group-1",
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
	if u.ProjectIDs != nil {
		t.Errorf("ProjectIDs should be cleared, got %v", u.ProjectIDs)
	}
	if u.GroupID != "" {
		t.Errorf("GroupID should be cleared, got %q", u.GroupID)
	}
}

func TestUndoContextClearResetsAllFields(t *testing.T) {
	u := UndoContext{
		Action:     "Delete Task",
		Context:    "#1 foo",
		ProjectIDs: []string{"p1"},
		TaskIDs:    []string{"t1"},
		GroupID:    "group-1",
	}
	u.Clear()
	if u.Action != "" || u.Context != "" {
		t.Errorf("expected empty strings, got %q / %q", u.Action, u.Context)
	}
	if u.ProjectIDs != nil || u.TaskIDs != nil || u.GroupID != "" {
		t.Errorf("expected nil slices and empty group, got %v / %v / %q", u.ProjectIDs, u.TaskIDs, u.GroupID)
	}
}
