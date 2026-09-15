package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// relocationAsyncEnv wires the whole asynchronous mutation chain over a real
// projection: executor, committed batch, in-memory refresh, and the disk reload
// that lands last. These tests exist because that chain, not the synchronous
// planning path, is what wd, wc, ww, wg and every bulk sweep travel.
//
// The mode and the seeded state are parameters rather than the Inbox constants
// they once were, because recovery is no longer a mode's declared property: the
// same chain has to land the cursor the same way wherever the mutation was
// issued from, and All is where a departing task stays visible in another lane.
type relocationAsyncEnv struct {
	env      *appBulkPickerEnv
	model    *AppModel
	outliner *outliner.OutlinerModel
}

func newInboxAsyncEnv(t *testing.T, taskGroups ...[]*models.Task) *relocationAsyncEnv {
	t.Helper()
	return newRelocationAsyncEnv(t, chrome.ModeInbox, models.StateInbox, taskGroups...)
}

func newRelocationAsyncEnv(t *testing.T, mode chrome.OutlinerModeID, seeded models.WorkflowState, taskGroups ...[]*models.Task) *relocationAsyncEnv {
	t.Helper()
	for _, tasks := range taskGroups {
		for _, task := range tasks {
			task.WorkflowState = seeded
		}
	}
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
	outlinerView.SelectOutlinerMode(mode)
	model.crossOutliner = outlinerView
	model.viewNav.views.Register(presentation.ViewCrossProjectOutliner, outlinerView)
	model.wireOutlinerOperationExecutor(outlinerView)
	return &relocationAsyncEnv{env: env, model: model, outliner: outlinerView}
}

// commitState drives operationTaskSetState the way a $-chord does and returns
// the command that carries the disk reload, without applying that reload yet.
func (e *relocationAsyncEnv) commitState(t *testing.T, state models.WorkflowState) tea.Cmd {
	t.Helper()
	result := e.model.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:       operation.OperationTaskSetState,
		Context:  e.model.operationCommandContext(),
		Value:    state,
		HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("execute set state %s: %v", state, result.Err)
	}
	command, ok := result.Message.(operation.OperationCommandResultMsg)
	if !ok {
		t.Fatalf("state result = %T, want OperationCommandResultMsg", result.Message)
	}
	batch := runBatchResult(t, command.Cmd)
	if batch.Relocation == nil {
		t.Fatal("successful async state update carried no relocation capture")
	}
	return e.model.handleBatchTransactionResult(batch)
}

// applyReload unwraps the nested reload command and feeds the loaded snapshot
// back through the accepted-reload path, which is where the advance resolves.
func (e *relocationAsyncEnv) applyReload(t *testing.T, saveCmd tea.Cmd) {
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
		t.Fatalf("nested reload command = %T, want crossProjectsLoadedMsg", msg)
	}
	e.outliner.HandleCrossProjectsLoaded(loaded)
}

