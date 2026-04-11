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
	undoDir := getUndoDir(project)
	snapshotPath := filepath.Join(undoDir, "0001.md")
	if _, statErr := os.Stat(snapshotPath); os.IsNotExist(statErr) {
		t.Error("snapshot file was not created")
	}

	// Verify metadata was created
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
	undoDir := getUndoDir(project)
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

func TestPerformUndoCreatesRedoEntry(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	project := &models.Project{ID: "test-id", Name: "Test Project", Slug: "test-project", Path: projectPath}

	initialContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	tasksPath := filepath.Join(projectPath, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}

	if err := RecordUndoSnapshot(project, "Add Task", "task1", 10); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	modifiedContent := "# Project: Test Project\n\n## Task: Modified task\n<!-- id: 1, status: done, priority: 1 -->\n"
	if err := os.WriteFile(tasksPath, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to write modified tasks file: %v", err)
	}

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	// After undo, there should be a redo entry
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

	// Redo snapshot file must exist
	undoDir := getUndoDir(project)
	redoFile := filepath.Join(undoDir, "redo_0001.md")
	if _, statErr := os.Stat(redoFile); os.IsNotExist(statErr) {
		t.Error("redo snapshot file was not created")
	}

	// Redo snapshot content should be the modified (pre-undo) state
	content, err := os.ReadFile(redoFile)
	if err != nil {
		t.Fatalf("failed to read redo snapshot: %v", err)
	}
	if string(content) != modifiedContent {
		t.Error("redo snapshot content does not match pre-undo state")
	}
}

func TestPerformRedo(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	project := &models.Project{ID: "test-id", Name: "Test Project", Slug: "test-project", Path: projectPath}

	initialContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	modifiedContent := "# Project: Test Project\n\n## Task: Modified task\n<!-- id: 1, status: done, priority: 1 -->\n"
	tasksPath := filepath.Join(projectPath, "tasks.md")

	if err := os.WriteFile(tasksPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}
	if err := RecordUndoSnapshot(project, "Add Task", "task1", 10); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}
	if err := os.WriteFile(tasksPath, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to write modified tasks file: %v", err)
	}

	// Undo: restores initialContent, creates redo entry
	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	// Verify we are at initialContent
	content, _ := os.ReadFile(tasksPath)
	if string(content) != initialContent {
		t.Error("expected initialContent after undo")
	}

	// Redo: should restore modifiedContent
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

	// Redo stack should now be empty
	redoEntries, _ := LoadRedoHistory(project)
	if len(redoEntries) != 0 {
		t.Errorf("expected 0 redo entries after redo, got %d", len(redoEntries))
	}

	// Undo stack should have one entry (the state before redo)
	if !UndoHistoryExists(project) {
		t.Error("expected undo history after redo")
	}
}

func TestNewMutationClearsRedoStack(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	project := &models.Project{ID: "test-id", Name: "Test Project", Slug: "test-project", Path: projectPath}

	content1 := "# Project: Test Project\n\n## Task: First task\n<!-- id: 1, status: todo, priority: 3 -->\n"
	content2 := "# Project: Test Project\n\n## Task: Modified task\n<!-- id: 1, status: done, priority: 1 -->\n"
	content3 := "# Project: Test Project\n\n## Task: New task\n<!-- id: 2, status: todo, priority: 2 -->\n"
	tasksPath := filepath.Join(projectPath, "tasks.md")

	if err := os.WriteFile(tasksPath, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}
	if err := RecordUndoSnapshot(project, "Action1", "", 10); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}
	if err := os.WriteFile(tasksPath, []byte(content2), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}

	// Undo → creates redo entry
	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	redoEntries, _ := LoadRedoHistory(project)
	if len(redoEntries) != 1 {
		t.Fatalf("expected 1 redo entry before new mutation, got %d", len(redoEntries))
	}

	// New mutation: record a snapshot (clears redo)
	if err := os.WriteFile(tasksPath, []byte(content3), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}
	// First record the snapshot for content1 (current state is content1 after undo)
	if err := RecordUndoSnapshot(project, "Action2", "", 10); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	redoEntries, _ = LoadRedoHistory(project)
	if len(redoEntries) != 0 {
		t.Errorf("expected 0 redo entries after new mutation, got %d", len(redoEntries))
	}

	// Verify redo snapshot file is deleted
	undoDir := getUndoDir(project)
	redoFile := filepath.Join(undoDir, "redo_0001.md")
	if _, statErr := os.Stat(redoFile); !os.IsNotExist(statErr) {
		t.Error("redo snapshot file should have been deleted after new mutation")
	}
}

func TestRedoRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	project := &models.Project{ID: "test-id", Name: "Test Project", Slug: "test-project", Path: projectPath}

	stateA := "# Project: Test Project\n\n## Task: State A\n<!-- id: 1, status: todo, priority: 3 -->\n"
	stateB := "# Project: Test Project\n\n## Task: State B\n<!-- id: 1, status: done, priority: 1 -->\n"
	tasksPath := filepath.Join(projectPath, "tasks.md")

	if err := os.WriteFile(tasksPath, []byte(stateA), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}
	if err := RecordUndoSnapshot(project, "Edit", "", 10); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}
	if err := os.WriteFile(tasksPath, []byte(stateB), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}

	// Undo: B → A
	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}
	content, _ := os.ReadFile(tasksPath)
	if string(content) != stateA {
		t.Error("expected stateA after undo")
	}

	// Redo: A → B
	if _, err := PerformRedo(project); err != nil {
		t.Fatalf("PerformRedo failed: %v", err)
	}
	content, _ = os.ReadFile(tasksPath)
	if string(content) != stateB {
		t.Error("expected stateB after redo")
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
	for i := range 5 {
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
