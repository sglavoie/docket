package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func newTestProjectFocusView(t *testing.T) (*ProjectFocusViewModel, *models.Project) {
	t.Helper()
	cfg := newTestConfig()
	project := testutil.NewTestProject("focus-test")

	// Add a Focus Queue task with a due date.
	task := mustNewTestTask(project, "Active task with due")
	due := time.Now().Add(24 * time.Hour)
	task.DueDate = &due
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()
	cfg.Planning().SetFocusQueue([]config.FocusQueueItem{{TaskID: task.ID, ProjectID: project.ID}})

	m := NewProjectFocusViewModel(project, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(80, 24)
	return m, project
}

func mustNewTestTask(project *models.Project, title string) *models.Task {
	task, err := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeTask, title)
	if err != nil {
		panic(err)
	}
	project.NextShortID++
	return task
}

func TestProjectFocusViewModel_GetRuntimeNotNil(t *testing.T) {
	m, _ := newTestProjectFocusView(t)

	if m.getRuntime() == nil {
		t.Fatal("getRuntime() returned nil; expected non-nil *config.RuntimeState")
	}
}

func TestProjectFocusViewModel_RenderWithDueDate_NoPanic(t *testing.T) {
	m, _ := newTestProjectFocusView(t)

	// Rendering with a domain-backed runtime must remain safe for due dates.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked: %v", r)
		}
	}()
	_ = m.View()
}

func TestProjectFocusViewModel_RendersQueueIndicatorWithoutActiveMarker(t *testing.T) {
	m, _ := newTestProjectFocusView(t)
	if len(m.tasks) != 1 {
		t.Fatalf("expected one queued task, got %d", len(m.tasks))
	}

	line := ansi.Strip(m.renderTaskLine(m.tasks[0], false, 40, nil, 0))
	if !strings.Contains(line, "q ") {
		t.Fatalf("project Focus Queue row should render q, got %q", line)
	}
	if strings.Contains(line, "*") {
		t.Fatalf("project Focus Queue row should not render an active marker, got %q", line)
	}
}

func TestProjectFocusViewModel_LoadTasks_FiltersCorrectly(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("filter-test")

	queued := mustNewTestTask(project, "Queued")
	project.Tasks = append(project.Tasks, queued)

	notQueued := mustNewTestTask(project, "Not queued")
	project.Tasks = append(project.Tasks, notQueued)

	completed := mustNewTestTask(project, "Completed")
	completed.WorkflowState = models.StateDone
	project.Tasks = append(project.Tasks, completed)

	snoozed := mustNewTestTask(project, "Snoozed")
	snoozeDate := time.Now().Add(48 * time.Hour)
	snoozed.SnoozedUntil = &snoozeDate
	project.Tasks = append(project.Tasks, snoozed)

	project.BuildTaskTree()
	// Only queue members are included; completed and snoozed members are filtered out.
	cfg.Planning().SetFocusQueue([]config.FocusQueueItem{
		{TaskID: queued.ID, ProjectID: project.ID},
		{TaskID: completed.ID, ProjectID: project.ID},
		{TaskID: snoozed.ID, ProjectID: project.ID},
	})

	m := NewProjectFocusViewModel(project, testRuntimeAt(cfg.DataDir(), cfg))

	if len(m.tasks) != 1 {
		t.Errorf("expected 1 queued non-complete non-snoozed task, got %d", len(m.tasks))
	}
	if len(m.tasks) > 0 && m.tasks[0].Title != "Queued" {
		t.Errorf("expected task title 'Queued', got %q", m.tasks[0].Title)
	}
}

func TestProjectFocusViewModel_EmptyRuntimeConstructor(t *testing.T) {
	// An empty runtime should not panic during construction.
	project := testutil.NewTestProject("nil-cfg")
	m := NewProjectFocusViewModel(project, config.EmptyRuntimeState(""))

	if m.getRuntime() == nil {
		t.Error("expected getRuntime() to return the supplied empty runtime")
	}
}

func TestProjectFocusViewModel_FilterMachineAvailable(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("stages-test")
	task := mustNewTestTask(project, "Staged task")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	m := NewProjectFocusViewModel(project, testRuntimeAt(cfg.DataDir(), cfg))

	if m.getFilterMachine() == nil {
		t.Fatal("expected domain-backed filter machine")
	}
}