func (e *relocationAsyncEnv) identity(t *testing.T, projectIndex int, taskID string) taskref.TaskIdentity {
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

// TestInboxAsyncDoneAdvancesAcrossAProjectGroupHeader is the reconciliation: wd
// travels the asynchronous route and must land exactly where Session 1's
// synchronous wb lands from the same row — the first task of the next group,
// never that group's header.
func TestInboxAsyncDoneAdvancesAcrossAProjectGroupHeader(t *testing.T) {
	alphaOne := newBulkPickerTestTask(t, 1, "alpha one")
	alphaTwo := newBulkPickerTestTask(t, 2, "alpha two")
	betaOne := newBulkPickerTestTask(t, 1, "beta one")
	betaTwo := newBulkPickerTestTask(t, 2, "beta two")
	async := newInboxAsyncEnv(t,
		[]*models.Task{alphaOne, alphaTwo},
		[]*models.Task{betaOne, betaTwo})

	cursorToTaskRow(t, async.outliner, alphaTwo.ID)
	saveCmd := async.commitState(t, models.StateDone)
	if async.outliner.RelocationHistory.Pending == nil {
		t.Fatal("committed async state update did not survive its own refresh")
	}
	async.applyReload(t, saveCmd)

	requireCursorRow(t, async.outliner, betaOne.ID)
	requireCursorTask(t, async.outliner, async.identity(t, 1, betaOne.ID))
}

// TestInboxAsyncBulkSweepLandsPastTheWholeSelection pins the bulk semantics:
// Following is the first row after the cursor that is not itself a target, so a
// sweep lands on live unselected work rather than inside its own selection. The
// advance reads that anchor alone and never IsMultiSelectActive(), so it makes
// no claim on the selection: MultiSelectState.FinishBulk is the authority that a
// successful non-delete bulk leaves the persistent selection standing, and only
// delete owns a post-commit clear path.
func TestInboxAsyncBulkSweepLandsPastTheWholeSelection(t *testing.T) {
	alphaOne := newBulkPickerTestTask(t, 1, "alpha one")
	alphaTwo := newBulkPickerTestTask(t, 2, "alpha two")
	betaOne := newBulkPickerTestTask(t, 1, "beta one")
	betaTwo := newBulkPickerTestTask(t, 2, "beta two")
	async := newInboxAsyncEnv(t,
		[]*models.Task{alphaOne, alphaTwo},
		[]*models.Task{betaOne, betaTwo})

	cursorToTaskRow(t, async.outliner, alphaTwo.ID)
	async.outliner.ReplaceSelection([]taskref.TaskIdentity{
		async.identity(t, 0, alphaTwo.ID),
		async.identity(t, 1, betaOne.ID),
	})
	if !async.outliner.IsMultiSelectActive() {
		t.Fatal("visual selection did not activate multi-select")
	}

	saveCmd := async.commitState(t, models.StateBacklog)
	async.applyReload(t, saveCmd)

	requireCursorRow(t, async.outliner, betaTwo.ID)
	requireCursorTask(t, async.outliner, async.identity(t, 1, betaTwo.ID))
	if got := async.outliner.SelectedCount(); got != 2 {
		t.Fatalf("committed bulk sweep holds %d selected identities, want the retained 2", got)
	}
	if !async.outliner.IsMultiSelectActive() {
		t.Fatal("committed non-delete bulk left multi-select mode")
	}
}

// TestInboxAsyncCursorMoveAwayAndBackBeforeReloadVoidsAdvance closes the hole a
// purely numeric reload guard leaves open: the user navigates away from the
// settled cursor and back onto it before the disk reload lands. The cursor index
// is then identical to the one the mutation's own refresh produced, so nothing
// at reload time can tell the two apart — the navigation itself has to void the
// capture as it happens. Both moves travel the real coreMoveCursor path, which
// is the only thing that distinguishes this from the one-way case below.
func TestInboxAsyncCursorMoveAwayAndBackBeforeReloadVoidsAdvance(t *testing.T) {
	alphaOne := newBulkPickerTestTask(t, 1, "alpha one")
	alphaTwo := newBulkPickerTestTask(t, 2, "alpha two")
	betaOne := newBulkPickerTestTask(t, 1, "beta one")
	betaTwo := newBulkPickerTestTask(t, 2, "beta two")
	async := newInboxAsyncEnv(t,
		[]*models.Task{alphaOne, alphaTwo},
		[]*models.Task{betaOne, betaTwo})

	cursorToTaskRow(t, async.outliner, alphaTwo.ID)
	saveCmd := async.commitState(t, models.StateDone)
	pending := async.outliner.RelocationHistory.Pending
	if pending == nil {
		t.Fatal("committed async state update did not survive its own refresh")
	}
	settled := pending.SettledCursor
	if async.outliner.Cursor != settled {
		t.Fatalf("cursor = %d, want the settled cursor %d", async.outliner.Cursor, settled)
	}

	// j then k while the disk refresh is still in flight: the cursor ends up back
	// on the settled row, so only the navigation itself can void the capture.
	async.model.coreMoveCursor("j")
	if async.outliner.Cursor == settled {
		t.Fatal("the fixture did not actually move the cursor off the settled row")
	}
	async.model.coreMoveCursor("k")
	if async.outliner.Cursor != settled {
		t.Fatalf("cursor = %d after moving back, want the settled cursor %d", async.outliner.Cursor, settled)
	}
	returned := async.outliner.CurrentTaskIdentity()

	async.applyReload(t, saveCmd)

	if async.outliner.RelocationHistory.Pending != nil {
		t.Fatal("a cursor move away and back left the relocation capture armed")
	}
	if got := async.outliner.CurrentTaskIdentity(); got != returned {
		t.Fatalf("cursor task = %+v, want the row the user returned to %+v", got, returned)
	}
}

// TestInboxAsyncGroupJumpAwayAndBackBeforeReloadVoidsAdvance is the same hole
// reached through the other navigation family. A last-task-of-project departure
// settles the cursor on the row that opens the following project's group, so n
// and N are the keys that move from there — and n wrapping to the other group
// and N returning lands the cursor back on the settled index without changing
// the projection fingerprint. Only voiding at the group jump itself keeps the
// accepted reload from applying the successor advance over the row the user
// chose.
func TestInboxAsyncGroupJumpAwayAndBackBeforeReloadVoidsAdvance(t *testing.T) {
	alphaOne := newBulkPickerTestTask(t, 1, "alpha one")
	alphaTwo := newBulkPickerTestTask(t, 2, "alpha two")
	betaOne := newBulkPickerTestTask(t, 1, "beta one")
	betaTwo := newBulkPickerTestTask(t, 2, "beta two")
	async := newInboxAsyncEnv(t,
		[]*models.Task{alphaOne, alphaTwo},
		[]*models.Task{betaOne, betaTwo})

	cursorToTaskRow(t, async.outliner, alphaTwo.ID)
	saveCmd := async.commitState(t, models.StateDone)
	pending := async.outliner.RelocationHistory.Pending
	if pending == nil {
		t.Fatal("committed async state update did not survive its own refresh")
	}
	settled := pending.SettledCursor
	if async.outliner.Cursor != settled {
		t.Fatalf("cursor = %d, want the settled cursor %d", async.outliner.Cursor, settled)
	}
	if settled < 0 || settled >= len(async.outliner.Rows) {
		t.Fatalf("settled cursor %d is outside the projection", settled)
	}
	// The settled row opens its project's block, which is what n and N stop on:
	// an expanded group draws no heading, so the first row of the run is the
	// group's own beginning.
	settledRow := async.outliner.Rows[settled]
	if settled == 0 || settledRow.Group == async.outliner.Rows[settled-1].Group {
		t.Fatalf("the fixture did not settle the cursor on a group start at row %d", settled)
	}

	// n then N while the disk refresh is still in flight: the group jump wraps to
	// the other project's block and back onto the settled one.
	if _, moved := async.outliner.MoveCursorToAdjacentHeader(1); !moved {
		t.Fatal("n found no group to jump to")
	}
	if async.outliner.Cursor == settled {
		t.Fatal("the fixture did not actually move the cursor off the settled group")
	}
	if _, moved := async.outliner.MoveCursorToAdjacentHeader(-1); !moved {
		t.Fatal("N found no group to jump back to")
	}
	if async.outliner.Cursor != settled {
		t.Fatalf("cursor = %d after jumping back, want the settled cursor %d", async.outliner.Cursor, settled)
	}
	returned := async.outliner.CurrentTaskIdentity()

	async.applyReload(t, saveCmd)

	if async.outliner.RelocationHistory.Pending != nil {
		t.Fatal("a group jump away and back left the relocation capture armed")
	}
	if async.outliner.Cursor < 0 || async.outliner.Cursor >= len(async.outliner.Rows) {
		t.Fatalf("cursor = %d is outside the reloaded projection", async.outliner.Cursor)
	}
	if got := async.outliner.CurrentTaskIdentity(); got != returned {
		t.Fatalf("cursor task = %+v, want the group the user returned to %+v", got, returned)
	}
}

// TestInboxAsyncCursorMoveBeforeTheReloadVoidsTheAdvance is the guard's other
// half: a genuine keypress between the commit and the disk reload must not be
// mistaken for the rebuild displacement the mutation caused.
func TestInboxAsyncCursorMoveBeforeTheReloadVoidsTheAdvance(t *testing.T) {
	alphaOne := newBulkPickerTestTask(t, 1, "alpha one")
	alphaTwo := newBulkPickerTestTask(t, 2, "alpha two")
	betaOne := newBulkPickerTestTask(t, 1, "beta one")
	betaTwo := newBulkPickerTestTask(t, 2, "beta two")
	async := newInboxAsyncEnv(t,
		[]*models.Task{alphaOne, alphaTwo},
		[]*models.Task{betaOne, betaTwo})

	cursorToTaskRow(t, async.outliner, alphaTwo.ID)
	saveCmd := async.commitState(t, models.StateDone)

	// One j while the disk refresh is still in flight. It goes through the key's
	// own call rather than writing Cursor, because a genuine keypress also moves
	// the anchor the reload restores from and writing the field moves neither.
	moved := async.outliner.CurrentTaskIdentity()
	async.model.coreMoveCursor("j")
	if async.outliner.CurrentTaskIdentity() == moved {
		t.Fatal("the fixture did not actually move the cursor")
	}
	moved = async.outliner.CurrentTaskIdentity()

	async.applyReload(t, saveCmd)

	if async.outliner.RelocationHistory.Pending != nil {
		t.Fatal("a genuine cursor move left the relocation capture armed")
	}
	if got := async.outliner.CurrentTaskIdentity(); got != moved {
		t.Fatalf("cursor task = %+v, want the row the user moved to %+v", got, moved)
	}
}

// TestAsyncAllModeDepartureDoesNotFollowTheTaskIntoAnotherLane is the
// generalization traveling the real chain. All keeps a task that changes state
// on screen, one lane down, so bare visibility would carry the cursor after it
// through the same commit, refresh, and reload the Inbox cases above take. The
// captured group is what refuses that: the task left the block the keypress was
// aimed from, so the sweep continues from the position it vacated.
func TestAsyncAllModeDepartureDoesNotFollowTheTaskIntoAnotherLane(t *testing.T) {
	one := newBulkPickerTestTask(t, 1, "backlog one")
	two := newBulkPickerTestTask(t, 2, "backlog two")
	three := newBulkPickerTestTask(t, 3, "backlog three")
	async := newRelocationAsyncEnv(t, chrome.ModeAll, models.StateBacklog,
		[]*models.Task{one, two, three})

	cursorToTaskRow(t, async.outliner, two.ID)
	saveCmd := async.commitState(t, models.StateWaiting)
	if async.outliner.RelocationHistory.Pending == nil {
		t.Fatal("committed async state update did not survive its own refresh")
	}
	async.applyReload(t, saveCmd)

	requireCursorRow(t, async.outliner, three.ID)
	requireCursorTask(t, async.outliner, async.identity(t, 0, three.ID))
	if _, visible := async.outliner.RowIndexForTask(async.identity(t, 0, two.ID)); !visible {
		t.Fatal("the departed task left the projection, so this fixture proves nothing about following it")
	}
}

// TestAsyncAllModeBulkSweepFallsBackToThePrecedingSurvivor is the partially
// surviving bulk shape in a lane departure: every task after the cursor is
// itself a target, so Following resolves to nothing and the recovery takes the
// preceding unselected task rather than a row inside its own selection.
func TestAsyncAllModeBulkSweepFallsBackToThePrecedingSurvivor(t *testing.T) {
	one := newBulkPickerTestTask(t, 1, "backlog one")
	two := newBulkPickerTestTask(t, 2, "backlog two")
	three := newBulkPickerTestTask(t, 3, "backlog three")
	async := newRelocationAsyncEnv(t, chrome.ModeAll, models.StateBacklog,
		[]*models.Task{one, two, three})

	cursorToTaskRow(t, async.outliner, two.ID)
	async.outliner.ReplaceSelection([]taskref.TaskIdentity{
		async.identity(t, 0, two.ID),
		async.identity(t, 0, three.ID),
	})
	if !async.outliner.IsMultiSelectActive() {
		t.Fatal("visual selection did not activate multi-select")
	}

	saveCmd := async.commitState(t, models.StateWaiting)
	async.applyReload(t, saveCmd)

	requireCursorRow(t, async.outliner, one.ID)
	requireCursorTask(t, async.outliner, async.identity(t, 0, one.ID))
}
