package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
)

const dueStepSettleTimeout = 2 * time.Second

// dueStepKeyEnv is the fixture the deadline-stepping keyflows share. One task
// carries a wall clock, one carries a date-only deadline, one carries none, and
// a parent/child pair supplies the member a fold can hide: folding the parent
// takes the child off the screen without touching a selection naming it.
type dueStepKeyEnv struct {
	driver    *UITestDriver
	projectID string
	timedID   string
	plainID   string
	noneID    string
	parentID  string
	childID   string
	doneID    string
}

// dueStepDay is one local calendar day relative to today, at an explicit wall
// clock. Midnight is the durable date-only spelling, so hour and minute zero is
// how a fixture writes a deadline that carries no time.
func dueStepDay(offset, hour, minute int) time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()+offset, hour, minute, 0, 0, now.Location())
}

func newDueStepKeyEnv(t *testing.T) *dueStepKeyEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var timedID, plainID, noneID, parentID, childID, doneID string
	project, err := workspace.addProject("Deadline Stepping", func(project *models.Project) {
		timed := newFixtureTask(project, "Timed deadline")
		timed.ApplyWorkflowState(models.StateBacklog)
		due := dueStepDay(0, 14, 30)
		timed.DueDate = &due
		timedID = timed.ID

		plain := newFixtureTask(project, "Date only deadline")
		plain.ApplyWorkflowState(models.StateBacklog)
		plainDue := dueStepDay(2, 0, 0)
		plain.DueDate = &plainDue
		plainID = plain.ID

		none := newFixtureTask(project, "No deadline at all")
		none.ApplyWorkflowState(models.StateBacklog)
		noneID = none.ID

		parent := newFixtureTask(project, "Parent of the hideable child")
		parent.ApplyWorkflowState(models.StateBacklog)
		parentID = parent.ID

		child := newFixtureTask(project, "Hideable child")
		child.ApplyWorkflowState(models.StateBacklog)
		child.ParentID = &parent.ID
		childDue := dueStepDay(4, 0, 0)
		child.DueDate = &childDue
		childID = child.ID

		// A completed task due tomorrow is one backward step away from today,
		// which is the planning move a terminal task has no right to make. It is
		// stored rather than completed through the keyboard because the open
		// projection stops showing it the moment it is done.
		done := newFixtureTask(project, "Finished but due tomorrow")
		done.ApplyWorkflowState(models.StateDone)
		doneDue := dueStepDay(1, 0, 0)
		done.DueDate = &doneDue
		doneID = done.ID
	})
	if err != nil {
		t.Fatalf("add project: %v", err)
	}
	workspace.setCurrentProject(project.ID)
	driver, err := workspace.launch(100, 35)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	driver.SetDrainTimeout(750 * time.Millisecond)
	return &dueStepKeyEnv{
		driver:    driver,
		projectID: project.ID,
		timedID:   timedID,
		plainID:   plainID,
		noneID:    noneID,
		parentID:  parentID,
		childID:   childID,
		doneID:    doneID,
	}
}

// typeText feeds a query one rune at a time, which is what a search input reads;
// SendKey spells one key and a query is characters.
func (env *dueStepKeyEnv) typeText(text string) {
	for _, r := range text {
		env.driver.dispatch(tea.KeyPressMsg{Code: r, Text: string(r)})
		env.driver.drain()
	}
}

func (env *dueStepKeyEnv) press(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := env.driver.SendKey(key); err != nil {
			t.Fatalf("send key %q: %v", key, err)
		}
		env.driver.drain()
	}
}

// step presses one deadline key and waits for the transaction it starts, so an
// assertion reads committed state rather than a command still in flight.
func (env *dueStepKeyEnv) step(t *testing.T, key string) {
	t.Helper()
	env.press(t, key)
	env.driver.Wait(dueStepSettleTimeout)
}

func (env *dueStepKeyEnv) focusTask(t *testing.T, taskID string) {
	t.Helper()
	// A launch folds All's Backlog and Someday lanes, so a miss opens every fold and walks again.
	for attempt := range 2 {
		if attempt > 0 {
			env.driver.App().crossOutliner.ClearAllFolds()
		}
		env.press(t, "home")
		for range len(env.driver.App().crossOutliner.Rows) + 1 {
			if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == taskID {
				return
			}
			env.press(t, "down")
		}
	}
	t.Fatalf("the cursor never reached task %s", taskID)
}

func (env *dueStepKeyEnv) task(t *testing.T, taskID string) *models.Task {
	t.Helper()
	project := env.driver.App().projectByID(env.projectID)
	if project == nil {
		t.Fatalf("project %s is no longer loaded", env.projectID)
	}
	task := project.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("task %s is no longer loaded", taskID)
	}
	return task
}

// due reads one task's committed deadline as a comparable local string, so an
// assertion names the date and clock a user would read rather than an instant.
func (env *dueStepKeyEnv) due(t *testing.T, taskID string) string {
	t.Helper()
	task := env.task(t, taskID)
	if task.DueDate == nil {
		return ""
	}
	if task.DueHasTime() {
		return task.DueDate.Format("2006-01-02 15:04")
	}
	return task.DueDate.Format("2006-01-02")
}

func (env *dueStepKeyEnv) requireDue(t *testing.T, taskID, want, context string) {
	t.Helper()
	if got := env.due(t, taskID); got != want {
		t.Fatalf("%s: deadline = %q, want %q", context, got, want)
	}
}

// dispatchKey feeds one key event the harness cannot spell by name. The three
// deadline keyflows that need it are the two spellings a terminal may report for
// `+` and the unshifted `=` beside them.
func (env *dueStepKeyEnv) dispatchKey(t *testing.T, key tea.KeyPressMsg) {
	t.Helper()
	env.driver.dispatch(key)
	env.driver.drain()
	env.driver.Wait(dueStepSettleTimeout)
}
