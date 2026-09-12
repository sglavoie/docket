package tui

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// The acceptance half of the two mutations this release added. Every other test
// for them reads the answer out of a fake repository or out of the projection
// the shell is holding; each one here reads it back out of the canonical
// Markdown, and several of them throw the session away first.
//
// That is the whole distinction. A deadline step and a checklist promotion both
// commit under a strict read-set precondition through one checked transaction,
// so what they owe is not only the right arithmetic but the right bytes: a
// document the canonical reader accepts, a journal with nothing left in it, and
// a ledger image an undo can put back. A restart is the only way to ask whether
// a session was remembering something the workspace never stored — which is
// exactly the question the remembered due clocks raise, since they are session
// memory by construction and a machine-local file would be the wrong home for
// them.

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
	env.press(t, "home")
	view := env.driver.App().crossOutliner
	for range len(view.Rows) + 1 {
		if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == taskID {
			return
		}
		env.press(t, "down")
	}
	t.Fatalf("the cursor never reached task %s", taskID)
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

// TestDeadlineStepAndPromotionSurviveARestart is the durability claim for both
// mutations at once. A restart rebuilds the app from durable state alone, so a
// deadline or a child that is still there afterwards was written rather than
// held; and both are checked in the stored document first, so a restart that
// somehow reconstructed them from elsewhere could not pass either.
func TestDeadlineStepAndPromotionSurviveARestart(t *testing.T) {
	env := newDeadlineAcceptanceEnv(t)

	env.focusTask(t, env.timedID)
	env.settle(t, "+")
	env.requireStoredDue(t, env.timedID, dueStepDay(1, 14, 30).Format("2006-01-02 15:04"),
		"one forward step off today")

	env.promote(t, deadlineAcceptanceFirstItem)
	if got := env.storedChecklist(t, env.ownerID); !slices.Equal(got, []string{deadlineAcceptanceSecondItem}) {
		t.Fatalf("the stored checklist is %q, want the promoted item gone", got)
	}
	child, childProject := env.storedTaskTitled(t, deadlineAcceptanceFirstItem)
	if childProject.ID != env.projectID {
		t.Fatalf("the child landed in %q, want the owner's project", childProject.Name)
	}
	if child.ParentID == nil || *child.ParentID != env.ownerID {
		t.Fatalf("the stored child's parent is %v, want the owner %q", child.ParentID, env.ownerID)
	}

	if err := env.driver.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}

	// The same two claims, now about a session that has only ever seen the files.
	env.requireStoredDue(t, env.timedID, dueStepDay(1, 14, 30).Format("2006-01-02 15:04"),
		"the deadline after a restart")
	reloaded, _ := workspaceTaskTitled(t, env.driver.App(), deadlineAcceptanceFirstItem)
	if reloaded.ID != child.ID {
		t.Fatalf("the relaunched session holds child %q, want the committed %q", reloaded.ID, child.ID)
	}
}

// TestRememberedDueClockIsSessionMemoryAndARestartForgetsIt pins where the
// remembered clock lives. Inside one session, stepping through unset and back
// restores the wall clock the clear took away — that is covered where the
// sequence is. The claim here is the other half: the memory is the session's, so
// a restart must produce the date-only deadline a task with no remembered clock
// gets, rather than resurrecting 14:30 from a file that has no business holding
// it.
func TestRememberedDueClockIsSessionMemoryAndARestartForgetsIt(t *testing.T) {
	env := newDeadlineAcceptanceEnv(t)

	env.focusTask(t, env.timedID)
	env.settle(t, "-")
	env.requireStoredDue(t, env.timedID, "", "stepping backward off today")

	if err := env.driver.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}

	env.focusTask(t, env.timedID)
	env.settle(t, "+")
	env.requireStoredDue(t, env.timedID, dueStepDay(0, 0, 0).Format("2006-01-02"),
		"stepping back onto today in a session that never saw the clock")
}

