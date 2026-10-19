package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operationhost"
)

const resumeNoteSettleTimeout = 2 * time.Second

// resumeNoteEnv is the fixture the resume keyflows share. The tasks are spread
// across the lifecycle on purpose: a resume note is content rather than a step,
// so the key has to reach a done task exactly as it reaches a doing one.
type resumeNoteEnv struct {
	driver    *UITestDriver
	projectID string
	doingID   string
	doneID    string
	inboxID   string
}

func newResumeNoteEnv(t *testing.T) *resumeNoteEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var doingID, doneID, inboxID string
	project, err := workspace.addProject("Resume Note", func(project *models.Project) {
		doing := newFixtureTask(project, "Wire the importer")
		doing.ApplyWorkflowState(models.StateDoing)
		doingID = doing.ID

		done := newFixtureTask(project, "Ship the parser")
		done.ApplyWorkflowState(models.StateDone)
		doneID = done.ID

		inbox := newFixtureTask(project, "Triage the crash report")
		inbox.ApplyWorkflowState(models.StateInbox)
		inboxID = inbox.ID
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
	return &resumeNoteEnv{
		driver:    driver,
		projectID: project.ID,
		doingID:   doingID,
		doneID:    doneID,
		inboxID:   inboxID,
	}
}

func (env *resumeNoteEnv) press(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := env.driver.SendKey(key); err != nil {
			t.Fatalf("send key %q: %v", key, err)
		}
		env.driver.drain()
	}
}

func (env *resumeNoteEnv) typeText(t *testing.T, text string) {
	t.Helper()
	for _, r := range text {
		if r == ' ' {
			env.press(t, "space")
			continue
		}
		env.press(t, string(r))
	}
}

// clearPrompt empties the buffer the way a user does, which is also the only
// spelling of "clear this note".
func (env *resumeNoteEnv) clearPrompt(t *testing.T) {
	t.Helper()
	prompt := env.prompt(t)
	for range len(prompt.Value()) {
		env.press(t, "backspace")
	}
}

func (env *resumeNoteEnv) prompt(t *testing.T) *operationhost.OperationTextPromptModel {
	t.Helper()
	prompt := env.driver.App().operations.TextPrompt
	if prompt == nil {
		t.Fatal("no text prompt is mounted")
	}
	return prompt
}

func (env *resumeNoteEnv) focusTask(t *testing.T, taskID string) {
	t.Helper()
	env.press(t, "home")
	for range len(env.driver.App().crossOutliner.Rows) + 1 {
		if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == taskID {
			return
		}
		env.press(t, "down")
	}
	t.Fatalf("the cursor never reached task %s", taskID)
}

// inToday reads the planning membership the note must not disturb.
func (env *resumeNoteEnv) inToday(taskID string) bool {
	for _, item := range env.driver.App().domain.runtime.Planning().GetToday() {
		if item.ProjectID == env.projectID && item.TaskID == taskID {
			return true
		}
	}
	return false
}

func (env *resumeNoteEnv) task(t *testing.T, taskID string) *models.Task {
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

// setResumeNote is the whole keyflow in one call: open the prompt on the cursor
// row, replace whatever it prefilled, and commit.
func (env *resumeNoteEnv) setResumeNote(t *testing.T, taskID, note string) {
	t.Helper()
	env.focusTask(t, taskID)
	env.press(t, "r")
	env.clearPrompt(t)
	env.typeText(t, note)
	env.press(t, "enter")
	env.driver.Wait(resumeNoteSettleTimeout)
}
