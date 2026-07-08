package tui

import (
	"fmt"
	"sync"
	"testing"
)

func TestSessionMutationRegistryConcurrentSafeAndReset(t *testing.T) {
	resetSessionMutations()
	t.Cleanup(resetSessionMutations)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			taskID := fmt.Sprintf("task-%d", i)
			markSessionTaskMutated("project-1", []string{taskID})
			markParkPrompted(taskID)
			_ = wasParkPrompted(taskID)
		}(i)
	}
	wg.Wait()

	if _, ok := lastSessionMutation(); !ok {
		t.Fatal("lastSessionMutation should report the last recorded mutation")
	}

	resetSessionMutations()
	if _, ok := lastSessionMutation(); ok {
		t.Fatal("lastSessionMutation should be empty after reset")
	}
	if wasParkPrompted("task-1") {
		t.Fatal("prompted tasks should be cleared after reset")
	}
}

func TestRecordUndoSnapshotForSaveMarksMutationWithoutUndoDepth(t *testing.T) {
	resetSessionMutations()
	t.Cleanup(resetSessionMutations)

	task := newBulkPickerTestTask(t, 1, "record mutation")
	project := newBulkPickerTestProject(t, "Record Mutation", task)

	recordUndoSnapshotForSave(project, ProjectSaveOptions{
		UndoAction:  "Edit Task",
		UndoTaskIDs: []string{task.ID},
	})

	mutation, ok := lastSessionMutation()
	if !ok {
		t.Fatal("recordUndoSnapshotForSave should mark task mutations even when undo depth is disabled")
	}
	if mutation.ProjectID != project.ID || mutation.TaskID != task.ID {
		t.Fatalf("mutation = (%q, %q), want (%q, %q)", mutation.ProjectID, mutation.TaskID, project.ID, task.ID)
	}
}