// TestRememberedDueClockFollowsATaskAcrossAProjectMove is the remapping claim
// made over the real files. The memory is keyed by TaskLocator, so a move
// changes the key: a step taken after the move would restore midnight if the
// remembered clock had stayed filed under the project the task left.
func TestRememberedDueClockFollowsATaskAcrossAProjectMove(t *testing.T) {
	env := newDeadlineAcceptanceEnv(t)

	env.focusTask(t, env.timedID)
	env.settle(t, "-")
	env.requireStoredDue(t, env.timedID, "", "stepping backward off today")

	// `%` opens Move to Project over the cursor; the destination list excludes
	// the task's own project and Enter takes the first row of it.
	env.focusTask(t, env.timedID)
	env.settle(t, "%", "enter")
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("moving the task failed: %v", err)
	}
	moved, movedProject := env.storedTask(t, env.timedID)
	if movedProject.ID == env.projectID {
		t.Fatalf("the task is still in %q, so nothing was moved", movedProject.Name)
	}
	if moved.DueDate != nil {
		t.Fatalf("the move restored a deadline the step had cleared: %v", moved.DueDate)
	}

	env.focusTask(t, env.timedID)
	env.settle(t, "+")
	env.requireStoredDue(t, env.timedID, dueStepDay(0, 14, 30).Format("2006-01-02 15:04"),
		"stepping back onto today after the move")
	if _, project := env.storedTask(t, env.timedID); project.ID == env.projectID {
		t.Fatalf("the step wrote the task back into %q", project.Name)
	}
}

// TestDeadlineStepUndoRedoRoundTripsTheCanonicalDocument holds undo to the bytes.
// One gesture is one transaction and one ledger entry, so a single `u` has to
// reproduce the document the step started from exactly and `Ctrl+R` the one it
// produced — not an equivalent document, the same one.
func TestDeadlineStepUndoRedoRoundTripsTheCanonicalDocument(t *testing.T) {
	env := newDeadlineAcceptanceEnv(t)
	before := env.documentBytes(t, env.projectID)

	env.focusTask(t, env.timedID)
	env.settle(t, "+")
	after := env.documentBytes(t, env.projectID)
	if string(after) == string(before) {
		t.Fatal("a committed step left the document unchanged")
	}

	env.settle(t, "u")
	if got := env.documentBytes(t, env.projectID); string(got) != string(before) {
		t.Fatalf("undo left a different document:\n got: %s\nwant: %s", got, before)
	}
	env.settle(t, "ctrl+r")
	if got := env.documentBytes(t, env.projectID); string(got) != string(after) {
		t.Fatalf("redo left a different document:\n got: %s\nwant: %s", got, after)
	}
}

// TestChecklistPromotionUndoRedoRoundTripsTheCanonicalDocument is the same claim
// for the mutation that writes two things at once. The child and the item's
// removal are one transaction, so the bytes are the only assertion that covers
// both halves without naming either.
func TestChecklistPromotionUndoRedoRoundTripsTheCanonicalDocument(t *testing.T) {
	env := newDeadlineAcceptanceEnv(t)
	before := env.documentBytes(t, env.projectID)

	env.promote(t, deadlineAcceptanceSecondItem)
	after := env.documentBytes(t, env.projectID)

	env.settle(t, "u")
	if got := env.documentBytes(t, env.projectID); string(got) != string(before) {
		t.Fatalf("undo left a different document:\n got: %s\nwant: %s", got, before)
	}
	if got := env.storedChecklist(t, env.ownerID); !slices.Equal(got,
		[]string{deadlineAcceptanceFirstItem, deadlineAcceptanceSecondItem}) {
		t.Fatalf("the stored checklist is %q after an undo, want both items in order", got)
	}
	env.settle(t, "ctrl+r")
	if got := env.documentBytes(t, env.projectID); string(got) != string(after) {
		t.Fatalf("redo left a different document:\n got: %s\nwant: %s", got, after)
	}
}

