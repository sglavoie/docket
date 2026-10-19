package tui

import (
	"os"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

const deadlineAcceptanceProject = "Release"
const deadlineAcceptanceDestination = "Destination"

const (
	deadlineAcceptanceTimedTitle = "Timed deadline"
	deadlineAcceptanceOwnerTitle = "Owner of the checklist"
	deadlineAcceptanceFirstItem  = "Collect the inventory"
	deadlineAcceptanceSecondItem = "Name the rollback owner"
)

// deadlineAcceptanceEnv is one real workspace, its driver, and the identities
// the assertions address. The second project exists for the move: a remembered
// clock is keyed by TaskLocator, so a task that changes projects changes key,
// and nothing about that is observable inside one project.
type deadlineAcceptanceEnv struct {
	workspace *uitestWorkspace
	driver    *UITestDriver
	projectID string
	timedID   string
	ownerID   string
}

func newDeadlineAcceptanceEnv(t *testing.T) *deadlineAcceptanceEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var timedID, ownerID string
	project, err := workspace.addProject(deadlineAcceptanceProject, func(project *models.Project) {
		timed := newFixtureTask(project, deadlineAcceptanceTimedTitle)
		timed.ApplyWorkflowState(models.StateBacklog)
		due := dueStepDay(0, 14, 30)
		timed.DueDate = &due
		timedID = timed.ID

		owner := newFixtureTask(project, deadlineAcceptanceOwnerTitle)
		owner.ApplyWorkflowState(models.StateBacklog)
		owner.Checklist = []models.ChecklistItem{
			{Text: deadlineAcceptanceFirstItem},
			{Text: deadlineAcceptanceSecondItem},
		}
		ownerID = owner.ID
	})
	if err != nil {
		t.Fatalf("add project: %v", err)
	}
	if _, destErr := workspace.addProject(deadlineAcceptanceDestination, nil); destErr != nil {
		t.Fatalf("add destination project: %v", destErr)
	}
	workspace.setCurrentProject(project.ID)

	driver, err := workspace.launch(100, 35)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	driver.SetDrainTimeout(750 * time.Millisecond)
	return &deadlineAcceptanceEnv{
		workspace: workspace,
		driver:    driver,
		projectID: project.ID,
		timedID:   timedID,
		ownerID:   ownerID,
	}
}

func (env *deadlineAcceptanceEnv) press(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := env.driver.SendKey(key); err != nil {
			t.Fatalf("send key %q: %v", key, err)
		}
		env.driver.drain()
	}
}

// settle presses one key and waits for the transaction it raised, so an
// assertion reads committed bytes rather than a command still in flight.
func (env *deadlineAcceptanceEnv) settle(t *testing.T, keys ...string) {
	t.Helper()
	env.press(t, keys...)
	env.driver.Wait(dueStepSettleTimeout)
}

func (env *deadlineAcceptanceEnv) focusTask(t *testing.T, taskID string) {
	t.Helper()
	if env.walkToTask(t, taskID) {
		return
	}
	// A launch folds All's Backlog and Someday lanes, so a task living there is reached by
	// opening every fold first.
	env.driver.App().crossOutliner.ClearAllFolds()
	if env.walkToTask(t, taskID) {
		return
	}
	t.Fatalf("the cursor never reached task %s", taskID)
}

func (env *deadlineAcceptanceEnv) walkToTask(t *testing.T, taskID string) bool {
	t.Helper()
	env.press(t, "home")
	view := env.driver.App().crossOutliner
	for range len(view.Rows) + 1 {
		if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == taskID {
			return true
		}
		env.press(t, "down")
	}
	return false
}

// promote opens the owner's checklist and presses `S` on the named item. It goes
// through the projection rather than the service because an item row is the only
// place the key answers from, and the identity the commit freezes is the list
// that row belongs to.
func (env *deadlineAcceptanceEnv) promote(t *testing.T, item string) {
	t.Helper()
	view := env.driver.App().crossOutliner
	expandChecklistOf(t, view, deadlineAcceptanceOwnerTitle)
	seatCursorOnChecklistItem(t, view, item)
	env.settle(t, "S")
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("promoting %q failed: %v", item, err)
	}
}

// storedProjects reads every project in the workspace back off disk through the
// canonical reader. Nothing here consults the loaded session: the question each
// assertion asks is what a second process opening this directory would find.
func (env *deadlineAcceptanceEnv) storedProjects(t *testing.T) []*models.Project {
	t.Helper()
	cataloged := env.workspace.Runtime.AvailableProjects()
	loaded := make([]*models.Project, 0, len(cataloged))
	for _, project := range cataloged {
		stored := &models.Project{
			ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path,
		}
		if err := storage.LoadProject(stored); err != nil {
			t.Fatalf("LoadProject(%q): %v", project.Name, err)
		}
		loaded = append(loaded, stored)
	}
	return loaded
}

