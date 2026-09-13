package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// reviewAsyncEnv wires the whole R chain over a real Backlog projection:
// executor, committed transaction, in-memory refresh, and the disk reload that
// lands last. The advance resolves in that final step, so a test that stops at
// the commit proves nothing about where the cursor ends up.
type reviewAsyncEnv struct {
	env      *appBulkPickerEnv
	model    *AppModel
	outliner *outliner.OutlinerModel
}

func newReviewAsyncEnv(t *testing.T, taskGroups ...[]*models.Task) *reviewAsyncEnv {
	t.Helper()
	env := newAppBulkPickerEnv(t, taskGroups...)
	model := env.appModel()
	model.operations.Registry = operation.NewOperationRegistry()
	model.operations.Executor = operation.NewOperationExecutor(model.operations.Registry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task operations: %v", err)
	}

	outlinerView := outliner.NewOutlinerModel(nil, model.domain.manager, model.domain.runtime)
	outlinerView.Population.Projects = env.projects
	outlinerView.Population.Invalidate()
	outlinerView.SelectOutlinerMode(chrome.ModeBacklog)
	model.crossOutliner = outlinerView
	model.viewNav.views.Register(presentation.ViewCrossProjectOutliner, outlinerView)
	model.wireOutlinerOperationExecutor(outlinerView)
	return &reviewAsyncEnv{env: env, model: model, outliner: outlinerView}
}

// reviewCandidateTask is Backlog work the review axis admits by flag alone, so
// the fixture decides which rows are candidates and staleness stays out of it.
func reviewCandidateTask(t *testing.T, shortID int, title string) *models.Task {
	t.Helper()
	task := newBulkPickerTestTask(t, shortID, title)
	task.WorkflowState = models.StateBacklog
	task.Review = true
	return task
}

func reviewSettledTask(t *testing.T, shortID int, title string) *models.Task {
	t.Helper()
	task := newBulkPickerTestTask(t, shortID, title)
	task.WorkflowState = models.StateBacklog
	return task
}

// execute runs R exactly as the input strategy does: no value, targets resolved
// from the live cursor.
func (e *reviewAsyncEnv) execute() operation.OperationExecutionResult {
	return e.model.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationTaskMarkReviewed,
		Context: e.model.operationCommandContext(),
	})
}

// acknowledge presses R and returns the command carrying the disk reload,
// without applying that reload yet.
func (e *reviewAsyncEnv) acknowledge(t *testing.T) tea.Cmd {
	t.Helper()
	result := e.execute()
	if result.Err != nil {
		t.Fatalf("execute mark reviewed: %v", result.Err)
	}
	command, ok := result.Message.(operation.OperationCommandResultMsg)
	if !ok {
		t.Fatalf("mark reviewed result = %T, want OperationCommandResultMsg", result.Message)
	}
	if command.Cmd == nil {
		t.Fatal("mark reviewed produced no command")
	}
	msg, ok := command.Cmd().(coordinator.TransactionResultMsg)
	if !ok {
		t.Fatalf("mark reviewed committed %T, want coordinator.TransactionResultMsg", command.Cmd())
	}
	if msg.Relocation == nil {
		t.Fatal("a committed acknowledgement carried no advance capture")
	}
	return e.model.handleTransactionResult(msg)
}

// applyReload unwraps the nested reload command and feeds the loaded snapshot
// back through the accepted-reload path, which is where the advance resolves.
func (e *reviewAsyncEnv) applyReload(t *testing.T, saveCmd tea.Cmd) tea.Cmd {
	t.Helper()
	msg := saveCmd()
	for depth := 0; depth < 4; depth++ {
		batch, batched := msg.(tea.BatchMsg)
		if !batched || len(batch) == 0 {
			break
		}
		msg = batch[0]()
	}
	loaded, ok := msg.(outliner.CrossProjectsLoadedMsg)
	if !ok {
		t.Fatalf("nested reload command = %T, want CrossProjectsLoadedMsg", msg)
	}
	return e.outliner.HandleCrossProjectsLoaded(loaded)
}

