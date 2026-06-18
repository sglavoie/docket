package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

	if meta.Entries[0].GroupID != "" {
		t.Errorf("legacy snapshot should not set group ID, got %q", meta.Entries[0].GroupID)
	}

	metaBytes, err := os.ReadFile(getUndoMetaPath(project))
	if err != nil {
		t.Fatalf("failed to read undo meta JSON: %v", err)
	}
	metaJSON := string(metaBytes)
	if strings.Contains(metaJSON, "group_id") || strings.Contains(metaJSON, "group_project_ids") {
		t.Fatalf("legacy snapshot JSON should omit empty group metadata:\n%s", metaJSON)
	}
}

func TestRecordUndoSnapshotWithOptionsRecordsGroupMetadata(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	writeUndoTasks(t, tasksPath, "# Project: Test Project\n\n## Task: First task\n<!-- id: task-a, status: todo, priority: 3 -->\n")

	opts := UndoSnapshotOptions{
		Action:          "Move Task",
		Context:         "task-a",
		MaxDepth:        10,
		TaskIDs:         []string{"task-a"},
		GroupID:         "move-group-1",
		GroupProjectIDs: []string{"source-project", "target-project"},
	}
	if err := RecordUndoSnapshotWithOptions(project, opts); err != nil {
		t.Fatalf("RecordUndoSnapshotWithOptions failed: %v", err)
	}

	meta := readUndoMetaForTest(t, project)
	assertUndoMetaCounts(t, meta, 1, 0)

	entry := meta.Entries[0]
	if entry.Action != opts.Action {
		t.Errorf("entry action = %q, want %q", entry.Action, opts.Action)
	}
	if entry.Context != opts.Context {
		t.Errorf("entry context = %q, want %q", entry.Context, opts.Context)
	}
	if !reflect.DeepEqual(entry.TaskIDs, opts.TaskIDs) {
		t.Errorf("entry task IDs = %v, want %v", entry.TaskIDs, opts.TaskIDs)
	}
	if entry.GroupID != opts.GroupID {
		t.Errorf("entry group ID = %q, want %q", entry.GroupID, opts.GroupID)
	}
	if !reflect.DeepEqual(entry.GroupProjectIDs, opts.GroupProjectIDs) {
		t.Errorf("entry group project IDs = %v, want %v", entry.GroupProjectIDs, opts.GroupProjectIDs)
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

func TestRecordUndoSnapshotWithOptionsDisabled(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	writeUndoTasks(t, tasksPath, "# Test\n")

	if err := RecordUndoSnapshotWithOptions(project, UndoSnapshotOptions{
		Action:   "Test",
		MaxDepth: 0,
		GroupID:  "ignored-group",
	}); err != nil {
		t.Fatalf("RecordUndoSnapshotWithOptions with depth 0 failed: %v", err)
	}

	undoDir := getUndoDir(project)
	if _, err := os.Stat(undoDir); !os.IsNotExist(err) {
		t.Error("undo directory should not exist when options snapshot is disabled")
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

func TestRecordUndoSnapshotPreservesRedoFilesOnMetaSaveFailure(t *testing.T) {
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

	undoDir := getUndoDir(project)
	redoFile := filepath.Join(undoDir, "redo_0001.md")
	if _, statErr := os.Stat(redoFile); statErr != nil {
		t.Fatalf("expected redo file to exist before failure scenario: %v", statErr)
	}

	meta, err := loadUndoMeta(project)
	if err != nil {
		t.Fatalf("loadUndoMeta failed: %v", err)
	}
	if len(meta.RedoEntries) == 0 {
		t.Fatalf("expected redo entries in meta after PerformUndo, got 0")
	}

	if chmodErr := os.Chmod(undoDir, 0555); chmodErr != nil {
		t.Fatalf("failed to chmod undo dir read-only: %v", chmodErr)
	}
	defer func() { _ = os.Chmod(undoDir, 0755) }()

	writeUndoTasks(t, tasksPath, content3)
	err = RecordUndoSnapshot(project, "Action2", "", 10, nil)
	if err == nil {
		t.Fatal("expected RecordUndoSnapshot to fail when undo dir is read-only")
	}

	if _, statErr := os.Stat(redoFile); os.IsNotExist(statErr) {
		t.Error("redo snapshot file was deleted even though meta save failed; meta still references it on disk, which would corrupt redo")
	}
}

func TestRecordUndoSnapshotPrunesOrphanedSnapshotFiles(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	maxDepth := 2
	for i := 1; i <= 3; i++ {
		content := "# Project: Test Project\n\n## Task: State " + string(rune('A'+i-1)) + "\n<!-- id: 1, status: todo, priority: 3 -->\n"
		writeUndoTasks(t, tasksPath, content)
		if err := RecordUndoSnapshot(project, "Edit", "", maxDepth, nil); err != nil {
			t.Fatalf("RecordUndoSnapshot %d failed: %v", i, err)
		}
	}

	meta := readUndoMetaForTest(t, project)
	assertUndoMetaCounts(t, meta, maxDepth, 0)
	if meta.Entries[0].Num != 2 || meta.Entries[1].Num != 3 {
		t.Fatalf("entries = %+v, want snapshot nums 2 and 3 after pruning", meta.Entries)
	}
	if _, statErr := os.Stat(getSnapshotPath(project, 1)); !os.IsNotExist(statErr) {
		t.Fatalf("pruned snapshot 1 should be removed, stat err: %v", statErr)
	}
	for _, num := range []int{2, 3} {
		if _, statErr := os.Stat(getSnapshotPath(project, num)); statErr != nil {
			t.Fatalf("kept snapshot %d should exist: %v", num, statErr)
		}
	}
}

func TestRecordUndoSnapshotStartsFreshWhenMetadataCorrupt(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)
	writeUndoTasks(t, tasksPath, "# Project: Test Project\n\n## Task: Current\n<!-- id: 1, status: todo, priority: 3 -->\n")

	undoDir := getUndoDir(project)
	if err := os.MkdirAll(undoDir, DirPermissions); err != nil {
		t.Fatalf("failed to create undo dir: %v", err)
	}
	if err := os.WriteFile(getUndoMetaPath(project), []byte("{not-json"), FilePermissions); err != nil {
		t.Fatalf("failed to write corrupt undo meta: %v", err)
	}

	if err := RecordUndoSnapshot(project, "Recovered Edit", "after corrupt meta", 10, []string{"1"}); err != nil {
		t.Fatalf("RecordUndoSnapshot should recover from corrupt meta: %v", err)
	}

	meta := readUndoMetaForTest(t, project)
	assertUndoMetaCounts(t, meta, 1, 0)
	if meta.Entries[0].Num != 1 {
		t.Fatalf("fresh metadata should restart at snapshot 1, got %+v", meta.Entries[0])
	}
	if meta.Entries[0].Action != "Recovered Edit" || meta.Entries[0].Context != "after corrupt meta" {
		t.Fatalf("recovered entry metadata = %+v", meta.Entries[0])
	}
	if _, statErr := os.Stat(getSnapshotPath(project, 1)); statErr != nil {
		t.Fatalf("recovered snapshot should exist: %v", statErr)
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
