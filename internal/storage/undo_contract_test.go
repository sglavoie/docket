package storage

import (
	"os"
	"strings"
	"testing"
)

func TestUndoRedoContractMovesEntriesBetweenLinearStacks(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	original := "# Project: Test Project\n\n## Task: Original task\n<!-- id: task-1, status: todo, priority: 3 -->\n"
	edited := "# Project: Test Project\n\n## Task: Edited task\n<!-- id: task-1, status: done, priority: 1 -->\n"

	writeUndoTasks(t, tasksPath, original)
	if err := RecordUndoSnapshot(project, "Edit Task", "#1 Original task", 10, []string{"task-1"}); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}
	writeUndoTasks(t, tasksPath, edited)

	if action, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	} else if action != "Edit Task: #1 Original task" {
		t.Fatalf("PerformUndo action = %q, want %q", action, "Edit Task: #1 Original task")
	}

	meta := readUndoMetaForTest(t, project)
	assertUndoMetaCounts(t, meta, 0, 1)
	if got := meta.RedoEntries[0].TaskIDs; len(got) != 1 || got[0] != "task-1" {
		t.Fatalf("redo TaskIDs = %v, want [task-1]", got)
	}
	assertUndoTasksContain(t, tasksPath, "Original task")

	if action, err := PerformRedo(project); err != nil {
		t.Fatalf("PerformRedo failed: %v", err)
	} else if action != "Edit Task: #1 Original task" {
		t.Fatalf("PerformRedo action = %q, want %q", action, "Edit Task: #1 Original task")
	}

	meta = readUndoMetaForTest(t, project)
	assertUndoMetaCounts(t, meta, 1, 0)
	assertUndoTasksContain(t, tasksPath, "Edited task")
}

func TestUndoMultiProjectFixtureCreatesIndependentHistories(t *testing.T) {
	fixtures := newUndoTestProjects(t, "alpha", "beta")

	for _, fixture := range fixtures {
		writeUndoTasks(t, fixture.TasksPath, "# Project: "+fixture.Project.Name+"\n")
		if err := RecordUndoSnapshot(fixture.Project, "Seed", fixture.Project.Slug, 10, nil); err != nil {
			t.Fatalf("RecordUndoSnapshot(%s) failed: %v", fixture.Project.Slug, err)
		}
	}

	for _, fixture := range fixtures {
		meta := readUndoMetaForTest(t, fixture.Project)
		assertUndoMetaCounts(t, meta, 1, 0)
		if meta.Entries[0].Context != fixture.Project.Slug {
			t.Fatalf("entry context for %s = %q, want slug", fixture.Project.Slug, meta.Entries[0].Context)
		}
	}
}

func assertUndoTasksContain(t *testing.T, tasksPath, want string) {
	t.Helper()

	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("tasks file does not contain %q:\n%s", want, string(content))
	}
}
