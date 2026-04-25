package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordUndoSnapshot(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	writeUndoTasks(t, tasksPath, initialContent)

	if err := RecordUndoSnapshot(project, "Test Action", "test context", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	undoDir := getUndoDir(project)
	snapshotPath := filepath.Join(undoDir, "0001.md")
	if _, statErr := os.Stat(snapshotPath); os.IsNotExist(statErr) {
		t.Error("snapshot file was not created")
	}

	meta, err := loadUndoMeta(project)
	if err != nil {
		t.Fatalf("loadUndoMeta failed: %v", err)
	}

	if len(meta.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(meta.Entries))
	}

	if meta.Entries[0].Action != "Test Action" {
		t.Errorf("expected action 'Test Action', got %q", meta.Entries[0].Action)
	}

	if meta.Entries[0].Context != "test context" {
		t.Errorf("expected context 'test context', got %q", meta.Entries[0].Context)
	}
}

func TestRecordUndoSnapshotDisabled(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	writeUndoTasks(t, tasksPath, "# Test\n")

	if err := RecordUndoSnapshot(project, "Test", "", 0, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot with depth 0 failed: %v", err)
	}

	undoDir := getUndoDir(project)
	if _, err := os.Stat(undoDir); !os.IsNotExist(err) {
		t.Error("undo directory should not exist when disabled")
	}
}

func TestNewMutationClearsRedoStack(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	content1 := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	content2 := "# Project: Test Project\n\n## Task: Modified task\n<!-- id: 1, status: done, priority: 1 -->\n"
	content3 := "# Project: Test Project\n\n## Task: New task\n<!-- id: 2, status: todo, priority: 2 -->\n"

	writeUndoTasks(t, tasksPath, content1)
	if err := RecordUndoSnapshot(project, "Action1", "", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}
	writeUndoTasks(t, tasksPath, content2)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	redoEntries, _ := LoadRedoHistory(project)
	if len(redoEntries) != 1 {
		t.Fatalf("expected 1 redo entry before new mutation, got %d", len(redoEntries))
	}

	writeUndoTasks(t, tasksPath, content3)
	if err := RecordUndoSnapshot(project, "Action2", "", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	redoEntries, _ = LoadRedoHistory(project)
	if len(redoEntries) != 0 {
		t.Errorf("expected 0 redo entries after new mutation, got %d", len(redoEntries))
	}

	undoDir := getUndoDir(project)
	redoFile := filepath.Join(undoDir, "redo_0001.md")
	if _, statErr := os.Stat(redoFile); !os.IsNotExist(statErr) {
		t.Error("redo snapshot file should have been deleted after new mutation")
	}
}

func TestUndoHistoryPruning(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	maxDepth := 3
	for i := range 5 {
		content := "# Test content " + string(rune('A'+i)) + "\n"
		writeUndoTasks(t, tasksPath, content)
		if err := RecordUndoSnapshot(project, "Action", "", maxDepth, nil); err != nil {
			t.Fatalf("RecordUndoSnapshot failed: %v", err)
		}
	}

	entries, err := LoadUndoHistory(project)
	if err != nil {
		t.Fatalf("LoadUndoHistory failed: %v", err)
	}

	if len(entries) != maxDepth {
		t.Errorf("expected %d entries after pruning, got %d", maxDepth, len(entries))
	}
}