func (e *reviewAsyncEnv) identity(t *testing.T, projectIndex int, taskID string) taskref.TaskIdentity {
	t.Helper()
	project := e.env.projects[projectIndex]
	for _, task := range project.Tasks {
		if task.ID == taskID {
			return taskref.For(task, project)
		}
	}
	t.Fatalf("task %q is not in project %d", taskID, projectIndex)
	return taskref.TaskIdentity{}
}

// storedTask reads the acknowledged task back off disk. Every fixture puts the
// task R is pressed on in the first project, so that is the one reloaded: the
// assertions are about what the transaction wrote, not about where it lived.
func (e *reviewAsyncEnv) storedTask(t *testing.T, shortID int) *models.Task {
	t.Helper()
	for _, task := range e.env.reload(t, 0).Tasks {
		if task.ShortID == shortID {
			return task
		}
	}
	t.Fatalf("short ID %d is not in the reloaded source project", shortID)
	return nil
}

// collectStatus walks a command tree for the one info status a review pass
// emits, since the reload path batches it with the rest of its work.
func collectStatus(cmd tea.Cmd) (status.InfoStatusMsg, bool) {
	if cmd == nil {
		return "", false
	}
	switch msg := cmd().(type) {
	case status.InfoStatusMsg:
		return msg, true
	case tea.BatchMsg:
		for _, nested := range msg {
			if found, ok := collectStatus(nested); ok {
				return found, true
			}
		}
	}
	return "", false
}

// TestMarkReviewedClearsTheFlagAndAdvancesAcrossAProjectHeader is the session's
// two contracts meeting on one keystroke: the mutation writes the flag and the
// touch alone, and the pass lands on the next visible candidate — skipping the
// settled row and the project header between them.
func TestMarkReviewedClearsTheFlagAndAdvancesAcrossAProjectHeader(t *testing.T) {
	source := reviewCandidateTask(t, 1, "source")
	trailing := reviewSettledTask(t, 2, "trailing")
	betaSettled := reviewSettledTask(t, 1, "beta settled")
	betaCandidate := reviewCandidateTask(t, 2, "beta candidate")
	async := newReviewAsyncEnv(t,
		[]*models.Task{source, trailing},
		[]*models.Task{betaSettled, betaCandidate})
	before := *source

	inboxCursorTo(t, async.outliner, source.ID)
	saveCmd := async.acknowledge(t)
	if cmd := async.applyReload(t, saveCmd); cmd != nil {
		if msg, ok := collectStatus(cmd); ok {
			t.Fatalf("an advance that moved reported %q", string(msg))
		}
	}

	requireCursorRow(t, async.outliner, betaCandidate.ID)
	requireCursorTask(t, async.outliner, async.identity(t, 1, betaCandidate.ID))

	stored := async.storedTask(t, source.ShortID)
	if stored.Review {
		t.Error("the review flag survived the acknowledgement")
	}
	if !stored.UpdatedAt.After(before.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want a touch later than %v", stored.UpdatedAt, before.UpdatedAt)
	}
	if stored.State() != before.State() {
		t.Errorf("state = %s, want the preserved %s", stored.State(), before.State())
	}
	if !stored.StateChangedAt.Equal(before.StateChangedAt) {
		t.Errorf("StateChangedAt = %v, want the untouched %v", stored.StateChangedAt, before.StateChangedAt)
	}
	if stored.Pinned != before.Pinned || stored.Priority != before.Priority {
		t.Errorf("acknowledgement rewrote unrelated content: %+v", stored)
	}
}

