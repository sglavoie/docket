package taskform

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/status"
)

// ── Create happy path ───────────────────────────────────────────────────────

func TestTaskForm_Save_CreateHappyPath_CommitsThroughCoordinator(t *testing.T) {
	env := newScenarioEnv(t, "create")
	m := scenarioCreateForm(t, env)

	m.draft.title = "My New Task"
	m.titleInput.SetValue("My New Task")
	m.draft.priority = 2
	m.draft.tags = []string{"go", "tui"}
	m.draft.desc = "Task description"

	msg := runCreate(t, m)

	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("created task absent from reloaded project")
	}
	if task.Title != "My New Task" {
		t.Errorf("Title = %q, want My New Task", task.Title)
	}
	if task.Priority != 2 {
		t.Errorf("Priority = %d, want 2", task.Priority)
	}
	if len(task.Tags) != 2 {
		t.Errorf("Tags = %v, want 2", task.Tags)
	}
	if task.Description != "Task description" {
		t.Errorf("Description = %q, want Task description", task.Description)
	}

	// undo removes the created task.
	env.undo(t)
	if env.reload(t).GetTaskByID(msg.Result.TaskID) != nil {
		t.Fatal("created task survived undo")
	}
}

func TestTaskFormCreateDoubleSubmitCreatesOneShortIDAndOneCommit(t *testing.T) {
	env := newScenarioEnv(t, "create-double-submit")
	m := scenarioCreateForm(t, env)
	m.titleInput.SetValue("Create once")

	first := m.handleSave()
	duplicate := m.handleSave()
	if first == nil || duplicate != nil || !m.submissionInFlight {
		t.Fatalf("commands first=%v duplicate=%v inFlight=%v", first != nil, duplicate != nil, m.submissionInFlight)
	}
	msg, ok := first().(coordinator.TaskCreatedMsg)
	if !ok || msg.Result == nil {
		t.Fatalf("first result = %#v", msg)
	}
	m.Update(msg)
	if m.submissionInFlight {
		t.Fatal("success did not release the create-form submission guard")
	}

	fresh := env.reload(t)
	if len(fresh.Tasks) != 1 || fresh.Tasks[0].ShortID != 1 || fresh.NextShortID != 2 {
		t.Fatalf("created tasks=%+v nextShortID=%d", fresh.Tasks, fresh.NextShortID)
	}
	env.undo(t)
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("one undo left %d tasks; create form committed twice", got)
	}
}

func TestTaskFormCreateSubmissionGuardClearsAfterError(t *testing.T) {
	stub := &stubMutator{err: errors.New("create failed")}
	features := coordinator.NewFeatureCoordinators(stub)
	m := newTestTaskForm()
	m.SetTaskFormCoordinator(features.TaskForm)
	m.titleInput.SetValue("Retry create")

	first := m.handleSave()
	duplicate := m.handleSave()
	if first == nil || duplicate != nil {
		t.Fatalf("commands first=%v duplicate=%v", first != nil, duplicate != nil)
	}
	failure, ok := first().(status.ErrorMsg)
	if !ok || failure.Err == nil || stub.calls != 1 {
		t.Fatalf("failure=%#v calls=%d", failure, stub.calls)
	}
	m.Update(failure)
	if m.submissionInFlight || m.err == nil {
		t.Fatalf("error lifecycle inFlight=%v err=%v", m.submissionInFlight, m.err)
	}
	if retry := m.handleSave(); retry == nil {
		t.Fatal("create form did not allow retry after failure")
	}
}

func TestTaskForm_Save_CreateHappyPath_AddsChecklistItems(t *testing.T) {
	env := newScenarioEnv(t, "create-checklist")
	m := scenarioCreateForm(t, env)

	m.draft.title = "Task with checklist"
	m.titleInput.SetValue("Task with checklist")
	m.draft.checklist = []models.ChecklistItem{
		{Text: "step one", Done: false},
		{Text: "  ", Done: false},
		{Text: "step two", Done: true},
	}

	msg := runCreate(t, m)
	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("created task absent from reloaded project")
	}
	if len(task.Checklist) != 2 {
		t.Fatalf("expected 2 checklist items (blank filtered), got %d", len(task.Checklist))
	}
	if task.Checklist[0].Text != "step one" {
		t.Errorf("first item = %q, want step one", task.Checklist[0].Text)
	}
	if !task.Checklist[1].Done {
		t.Error("second checklist item should preserve Done=true")
	}
}

func TestTaskForm_Save_CreateResultCarriesProjectID(t *testing.T) {
	env := newScenarioEnv(t, "create-projid")
	m := scenarioCreateForm(t, env)

	m.draft.title = "Task"
	m.titleInput.SetValue("Task")

	msg := runCreate(t, m)
	if msg.Result.ProjectID != env.project.ID {
		t.Errorf("Result.ProjectID = %q, want %q", msg.Result.ProjectID, env.project.ID)
	}
}

func TestTaskForm_Save_CreateHappyPath_AddsSubtasks(t *testing.T) {
	env := newScenarioEnv(t, "create-parent")
	m := scenarioCreateForm(t, env)

	m.draft.title = "Parent Task"
	m.titleInput.SetValue("Parent Task")
	m.draft.subtasks = []formSubtaskDraft{
		{title: "child one"},
		{title: ""},
		{title: "child two"},
	}

	msg := runCreate(t, m)
	fresh := env.reload(t)

	parent := fresh.GetTaskByID(msg.Result.TaskID)
	if parent == nil {
		t.Fatal("parent absent from reloaded project")
	}
	var childOne, childTwo *models.Task
	for _, task := range fresh.Tasks {
		switch task.Title {
		case "child one":
			childOne = task
		case "child two":
			childTwo = task
		}
	}
	if childOne == nil || childTwo == nil {
		t.Fatalf("subtasks missing: one=%v two=%v", childOne, childTwo)
	}
	if childOne.ParentID == nil || *childOne.ParentID != parent.ID {
		t.Errorf("child one not parented to the parent task")
	}
}

func TestTaskForm_Save_CreateEditAfterSubtaskTargetsChild(t *testing.T) {
	env := newScenarioEnv(t, "create-editafter")
	m := scenarioCreateForm(t, env)

	m.draft.title = "Parent Task"
	m.titleInput.SetValue("Parent Task")
	m.draft.subtasks = []formSubtaskDraft{
		{title: "edit-after subtask"},
	}
	m.subtaskOpenContent = true
	m.subtaskOpenIdx = 0

	msg := runCreate(t, m)
	if !msg.OpenContent {
		t.Fatal("edit-after result did not request description editing")
	}
	if msg.OpenTaskID == "" {
		t.Fatal("OpenTaskID should target the created subtask")
	}
	if msg.OpenTaskID == msg.Result.TaskID {
		t.Error("OpenTaskID should target the subtask, not the parent task")
	}
	child := env.reload(t).GetTaskByID(msg.OpenTaskID)
	if child == nil || child.Title != "edit-after subtask" {
		t.Fatalf("edit-after target is not the subtask: %v", child)
	}
}

// TestTaskForm_Save_CreateWithoutCoordinatorBuildsCmd guards that a create form
// with no coordinator wired (isolated unit tests) still builds a command without
// panicking; only executing it needs the coordinator.
func TestTaskForm_Save_CreateWithoutCoordinatorBuildsCmd(t *testing.T) {
	m := newTestTaskForm()
	m.draft.title = "no coordinator"
	m.titleInput.SetValue("no coordinator")
	cmd := m.saveCreate()
	if cmd == nil {
		t.Fatal("saveCreate should build a command even without a coordinator")
	}
}
