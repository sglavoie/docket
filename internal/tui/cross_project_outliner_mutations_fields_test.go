package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// crossFieldFixture builds two disk-backed projects and a cross-project
// OutlinerModel with cursor on the first beta task (project B, non-first).
func crossFieldFixture(t *testing.T) (m *OutlinerModel, beta *models.Project, betaTaskID string) {
	t.Helper()

	alphaDir := t.TempDir()
	betaDir := t.TempDir()

	alphaTask, err := models.NewTask("alpha-task-1", 1, models.TaskTypeTask, "Alpha task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	alpha := &models.Project{ID: "alpha-proj", Name: "Alpha", Path: alphaDir}
	alpha.Tasks = []*models.Task{alphaTask}
	alpha.NextShortID = 2
	alpha.BuildTaskTree()
	if saveErr := storage.SaveProject(alpha); saveErr != nil {
		t.Fatalf("save alpha: %v", saveErr)
	}

	betaTask, err := models.NewTask("beta-task-1", 1, models.TaskTypeTask, "Beta task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	beta = &models.Project{ID: "beta-proj", Name: "Beta", Path: betaDir}
	beta.Tasks = []*models.Task{betaTask}
	beta.NextShortID = 2
	beta.BuildTaskTree()
	if saveErr := storage.SaveProject(beta); saveErr != nil {
		t.Fatalf("save beta: %v", saveErr)
	}

	if loadErr := storage.LoadProject(alpha); loadErr != nil {
		t.Fatalf("load alpha: %v", loadErr)
	}
	if loadErr := storage.LoadProject(beta); loadErr != nil {
		t.Fatalf("load beta: %v", loadErr)
	}

	loaded := beta.GetTaskByID("beta-task-1")
	if loaded == nil {
		t.Fatal("beta task not found after reload")
	}

	m = &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			projectService: NewProjectService(),
			bulkOp:         NewBulkOperationComponent(),
			quickEdit:      NewQuickEditComponent(),
		},
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 1,
		loadedProjects:    []*models.Project{alpha, beta},
		rows:              []outlinerRow{{task: loaded, ProjectID: beta.ID}},
	}

	return m, beta, "beta-task-1"
}

// crossRecurringFixture is like crossFieldFixture but beta's task has a weekly
// recurrence and a due date so skip/defer assertions are deterministic.
func crossRecurringFixture(t *testing.T) (m *OutlinerModel, beta *models.Project, betaTaskID string, baseDate time.Time) {
	t.Helper()

	m, beta, betaTaskID = crossFieldFixture(t)
	baseDate = time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)

	task := beta.GetTaskByID(betaTaskID)
	if task == nil {
		t.Fatal("beta task not found")
	}
	task.DueDate = &baseDate
	task.Recurrence = &models.RecurrencePattern{
		Unit:     models.RecurrenceWeekly,
		Interval: 1,
	}
	if saveErr := storage.SaveProject(beta); saveErr != nil {
		t.Fatalf("save recurring beta: %v", saveErr)
	}
	if loadErr := storage.LoadProject(beta); loadErr != nil {
		t.Fatalf("load recurring beta: %v", loadErr)
	}

	loaded := beta.GetTaskByID(betaTaskID)
	if loaded == nil {
		t.Fatal("recurring beta task not found after reload")
	}
	m.rows = []outlinerRow{{task: loaded, ProjectID: beta.ID}}

	return m, beta, betaTaskID, baseDate
}

func crossMultiFieldFixture(t *testing.T) (m *OutlinerModel, alpha *models.Project, beta *models.Project, alphaTaskID string, betaTaskID string) {
	t.Helper()

	alphaDir := t.TempDir()
	betaDir := t.TempDir()

	alphaTask, err := models.NewTask("alpha-task-1", 1, models.TaskTypeTask, "Alpha task")
	if err != nil {
		t.Fatalf("NewTask alpha: %v", err)
	}
	alpha = &models.Project{ID: "alpha-proj", Name: "Alpha", Path: alphaDir}
	alpha.Tasks = []*models.Task{alphaTask}
	alpha.NextShortID = 2
	alpha.BuildTaskTree()
	if saveErr := storage.SaveProject(alpha); saveErr != nil {
		t.Fatalf("save alpha: %v", saveErr)
	}

	betaTask, err := models.NewTask("beta-task-1", 1, models.TaskTypeTask, "Beta task")
	if err != nil {
		t.Fatalf("NewTask beta: %v", err)
	}
	beta = &models.Project{ID: "beta-proj", Name: "Beta", Path: betaDir}
	beta.Tasks = []*models.Task{betaTask}
	beta.NextShortID = 2
	beta.BuildTaskTree()
	if saveErr := storage.SaveProject(beta); saveErr != nil {
		t.Fatalf("save beta: %v", saveErr)
	}

	if loadErr := storage.LoadProject(alpha); loadErr != nil {
		t.Fatalf("load alpha: %v", loadErr)
	}
	if loadErr := storage.LoadProject(beta); loadErr != nil {
		t.Fatalf("load beta: %v", loadErr)
	}

	alphaLoaded := alpha.GetTaskByID("alpha-task-1")
	betaLoaded := beta.GetTaskByID("beta-task-1")
	if alphaLoaded == nil || betaLoaded == nil {
		t.Fatal("tasks not found after reload")
	}

	m = &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			projectService: NewProjectService(),
			bulkOp:         NewBulkOperationComponent(),
			quickEdit:      NewQuickEditComponent(),
		},
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState: MultiSelectState{
			multiSelectMode: true,
			selectedTasks: map[string]bool{
				alphaLoaded.ID: true,
				betaLoaded.ID:  true,
			},
		},
		activePerspective: 1,
		loadedProjects:    []*models.Project{alpha, beta},
		rows: []outlinerRow{
			{IsSectionHeader: true, SectionName: "Selected"},
			{task: alphaLoaded, ProjectID: alpha.ID},
			{task: betaLoaded, ProjectID: beta.ID},
		},
	}

	return m, alpha, beta, "alpha-task-1", "beta-task-1"
}

