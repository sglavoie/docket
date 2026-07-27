package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// scenarioCreateForm builds a create task form for env.project wired to the
// scenario task-form coordinator, so handleSave commits through the real
// transactional stack (mutation → reload) instead of a raw project save.
func scenarioCreateForm(t *testing.T, env *scenarioEnv) *TaskFormModel {
	t.Helper()
	cfg := newTestConfig()
	m := NewTaskFormModelCreate(env.project, testRuntimeAt(cfg.DataDir(), cfg), nil)
	m.SetTaskFormCoordinator(env.features.TaskForm)
	return m
}

// runCreate executes handleSave and returns the committed TaskCreatedMsg.
func runCreate(t *testing.T, m *TaskFormModel) TaskCreatedMsg {
	t.Helper()
	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd on successful create; err=%v", m.err)
	}
	msg, ok := cmd().(TaskCreatedMsg)
	if !ok {
		t.Fatalf("expected TaskCreatedMsg, got %T", cmd())
	}
	if msg.Result == nil {
		t.Fatal("TaskCreatedMsg.Result should be non-nil on the coordinator path")
	}
	return msg
}

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

func TestTaskForm_Save_CreateInitializesCollectionFields(t *testing.T) {
	env := newScenarioEnv(t, "create-collections")
	m := scenarioCreateForm(t, env)

	m.draft.title = "Field init test"
	m.titleInput.SetValue("Field init test")

	msg := runCreate(t, m)
	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("created task absent from reloaded project")
	}
	if task.DependsOn == nil {
		t.Error("DependsOn should be initialized to empty slice, got nil")
	}
	if task.RelatedTo == nil {
		t.Error("RelatedTo should be initialized to empty slice, got nil")
	}
	if task.ScoreDimensions == nil {
		t.Error("ScoreDimensions should be initialized to empty map, got nil")
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

func TestTaskForm_Save_CreateHappyPath_AddsSubtasksForEpic(t *testing.T) {
	env := newScenarioEnv(t, "create-epic")
	m := scenarioCreateForm(t, env)

	m.draft.taskType = models.TaskTypeEpic
	m.draft.title = "Parent Epic"
	m.titleInput.SetValue("Parent Epic")
	m.draft.subtasks = []formSubtaskDraft{
		{title: "child one", taskType: models.TaskTypeTask},
		{title: "", taskType: models.TaskTypeTask},
		{title: "child two", taskType: models.TaskTypeBug},
	}

	msg := runCreate(t, m)
	fresh := env.reload(t)

	parent := fresh.GetTaskByID(msg.Result.TaskID)
	if parent == nil {
		t.Fatal("epic absent from reloaded project")
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
		t.Errorf("child one not parented to the epic")
	}
	if childTwo.Type != models.TaskTypeBug {
		t.Errorf("child two type = %v, want bug", childTwo.Type)
	}
	// The blank-title subtask was skipped.
	if childOne.DependsOn == nil || childOne.ScoreDimensions == nil {
		t.Error("subtask collection fields should be initialized")
	}
}

func TestTaskForm_Save_CreateOpenAfterSubtaskTargetsChild(t *testing.T) {
	env := newScenarioEnv(t, "create-openafter")
	m := scenarioCreateForm(t, env)

	m.draft.taskType = models.TaskTypeEpic
	m.draft.title = "Parent Epic"
	m.titleInput.SetValue("Parent Epic")
	m.draft.subtasks = []formSubtaskDraft{
		{title: "open-after subtask", taskType: models.TaskTypeTask},
	}
	m.subtaskOpenDetail = true
	m.subtaskOpenIdx = 0

	msg := runCreate(t, m)
	if !msg.OpenDetail {
		t.Error("OpenDetail should be true on the open-after-subtask path")
	}
	if msg.OpenTaskID == "" {
		t.Fatal("OpenTaskID should target the created subtask")
	}
	if msg.OpenTaskID == msg.Result.TaskID {
		t.Error("OpenTaskID should target the subtask, not the parent epic")
	}
	child := env.reload(t).GetTaskByID(msg.OpenTaskID)
	if child == nil || child.Title != "open-after subtask" {
		t.Fatalf("open-after target is not the subtask: %v", child)
	}
}

// ── Dimmed-aware insertion ──────────────────────────────────────────────────

func seedDimmedTask(t *testing.T, shortID int, title string, state models.WorkflowState) *models.Task {
	t.Helper()
	task := seedTask(t, shortID, title, models.StateReady)
	task.ApplyWorkflowState(state)
	task.Dimmed = true
	return task
}

func TestTaskForm_Save_Create_InsertsBeforeDimmedSameSection(t *testing.T) {
	dimmed := seedDimmedTask(t, 1, "Dimmed existing", models.StateNext)
	env := newScenarioEnv(t, "create-dimmed", dimmed)
	m := scenarioCreateForm(t, env)

	m.draft.title = "New important task"
	m.titleInput.SetValue("New important task")
	m.draft.state = models.StateNext

	msg := runCreate(t, m)
	fresh := env.reload(t)

	newIdx, dimmedIdx := -1, -1
	for i, task := range fresh.Tasks {
		switch task.Title {
		case "New important task":
			newIdx = i
		case "Dimmed existing":
			dimmedIdx = i
		}
	}
	if newIdx < 0 || dimmedIdx < 0 {
		t.Fatalf("tasks not found: new=%d dimmed=%d", newIdx, dimmedIdx)
	}
	if newIdx >= dimmedIdx {
		t.Errorf("new task at %d should precede dimmed task at %d", newIdx, dimmedIdx)
	}
	_ = msg
}

func TestTaskForm_Save_Create_DimmedInDifferentSectionIgnored(t *testing.T) {
	dimmed := seedDimmedTask(t, 1, "Dimmed in Inbox", models.StateInbox)
	env := newScenarioEnv(t, "create-dimmed-other", dimmed)
	m := scenarioCreateForm(t, env)

	m.draft.title = "New Next task"
	m.titleInput.SetValue("New Next task")
	m.draft.state = models.StateNext

	runCreate(t, m)
	fresh := env.reload(t)

	newIdx, dimmedIdx := -1, -1
	for i, task := range fresh.Tasks {
		switch task.Title {
		case "New Next task":
			newIdx = i
		case "Dimmed in Inbox":
			dimmedIdx = i
		}
	}
	if newIdx < 0 || dimmedIdx < 0 {
		t.Fatalf("tasks not found: new=%d dimmed=%d", newIdx, dimmedIdx)
	}
	// The dimmed task is in a different section, so it must not pull the new task
	// before it.
	if newIdx <= dimmedIdx {
		t.Errorf("new Next task (%d) should not precede Inbox-dimmed task (%d)", newIdx, dimmedIdx)
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