// TestMarkReviewedReportsExhaustionOnTheLastCandidate is the end of a pass: the
// source is still visible, so it keeps the cursor, and the status says the
// obvious thing rather than leaving the user pressing R at nothing.
func TestMarkReviewedReportsExhaustionOnTheLastCandidate(t *testing.T) {
	leading := reviewCandidateTask(t, 1, "leading")
	source := reviewCandidateTask(t, 2, "source")
	trailing := reviewSettledTask(t, 3, "trailing")
	async := newReviewAsyncEnv(t, []*models.Task{leading, source, trailing})

	inboxCursorTo(t, async.outliner, source.ID)
	saveCmd := async.acknowledge(t)
	msg, ok := collectStatus(async.applyReload(t, saveCmd))
	if !ok {
		t.Fatal("an exhausted review pass reported nothing")
	}
	if !strings.Contains(string(msg), "No further visible review candidates") {
		t.Fatalf("exhaustion status = %q, want the no-candidates line", string(msg))
	}

	requireCursorRow(t, async.outliner, source.ID)
	if !leading.Review {
		t.Fatal("the fixture stopped proving a candidate remained above the cursor")
	}
}

// TestMarkReviewedRefusesATaskNoAxisAdmits keeps R from being a way to touch any
// row. The refusal names both ways in that the user controls, and nothing moves.
func TestMarkReviewedRefusesATaskNoAxisAdmits(t *testing.T) {
	settled := reviewSettledTask(t, 1, "settled")
	candidate := reviewCandidateTask(t, 2, "candidate")
	async := newReviewAsyncEnv(t, []*models.Task{settled, candidate})

	inboxCursorTo(t, async.outliner, settled.ID)
	before := async.outliner.Cursor

	result := async.execute()
	if result.Err == nil {
		t.Fatal("R acknowledged a task no review axis admits")
	}
	if !strings.Contains(result.Err.Error(), "no review to acknowledge") {
		t.Errorf("error = %v, want the nothing-to-acknowledge refusal", result.Err)
	}
	// The refusal names the two routes the user controls and nothing else:
	// waiting for staleness is no longer one of them.
	if strings.Contains(result.Err.Error(), "stale") {
		t.Errorf("error = %v, still offers staleness as a review route", result.Err)
	}
	if async.outliner.Cursor != before {
		t.Errorf("cursor = %d after a refusal, want the untouched %d", async.outliner.Cursor, before)
	}
	if async.storedTask(t, settled.ShortID).UpdatedAt.After(settled.UpdatedAt) {
		t.Error("a refused acknowledgement still touched the task")
	}
}

// TestMarkReviewedRefusesUnderAVisualSelection is what DirectOnly buys: an
// acknowledgement is one task's act, so a selection is a refusal rather than a
// silent fallback to the cursor.
func TestMarkReviewedRefusesUnderAVisualSelection(t *testing.T) {
	source := reviewCandidateTask(t, 1, "source")
	other := reviewCandidateTask(t, 2, "other")
	async := newReviewAsyncEnv(t, []*models.Task{source, other})

	inboxCursorTo(t, async.outliner, source.ID)
	async.outliner.ReplaceSelection([]taskref.TaskIdentity{
		async.identity(t, 0, source.ID),
		async.identity(t, 0, other.ID),
	})
	if !async.outliner.IsMultiSelectActive() {
		t.Fatal("the fixture did not activate a visual selection")
	}

	if result := async.execute(); result.Err == nil {
		t.Fatal("R ran under a visual selection")
	}
	if !source.Review || !other.Review {
		t.Fatal("a refused acknowledgement cleared a flag anyway")
	}
}

// TestMarkReviewedUndoRestoresTheFlagAndLeavesTheCursor pins the two halves
// apart. The mutation is one ordinary ledger entry, so u takes the flag back;
// the advance is a cursor move nothing recorded, so u does not rewind it.
func TestMarkReviewedUndoRestoresTheFlagAndLeavesTheCursor(t *testing.T) {
	source := reviewCandidateTask(t, 1, "source")
	successor := reviewCandidateTask(t, 2, "successor")
	async := newReviewAsyncEnv(t, []*models.Task{source, successor})

	inboxCursorTo(t, async.outliner, source.ID)
	async.applyReload(t, async.acknowledge(t))
	requireCursorRow(t, async.outliner, successor.ID)
	landed := async.outliner.CurrentTaskIdentity()
	if async.storedTask(t, source.ShortID).Review {
		t.Fatal("the acknowledgement did not reach disk")
	}

	async.env.undo(t)
	async.model.reloadProjectByID(async.env.projects[0].ID)
	async.outliner.Population.Invalidate()
	async.outliner.RebuildRows()

	if !async.storedTask(t, source.ShortID).Review {
		t.Error("undo did not restore the review flag")
	}
	if got := async.outliner.CurrentTaskIdentity(); got != landed {
		t.Errorf("cursor task = %+v after undo, want the row the pass advanced to %+v", got, landed)
	}
}