// storedTask finds one task by identity anywhere in the workspace, and names the
// project it was found in. A move changes that project, so the pair is what a
// claim about a moved task is made of.
func (env *deadlineAcceptanceEnv) storedTask(t *testing.T, taskID string) (*models.Task, *models.Project) {
	t.Helper()
	for _, project := range env.storedProjects(t) {
		for _, task := range project.Tasks {
			if task != nil && task.ID == taskID {
				return task, project
			}
		}
	}
	t.Fatalf("no task %s in the stored workspace", taskID)
	return nil, nil
}

// storedTaskTitled is the same lookup for a task whose identity the commit
// minted, which is the only way to address a promoted child from outside the
// session that created it.
func (env *deadlineAcceptanceEnv) storedTaskTitled(t *testing.T, title string) (*models.Task, *models.Project) {
	t.Helper()
	for _, project := range env.storedProjects(t) {
		for _, task := range project.Tasks {
			if task != nil && task.Title == title {
				return task, project
			}
		}
	}
	t.Fatalf("no task titled %q in the stored workspace", title)
	return nil, nil
}

// storedDue spells one stored deadline the way a reader sees it: a date alone
// when the task carries no clock, and a date with its clock when it does. The
// difference is the whole subject of the restart case, so it may not be
// normalized away.
func (env *deadlineAcceptanceEnv) storedDue(t *testing.T, taskID string) string {
	t.Helper()
	task, _ := env.storedTask(t, taskID)
	switch {
	case task.DueDate == nil:
		return ""
	case task.DueHasTime():
		return task.DueDate.Format("2006-01-02 15:04")
	default:
		return task.DueDate.Format("2006-01-02")
	}
}

func (env *deadlineAcceptanceEnv) requireStoredDue(t *testing.T, taskID, want, context string) {
	t.Helper()
	if got := env.storedDue(t, taskID); got != want {
		t.Fatalf("%s: the stored deadline is %q, want %q", context, got, want)
	}
}

// storedChecklist reads one task's checklist texts off disk in stored order,
// which is the shape a promotion changes and an undo restores.
func (env *deadlineAcceptanceEnv) storedChecklist(t *testing.T, taskID string) []string {
	t.Helper()
	task, _ := env.storedTask(t, taskID)
	texts := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		texts = append(texts, item.Text)
	}
	return texts
}

// documentBytes is the canonical project document verbatim. Undo and redo are
// asserted on it rather than on parsed fields because the ledger restores an
// image: a round trip that produced equivalent state through different bytes
// would leave every stored revision in the workspace disagreeing with itself.
func (env *deadlineAcceptanceEnv) documentBytes(t *testing.T, projectID string) []byte {
	t.Helper()
	for _, project := range env.workspace.Runtime.AvailableProjects() {
		if project.ID != projectID {
			continue
		}
		raw, err := os.ReadFile(storage.TasksFilePath(project.Path))
		if err != nil {
			t.Fatalf("read the stored document for %q: %v", project.Name, err)
		}
		return raw
	}
	t.Fatalf("no project %s in the catalog", projectID)
	return nil
}

// requireRecoveryFindsNothing runs the pass a launch runs and demands an empty
// report in every field. It is spelled once because the claim is the same for
// every mutation: the transaction directory a completed commit leaves behind has
// nothing in it for recovery to decide about.
func requireRecoveryFindsNothing(t *testing.T, dataDir string) {
	t.Helper()
	report, err := storage.NewCommitService(
		config.TransactionsDir(dataDir),
		[]string{dataDir, config.LocalDataRoot()},
	).RecoverWorkspace()
	if err != nil {
		t.Fatalf("RecoverWorkspace: %v", err)
	}
	if len(report.RolledForward) != 0 {
		t.Errorf("recovery rolled %d transaction(s) forward: %v", len(report.RolledForward), report.RolledForward)
	}
	if len(report.CleanedUp) != 0 {
		t.Errorf("recovery reclaimed %d transaction(s): %v", len(report.CleanedUp), report.CleanedUp)
	}
	if len(report.SweptOrphans) != 0 {
		t.Errorf("recovery swept %d orphaned staging file(s): %v", len(report.SweptOrphans), report.SweptOrphans)
	}
	if len(report.Quarantined) != 0 {
		t.Errorf("recovery quarantined %d manifest(s): %+v", len(report.Quarantined), report.Quarantined)
	}
	if len(report.ForeignLive) != 0 {
		t.Errorf("recovery found %d live transaction(s) from another machine: %+v",
			len(report.ForeignLive), report.ForeignLive)
	}
}

// fixedAcceptanceClock stands the service on one instant, which is the only way
// to reach a calendar the real clock will not be standing on again.
type fixedAcceptanceClock struct{ at time.Time }

func (c fixedAcceptanceClock) Now() time.Time { return c.at }
