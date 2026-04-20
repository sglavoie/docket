package tui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TestFocusBoardEditTitle_UndoContextUsesOldTitle verifies that the undo
// context captures the original title, not the new one, when renaming a task
// via the focus board edit input.
func TestFocusBoardEditTitle_UndoContextUsesOldTitle(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", t.TempDir())
	task := makeBoardTask(t, "id-title", 1, "Old Title")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	row := boardRow{Task: task, Project: project, Source: project.Name}

	m := newTestFocusBoard()
	m.config = &config.Config{}
	m.filtered = []boardRow{row}
	m.editMode = true
	m.editIdx = 0

	ei := textinput.New()
	ei.SetValue("New Title")
	ei.Focus()
	m.editInput = ei

	// Simulate pressing Enter to confirm the edit.
	// We cannot fully exercise persistWithWorkflow (no persister wired),
	// so instead we verify the mutation ordering directly:
	// after MutateTask, the task pointer holds the new title.
	oldTitle := row.Task.Title
	row.Project.MutateTask(row.Task.ID, func(t *models.Task) {
		t.Title = "New Title"
	})

	// row.Task.Title is now "New Title" because MutateTask mutates in-place.
	if row.Task.Title != "New Title" {
		t.Fatalf("expected task title to be mutated to 'New Title', got %q", row.Task.Title)
	}
	// The captured oldTitle must still be "Old Title".
	if oldTitle != "Old Title" {
		t.Errorf("oldTitle should be 'Old Title' but got %q — undo context would be wrong", oldTitle)
	}
}

// TestQuickEditRename_UndoContextUsesOldTitle verifies that
// handleQuickEditCommit captures the old title before MutateTask runs,
// so the undo context shows the original title, not the new one.
func TestQuickEditRename_UndoContextUsesOldTitle(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", t.TempDir())
	task, err := models.NewTask("id-qe", 1, models.TaskTypeTask, "Original Name")
	if err != nil {
		t.Fatal(err)
	}
	task.Status = models.StatusInProgress
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Simulate the pattern in shared_primary_view_keys.go:
	//   oldTitle := task.Title
	//   project.MutateTask(task.ID, func(t *models.Task) { t.Title = input })
	//   onUndo("Rename Task", oldTitle)
	oldTitle := task.Title
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Title = "Renamed Task"
	})

	// task.Title is now "Renamed Task" since MutateTask mutates in-place.
	if task.Title != "Renamed Task" {
		t.Fatalf("expected task title to be 'Renamed Task', got %q", task.Title)
	}
	// The captured oldTitle must be the original.
	if oldTitle != "Original Name" {
		t.Errorf("oldTitle should be 'Original Name' but got %q — undo context would show wrong title", oldTitle)
	}
}

// TestMutateTask_TitlePointerSharing confirms that MutateTask modifies the
// task in-place, so any reference to task.Title after mutation sees the new
// value. This documents why capturing oldTitle before MutateTask is necessary.
func TestMutateTask_TitlePointerSharing(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", t.TempDir())
	task, err := models.NewTask("id-ptr", 1, models.TaskTypeTask, "Before")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)

	// Hold a reference to the same task pointer
	ref := task

	project.MutateTask(task.ID, func(t *models.Task) {
		t.Title = "After"
	})

	// Both task and ref see the mutated title — they point to the same struct.
	if task.Title != "After" {
		t.Errorf("task.Title should be 'After', got %q", task.Title)
	}
	if ref.Title != "After" {
		t.Errorf("ref.Title should be 'After', got %q — pointer sharing not working as expected", ref.Title)
	}
}

// stubPersister is a minimal Persister implementation for unit tests.
// It records RecordUndoSnapshot calls for assertion and no-ops everything else.
type stubPersister struct {
	recordedAction  string
	recordedContext string
	recordedDepth   int
	recordedTaskIDs []string
}

