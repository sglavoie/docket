package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// inboxAsyncEnv wires the whole asynchronous mutation chain over a real Inbox
// projection: executor, committed batch, in-memory refresh, and the disk reload
// that lands last. These tests exist because that chain, not the synchronous
// planning path, is what $d, $c, $w, $g and every bulk sweep travel.
type inboxAsyncEnv struct {
	env      *appBulkPickerEnv
	model    *AppModel
	outliner *OutlinerModel
}

func newInboxAsyncEnv(t *testing.T, taskGroups ...[]*models.Task) *inboxAsyncEnv {
	t.Helper()
	for _, tasks := range taskGroups {
		for _, task := range tasks {
			task.WorkflowState = models.StateInbox
		}
	}
	env := newAppBulkPickerEnv(t, taskGroups...)
	model := env.appModel()
	model.operationRegistry = NewOperationRegistry()
	model.operationExecutor = NewOperationExecutor(model.operationRegistry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task operations: %v", err)
	}

	outliner := NewOutlinerModel(nil, model.configManager, model.runtime)
	outliner.loadedProjects = env.projects
	outliner.crossForceRebuild = true
	outliner.selectOutlinerMode(ModeInbox)
	model.crossOutliner = outliner
	model.views.Register(ViewCrossProjectOutliner, outliner)
	model.wireOutlinerOperationExecutor(outliner)
	return &inboxAsyncEnv{env: env, model: model, outliner: outliner}
}

// commitState drives operationTaskSetState the way a $-chord does and returns
// the command that carries the disk reload, without applying that reload yet.
func (e *inboxAsyncEnv) commitState(t *testing.T, state models.WorkflowState) tea.Cmd {
	t.Helper()
	result := e.model.operationExecutor.Execute(OperationExecutionRequest{
		ID:       operationTaskSetState,
		Context:  e.model.operationCommandContext(),
		Value:    state,
		HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("execute set state %s: %v", state, result.Err)
	}
	command, ok := result.Message.(OperationCommandResultMsg)
	if !ok {
		t.Fatalf("state result = %T, want OperationCommandResultMsg", result.Message)
	}
	batch := runBatchResult(t, command.Cmd)
	if batch.outlinerRelocation == nil {
		t.Fatal("successful async state update carried no relocation capture")
	}
	return e.model.handleBatchTransactionResult(batch)
}

// applyReload unwraps the nested reload command and feeds the loaded snapshot
// back through the accepted-reload path, which is where the advance resolves.
func (e *inboxAsyncEnv) applyReload(t *testing.T, saveCmd tea.Cmd) {
	t.Helper()
	msg := saveCmd()
	for depth := 0; depth < 4; depth++ {
		batch, batched := msg.(tea.BatchMsg)
		if !batched || len(batch) == 0 {
			break
		}
		msg = batch[0]()
	}
	loaded, ok := msg.(crossProjectsLoadedMsg)
	if !ok {
		t.Fatalf("nested reload command = %T, want crossProjectsLoadedMsg", msg)
	}
	e.outliner.handleCrossProjectsLoaded(loaded)
}

func (e *inboxAsyncEnv) identity(t *testing.T, projectIndex int, taskID string) TaskIdentity {
	t.Helper()
	project := e.env.projects[projectIndex]
	for _, task := range project.Tasks {
		if task.ID == taskID {
			return taskIdentityFor(task, project)
		}
	}
	t.Fatalf("task %q is not in project %d", taskID, projectIndex)
	return TaskIdentity{}
}

// TestInboxAsyncDoneAdvancesAcrossAProjectGroupHeader is the reconciliation: $d
// travels the asynchronous route and must land exactly where Session 1's
// synchronous $b lands from the same row — the first task of the next group,
// never that group's header.
func TestInboxAsyncDoneAdvancesAcrossAProjectGroupHeader(t *testing.T) {
	alphaOne := newBulkPickerTestTask(t, 1, "alpha one")
	alphaTwo := newBulkPickerTestTask(t, 2, "alpha two")
	betaOne := newBulkPickerTestTask(t, 1, "beta one")
	betaTwo := newBulkPickerTestTask(t, 2, "beta two")
	async := newInboxAsyncEnv(t,
		[]*models.Task{alphaOne, alphaTwo},
		[]*models.Task{betaOne, betaTwo})

	inboxCursorTo(t, async.outliner, alphaTwo.ID)
	saveCmd := async.commitState(t, models.StateDone)
	if async.outliner.relocationHistory.pending == nil {
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

	inboxCursorTo(t, async.outliner, alphaTwo.ID)
	async.outliner.ReplaceSelection([]TaskIdentity{
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

	inboxCursorTo(t, async.outliner, alphaTwo.ID)
	saveCmd := async.commitState(t, models.StateDone)

	// One j while the disk refresh is still in flight.
	moved := async.outliner.currentTaskIdentity()
	async.outliner.cursor++
	if async.outliner.currentTaskIdentity() == moved {
		t.Fatal("the fixture did not actually move the cursor")
	}
	moved = async.outliner.currentTaskIdentity()

	async.applyReload(t, saveCmd)

	if async.outliner.relocationHistory.pending != nil {
		t.Fatal("a genuine cursor move left the relocation capture armed")
	}
	if got := async.outliner.currentTaskIdentity(); got != moved {
		t.Fatalf("cursor task = %+v, want the row the user moved to %+v", got, moved)
	}
}