// TestDeadlineStepAndPromotionLeaveNoIncompleteTransaction is the journal claim.
// Recovery is what a launch runs before anything reads the workspace, so a
// mutation that left a prepared manifest, a staged orphan or a quarantined
// bundle behind would be repaired on the next launch rather than reported — and
// a repair that happens every time is indistinguishable from a commit that never
// finished. Asserting that a fresh pass finds nothing at all is what separates
// them.
func TestDeadlineStepAndPromotionLeaveNoIncompleteTransaction(t *testing.T) {
	env := newDeadlineAcceptanceEnv(t)

	env.focusTask(t, env.timedID)
	env.settle(t, "+")
	env.promote(t, deadlineAcceptanceFirstItem)
	env.settle(t, "u")

	requireRecoveryFindsNothing(t, env.workspace.DataDir)
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

// TestDeadlineStepRefusesADaylightSavingSkipOverTheCanonicalWorkspace takes the
// one refusal the calculation owns down to the files. The arithmetic is proven
// where it lives, over a fake repository; what cannot be asked there is whether
// the refusal reaches the workspace as a refusal — a step that computed a
// nonexistent wall clock and then wrote a normalized one would move a deadline
// an hour and record it, and every document-level claim would still pass.
//
// It builds its own service rather than driving the keyboard, because the input
// is a clock: the calculation is anchored on the day the operation runs, so the
// skip is only reachable from a machine standing on the transition.
func TestDeadlineStepRefusesADaylightSavingSkipOverTheCanonicalWorkspace(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var skippedID, untouchedID string
	project, err := workspace.addProject("Spring Forward", func(project *models.Project) {
		// 02:30 on the day before the transition steps onto an instant the zone
		// skips outright, which is the one date arithmetic may not normalize.
		skipped := newFixtureTask(project, "Lands on the hour that does not exist")
		skipped.ApplyWorkflowState(models.StateBacklog)
		skippedDue := time.Date(2026, time.March, 7, 2, 30, 0, 0, zone)
		skipped.DueDate = &skippedDue
		skippedID = skipped.ID

		// A companion two weeks away, so the assertion is that the refusal covers
		// the whole selection rather than the target that caused it.
		untouched := newFixtureTask(project, "A deadline nothing is wrong with")
		untouched.ApplyWorkflowState(models.StateBacklog)
		untouchedDue := time.Date(2026, time.March, 20, 9, 0, 0, 0, zone)
		untouched.DueDate = &untouchedDue
		untouchedID = untouched.ID
	})
	if err != nil {
		t.Fatalf("add project: %v", err)
	}
	if reloadErr := workspace.Runtime.Reload(); reloadErr != nil {
		t.Fatalf("reload workspace domains: %v", reloadErr)
	}

	stored := &models.Project{
		ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path,
	}
	before, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read the stored document: %v", err)
	}

	ledger := workspaceLedgerFromConfig(workspace.Runtime)
	service := app.NewServiceWithArtifacts(
		newTUIAppRepository(workspace.DataDir, workspace.Manager.RuntimeDependencies(), ledger),
		fixedAcceptanceClock{at: time.Date(2026, time.March, 7, 12, 0, 0, 0, zone)},
		nil,
		app.NewStorageWorkspaceNotePort(workspace.DataDir),
	).WithPlanningArtifacts(coordinator.ManagerPlanningArtifacts(workspace.Manager))

	_, stepErr := service.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationStepDueDate,
		Targets: []app.TaskLocator{
			{ProjectID: project.ID, TaskID: skippedID},
			{ProjectID: project.ID, TaskID: untouchedID},
		},
		DueStep: app.DueStepRequest{Direction: app.DueStepForward},
	})
	if stepErr == nil {
		t.Fatal("a step onto a skipped wall clock committed")
	}

	if after, readErr := os.ReadFile(storage.TasksFilePath(project.Path)); readErr != nil {
		t.Fatalf("reread the stored document: %v", readErr)
	} else if string(after) != string(before) {
		t.Fatalf("a refused step rewrote the document:\n got: %s\nwant: %s", after, before)
	}
	if err := storage.LoadProject(stored); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	// Compared as the wall clock it is spelled with rather than as an instant: a
	// stored deadline is a calendar date and an optional clock, so reading one
	// back on a machine in another zone names the same day and time under a
	// different offset, and an instant comparison would be asking the wrong
	// question of the right bytes.
	if got := stored.GetTaskByID(untouchedID); got.DueDate == nil ||
		got.DueDate.Format(helperDeadlineLayout) != "2026-03-20 09:00" {
		t.Fatalf("the companion's deadline is %v, want the whole selection refused", got.DueDate)
	}
	// A precondition-free refusal still owes the same absences a stale one does:
	// nothing staged, nothing journaled, nothing to undo.
	requireRecoveryFindsNothing(t, workspace.DataDir)
	if entry, ok, err := ledger.SelectNewestUndo(storage.GlobalScope()); err != nil {
		t.Fatalf("read the workspace ledger: %v", err)
	} else if ok {
		t.Fatalf("a refused step recorded the ledger entry %q", entry.Action)
	}
}

// fixedAcceptanceClock stands the service on one instant, which is the only way
// to reach a calendar the real clock will not be standing on again.
type fixedAcceptanceClock struct{ at time.Time }

func (c fixedAcceptanceClock) Now() time.Time { return c.at }
