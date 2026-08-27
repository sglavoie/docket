package app_test

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TestRecurrenceIsAbsentFromLiveTypes locks the removal at every application
// boundary that could otherwise reintroduce recurrence into a write or result.
func TestRecurrenceIsAbsentFromLiveTypes(t *testing.T) {
	types := []struct {
		name  string
		value any
	}{
		{"task", models.Task{}},
		{"add request", app.AddTaskRequest{}},
		{"task result", app.TaskResult{}},
	}
	for _, item := range types {
		typ := reflect.TypeOf(item.value)
		if _, ok := typ.FieldByName("Recurrence"); ok {
			t.Errorf("%s exposes Recurrence", item.name)
		}
		if _, ok := typ.FieldByName("RecurringGenerated"); ok {
			t.Errorf("%s exposes RecurringGenerated", item.name)
		}
	}

	for _, method := range []string{
		"SetRecurrence", "ClearRecurrence", "SkipRecurrence", "DeferRecurrence",
		"NextDueDate", "NextDueDateAfter", "NextNDueDates", "IsRecurring",
	} {
		if _, ok := reflect.TypeOf(&models.Task{}).MethodByName(method); ok {
			t.Errorf("live task still exposes %s", method)
		}
		if _, ok := reflect.TypeOf(&app.Service{}).MethodByName(method); ok {
			t.Errorf("application service still exposes %s", method)
		}
	}
	for _, method := range []string{"GenerateRecurringInstances", "GenerateRecurringInstancesAt", "GenerateRecurringInstancesAtWithID"} {
		if _, ok := reflect.TypeOf(&models.Project{}).MethodByName(method); ok {
			t.Errorf("live project still exposes %s", method)
		}
	}
}

func TestCompleteFinishesOnlySelectedTask(t *testing.T) {
	task := newTask("task", 1, "Finish once")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := svc.Transition(app.StateRequest{Locator: repo.loc(task.ID), Target: models.StateDone}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if task.State() != models.StateDone {
		t.Fatalf("state = %q, want done", task.State())
	}
	if len(project.Tasks) != 1 || project.Tasks[0] != task {
		t.Fatalf("completion changed task identity set: %+v", project.Tasks)
	}
}

// rootOrder reads the root-task IDs in slice order.
func rootOrder(p *models.Project) []string {
	var ids []string
	for _, t := range p.Tasks {
		if t.IsRoot() {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

func assertOrder(t *testing.T, p *models.Project, want ...string) {
	t.Helper()
	got := rootOrder(p)
	if len(got) != len(want) {
		t.Fatalf("root order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("root order = %v, want %v", got, want)
		}
	}
}

// TestMoveTaskToTopBubblesToFront proves MoveTaskToTop moves a task to
// the top of its siblings across multiple positions.
func TestMoveTaskToTopBubblesToFront(t *testing.T) {
	a := newTask("a", 1, "A")
	b := newTask("b", 2, "B")
	c := newTask("c", 3, "C")
	proj := newProject("proj-a", "Alpha", a, b, c)
	repo := newFakeRepo(proj)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskToTop(app.TaskLocator{ProjectID: proj.ID, TaskID: "c"}); err != nil {
		t.Fatalf("MoveTaskToTop: %v", err)
	}
	assertOrder(t, proj, "c", "a", "b")
}

// TestMoveTaskToBottomBubblesToEnd proves MoveTaskToBottom moves a task
// to the bottom of its siblings across multiple positions.
func TestMoveTaskToBottomBubblesToEnd(t *testing.T) {
	a := newTask("a", 1, "A")
	b := newTask("b", 2, "B")
	c := newTask("c", 3, "C")
	proj := newProject("proj-a", "Alpha", a, b, c)
	repo := newFakeRepo(proj)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskToBottom(app.TaskLocator{ProjectID: proj.ID, TaskID: "a"}); err != nil {
		t.Fatalf("MoveTaskToBottom: %v", err)
	}
	assertOrder(t, proj, "b", "c", "a")
}

// TestMoveTaskToTopAlreadyAtTopIsIdempotent proves a task already at the top
// commits unchanged rather than erroring (unlike MoveTaskUp, which rejects it).
func TestMoveTaskToTopAlreadyAtTopIsIdempotent(t *testing.T) {
	a := newTask("a", 1, "A")
	b := newTask("b", 2, "B")
	proj := newProject("proj-a", "Alpha", a, b)
	repo := newFakeRepo(proj)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskToTop(app.TaskLocator{ProjectID: proj.ID, TaskID: "a"}); err != nil {
		t.Fatalf("MoveTaskToTop (already at top): %v", err)
	}
	assertOrder(t, proj, "a", "b")
	if len(repo.commits) != 1 {
		t.Errorf("commits = %d, want 1 (idempotent still commits once)", len(repo.commits))
	}
}

// TestMoveTaskToTopStopsAtCompletionBoundary proves a completed root task cannot
// bubble above an uncompleted root sibling.
func TestMoveTaskToTopStopsAtCompletionBoundary(t *testing.T) {
	a := newTask("a", 1, "A")
	b := newTask("b", 2, "B")
	b.Complete()
	proj := newProject("proj-a", "Alpha", a, b)
	repo := newFakeRepo(proj)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskToTop(app.TaskLocator{ProjectID: proj.ID, TaskID: "b"}); err != nil {
		t.Fatalf("MoveTaskToTop: %v", err)
	}
	// b (completed) cannot cross above a (uncompleted): order stays a, b.
	assertOrder(t, proj, "a", "b")
}