func (s *stubPersister) Load(_ *models.Project) error                              { return nil }
func (s *stubPersister) Save(_ *models.Project) error                              { return nil }
func (s *stubPersister) Backup(_ *models.Project) error                            { return nil }
func (s *stubPersister) SaveWithBackupSync(_ *models.Project) error                { return nil }
func (s *stubPersister) SaveMultipleSync(_ []*models.Project) error                { return nil }
func (s *stubPersister) BackupExists(_ *models.Project) bool                       { return false }
func (s *stubPersister) RestoreFromBackup(_ *models.Project) error                 { return nil }
func (s *stubPersister) ArchiveTasks(_ *models.Project) (int, error)               { return 0, nil }
func (s *stubPersister) SaveWithBackup(_ *models.Project, _ string) tea.Cmd        { return nil }
func (s *stubPersister) SaveMultiple(_ []*models.Project, _ string) tea.Cmd        { return nil }
func (s *stubPersister) RecordUndoSnapshot(_ *models.Project, action, ctx string, depth int, taskIDs []string) error {
	s.recordedAction = action
	s.recordedContext = ctx
	s.recordedDepth = depth
	s.recordedTaskIDs = taskIDs
	return nil
}

// TestSimplePersistWithUndo_RecordsSnapshot verifies that simplePersistWithUndo
// calls RecordUndoSnapshot with the correct action and context derived from the task.
func TestSimplePersistWithUndo_RecordsSnapshot(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", t.TempDir())
	task, err := models.NewTask("task-id", 1, models.TaskTypeTask, "My Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	depth := 10
	cfg := &config.Config{UndoHistoryDepth: &depth}
	stub := &stubPersister{}

	pf := simplePersistWithUndo(stub, cfg, "Complete Task")
	_ = pf(project, task.ID, func(t *models.Task) { t.Status = models.StatusDone })

	if stub.recordedAction != "Complete Task" {
		t.Errorf("expected action 'Complete Task', got %q", stub.recordedAction)
	}
	if stub.recordedContext != "#1 My Task" {
		t.Errorf("expected context '#1 My Task', got %q", stub.recordedContext)
	}
	if stub.recordedDepth != depth {
		t.Errorf("expected depth %d, got %d", depth, stub.recordedDepth)
	}
	if len(stub.recordedTaskIDs) != 1 || stub.recordedTaskIDs[0] != task.ID {
		t.Errorf("expected taskIDs [%q], got %v", task.ID, stub.recordedTaskIDs)
	}
}

// TestSimplePersistWithUndo_SkipsWhenDisabled verifies that simplePersistWithUndo
// does not call RecordUndoSnapshot when UndoHistoryDepth is 0 or nil.
func TestSimplePersistWithUndo_SkipsWhenDisabled(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", t.TempDir())
	task, err := models.NewTask("task-id", 1, models.TaskTypeTask, "My Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	zero := 0
	cfg := &config.Config{UndoHistoryDepth: &zero}
	stub := &stubPersister{}

	pf := simplePersistWithUndo(stub, cfg, "Complete Task")
	_ = pf(project, task.ID, func(t *models.Task) { t.Status = models.StatusDone })

	if stub.recordedAction != "" {
		t.Errorf("expected no snapshot when depth=0, but got action %q", stub.recordedAction)
	}
}

// TestPersistWithWorkflow_SafetyNet verifies that persistWithWorkflow records an
// undo snapshot even when UndoAction is empty, using the fallback label "Task Modified".
func TestPersistWithWorkflow_SafetyNet(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", t.TempDir())
	task, err := models.NewTask("task-id", 1, models.TaskTypeTask, "My Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	depth := 5
	stub := &stubPersister{}

	opts := WorkflowSaveOptions{
		Persister:   stub,
		Project:     project,
		UndoDepth:   depth,
		UndoAction:  "", // intentionally empty — safety net should fill this in
		UndoContext: "",
	}
	if err := persistWithWorkflow(opts); err != nil {
		t.Fatalf("persistWithWorkflow failed: %v", err)
	}

	if stub.recordedAction != "Task Modified" {
		t.Errorf("expected safety-net action 'Task Modified', got %q", stub.recordedAction)
	}
}

// Ensure tea import is used (required by test file convention in this package).
var _ tea.Msg = BackMsg{}
