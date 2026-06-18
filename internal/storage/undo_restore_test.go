package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func assertUndoEntryMetadata(t *testing.T, entry UndoEntry, opts UndoSnapshotOptions) {
	t.Helper()

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

func TestPerformUndoRedoPreservesGroupMetadata(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: task-1, status: todo, priority: 3 -->\n"
	modifiedContent := "# Project: Test Project\n\n## Task: First task\n<!-- id: task-1, status: done, priority: 3 -->\n"

	writeUndoTasks(t, tasksPath, initialContent)
	opts := UndoSnapshotOptions{
		Action:          "Grouped Change",
		Context:         "task-1",
		MaxDepth:        10,
		TaskIDs:         []string{"task-1"},
		GroupID:         "group-1",
		GroupProjectIDs: []string{project.ID},
	}
	if err := RecordUndoSnapshotWithOptions(project, opts); err != nil {
		t.Fatalf("RecordUndoSnapshotWithOptions failed: %v", err)
	}
	writeUndoTasks(t, tasksPath, modifiedContent)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	redoEntries, err := LoadRedoHistory(project)
	if err != nil {
		t.Fatalf("LoadRedoHistory failed: %v", err)
	}
	if len(redoEntries) != 1 {
		t.Fatalf("expected 1 redo entry, got %d", len(redoEntries))
	}
	assertUndoEntryMetadata(t, redoEntries[0], opts)

	if _, err := PerformRedo(project); err != nil {
		t.Fatalf("PerformRedo failed: %v", err)
	}

	undoEntries, err := LoadUndoHistory(project)
	if err != nil {
		t.Fatalf("LoadUndoHistory failed: %v", err)
	}
	if len(undoEntries) != 1 {
		t.Fatalf("expected 1 undo entry after redo, got %d", len(undoEntries))
	}
	assertUndoEntryMetadata(t, undoEntries[0], opts)
}

func TestPerformGroupedUndoRestoresAllProjects(t *testing.T) {
	fixtures := newUndoTestProjects(t, "alpha", "beta")
	projects := undoFixtureProjects(fixtures)
	initialContents, modifiedContents := seedGroupedUndoProjects(t, fixtures, "group-undo")

	actionDesc, err := PerformUndoForProjects(fixtures[0].Project, projects)
	if err != nil {
		t.Fatalf("PerformUndoForProjects failed: %v", err)
	}
	if actionDesc != "Grouped Change: two projects" {
		t.Fatalf("action desc = %q, want grouped action", actionDesc)
	}

	groupProjectIDs := undoFixtureProjectIDs(fixtures)
	for i, fixture := range fixtures {
		assertUndoTasksEqual(t, fixture.TasksPath, initialContents[i])
		meta := readUndoMetaForTest(t, fixture.Project)
		assertUndoMetaCounts(t, meta, 0, 1)
		assertUndoEntryGroup(t, meta.RedoEntries[0], "group-undo", groupProjectIDs)
	}

	for i, fixture := range fixtures {
		if string(mustReadFile(t, getRedoSnapshotPath(fixture.Project, 1))) != modifiedContents[i] {
			t.Fatalf("redo snapshot for %s does not contain pre-undo content", fixture.Project.Slug)
		}
	}
}

func TestPerformGroupedRedoRestoresAllProjects(t *testing.T) {
	fixtures := newUndoTestProjects(t, "alpha", "beta")
	projects := undoFixtureProjects(fixtures)
	_, modifiedContents := seedGroupedUndoProjects(t, fixtures, "group-redo")

	if _, err := PerformUndoForProjects(fixtures[0].Project, projects); err != nil {
		t.Fatalf("PerformUndoForProjects failed: %v", err)
	}

	actionDesc, err := PerformRedoForProjects(fixtures[1].Project, projects)
	if err != nil {
		t.Fatalf("PerformRedoForProjects failed: %v", err)
	}
	if actionDesc != "Grouped Change: two projects" {
		t.Fatalf("action desc = %q, want grouped action", actionDesc)
	}

	groupProjectIDs := undoFixtureProjectIDs(fixtures)
	for i, fixture := range fixtures {
		assertUndoTasksEqual(t, fixture.TasksPath, modifiedContents[i])
		meta := readUndoMetaForTest(t, fixture.Project)
		assertUndoMetaCounts(t, meta, 1, 0)
		assertUndoEntryGroup(t, meta.Entries[0], "group-redo", groupProjectIDs)
	}
}

func TestPerformGroupedUndoAbortsWhenMemberProjectMissing(t *testing.T) {
	fixtures := newUndoTestProjects(t, "alpha", "beta")
	_, modifiedContents := seedGroupedUndoProjects(t, fixtures, "group-missing")

	_, err := PerformUndoForProjects(fixtures[0].Project, []*models.Project{fixtures[0].Project})
	if err == nil {
		t.Fatal("expected grouped undo to fail when a member project is missing")
	}

	for i, fixture := range fixtures {
		assertUndoTasksEqual(t, fixture.TasksPath, modifiedContents[i])
		meta := readUndoMetaForTest(t, fixture.Project)
		assertUndoMetaCounts(t, meta, 1, 0)
		if _, statErr := os.Stat(getRedoSnapshotPath(fixture.Project, 1)); !os.IsNotExist(statErr) {
			t.Fatalf("redo snapshot should not be written for %s after preflight failure, stat err: %v", fixture.Project.Slug, statErr)
		}
	}
}

func TestPerformGroupedUndoAbortsWhenTopHistoryDiverged(t *testing.T) {
	fixtures := newUndoTestProjects(t, "alpha", "beta")
	projects := undoFixtureProjects(fixtures)
	_, modifiedContents := seedGroupedUndoProjects(t, fixtures, "group-diverged")

	if err := RecordUndoSnapshot(fixtures[1].Project, "Independent Edit", "beta", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}
	betaDiverged := groupedUndoTasksMarkdown(fixtures[1].Project, "task-beta", "Beta diverged", "done")
	writeUndoTasks(t, fixtures[1].TasksPath, betaDiverged)

	_, err := PerformUndoForProjects(fixtures[0].Project, projects)
	if err == nil {
		t.Fatal("expected grouped undo to fail when a member top history entry diverged")
	}

	assertUndoTasksEqual(t, fixtures[0].TasksPath, modifiedContents[0])
	assertUndoTasksEqual(t, fixtures[1].TasksPath, betaDiverged)
	assertUndoMetaCounts(t, readUndoMetaForTest(t, fixtures[0].Project), 1, 0)
	assertUndoMetaCounts(t, readUndoMetaForTest(t, fixtures[1].Project), 2, 0)
}

func TestPerformGroupedRedoAbortsAfterNewEditClearsOneMemberRedo(t *testing.T) {
	fixtures := newUndoTestProjects(t, "alpha", "beta")
	projects := undoFixtureProjects(fixtures)
	initialContents, _ := seedGroupedUndoProjects(t, fixtures, "group-redo-cleared")

	if _, err := PerformUndoForProjects(fixtures[0].Project, projects); err != nil {
		t.Fatalf("PerformUndoForProjects failed: %v", err)
	}

	if err := RecordUndoSnapshot(fixtures[0].Project, "New Edit", "alpha", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}
	alphaNew := groupedUndoTasksMarkdown(fixtures[0].Project, "task-alpha", "Alpha new edit", "todo")
	writeUndoTasks(t, fixtures[0].TasksPath, alphaNew)

	alphaRedo, err := LoadRedoHistory(fixtures[0].Project)
	if err != nil {
		t.Fatalf("LoadRedoHistory(alpha) failed: %v", err)
	}
	if len(alphaRedo) != 0 {
		t.Fatalf("alpha redo entries = %d, want 0 after new edit", len(alphaRedo))
	}

	_, err = PerformRedoForProjects(fixtures[1].Project, projects)
	if err == nil {
		t.Fatal("expected grouped redo to fail when a member redo entry was cleared")
	}

	assertUndoTasksEqual(t, fixtures[0].TasksPath, alphaNew)
	assertUndoTasksEqual(t, fixtures[1].TasksPath, initialContents[1])
	betaMeta := readUndoMetaForTest(t, fixtures[1].Project)
	assertUndoMetaCounts(t, betaMeta, 0, 1)
}

func TestPerformUndoMissingSnapshotLeavesHistoryUntouched(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	currentContent := "# Project: Test Project\n\n## Task: Current\n<!-- id: task-1, status: done, priority: 1 -->\n"
	writeUndoTasks(t, tasksPath, currentContent)

	if err := os.MkdirAll(getUndoDir(project), DirPermissions); err != nil {
		t.Fatalf("failed to create undo dir: %v", err)
	}
	meta := &UndoMeta{
		Entries: []UndoEntry{{
			Num:    1,
			Action: "Edit",
		}},
	}
	if err := saveUndoMeta(project, meta); err != nil {
		t.Fatalf("failed to save undo meta: %v", err)
	}

	_, err := PerformUndo(project)
	if err == nil {
		t.Fatal("expected PerformUndo to fail when metadata points at a missing snapshot")
	}
	if !strings.Contains(err.Error(), "failed to read undo snapshot 1") {
		t.Fatalf("missing snapshot error = %v", err)
	}

	assertUndoTasksEqual(t, tasksPath, currentContent)
	meta = readUndoMetaForTest(t, project)
	assertUndoMetaCounts(t, meta, 1, 0)
	if _, statErr := os.Stat(getRedoSnapshotPath(project, 1)); !os.IsNotExist(statErr) {
		t.Fatalf("redo snapshot should not be written when source snapshot is missing, stat err: %v", statErr)
	}
}

func TestPerformGroupedUndoPreflightMissingSnapshotLeavesTransactionUntouched(t *testing.T) {
	fixtures := newUndoTestProjects(t, "alpha", "beta")
	projects := undoFixtureProjects(fixtures)
	_, modifiedContents := seedGroupedUndoProjects(t, fixtures, "group-missing-snapshot")

	if err := os.Remove(getSnapshotPath(fixtures[1].Project, 1)); err != nil {
		t.Fatalf("failed to remove beta snapshot: %v", err)
	}

	_, err := PerformUndoForProjects(fixtures[0].Project, projects)
	if err == nil {
		t.Fatal("expected grouped undo to fail when a member snapshot is missing")
	}
	if !strings.Contains(err.Error(), "failed to read undo snapshot 1") {
		t.Fatalf("missing grouped snapshot error = %v", err)
	}

	for i, fixture := range fixtures {
		assertUndoTasksEqual(t, fixture.TasksPath, modifiedContents[i])
		meta := readUndoMetaForTest(t, fixture.Project)
		assertUndoMetaCounts(t, meta, 1, 0)
		if _, statErr := os.Stat(getRedoSnapshotPath(fixture.Project, 1)); !os.IsNotExist(statErr) {
			t.Fatalf("redo snapshot should not be written for %s after preflight failure, stat err: %v", fixture.Project.Slug, statErr)
		}
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

func TestPerformRestoreRollsBackTasksWhenMetaSaveFails(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	undoDir := getUndoDir(project)
	if err := os.MkdirAll(undoDir, 0755); err != nil {
		t.Fatalf("failed to create undo dir: %v", err)
	}

	snapshotContent := "# Project: Test Project\n\n## Task: Snapshot task\n<!-- id: task-1, status: todo, priority: 3 -->\n"
	currentContent := "# Project: Test Project\n\n## Task: Current task\n<!-- id: task-1, status: done, priority: 1 -->\n"
	writeUndoTasks(t, tasksPath, currentContent)

	snapshotPath := getSnapshotPath(project, 1)
	if err := os.WriteFile(snapshotPath, []byte(snapshotContent), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}

	metaPath := getUndoMetaPath(project)
	if err := os.Mkdir(metaPath, 0755); err != nil {
		t.Fatalf("failed to create meta path directory: %v", err)
	}

	meta := &UndoMeta{
		Entries: []UndoEntry{{
			Num:    1,
			Action: "Edit",
		}},
		RedoEntries: []UndoEntry{},
	}

	_, err := performRestore(project, meta, restoreParams{
		label:          "undo",
		sourceEntries:  &meta.Entries,
		reverseEntries: &meta.RedoEntries,
		snapshotPath:   getSnapshotPath,
		reversePath:    getRedoSnapshotPath,
	})
	if err == nil {
		t.Fatal("expected performRestore to fail when meta path is a directory")
	}

	content, readErr := os.ReadFile(tasksPath)
	if readErr != nil {
		t.Fatalf("failed to read tasks file: %v", readErr)
	}
	if string(content) != currentContent {
		t.Fatalf("tasks.md was not rolled back after meta save failure:\n%s", string(content))
	}

	if _, statErr := os.Stat(getRedoSnapshotPath(project, 1)); !os.IsNotExist(statErr) {
		t.Fatalf("redo snapshot should be removed after meta save failure, stat err: %v", statErr)
	}
	if len(meta.Entries) != 1 || meta.Entries[0].Num != 1 {
		t.Fatalf("source entries were not restored in memory: %+v", meta.Entries)
	}
	if len(meta.RedoEntries) != 0 {
		t.Fatalf("reverse entries were not restored in memory: %+v", meta.RedoEntries)
	}
}

func seedGroupedUndoProjects(t *testing.T, fixtures []undoTestProjectFixture, groupID string) ([]string, []string) {
	t.Helper()

	groupProjectIDs := undoFixtureProjectIDs(fixtures)
	initialContents := make([]string, 0, len(fixtures))
	modifiedContents := make([]string, 0, len(fixtures))
	for _, fixture := range fixtures {
		taskID := "task-" + fixture.Project.Slug
		initial := groupedUndoTasksMarkdown(fixture.Project, taskID, fixture.Project.Slug+" original", "todo")
		modified := groupedUndoTasksMarkdown(fixture.Project, taskID, fixture.Project.Slug+" modified", "done")
		writeUndoTasks(t, fixture.TasksPath, initial)
		if err := RecordUndoSnapshotWithOptions(fixture.Project, UndoSnapshotOptions{
			Action:          "Grouped Change",
			Context:         "two projects",
			MaxDepth:        10,
			GroupID:         groupID,
			GroupProjectIDs: groupProjectIDs,
		}); err != nil {
			t.Fatalf("RecordUndoSnapshotWithOptions(%s) failed: %v", fixture.Project.Slug, err)
		}
		writeUndoTasks(t, fixture.TasksPath, modified)
		initialContents = append(initialContents, initial)
		modifiedContents = append(modifiedContents, modified)
	}
	return initialContents, modifiedContents
}

func groupedUndoTasksMarkdown(project *models.Project, taskID, title, status string) string {
	return fmt.Sprintf("# Project: %s\n\n## Task: %s\n<!-- id: %s, status: %s, priority: 3 -->\n", project.Name, title, taskID, status)
}

func undoFixtureProjects(fixtures []undoTestProjectFixture) []*models.Project {
	projects := make([]*models.Project, 0, len(fixtures))
	for _, fixture := range fixtures {
		projects = append(projects, fixture.Project)
	}
	return projects
}

func undoFixtureProjectIDs(fixtures []undoTestProjectFixture) []string {
	ids := make([]string, 0, len(fixtures))
	for _, fixture := range fixtures {
		ids = append(ids, fixture.Project.ID)
	}
	return ids
}

func assertUndoTasksEqual(t *testing.T, tasksPath, want string) {
	t.Helper()

	got := string(mustReadFile(t, tasksPath))
	if got != want {
		t.Fatalf("tasks content mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", tasksPath, got, want)
	}
}

func assertUndoEntryGroup(t *testing.T, entry UndoEntry, groupID string, groupProjectIDs []string) {
	t.Helper()

	if entry.GroupID != groupID {
		t.Fatalf("entry group ID = %q, want %q", entry.GroupID, groupID)
	}
	if !reflect.DeepEqual(entry.GroupProjectIDs, groupProjectIDs) {
		t.Fatalf("entry group project IDs = %v, want %v", entry.GroupProjectIDs, groupProjectIDs)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return content
}
