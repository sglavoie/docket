package models

import "testing"

func newLifecycleTestTask(t *testing.T, id string, order int) *Task {
	t.Helper()

	task, err := NewTask(id, order, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	return task
}
