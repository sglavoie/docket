package tui

import (
	"os"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

// These tests drive the D key end to end over a real on-disk workspace: the
// draft is only ever reached the way a user reaches it, and every assertion
// about what was copied or cleared reads the committed tasks.md back rather
// than the in-memory draft, which the taskform package already covers.

const (
	repeatKeyProjectName = "Delivery Stream"
	// The source title carries two capture sigils and non-ASCII text. Repeating
	// seeds the form directly, so both must survive verbatim: a title routed
	// through the capture grammar would lose #123 to a tag and $today to a
	// destination.
	repeatKeySourceTitle = "Fix bug #123 before $today — naïve build"
	repeatKeyChildTitle  = "Child that must not be copied"
)

// repeatKeySettleTimeout budgets the creation transaction the driver's default
// per-command drain is too short to finish inline.
const repeatKeySettleTimeout = 5 * time.Second

type repeatKeyEnv struct {
	workspace *uitestWorkspace
	driver    *UITestDriver
	project   *models.Project
	sourceID  string
	childID   string
}

// newRepeatKeyEnv builds one project holding a source task that fills every
// retained and every cleared field at once, plus a child the repeat must leave
// behind, and parks the cursor on the source.
func newRepeatKeyEnv(t *testing.T) *repeatKeyEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var sourceID, childID string
	project, err := workspace.addProject(repeatKeyProjectName, func(p *models.Project) {
		source := newFixtureTask(p, repeatKeySourceTitle)
		sourceID = source.ID
		source.ApplyWorkflowState(models.StateDoing)
		source.Description = "Reproduce on the naïve path, then patch the guard."
		source.Priority = 2
		source.Tags = []string{"bug", "release"}
		source.Context = "office"
		source.Checklist = []models.ChecklistItem{
			{Text: "Reproduce", Done: true},
			{Text: "Patch", Done: false},
			{Text: "Verify", Done: true},
		}
		setDue(source, 48*time.Hour)
		// An expired deferral: the stored instant is what must not be copied, and
		// an unexpired one would hide the source row the cursor has to reach.
		deferred := time.Now().Add(-6 * time.Hour)
		source.DeferredUntil = &deferred
		source.Pinned = true
		source.Review = true
		source.ResumeNote = "Left off in the retry loop"
		source.Comments = []*models.Comment{{Text: "Seen twice this week", CreatedAt: time.Now()}}
		source.Attachments = []models.Attachment{{Filename: "https://example.test/report", IsURL: true, AddedAt: time.Now()}}

		child := newFixtureTask(p, repeatKeyChildTitle)
		childID = child.ID
		child.ApplyWorkflowState(models.StateBacklog)
		child.ParentID = &sourceID
	})
	if err != nil {
		t.Fatalf("add project: %v", err)
	}
	// Today membership is planning state, not a task field, so it is seeded on
	// the planning domain the same way the fixtures do.
	workspace.Runtime.Planning().AddToToday(sourceID, project.ID)

	driver, err := workspace.launch(120, 40)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	env := &repeatKeyEnv{workspace: workspace, driver: driver, project: project, sourceID: sourceID, childID: childID}
	env.focusTask(t, sourceID)
	return env
}

func (env *repeatKeyEnv) press(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := env.driver.SendKey(key); err != nil {
			t.Fatalf("send key %q: %v", key, err)
		}
		env.driver.drain()
	}
}

// act sends keys and then resolves whatever the short drain budget abandoned,
// so a creation transaction has genuinely finished — or genuinely never
// started — before the assertions read the project back.
func (env *repeatKeyEnv) act(t *testing.T, keys ...string) {
	t.Helper()
	env.press(t, keys...)
	env.driver.Wait(repeatKeySettleTimeout)
}

func (env *repeatKeyEnv) focusTask(t *testing.T, taskID string) {
	t.Helper()
	env.press(t, "home")
	for range env.rowCount(t) + 1 {
		if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == taskID {
			return
		}
		env.press(t, "down")
	}
	t.Fatalf("the cursor never reached task %s", taskID)
}

func (env *repeatKeyEnv) rowCount(t *testing.T) int {
	t.Helper()
	out, ok := env.driver.App().outlinerPrimary()
	if !ok || out == nil {
		t.Fatal("the Outliner is not the active route")
	}
	return len(out.Rows)
}

func (env *repeatKeyEnv) reload(t *testing.T) *models.Project {
	t.Helper()
	fresh, err := models.NewProject(env.project.ID, env.project.Name, "")
	if err != nil {
		t.Fatalf("rebuild project: %v", err)
	}
	fresh.Path = env.project.Path
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("load project: %v", err)
	}
	fresh.BuildTaskTree()
	return fresh
}

func (env *repeatKeyEnv) tasksFileBytes(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(storage.TasksFilePath(env.project.Path))
	if err != nil {
		t.Fatalf("read tasks file: %v", err)
	}
	return string(content)
}

// composer returns the live composer overlay, failing when D opened nothing.
func (env *repeatKeyEnv) composer(t *testing.T) *taskform.TaskComposerModel {
	t.Helper()
	if view := env.driver.App().currentView(); view != presentation.ViewTaskComposer {
		t.Fatalf("current view = %s, want %s", view, presentation.ViewTaskComposer)
	}
	composer := env.driver.App().taskComposer
	if composer == nil {
		t.Fatal("the composer overlay is nil")
	}
	return composer
}

// repeatedTask returns the one task the repeat created: the task that is
// neither the source nor its child.
func (env *repeatKeyEnv) repeatedTask(t *testing.T, fresh *models.Project) *models.Task {
	t.Helper()
	var found *models.Task
	for _, task := range fresh.Tasks {
		if task.ID == env.sourceID || task.ID == env.childID {
			continue
		}
		if found != nil {
			t.Fatalf("more than one new task: %q and %q", found.Title, task.Title)
		}
		found = task
	}
	if found == nil {
		t.Fatal("the repeat created no task")
	}
	return found
}

func repeatChecklistTexts(items []models.ChecklistItem) []string {
	texts := make([]string, 0, len(items))
	for _, item := range items {
		texts = append(texts, item.Text)
	}
	return texts
}
