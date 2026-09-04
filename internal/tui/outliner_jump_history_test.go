package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func jumpHistoryFixture(taskIDs ...string) (*OutlinerModel, map[string]*models.Task) {
	const projectID = "alpha"
	tasks := make([]*models.Task, 0, len(taskIDs))
	byID := make(map[string]*models.Task, len(taskIDs))
	for _, taskID := range taskIDs {
		task := &models.Task{ID: taskID, Title: taskID, WorkflowState: models.StateBacklog}
		tasks = append(tasks, task)
		byID[taskID] = task
	}
	project := primaryRowsProject(projectID, tasks...)
	outliner, _ := primaryRowsOutliner(project)
	outliner.primaryMode = chrome.ModeAll
	setJumpHistoryRows(outliner, byID, taskIDs...)
	return outliner, byID
}

func setJumpHistoryRows(outliner *OutlinerModel, tasks map[string]*models.Task, taskIDs ...string) {
	outliner.rows = outliner.rows[:0]
	for _, taskID := range taskIDs {
		outliner.rows = append(outliner.rows, outlinerRow{task: tasks[taskID], ProjectID: "alpha"})
	}
}

func requireCurrentJumpTask(t *testing.T, outliner *OutlinerModel, want taskref.TaskIdentity) {
	t.Helper()
	if got := outliner.CurrentTaskIdentity(); got != want {
		t.Fatalf("current task = %+v, want %+v (cursor=%d)", got, want, outliner.cursor)
	}
}

func TestOutlinerRelocationJumpBackwardAndForward(t *testing.T) {
	outliner, tasks := jumpHistoryFixture("before", "target", "after", "tail")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outliner.cursor = 1
	outliner.viewScrollOffset = 7

	capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})
	if capture == nil {
		t.Fatal("relocation capture = nil")
	}
	if got := capture.Origin.Following; got != (taskref.TaskIdentity{ProjectID: "alpha", TaskID: "after"}) {
		t.Fatalf("following anchor = %+v, want after", got)
	}
	if got := capture.Origin.Preceding; got != (taskref.TaskIdentity{ProjectID: "alpha", TaskID: "before"}) {
		t.Fatalf("preceding anchor = %+v, want before", got)
	}

	setJumpHistoryRows(outliner, tasks, "target", "before", "after", "tail")
	outliner.cursor = 0 // The mutation refresh follows the relocated task.
	_, recorded := outliner.finishOutlinerRelocation(capture, true)
	if !recorded {
		t.Fatal("relocation moved rows but did not report recorded origin history")
	}
	if len(outliner.relocationHistory.back) != 1 || len(outliner.relocationHistory.forward) != 0 {
		t.Fatalf("history after relocation = back %d forward %d, want 1/0",
			len(outliner.relocationHistory.back), len(outliner.relocationHistory.forward))
	}
	requireCurrentJumpTask(t, outliner, target)

	outliner.jumpBackward()
	requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: "alpha", TaskID: "after"})
	if outliner.viewScrollOffset != 7 {
		t.Fatalf("backward scroll offset = %d, want 7", outliner.viewScrollOffset)
	}

	outliner.jumpForward()
	requireCurrentJumpTask(t, outliner, target)
}

func TestOutlinerRelocationReportsNoOriginWhenRowDoesNotMove(t *testing.T) {
	outliner, _ := jumpHistoryFixture("target", "after")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outliner.cursor = 0
	capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})
	if _, recorded := outliner.finishOutlinerRelocation(capture, true); recorded {
		t.Fatal("unchanged row advertised recorded origin history")
	}
}

func TestOutlinerRelocationOriginFallsBackToPrecedingThenNearestRow(t *testing.T) {
	t.Run("preceding", func(t *testing.T) {
		outliner, tasks := jumpHistoryFixture("before", "target", "after")
		target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
		outliner.cursor = 1
		capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})

		setJumpHistoryRows(outliner, tasks, "target", "before")
		outliner.finishOutlinerRelocation(capture, true)
		outliner.jumpBackward()
		requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: "alpha", TaskID: "before"})
	})

	t.Run("numeric", func(t *testing.T) {
		outliner, tasks := jumpHistoryFixture("before", "target", "after", "survivor")
		target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
		outliner.cursor = 1
		capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})

		setJumpHistoryRows(outliner, tasks, "target", "survivor")
		outliner.finishOutlinerRelocation(capture, true)
		outliner.jumpBackward()
		requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: "alpha", TaskID: "survivor"})
	})
}

