package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/testutil"
	"github.com/sglavoie/docket/internal/tui/coordinator"
)

func dueClockTestTask(t *testing.T, project *models.Project, title string, due *time.Time) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(title+"-id", project.NextShortID, title)
	if err != nil {
		t.Fatalf("build task %q: %v", title, err)
	}
	task.DueDate = due
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	return task
}

func dueClockLocator(project *models.Project, task *models.Task) app.TaskLocator {
	return app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
}

func clearedDueStep(target app.TaskLocator, hour, minute int) app.DueStepOutcome {
	moment := time.Date(2026, 9, 11, hour, minute, 0, 0, time.Local)
	return app.DueStepOutcome{Target: target, Before: &moment}
}

// A reload answers only for the tasks the shell can actually read. A task that
// regained a deadline retires its clock; a task still without one keeps it; and a
// task in a project this session never loaded — parked, archived, or simply not
// resident — says nothing about itself and keeps what was remembered.
func TestReconcileRememberedDueClocksReadsOnlyObservableTasks(t *testing.T) {
	shell := newTestAppModel()
	loaded := testutil.NewTestProject("Loaded")
	unloaded := testutil.NewTestProject("Unloaded")

	deadline := time.Date(2026, 9, 12, 9, 0, 0, 0, time.Local)
	external := dueClockTestTask(t, loaded, "external", &deadline)
	stillUnset := dueClockTestTask(t, loaded, "still-unset", nil)
	unavailable := dueClockTestTask(t, unloaded, "unavailable", nil)

	setTestAppProjects(shell, loaded)

	externalAt := dueClockLocator(loaded, external)
	stillUnsetAt := dueClockLocator(loaded, stillUnset)
	unavailableAt := dueClockLocator(unloaded, unavailable)

	shell.dueClocks.ObserveCommit("t1", coordinator.CommittedDueEffect{Steps: []app.DueStepOutcome{
		clearedDueStep(externalAt, 8, 30),
		clearedDueStep(stillUnsetAt, 9, 45),
		clearedDueStep(unavailableAt, 10, 15),
	}})

	shell.reconcileRememberedDueClocks()

	remembered := shell.dueClocks.RememberedClocks(nil)
	if _, ok := remembered[externalAt]; ok {
		t.Fatalf("a task that regained a deadline kept its remembered clock")
	}
	if clock := remembered[stillUnsetAt]; clock.Hour != 9 || clock.Minute != 45 {
		t.Fatalf("an undeferred unset task lost its clock: %+v", clock)
	}
	if clock := remembered[unavailableAt]; clock.Hour != 10 || clock.Minute != 15 {
		t.Fatalf("an unloaded task lost its clock: %+v", clock)
	}
}

// An explicit due token is an override even when the edit committed nothing,
// which is the one committed-edit notification a resulting deadline cannot carry
// on its own.
func TestTaskLineEditDueEffectReportsIntentNotOutcome(t *testing.T) {
	target := app.TaskLocator{ProjectID: "alpha", TaskID: "task-1"}
	moved := app.TaskLocator{ProjectID: "beta", TaskID: "task-1"}

	explicit := taskLineEditDueEffect(app.TaskLineEditResult{Task: target, DueEdited: true})
	if len(explicit.Edits) != 1 || explicit.Edits[0] != target {
		t.Fatalf("an explicit due token named %v", explicit.Edits)
	}

	titleOnly := taskLineEditDueEffect(app.TaskLineEditResult{Task: target, Changed: true})
	if len(titleOnly.Edits) != 0 {
		t.Fatalf("a title-only edit named %v", titleOnly.Edits)
	}

	crossProject := taskLineEditDueEffect(app.TaskLineEditResult{
		Task:  moved,
		Moves: []app.TaskAddressChange{{From: target, To: moved}},
	})
	if len(crossProject.Moves) != 1 || crossProject.Moves[0].To != moved {
		t.Fatalf("a cross-project edit reported moves %v", crossProject.Moves)
	}
}

