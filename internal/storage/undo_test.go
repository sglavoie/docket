package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestRecordUndoSnapshot(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Create project
	project := &models.Project{
		ID:   "test-id",
		Name: "Test Project",
		Slug: "test-project",
		Path: projectPath,
	}

	// Create initial tasks file
	initialContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	tasksPath := filepath.Join(projectPath, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}

	// Record a snapshot
	err := RecordUndoSnapshot(project, "Test Action", "test context", 10)
	if err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	// Verify snapshot was created
	undoDir := GetUndoDir(project)
	snapshotPath := filepath.Join(undoDir, "0001.md")
	if _, statErr := os.Stat(snapshotPath); os.IsNotExist(statErr) {
		t.Error("snapshot file was not created")
	}

	// Verify metadata was created
	meta, err := LoadUndoMeta(project)
	if err != nil {
		t.Fatalf("LoadUndoMeta failed: %v", err)
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
	// Create temp directory
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	project := &models.Project{
		ID:   "test-id",
		Name: "Test Project",
		Slug: "test-project",
		Path: projectPath,
	}

	// Create tasks file
	tasksPath := filepath.Join(projectPath, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}

	// Record with maxDepth 0 (disabled)
	err := RecordUndoSnapshot(project, "Test", "", 0)
	if err != nil {
		t.Fatalf("RecordUndoSnapshot with depth 0 failed: %v", err)
	}

	// Verify no undo directory was created
	undoDir := GetUndoDir(project)
	if _, err := os.Stat(undoDir); !os.IsNotExist(err) {
		t.Error("undo directory should not exist when disabled")
	}
}

func TestPerformUndo(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	project := &models.Project{
		ID:   "test-id",
		Name: "Test Project",
		Slug: "test-project",
		Path: projectPath,
	}

	// Create initial tasks file
	initialContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	tasksPath := filepath.Join(projectPath, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}

	// Record a snapshot
	if err := RecordUndoSnapshot(project, "Add Task", "task1", 10); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	// Modify the tasks file
	newContent := "# Project: Test Project\n\n## Task: Modified task\n<!-- id: 1, status: done, priority: 1 -->\n"
	if err := os.WriteFile(tasksPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write modified tasks file: %v", err)
	}

	// Perform undo
	actionDesc, err := PerformUndo(project)
	if err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	if actionDesc != "Add Task: task1" {
		t.Errorf("expected action desc 'Add Task: task1', got %q", actionDesc)
	}

	// Verify content was restored
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}

	if string(content) != initialContent {
		t.Errorf("content was not restored correctly")
	}

	// Verify undo history is empty
	if UndoHistoryExists(project) {
		t.Error("undo history should be empty after undo")
	}
}

func TestUndoHistoryPruning(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	project := &models.Project{
		ID:   "test-id",
		Name: "Test Project",
		Slug: "test-project",
		Path: projectPath,
	}

	tasksPath := filepath.Join(projectPath, "tasks.md")

	// Record more snapshots than maxDepth
	maxDepth := 3
	for i := 0; i < 5; i++ {
		content := "# Test content " + string(rune('A'+i)) + "\n"
		if err := os.WriteFile(tasksPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write tasks file: %v", err)
		}
		if err := RecordUndoSnapshot(project, "Action", "", maxDepth); err != nil {
			t.Fatalf("RecordUndoSnapshot failed: %v", err)
		}
	}

	// Verify only maxDepth entries remain
	entries, err := LoadUndoHistory(project)
	if err != nil {
		t.Fatalf("LoadUndoHistory failed: %v", err)
	}

	if len(entries) != maxDepth {
		t.Errorf("expected %d entries after pruning, got %d", maxDepth, len(entries))
	}
}
