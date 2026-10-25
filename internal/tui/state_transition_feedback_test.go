package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

const stateTransitionFeedbackSettleTimeout = 2 * time.Second

type stateTransitionFeedbackEnv struct {
	driver       *UITestDriver
	firstTaskID  string
	secondTaskID string
}

func newStateTransitionFeedbackEnv(t *testing.T) *stateTransitionFeedbackEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var firstTaskID, secondTaskID string
	project, err := workspace.addProject("State Feedback", func(project *models.Project) {
		first := newFixtureTask(project, "First transition target")
		first.ApplyWorkflowState(models.StateBacklog)
		firstTaskID = first.ID
		second := newFixtureTask(project, "Second transition target")
		second.ApplyWorkflowState(models.StateBacklog)
		secondTaskID = second.ID
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
	return &stateTransitionFeedbackEnv{
		driver:       driver,
		firstTaskID:  firstTaskID,
		secondTaskID: secondTaskID,
	}
}

func (env *stateTransitionFeedbackEnv) press(t *testing.T, key string) {
	t.Helper()
	if err := env.driver.SendKey(key); err != nil {
		t.Fatalf("send key %q: %v", key, err)
	}
	env.driver.drain()
}

func (env *stateTransitionFeedbackEnv) focusTask(t *testing.T, taskID string) {
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

func (env *stateTransitionFeedbackEnv) completeSelection(t *testing.T) {
	t.Helper()
	if err := env.driver.SendKey("w"); err != nil {
		t.Fatalf("send state prefix: %v", err)
	}
	if len(env.driver.App().operations.Chord) == 0 {
		t.Fatal("state prefix did not arm the direct-key chord")
	}
	if err := env.driver.SendKey("d"); err != nil {
		t.Fatalf("send done state: %v", err)
	}
	env.driver.drain()
	env.driver.Wait(stateTransitionFeedbackSettleTimeout)
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("wd failed: %v", err)
	}
}

func TestDirectStateTransitionFeedbackUsesResolvedTaskCardinality(t *testing.T) {
	t.Run("single task", func(t *testing.T) {
		env := newStateTransitionFeedbackEnv(t)
		env.focusTask(t, env.firstTaskID)

		env.completeSelection(t)

		const want = "Change Status: First transition target → Done · u undo"
		if got := env.driver.App().feedback.message; got != want {
			t.Fatalf("status = %q, want %q", got, want)
		}
		if strings.Contains(env.driver.App().feedback.message, "Bulk") {
			t.Fatalf("single-task feedback contains Bulk: %q", env.driver.App().feedback.message)
		}
	})

	t.Run("visual multi-selection", func(t *testing.T) {
		env := newStateTransitionFeedbackEnv(t)
		env.focusTask(t, env.firstTaskID)
		env.press(t, "V")
		env.focusTask(t, env.secondTaskID)
		if outlinerView := env.driver.App().crossOutliner; !outlinerView.IsMultiSelectActive() || outlinerView.SelectedCount() != 2 {
			t.Fatalf("visual selection = active %v, selected %d; want two selected tasks", outlinerView.IsMultiSelectActive(), outlinerView.SelectedCount())
		}

		env.completeSelection(t)

		const want = "Bulk Change Status: Done · u undo"
		if got := env.driver.App().feedback.message; got != want {
			t.Fatalf("status = %q, want %q", got, want)
		}
	})
}