// TestMarkReviewedAnswersAWaitingCheckInWithoutResettingItsWait is the Waiting
// task end to end, and it is scheduled rather than merely Waiting because that
// is now the only thing that makes it answerable: the state alone no longer
// admits R. The acknowledgement leaves the state, its clock, and the check-back
// deadline exactly where they were, so the waiting age the row shows does not
// restart, and it moves the schedule rather than the deadline.
func TestMarkReviewedAnswersAWaitingCheckInWithoutResettingItsWait(t *testing.T) {
	waiting := newBulkPickerTestTask(t, 1, "waiting on the vendor")
	waiting.WorkflowState = models.StateWaiting
	waiting.StateChangedAt = time.Now().AddDate(0, 0, -21)
	due := time.Now().AddDate(0, 0, 3)
	waiting.DueDate = &due
	markReviewedSchedule(t, waiting, -2, 7)
	successor := reviewCandidateTask(t, 2, "successor")
	async := newReviewAsyncEnv(t, []*models.Task{waiting, successor})
	async.outliner.SelectOutlinerMode(chrome.ModeAll)
	before := *waiting

	inboxCursorTo(t, async.outliner, waiting.ID)
	async.applyReload(t, async.acknowledge(t))

	stored := async.storedTask(t, waiting.ShortID)
	if !stored.IsInState(models.StateWaiting) {
		t.Errorf("state = %s, want the preserved Waiting", stored.State())
	}
	if !stored.StateChangedAt.Equal(before.StateChangedAt) {
		t.Errorf("StateChangedAt = %v, want the untouched %v", stored.StateChangedAt, before.StateChangedAt)
	}
	// DueDate is a calendar date, so it is compared as one: the stored clock time
	// is the writer's business and never the acknowledgement's.
	if stored.DueDate == nil || stored.DueDate.Format(time.DateOnly) != before.DueDate.Format(time.DateOnly) {
		t.Errorf("DueDate = %v, want the preserved check-back %v", stored.DueDate, before.DueDate)
	}
	// The cadence survives the occurrence it answered, counted from today rather
	// than from the overdue date it was owed on.
	wantNext := time.Now().AddDate(0, 0, 7).Format(time.DateOnly)
	if stored.NextCheckIn == nil || stored.NextCheckIn.Format(time.DateOnly) != wantNext {
		t.Errorf("NextCheckIn = %v, want %s", stored.NextCheckIn, wantNext)
	}
	if stored.CheckInRepeatDays != 7 {
		t.Errorf("repeat = %d, want the preserved 7", stored.CheckInRepeatDays)
	}
	if stored.LastReviewedAt == nil {
		t.Error("the acknowledgement stamped no review history")
	}
	if models.TaskNeedsAttention(stored, time.Now()) {
		t.Error("an answered check-in is still asking for attention")
	}
}

// markReviewedSchedule writes a check-in dayOffset calendar days from today
// through the model's own gate, so a fixture the production path would refuse
// fails here rather than making a misleading assertion downstream.
func markReviewedSchedule(t *testing.T, task *models.Task, dayOffset, repeatDays int) {
	t.Helper()
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, dayOffset)
	if err := task.SetCheckIn(
		models.CheckInSchedule{Next: &next, RepeatDays: repeatDays}, now,
	); err != nil {
		t.Fatalf("seed check-in: %v", err)
	}
}