func TestOutlinerRelocationUsesProjectQualifiedTaskAnchors(t *testing.T) {
	alpha := primaryRowsProject("alpha", &models.Task{ID: "same", Title: "alpha same"}, &models.Task{ID: "target", Title: "target"})
	beta := primaryRowsProject("beta", &models.Task{ID: "same", Title: "beta same"})
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.primaryMode = chrome.ModeAll
	outliner.rows = []outlinerRow{
		{task: alpha.Tasks[0], ProjectID: alpha.ID},
		{task: alpha.Tasks[1], ProjectID: alpha.ID},
		{task: beta.Tasks[0], ProjectID: beta.ID},
	}
	outliner.cursor = 1
	target := taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: "target"}
	capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})

	outliner.rows = []outlinerRow{
		{task: alpha.Tasks[1], ProjectID: alpha.ID},
		{task: alpha.Tasks[0], ProjectID: alpha.ID},
		{task: beta.Tasks[0], ProjectID: beta.ID},
	}
	outliner.finishOutlinerRelocation(capture, true)
	outliner.jumpBackward()
	requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: beta.ID, TaskID: "same"})
}

func TestOutlinerRelocationHistoryClearsOnProjectionChange(t *testing.T) {
	outliner, tasks := jumpHistoryFixture("before", "target", "after")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outliner.cursor = 1
	capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})
	setJumpHistoryRows(outliner, tasks, "target", "before", "after")
	outliner.finishOutlinerRelocation(capture, true)

	outliner.primaryMode = chrome.ModeInbox
	outliner.syncJumpProjection()
	if len(outliner.relocationHistory.back) != 0 || len(outliner.relocationHistory.forward) != 0 || outliner.relocationHistory.pending != nil {
		t.Fatalf("projection change retained history: %+v", outliner.relocationHistory)
	}
}

func TestOutlinerOrdinaryMotionPreservesForwardHistory(t *testing.T) {
	outliner, tasks := jumpHistoryFixture("before", "target", "after", "tail")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outliner.cursor = 1
	capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})
	setJumpHistoryRows(outliner, tasks, "target", "before", "after", "tail")
	outliner.finishOutlinerRelocation(capture, true)
	outliner.jumpBackward()

	outliner.cursor = 3 // Ordinary j/k-style movement must not clear Ctrl+I.
	outliner.jumpForward()
	requireCurrentJumpTask(t, outliner, target)
	outliner.jumpBackward()
	requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: "alpha", TaskID: "tail"})
}

func TestOutlinerRelocationHistoryIgnoresNoOpAndHiddenDestinations(t *testing.T) {
	t.Run("same row", func(t *testing.T) {
		outliner, _ := jumpHistoryFixture("target", "after")
		target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
		capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})
		outliner.finishOutlinerRelocation(capture, true)
		if len(outliner.relocationHistory.back) != 0 {
			t.Fatalf("same-row mutation recorded %d entries", len(outliner.relocationHistory.back))
		}
	})

	t.Run("hidden", func(t *testing.T) {
		outliner, tasks := jumpHistoryFixture("before", "target", "after")
		target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
		outliner.cursor = 1
		capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})
		setJumpHistoryRows(outliner, tasks, "before", "after")
		outliner.finishOutlinerRelocation(capture, true)
		if len(outliner.relocationHistory.back) != 0 {
			t.Fatalf("hidden destination recorded %d entries", len(outliner.relocationHistory.back))
		}
	})
}

func TestOutlinerRelocationHistoryIsBounded(t *testing.T) {
	var stack []outlinerJumpPosition
	for i := 0; i < outlinerJumpHistoryLimit+25; i++ {
		stack = appendBoundedJump(stack, outlinerJumpPosition{RowIndex: i})
	}
	if len(stack) != outlinerJumpHistoryLimit {
		t.Fatalf("history length = %d, want %d", len(stack), outlinerJumpHistoryLimit)
	}
	if got := stack[0].RowIndex; got != 25 {
		t.Fatalf("oldest retained row = %d, want 25", got)
	}
}