func TestProjectFocusViewModel_SharedFilters_AffectVisibleTasks(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("filter-shared")

	plain := mustNewTestTask(project, "Plain task")
	plain.Priority = 4
	project.Tasks = append(project.Tasks, plain)

	dimmed := mustNewTestTask(project, "Dimmed task")
	dimmed.Dimmed = true
	project.Tasks = append(project.Tasks, dimmed)

	highPriority := mustNewTestTask(project, "High priority")
	highPriority.Priority = 1
	project.Tasks = append(project.Tasks, highPriority)

	bug, err := models.NewTask("bug-id", project.NextShortID, models.TaskTypeBug, "Bug task")
	if err != nil {
		t.Fatalf("NewTask bug: %v", err)
	}
	project.NextShortID++
	project.Tasks = append(project.Tasks, bug)

	tagged := mustNewTestTask(project, "Tagged task")
	tagged.Tags = []string{"focus"}
	project.Tasks = append(project.Tasks, tagged)

	staged := mustNewTestTask(project, "Staged task")
	project.Tasks = append(project.Tasks, staged)

	project.BuildTaskTree()
	queue := make([]config.FocusQueueItem, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		queue = append(queue, config.FocusQueueItem{TaskID: task.ID, ProjectID: project.ID})
	}
	cfg.Planning().SetFocusQueue(queue)
	m := NewProjectFocusViewModel(project, testRuntimeAt(cfg.DataDir(), cfg))

	if got := len(m.filteredTasks()); got != 6 {
		t.Fatalf("baseline filteredTasks() = %d, want 6", got)
	}

	// Fh: hide dimmed.
	m.filter.HideDimmed = true
	if got := len(m.filteredTasks()); got != 5 {
		t.Errorf("HideDimmed filteredTasks() = %d, want 5", got)
	}
	for _, task := range m.filteredTasks() {
		if task.Dimmed {
			t.Errorf("HideDimmed should hide dimmed task %q", task.Title)
		}
	}
	clearCommonFilters(&m.filter)

	// Fp: priority filter shows high-priority only (priority <= 2).
	m.filter.Priority = 1
	tasks := m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "High priority" {
		t.Errorf("Priority=1 filteredTasks() = %v, want only 'High priority'", taskTitles(tasks))
	}
	clearCommonFilters(&m.filter)

	// Ft: type filter (bug).
	m.filter.Type = models.TaskTypeBug
	tasks = m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "Bug task" {
		t.Errorf("Type=bug filteredTasks() = %v, want only 'Bug task'", taskTitles(tasks))
	}
	clearCommonFilters(&m.filter)

	// F#: tag filter.
	m.filter.Tags = map[string]bool{"focus": true}
	tasks = m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "Tagged task" {
		t.Errorf("Tags={focus} filteredTasks() = %v, want only 'Tagged task'", taskTitles(tasks))
	}
	clearCommonFilters(&m.filter)

	// Fx (clear) returns the full set.
	if got := len(m.filteredTasks()); got != 6 {
		t.Errorf("after clear filteredTasks() = %d, want 6", got)
	}
}

func TestProjectFocusViewModel_FilterHandlers_DispatchThroughKeymap(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("filter-dispatch")

	plain := mustNewTestTask(project, "Plain")
	project.Tasks = append(project.Tasks, plain)

	dimmed := mustNewTestTask(project, "Dimmed")
	dimmed.Dimmed = true
	project.Tasks = append(project.Tasks, dimmed)

	project.BuildTaskTree()
	cfg.Planning().SetFocusQueue([]config.FocusQueueItem{
		{TaskID: plain.ID, ProjectID: project.ID},
		{TaskID: dimmed.ID, ProjectID: project.ID},
	})
	m := NewProjectFocusViewModel(project, testRuntimeAt(cfg.DataDir(), cfg))

	if got := len(m.filteredTasks()); got != 2 {
		t.Fatalf("baseline filteredTasks() = %d, want 2", got)
	}

	// Activate Fh through the same dispatch path used by Update().
	if _, _ = m.handleFilterPrefix("h"); m.filter.HideDimmed != true {
		t.Fatalf("handleFilterPrefix(\"h\") did not toggle HideDimmed; got %v", m.filter.HideDimmed)
	}
	if got := len(m.filteredTasks()); got != 1 {
		t.Errorf("after Fh filteredTasks() = %d, want 1", got)
	}

	// Fx clears.
	if _, _ = m.handleFilterPrefix("x"); m.filter.HideDimmed != false {
		t.Errorf("handleFilterPrefix(\"x\") did not clear HideDimmed; got %v", m.filter.HideDimmed)
	}
	if got := len(m.filteredTasks()); got != 2 {
		t.Errorf("after Fx filteredTasks() = %d, want 2", got)
	}
}

func taskTitles(tasks []*models.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.Title
	}
	return out
}
