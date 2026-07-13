package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestQuickCaptureCreatesThroughCoordinator proves Quick Capture routes creation
// through the CaptureCoordinator: createTask emits a TaskCreatedMsg carrying the
// committed application result, the task is prepended and persisted with its
// parsed metadata, and no raw project save happens in the view.
func TestQuickCaptureCreatesThroughCoordinator(t *testing.T) {
	existing := seedTask(t, 1, "existing", models.StateReady)
	env := newScenarioEnv(t, "qc", existing)
	m := NewQuickCaptureModel(env.project, []*models.Project{env.project}, false)
	m.SetCaptureCoordinator(env.features.Capture)
	m.input.SetValue("Buy milk !1 #groceries")

	cmd := m.createTask(false, false)
	if cmd == nil {
		t.Fatal("expected a command from createTask")
	}
	msg, ok := cmd().(TaskCreatedMsg)
	if !ok {
		t.Fatalf("expected TaskCreatedMsg, got %T", cmd())
	}
	if msg.Result == nil {
		t.Fatal("expected the committed application result on the message")
	}

	fresh := env.reload(t)
	task := fresh.GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("captured task absent from reloaded project")
	}
	if task.Priority != 1 {
		t.Errorf("priority = %d, want 1", task.Priority)
	}
	if !task.HasTag("groceries") {
		t.Errorf("tags = %v, want groceries", task.Tags)
	}
	if models.SectionForState(task.State()) != models.SectionInbox {
		t.Errorf("section = %q, want Inbox", models.SectionForState(task.State()))
	}
	// Prepend keeps the fresh capture visually first.
	if fresh.Tasks[0].ID != msg.Result.TaskID {
		t.Errorf("captured task not prepended: first is %q", fresh.Tasks[0].Title)
	}
}

// TestQuickCaptureStatusSigilAppliesState proves a $status sigil is applied as a
// workflow-state transition through the capture use case.
func TestQuickCaptureStatusSigilAppliesState(t *testing.T) {
	env := newScenarioEnv(t, "qc-state")
	m := NewQuickCaptureModel(env.project, []*models.Project{env.project}, false)
	m.SetCaptureCoordinator(env.features.Capture)
	m.input.SetValue("Ship it $review")

	cmd := m.createTask(false, false)
	msg, ok := cmd().(TaskCreatedMsg)
	if !ok || msg.Result == nil {
		t.Fatalf("expected TaskCreatedMsg with result, got %T", cmd())
	}
	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil || task.State() != models.StateReview {
		t.Fatalf("state = %v, want review", task)
	}
}

// TestFocusBoardCreatesThroughCoordinator proves the Focus Board quick-add routes
// creation through the CaptureCoordinator (CaptureBoard), committing into the
// selected project without a raw project save.
func TestFocusBoardCreatesThroughCoordinator(t *testing.T) {
	env := newScenarioEnv(t, "board")
	m := &FocusBoardModel{
		acProjectCache: []*models.Project{env.project},
		capture:        env.features.Capture,
	}

	_, cmd := m.createItemFromInput("%board Review PR !2 #urgent")
	if cmd == nil {
		t.Fatal("expected a command from createItemFromInput")
	}
	msg, ok := cmd().(TransactionResultMsg)
	if !ok {
		t.Fatalf("expected TransactionResultMsg, got %T", cmd())
	}
	if msg.Result.Title != "Review PR" {
		t.Errorf("result title = %q, want %q", msg.Result.Title, "Review PR")
	}

	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("focus-board task absent from reloaded project")
	}
	if task.Priority != 2 {
		t.Errorf("priority = %d, want 2", task.Priority)
	}
	if !task.HasTag("urgent") {
		t.Errorf("tags = %v, want urgent", task.Tags)
	}
}
