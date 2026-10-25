package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/testutil"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// externalDeadlineSnapshot copies one loaded project and gives the named task
// the deadline an external write would have left on it, which is what a reload
// finds on disk. The copy is deep enough to be a stranger to the shell's own
// tree: the snapshot a load delivers was read from storage and shares no task
// pointer with the cache it is about to replace.
func externalDeadlineSnapshot(
	shell *AppModel, projectID, taskID string, due time.Time,
) *models.Project {
	resident := shell.projectByID(projectID)
	snapshot := *resident
	snapshot.Tasks = make([]*models.Task, len(resident.Tasks))
	for index, original := range resident.Tasks {
		task := *original
		if task.ID == taskID {
			deadline := due
			task.DueDate = &deadline
		}
		snapshot.Tasks[index] = &task
	}
	snapshot.BuildTaskTree()
	return &snapshot
}

// An accepted external population is the one way a deadline reaches the shell
// with no committed effect of ours to announce it, so adoption is where the
// clock it superseded has to retire. Without that, the next + would hand back a
// 14:30 the workspace no longer agrees with.
//
// The controls ride along on the same adoption: a task the snapshot still shows
// unset, and a task in a project this session never loaded, both keep what was
// remembered for them.
func TestAcceptedExternalSnapshotRetiresRememberedDueClock(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.timedID)
	env.step(t, "-")

	shell := env.driver.App()
	timedAt := app.TaskLocator{ProjectID: env.projectID, TaskID: env.timedID}
	if clock, ok := shell.dueClocks.RememberedClocks(nil)[timedAt]; !ok || clock.Hour != 14 || clock.Minute != 30 {
		t.Fatalf("the keyboard clear remembered %+v, want 14:30", clock)
	}

	stillUnsetAt := app.TaskLocator{ProjectID: env.projectID, TaskID: env.noneID}
	unavailableAt := app.TaskLocator{ProjectID: "never-loaded", TaskID: "absent"}
	shell.dueClocks.ObserveCommit("controls", coordinator.CommittedDueEffect{Steps: []app.DueStepOutcome{
		clearedDueStep(stillUnsetAt, 9, 45),
		clearedDueStep(unavailableAt, 10, 15),
	}})

	external := dueStepDay(2, 0, 0)
	handled, _ := shell.handleGlobalLifecycleMessage(outliner.CrossProjectsLoadedMsg{
		Projects:   []*models.Project{externalDeadlineSnapshot(shell, env.projectID, env.timedID, external)},
		Generation: shell.crossOutliner.Population.LoadGen,
	})
	if !handled {
		t.Fatal("the shell did not handle the external population")
	}
	if got := env.task(t, env.timedID).DueDate; got == nil || !got.Equal(external) {
		t.Fatalf("adopted deadline = %v, want %v", got, external)
	}

	remembered := shell.dueClocks.RememberedClocks(nil)
	if clock, ok := remembered[timedAt]; ok {
		t.Fatalf("an externally set deadline left its superseded clock remembered: %+v", clock)
	}
	if clock := remembered[stillUnsetAt]; clock.Hour != 9 || clock.Minute != 45 {
		t.Fatalf("a task the snapshot still shows unset lost its clock: %+v", clock)
	}
	if clock := remembered[unavailableAt]; clock.Hour != 10 || clock.Minute != 15 {
		t.Fatalf("a task this session never loaded lost its clock: %+v", clock)
	}
}

// A superseded load is not an observation. It installs nothing, so it may not
// retire anything either: the population still on its way is the one that gets
// to speak for the workspace.
func TestRejectedExternalSnapshotKeepsRememberedDueClock(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.timedID)
	env.step(t, "-")

	shell := env.driver.App()
	timedAt := app.TaskLocator{ProjectID: env.projectID, TaskID: env.timedID}

	stale := externalDeadlineSnapshot(shell, env.projectID, env.timedID, dueStepDay(2, 0, 0))
	handled, _ := shell.handleGlobalLifecycleMessage(outliner.CrossProjectsLoadedMsg{
		Projects:   []*models.Project{stale},
		Generation: shell.crossOutliner.Population.LoadGen - 1,
	})
	if !handled {
		t.Fatal("the shell did not handle the stale population")
	}
	if got := env.task(t, env.timedID).DueDate; got != nil {
		t.Fatalf("a rejected snapshot installed the deadline %v", got)
	}
	if clock, ok := shell.dueClocks.RememberedClocks(nil)[timedAt]; !ok || clock.Hour != 14 || clock.Minute != 30 {
		t.Fatalf("a rejected snapshot retired the clock: %+v", clock)
	}
}

// Reconciliation hangs off the adoption itself rather than off either message
// that reaches it, which is what makes the published-lens population obey the
// same rule as the cross-project load without a second copy of the check.
func TestAdoptCrossProjectSnapshotReconcilesWhicheverLoadDeliveredIt(t *testing.T) {
	shell := newTestAppModel()
	project := testutil.NewTestProject("Loaded")
	external := dueClockTestTask(t, project, "external", nil)
	stillUnset := dueClockTestTask(t, project, "still-unset", nil)
	setTestAppProjects(shell, project)

	externalAt := dueClockLocator(project, external)
	stillUnsetAt := dueClockLocator(project, stillUnset)
	shell.dueClocks.ObserveCommit("t1", coordinator.CommittedDueEffect{Steps: []app.DueStepOutcome{
		clearedDueStep(externalAt, 8, 30),
		clearedDueStep(stillUnsetAt, 9, 45),
	}})

	deadline := time.Date(2026, 9, 12, 0, 0, 0, 0, time.Local)
	shell.adoptCrossProjectSnapshot([]*models.Project{
		externalDeadlineSnapshot(shell, project.ID, external.ID, deadline),
	})

	remembered := shell.dueClocks.RememberedClocks(nil)
	if clock, ok := remembered[externalAt]; ok {
		t.Fatalf("the adopted deadline left its superseded clock remembered: %+v", clock)
	}
	if clock := remembered[stillUnsetAt]; clock.Hour != 9 || clock.Minute != 45 {
		t.Fatalf("an untouched task lost its clock: %+v", clock)
	}
}