// The outline buffer restates every deadline, so only a changed one is an
// explicit instruction; its removals are addresses nothing may reattach to.
func TestOutlinePlanDueEffectCarriesEditsAndDeletions(t *testing.T) {
	edited := app.TaskLocator{ProjectID: "alpha", TaskID: "task-1"}
	effect := outlinePlanDueEffect(app.OutlinePlanResult{
		DueEdits: []app.TaskLocator{edited},
		Deleted:  []app.TaskResult{{ProjectID: "alpha", TaskID: "task-2"}},
		Updated:  []app.TaskResult{{ProjectID: "alpha", TaskID: "task-3"}},
	})
	if len(effect.Edits) != 1 || effect.Edits[0] != edited {
		t.Fatalf("outline edits were %v", effect.Edits)
	}
	if len(effect.Removed) != 1 || effect.Removed[0].TaskID != "task-2" {
		t.Fatalf("outline removals were %v", effect.Removed)
	}
	if len(effect.Moves) != 0 {
		t.Fatalf("an outline buffer never crosses a project boundary, got %v", effect.Moves)
	}
}

// The no-op edit path returns before any reload, so the override has to be
// folded in ahead of it.
func TestHandleTaskLineEditedFoldsUnchangedExplicitDueEdit(t *testing.T) {
	shell := newTestAppModel()
	project := testutil.NewTestProject("Loaded")
	task := dueClockTestTask(t, project, "unset", nil)
	setTestAppProjects(shell, project)
	target := dueClockLocator(project, task)

	shell.dueClocks.ObserveCommit("t1", coordinator.CommittedDueEffect{
		Steps: []app.DueStepOutcome{clearedDueStep(target, 13, 20)},
	})

	shell.handleTaskLineEdited(coordinator.TaskLineEditedMsg{
		Result: app.TaskLineEditResult{Task: target, DueEdited: true, Changed: false},
	})

	if _, ok := shell.dueClocks.RememberedClocks(nil)[target]; ok {
		t.Fatalf("an explicit clear of an already-absent deadline left the clock remembered")
	}
}

// A single-target move commits as one task-line edit, so its address map is what
// carries the remembered clock into the destination project.
func TestHandleTaskLineEditedRekeysClockAcrossAProjectMove(t *testing.T) {
	shell := newTestAppModel()
	source := testutil.NewTestProject("Source")
	destination := testutil.NewTestProject("Destination")
	task := dueClockTestTask(t, source, "moving", nil)
	setTestAppProjects(shell, source, destination)

	sourceAt := dueClockLocator(source, task)
	destinationAt := app.TaskLocator{ProjectID: destination.ID, TaskID: task.ID}

	shell.dueClocks.ObserveCommit("t1", coordinator.CommittedDueEffect{
		Steps: []app.DueStepOutcome{clearedDueStep(sourceAt, 7, 5)},
	})

	shell.handleTaskLineEdited(coordinator.TaskLineEditedMsg{
		Result: app.TaskLineEditResult{
			Task: destinationAt, Changed: true, TransactionID: "t2",
			AffectedProjectIDs: []string{source.ID, destination.ID},
			Moves:              []app.TaskAddressChange{{From: sourceAt, To: destinationAt}},
		},
	})

	remembered := shell.dueClocks.RememberedClocks(nil)
	if _, ok := remembered[sourceAt]; ok {
		t.Fatalf("the source address kept a clock the task took with it")
	}
	if clock := remembered[destinationAt]; clock.Hour != 7 || clock.Minute != 5 {
		t.Fatalf("the moved task lost its remembered clock: %+v", clock)
	}
}

// A committed edit that never mentioned a deadline leaves the memory alone, and
// the reload behind it does not clear it either.
func TestHandleTaskLineEditedKeepsClockThroughUnrelatedEdit(t *testing.T) {
	shell := newTestAppModel()
	project := testutil.NewTestProject("Loaded")
	task := dueClockTestTask(t, project, "unset", nil)
	setTestAppProjects(shell, project)
	target := dueClockLocator(project, task)

	shell.dueClocks.ObserveCommit("t1", coordinator.CommittedDueEffect{
		Steps: []app.DueStepOutcome{clearedDueStep(target, 13, 20)},
	})

	shell.handleTaskLineEdited(coordinator.TaskLineEditedMsg{
		Result: app.TaskLineEditResult{
			Task: target, Changed: true, TransactionID: "t2",
			AffectedProjectIDs: []string{project.ID},
		},
	})

	if clock := shell.dueClocks.RememberedClocks(nil)[target]; clock.Hour != 13 || clock.Minute != 20 {
		t.Fatalf("a title-only edit disturbed the remembered clock: %+v", clock)
	}
}
