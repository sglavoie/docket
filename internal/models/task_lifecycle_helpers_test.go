package models

import "testing"

func newLifecycleTestTask(t *testing.T, id string, order int) *Task {
	t.Helper()

	task, err := NewGenericTask(id, order, "Test")
	if err != nil {
		t.Fatal(err)
	}

	return task
}
