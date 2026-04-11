package models

import "testing"

func newDependencyTestProject(t *testing.T, id string) *Project {
	t.Helper()

	project, err := NewProject(id, "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	return project
}

func newDependencyTestTask(t *testing.T, id string, shortID int, title string) *Task {
	t.Helper()

	task, err := NewTask(id, shortID, TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}

	return task
}