func TestCrossProjectOutlinerBulkTagMessageUsesSelectedProjectRefs(t *testing.T) {
	m, alpha, beta, alphaTaskID, betaTaskID := crossMultiFieldFixture(t)

	_, cmd := m.handleTaskPrefix("t", nil)
	if cmd == nil {
		t.Fatal("expected bulk tag command")
	}
	rawMsg := cmd()
	msg, ok := rawMsg.(ShowBulkTagSelectorMsg)
	if !ok {
		t.Fatalf("expected ShowBulkTagSelectorMsg, got %T", rawMsg)
	}

	assertTaskProjectRefs(t, msg.TaskRefs, []TaskProjectRef{
		{Task: alpha.GetTaskByID(alphaTaskID), Project: alpha},
		{Task: beta.GetTaskByID(betaTaskID), Project: beta},
	})
}

func TestCrossProjectOutlinerBulkDueDateUpdatesMultipleProjects(t *testing.T) {
	m, alpha, beta, alphaTaskID, betaTaskID := crossMultiFieldFixture(t)

	model, cmd := m.handleDueDatePrefixOutliner("t")
	if cmd != nil {
		t.Fatalf("expected inline bulk due-date mutation, got command %T", cmd())
	}
	m = model.(*OutlinerModel)

	today := util.StartOfDay(time.Now())
	for name, task := range map[string]*models.Task{
		"alpha": alpha.GetTaskByID(alphaTaskID),
		"beta":  beta.GetTaskByID(betaTaskID),
	} {
		if task == nil {
			t.Fatalf("%s task not found", name)
		}
		if task.DueDate == nil {
			t.Fatalf("%s due date was not set", name)
		}
		if !task.DueDate.Equal(today) {
			t.Fatalf("%s due date = %v, want %v", name, *task.DueDate, today)
		}
	}
	if m.multiSelectMode || len(m.selectedTasks) != 0 {
		t.Fatal("expected cross-project bulk due date to clear outliner selection")
	}
}

func TestCrossHandlePriorityMutation_UpdatesBetaTask(t *testing.T) {
	m, beta, taskID := crossFieldFixture(t)

	cmd, ok := m.crossHandlePriorityMutation("3")
	if !ok {
		t.Fatal("expected ok=true for key '3'")
	}
	_ = cmd

	task := beta.GetTaskByID(taskID)
	if task == nil {
		t.Fatal("beta task not found after mutation")
	}
	if task.Priority != 3 {
		t.Errorf("priority: want 3, got %d", task.Priority)
	}
}

func TestCrossHandlePriorityMutation_UnknownKey(t *testing.T) {
	m, _, _ := crossFieldFixture(t)

	_, ok := m.crossHandlePriorityMutation("x")
	if ok {
		t.Error("expected ok=false for unknown key 'x'")
	}
}

func TestCrossHandleDueDateMutation_SetsToday(t *testing.T) {
	m, beta, taskID := crossFieldFixture(t)

	cmd, ok := m.crossHandleDueDateMutation("t")
	if !ok {
		t.Fatal("expected ok=true for key 't'")
	}
	_ = cmd

	task := beta.GetTaskByID(taskID)
	if task == nil {
		t.Fatal("beta task not found after mutation")
	}
	if task.DueDate == nil {
		t.Fatal("expected DueDate to be set after 'dt'")
	}
	today := util.StartOfDay(time.Now())
	if !task.DueDate.Equal(today) {
		t.Errorf("DueDate: want %v, got %v", today, *task.DueDate)
	}
}