func TestOutlinerAsyncRelocationCaptureOnlyArmsOnSuccessfulCurrentTarget(t *testing.T) {
	outliner, tasks := jumpHistoryFixture("before", "target", "after")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outliner.cursor = 1
	capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{target})

	wrapped := withOutlinerRelocationCapture(func() tea.Msg {
		return coordinator.BatchTransactionResultMsg{Action: "Set State"}
	}, capture)
	result, ok := wrapped().(coordinator.BatchTransactionResultMsg)
	if !ok || result.Relocation == nil {
		t.Fatalf("successful batch result = %#v, want attached relocation", wrapped())
	}
	outliner.armOutlinerRelocation(outlinerRelocationFrom(result.Relocation))
	setJumpHistoryRows(outliner, tasks, "target", "before", "after")
	outliner.cursor = 0
	outliner.finishPendingOutlinerRelocation()
	if len(outliner.relocationHistory.back) != 1 {
		t.Fatalf("accepted async relocation recorded %d entries, want 1", len(outliner.relocationHistory.back))
	}

	plain := withOutlinerRelocationCapture(func() tea.Msg { return "failed" }, capture)
	if got := plain(); got != "failed" {
		t.Fatalf("non-success result = %#v, want unchanged", got)
	}

	outliner2, _ := jumpHistoryFixture("before", "target", "after")
	outliner2.cursor = 1
	capture2 := outliner2.captureOutlinerRelocation([]taskref.TaskIdentity{target})
	outliner2.cursor = 2 // User moved while the mutation was in flight.
	outliner2.armOutlinerRelocation(capture2)
	if outliner2.relocationHistory.pending != nil {
		t.Fatal("async relocation armed after the cursor left its target")
	}

	outliner3, _ := jumpHistoryFixture("before", "target", "after")
	outliner3.cursor = 1
	capture3 := outliner3.captureOutlinerRelocation([]taskref.TaskIdentity{target})
	outliner3.armOutlinerRelocation(capture3)
	outliner3.cursor = 2 // User moved after commit but before the reload completed.
	outliner3.crossLoadGen = 1
	outliner3.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   outliner3.loadedProjects,
		generation: 1,
	})
	if outliner3.relocationHistory.pending != nil || len(outliner3.relocationHistory.back) != 0 {
		t.Fatal("accepted reload recorded relocation after the cursor left its target")
	}

	outliner4, tasks4 := jumpHistoryFixture("before", "target", "after")
	outliner4.cursor = 1
	capture4 := outliner4.captureOutlinerRelocation([]taskref.TaskIdentity{target})
	outliner4.armOutlinerRelocation(capture4)
	// The committed mutation's own refresh took the target off the projection and
	// left the cursor where it fell. That displacement is the mutation's doing, so
	// the capture must survive to the reload that follows it.
	setJumpHistoryRows(outliner4, tasks4, "before", "after")
	outliner4.cursor = 1
	outliner4.settleRelocationCursor()
	outliner4.discardDisplacedRelocationCapture()
	if outliner4.relocationHistory.pending == nil {
		t.Fatal("a settled capture was discarded by the rebuild displacement it caused")
	}
	if got := outliner4.relocationHistory.pending.SettledCursor; got != 1 {
		t.Fatalf("settled cursor = %d, want 1", got)
	}
	outliner4.cursor = 0 // A genuine j after the settle still voids it.
	outliner4.discardDisplacedRelocationCapture()
	if outliner4.relocationHistory.pending != nil {
		t.Fatal("a cursor move after the settle left the capture armed")
	}
}

