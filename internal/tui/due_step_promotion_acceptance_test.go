package tui

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
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