func TestCrossHandleDueDateMutation_DatePickerEmitsMsg(t *testing.T) {
	m, _, _ := crossFieldFixture(t)

	cmd, ok := m.crossHandleDueDateMutation("d")
	if !ok {
		t.Fatal("expected ok=true for 'dd' (date picker)")
	}
	if cmd == nil {
		t.Fatal("expected cmd to be non-nil for date picker")
	}
	msg := cmd()
	if _, isDateMsg := msg.(SetDueDateMsg); !isDateMsg {
		t.Errorf("expected SetDueDateMsg, got %T", msg)
	}
}

func TestCrossHandleSnoozeMutation_SetsNextWeek(t *testing.T) {
	m, beta, taskID := crossFieldFixture(t)

	cmd, ok := m.crossHandleSnoozeMutation("w")
	if !ok {
		t.Fatal("expected ok=true for snooze key 'w'")
	}
	_ = cmd

	task := beta.GetTaskByID(taskID)
	if task == nil {
		t.Fatal("beta task not found after mutation")
	}
	if task.SnoozedUntil == nil {
		t.Fatal("expected SnoozedUntil to be set")
	}
	expected := util.StartOfDay(time.Now()).AddDate(0, 0, 7)
	if !task.SnoozedUntil.Equal(expected) {
		t.Errorf("SnoozedUntil: want %v, got %v", expected, *task.SnoozedUntil)
	}
}

func TestCrossHandleSnoozeMutation_PickerEmitsMsg(t *testing.T) {
	m, _, _ := crossFieldFixture(t)

	cmd, ok := m.crossHandleSnoozeMutation("d")
	if !ok {
		t.Fatal("expected ok=true for snooze picker 'sd'")
	}
	if cmd == nil {
		t.Fatal("expected cmd to be non-nil for snooze picker")
	}
	msg := cmd()
	if _, isSnoozeMsg := msg.(SetSnoozeDateMsg); !isSnoozeMsg {
		t.Errorf("expected SetSnoozeDateMsg, got %T", msg)
	}
}

func TestCrossHandleSkipRecurrence_AdvancesDueDate(t *testing.T) {
	m, beta, taskID, baseDate := crossRecurringFixture(t)

	cmd, ok := m.crossHandleSkipRecurrence()
	if !ok {
		t.Fatal("expected ok=true")
	}
	_ = cmd

	task := beta.GetTaskByID(taskID)
	if task == nil {
		t.Fatal("beta task not found after skip")
	}
	if task.DueDate == nil {
		t.Fatal("expected DueDate after skip recurrence")
	}
	expected := baseDate.AddDate(0, 0, 7)
	if !task.DueDate.Equal(expected) {
		t.Errorf("DueDate after skip: want %v, got %v", expected, *task.DueDate)
	}
}

func TestCrossHandleSkipRecurrence_NonRecurring_NoMutation(t *testing.T) {
	m, beta, taskID := crossFieldFixture(t)

	beforeTask := beta.GetTaskByID(taskID)
	if beforeTask == nil {
		t.Fatal("beta task not found before skip")
	}
	before := beforeTask.UpdatedAt

	cmd, ok := m.crossHandleSkipRecurrence()
	if !ok {
		t.Fatal("expected ok=true even for non-recurring task")
	}
	_ = cmd

	if m.statusMessage != "Task is not recurring" {
		t.Errorf("status: want %q, got %q", "Task is not recurring", m.statusMessage)
	}

	task := beta.GetTaskByID(taskID)
	if task == nil {
		t.Fatal("beta task not found after skip")
	}
	if task.UpdatedAt != before {
		t.Error("expected no mutation for non-recurring task")
	}
}

func TestCrossHandleDeferRecurrence_DefersByWeek(t *testing.T) {
	m, beta, taskID, baseDate := crossRecurringFixture(t)

	cmd, ok := m.crossHandleDeferRecurrence("w")
	if !ok {
		t.Fatal("expected ok=true for defer key 'w'")
	}
	_ = cmd

	task := beta.GetTaskByID(taskID)
	if task == nil {
		t.Fatal("beta task not found after defer")
	}
	if task.DueDate == nil {
		t.Fatal("expected DueDate after defer recurrence")
	}
	expected := baseDate.AddDate(0, 0, 7)
	if !task.DueDate.Equal(expected) {
		t.Errorf("DueDate after defer: want %v, got %v", expected, *task.DueDate)
	}
}

func TestCrossHandleDeferRecurrence_UnknownKey(t *testing.T) {
	m, _, _, _ := crossRecurringFixture(t)

	_, ok := m.crossHandleDeferRecurrence("x")
	if ok {
		t.Error("expected ok=false for unknown defer key 'x'")
	}
}
