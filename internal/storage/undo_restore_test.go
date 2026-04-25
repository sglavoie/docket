package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPerformUndo(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	writeUndoTasks(t, tasksPath, initialContent)

	if err := RecordUndoSnapshot(project, "Add Task", "task1", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	newContent := "# Project: Test Project\n\n## Task: Modified task\n<!-- id: 1, status: done, priority: 1 -->\n"
	writeUndoTasks(t, tasksPath, newContent)

	actionDesc, err := PerformUndo(project)
	if err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	if actionDesc != "Add Task: task1" {
		t.Errorf("expected action desc 'Add Task: task1', got %q", actionDesc)
	}

	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}

	if string(content) != initialContent {
		t.Errorf("content was not restored correctly")
	}

	if UndoHistoryExists(project) {
		t.Error("undo history should be empty after undo")
	}
}

func TestPerformUndoCreatesRedoEntry(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	writeUndoTasks(t, tasksPath, initialContent)

	if err := RecordUndoSnapshot(project, "Add Task", "task1", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	modifiedContent := "# Project: Test Project\n\n## Task: Modified task\n<!-- id: 1, status: done, priority: 1 -->\n"
	writeUndoTasks(t, tasksPath, modifiedContent)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	redoEntries, err := LoadRedoHistory(project)
	if err != nil {
		t.Fatalf("LoadRedoHistory failed: %v", err)
	}
	if len(redoEntries) != 1 {
		t.Errorf("expected 1 redo entry, got %d", len(redoEntries))
	}
	if redoEntries[0].Action != "Add Task" {
		t.Errorf("expected redo action 'Add Task', got %q", redoEntries[0].Action)
	}

	undoDir := getUndoDir(project)
	redoFile := filepath.Join(undoDir, "redo_0001.md")
	if _, statErr := os.Stat(redoFile); os.IsNotExist(statErr) {
		t.Error("redo snapshot file was not created")
	}

	content, err := os.ReadFile(redoFile)
	if err != nil {
		t.Fatalf("failed to read redo snapshot: %v", err)
	}
	if string(content) != modifiedContent {
		t.Error("redo snapshot content does not match pre-undo state")
	}
}

func TestPerformRedo(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	modifiedContent := "# Project: Test Project\n\n## Task: Modified task\n<!-- id: 1, status: done, priority: 1 -->\n"

	writeUndoTasks(t, tasksPath, initialContent)
	if err := RecordUndoSnapshot(project, "Add Task", "task1", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}
	writeUndoTasks(t, tasksPath, modifiedContent)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	content, _ := os.ReadFile(tasksPath)
	if string(content) != initialContent {
		t.Error("expected initialContent after undo")
	}

	actionDesc, err := PerformRedo(project)
	if err != nil {
		t.Fatalf("PerformRedo failed: %v", err)
	}
	if actionDesc != "Add Task: task1" {
		t.Errorf("expected action desc 'Add Task: task1', got %q", actionDesc)
	}

	content, _ = os.ReadFile(tasksPath)
	if string(content) != modifiedContent {
		t.Error("expected modifiedContent after redo")
	}

	redoEntries, _ := LoadRedoHistory(project)
	if len(redoEntries) != 0 {
		t.Errorf("expected 0 redo entries after redo, got %d", len(redoEntries))
	}

	if !UndoHistoryExists(project) {
		t.Error("expected undo history after redo")
	}
}

func TestRedoRoundTrip(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	stateA := "# Project: Test Project\n\n## Task: State A\n<!-- id: 1, status: todo, priority: 3 -->\n"
	stateB := "# Project: Test Project\n\n## Task: State B\n<!-- id: 1, status: done, priority: 1 -->\n"

	writeUndoTasks(t, tasksPath, stateA)
	if err := RecordUndoSnapshot(project, "Edit", "", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}
	writeUndoTasks(t, tasksPath, stateB)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}
	content, _ := os.ReadFile(tasksPath)
	if string(content) != stateA {
		t.Error("expected stateA after undo")
	}

	if _, err := PerformRedo(project); err != nil {
		t.Fatalf("PerformRedo failed: %v", err)
	}
	content, _ = os.ReadFile(tasksPath)
	if string(content) != stateB {
		t.Error("expected stateB after redo")
	}
}