func TestCtrlOAndCtrlIDispatchThroughTheSimplifiedOperationPipeline(t *testing.T) {
	model := typedTestApp(t)
	outliner := model.crossOutliner
	project := outliner.loadedProjects[0]
	target := &models.Task{ID: "target", Title: "target", WorkflowState: models.StateDoing}
	before := &models.Task{ID: "before", Title: "before", WorkflowState: models.StateBacklog}
	after := &models.Task{ID: "after", Title: "after", WorkflowState: models.StateBacklog}
	project.Tasks = []*models.Task{target, before, after}
	project.BuildTaskTree()
	outliner.rows = []outlinerRow{
		{task: target, ProjectID: project.ID},
		{task: before, ProjectID: project.ID},
		{task: after, ProjectID: project.ID},
	}
	outliner.cursor = 0
	outliner.syncJumpProjection()
	outliner.pushJumpBack(outlinerJumpPosition{
		Following: taskref.TaskIdentity{ProjectID: project.ID, TaskID: after.ID},
		RowIndex:  1,
	})

	if handled, _ := model.handleGlobalKeyPress(syntheticCtrlKey('o')); !handled {
		t.Fatal("Ctrl+O was not handled by the simplified input pipeline")
	}
	requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: project.ID, TaskID: after.ID})

	if handled, _ := model.handleGlobalKeyPress(syntheticCtrlKey('i')); !handled {
		t.Fatal("Ctrl+I was not handled by the simplified input pipeline")
	}
	requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: project.ID, TaskID: target.ID})
}

func TestDirectStateMutationRecordsRelocationAfterCommittedReload(t *testing.T) {
	before := newBulkPickerTestTask(t, 1, "before")
	target := newBulkPickerTestTask(t, 2, "target")
	after := newBulkPickerTestTask(t, 3, "after")
	env := newAppBulkPickerEnv(t, []*models.Task{before, target, after})
	model := env.appModel()
	model.operationRegistry = operation.NewOperationRegistry()
	model.operationExecutor = operation.NewOperationExecutor(model.operationRegistry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task operations: %v", err)
	}

	outliner := NewOutlinerModel(nil, model.configManager, model.runtime)
	outliner.loadedProjects = env.projects
	outliner.crossForceRebuild = true
	outliner.selectOutlinerMode(chrome.ModeAll)
	model.crossOutliner = outliner
	model.views.Register(presentation.ViewCrossProjectOutliner, outliner)
	model.wireOutlinerOperationExecutor(outliner)

	targetIdentity := taskref.For(target, env.projects[0])
	targetRow, ok := outliner.rowIndexForTask(targetIdentity)
	if !ok {
		t.Fatal("initial All projection does not contain target")
	}
	outliner.cursor = targetRow
	originRow := targetRow

	result := model.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:       operation.OperationTaskSetState,
		Context:  model.operationCommandContext(),
		Value:    models.StateDoing,
		HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("execute wg state update: %v", result.Err)
	}
	command, ok := result.Message.(operation.OperationCommandResultMsg)
	if !ok {
		t.Fatalf("state result = %T, want OperationCommandResultMsg", result.Message)
	}
	batch := runBatchResult(t, command.Cmd)
	if batch.Relocation == nil {
		t.Fatal("successful direct state update carried no relocation capture")
	}

	saveCmd := model.handleBatchTransactionResult(batch)
	if outliner.relocationHistory.pending == nil {
		t.Fatal("committed direct state update did not arm relocation history")
	}
	if got := outliner.CurrentTaskIdentity(); got != targetIdentity {
		t.Fatalf("refresh cursor = %+v, want relocated target %+v", got, targetIdentity)
	}
	if outliner.cursor == originRow {
		t.Fatalf("Doing update left target at row %d; fixture did not exercise relocation", originRow)
	}

	loadedMsg := saveCmd()
	for depth := 0; depth < 4; depth++ {
		batch, batched := loadedMsg.(tea.BatchMsg)
		if !batched || len(batch) == 0 {
			break
		}
		loadedMsg = batch[0]()
	}
	loaded, ok := loadedMsg.(crossProjectsLoadedMsg)
	if !ok {
		t.Fatalf("nested reload command = %T, want crossProjectsLoadedMsg", loadedMsg)
	}
	outliner.handleCrossProjectsLoaded(loaded)
	if len(outliner.relocationHistory.back) != 1 {
		t.Fatalf("committed reload recorded %d jump entries, want 1", len(outliner.relocationHistory.back))
	}
	requireCurrentJumpTask(t, outliner, targetIdentity)

	outliner.jumpBackward()
	if outliner.CurrentTaskIdentity() == targetIdentity {
		t.Fatal("Ctrl+O equivalent did not leave the relocated task")
	}
	outliner.jumpForward()
	requireCurrentJumpTask(t, outliner, targetIdentity)
}
