package models

import "testing"

func TestNewGenericTaskUsesTargetIdentity(t *testing.T) {
	task, err := NewGenericTask("generic", 1, "Generic")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	if task.ID != "generic" || task.Title != "Generic" {
		t.Fatalf("task identity = %q/%q, want generic/Generic", task.ID, task.Title)
	}
	if task.State() != StateBacklog {
		t.Fatalf("State() = %q, want %q", task.State(), StateBacklog)
	}
}
