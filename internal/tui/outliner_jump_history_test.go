package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestOutlinerAsyncRelocationCaptureOnlyArmsOnSuccessfulCurrentTarget(t *testing.T) {
	outlinerView, tasks := jumpHistoryFixture("before", "target", "after")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outlinerView.Cursor = 1
	capture := outlinerView.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})

	wrapped := withOutlinerRelocationCapture(func() tea.Msg {
		return coordinator.BatchTransactionResultMsg{Action: "Set State"}
	}, capture)
	result, ok := wrapped().(coordinator.BatchTransactionResultMsg)
	if !ok || result.Relocation == nil {
		t.Fatalf("successful batch result = %#v, want attached relocation", wrapped())
	}
	outlinerView.ArmOutlinerRelocation(outliner.OutlinerRelocationFrom(result.Relocation))
	setJumpHistoryRows(outlinerView, tasks, "target", "before", "after")
	outlinerView.Cursor = 0
	outlinerView.FinishPendingOutlinerRelocation()
	if len(outlinerView.RelocationHistory.Back) != 1 {
		t.Fatalf("accepted async relocation recorded %d entries, want 1", len(outlinerView.RelocationHistory.Back))
	}

	plain := withOutlinerRelocationCapture(func() tea.Msg { return "failed" }, capture)
	if got := plain(); got != "failed" {
		t.Fatalf("non-success result = %#v, want unchanged", got)
	}

	outliner2, _ := jumpHistoryFixture("before", "target", "after")
	outliner2.Cursor = 1
	capture2 := outliner2.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})
	outliner2.Cursor = 2 // User moved while the mutation was in flight.
	outliner2.ArmOutlinerRelocation(capture2)
	if outliner2.RelocationHistory.Pending != nil {
		t.Fatal("async relocation armed after the cursor left its target")
	}

	outliner3, _ := jumpHistoryFixture("before", "target", "after")
	outliner3.Cursor = 1
	capture3 := outliner3.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})
	outliner3.ArmOutlinerRelocation(capture3)
	outliner3.Cursor = 2 // User moved after commit but before the reload completed.
	outliner3.Population.LoadGen = 1
	outliner3.HandleCrossProjectsLoaded(outliner.CrossProjectsLoadedMsg{
		Projects:   outliner3.Population.Projects,
		Generation: 1,
	})
	if outliner3.RelocationHistory.Pending != nil || len(outliner3.RelocationHistory.Back) != 0 {
		t.Fatal("accepted reload recorded relocation after the cursor left its target")
	}

	outliner4, tasks4 := jumpHistoryFixture("before", "target", "after")
	outliner4.Cursor = 1
	capture4 := outliner4.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})
	outliner4.ArmOutlinerRelocation(capture4)
	// The committed mutation's own refresh took the target off the projection and
	// left the cursor where it fell. That displacement is the mutation's doing, so
	// the capture must survive to the reload that follows it.
	setJumpHistoryRows(outliner4, tasks4, "before", "after")
	outliner4.Cursor = 1
	outliner4.SettleRelocationCursor()
	outliner4.DiscardDisplacedRelocationCapture()
	if outliner4.RelocationHistory.Pending == nil {
		t.Fatal("a settled capture was discarded by the rebuild displacement it caused")
	}
	if got := outliner4.RelocationHistory.Pending.SettledCursor; got != 1 {
		t.Fatalf("settled cursor = %d, want 1", got)
	}
	outliner4.Cursor = 0 // A genuine j after the settle still voids it.
	outliner4.DiscardDisplacedRelocationCapture()
	if outliner4.RelocationHistory.Pending != nil {
		t.Fatal("a cursor move after the settle left the capture armed")
	}
}

func TestCtrlOAndCtrlIDispatchThroughTheSimplifiedOperationPipeline(t *testing.T) {
	model := typedTestApp(t)
	outlinerView := model.crossOutliner
	project := outlinerView.Population.Projects[0]
	target := &models.Task{ID: "target", Title: "target", WorkflowState: models.StateDoing}
	before := &models.Task{ID: "before", Title: "before", WorkflowState: models.StateBacklog}
	after := &models.Task{ID: "after", Title: "after", WorkflowState: models.StateBacklog}
	project.Tasks = []*models.Task{target, before, after}
	project.BuildTaskTree()
	outlinerView.Rows = []outliner.OutlinerRow{
		{Task: target, ProjectID: project.ID},
		{Task: before, ProjectID: project.ID},
		{Task: after, ProjectID: project.ID},
	}
	outlinerView.Cursor = 0
	outlinerView.SyncJumpProjection()
	outlinerView.PushJumpBack(outliner.OutlinerJumpPosition{
		Following: taskref.TaskIdentity{ProjectID: project.ID, TaskID: after.ID},
		RowIndex:  1,
	})

	if handled, _ := model.handleGlobalKeyPress(syntheticCtrlKey('o')); !handled {
		t.Fatal("Ctrl+O was not handled by the simplified input pipeline")
	}
	requireCurrentJumpTask(t, outlinerView, taskref.TaskIdentity{ProjectID: project.ID, TaskID: after.ID})

	if handled, _ := model.handleGlobalKeyPress(syntheticCtrlKey('i')); !handled {
		t.Fatal("Ctrl+I was not handled by the simplified input pipeline")
	}
	requireCurrentJumpTask(t, outlinerView, taskref.TaskIdentity{ProjectID: project.ID, TaskID: target.ID})
}

func jumpHistoryFixture(taskIDs ...string) (*outliner.OutlinerModel, map[string]*models.Task) {
	const projectID = "alpha"
	tasks := make([]*models.Task, 0, len(taskIDs))
	byID := make(map[string]*models.Task, len(taskIDs))
	for _, taskID := range taskIDs {
		task := &models.Task{ID: taskID, Title: taskID, WorkflowState: models.StateBacklog}
		tasks = append(tasks, task)
		byID[taskID] = task
	}
	project := primaryRowsProject(projectID, tasks...)
	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.PrimaryMode = chrome.ModeAll
	setJumpHistoryRows(outlinerView, byID, taskIDs...)
	return outlinerView, byID
}

func setJumpHistoryRows(outlinerView *outliner.OutlinerModel, tasks map[string]*models.Task, taskIDs ...string) {
	outlinerView.Rows = outlinerView.Rows[:0]
	for _, taskID := range taskIDs {
		outlinerView.Rows = append(outlinerView.Rows, outliner.OutlinerRow{Task: tasks[taskID], ProjectID: "alpha"})
	}
}

func requireCurrentJumpTask(t *testing.T, outlinerView *outliner.OutlinerModel, want taskref.TaskIdentity) {
	t.Helper()
	if got := outlinerView.CurrentTaskIdentity(); got != want {
		t.Fatalf("current task = %+v, want %+v (cursor=%d)", got, want, outlinerView.Cursor)
	}
}

func TestDirectStateMutationRecordsRelocationAfterCommittedReload(t *testing.T) {
	before := newBulkPickerTestTask(t, 1, "before")
	target := newBulkPickerTestTask(t, 2, "target")
	after := newBulkPickerTestTask(t, 3, "after")
	env := newAppBulkPickerEnv(t, []*models.Task{before, target, after})
	model := env.appModel()
	model.operations.registry = operation.NewOperationRegistry()
	model.operations.executor = operation.NewOperationExecutor(model.operations.registry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task operations: %v", err)
	}

	outlinerView := outliner.NewOutlinerModel(nil, model.domain.manager, model.domain.runtime)
	outlinerView.Population.Projects = env.projects
	outlinerView.Population.Invalidate()
	outlinerView.SelectOutlinerMode(chrome.ModeAll)
	model.crossOutliner = outlinerView
	model.viewNav.views.Register(presentation.ViewCrossProjectOutliner, outlinerView)
	model.wireOutlinerOperationExecutor(outlinerView)

	targetIdentity := taskref.For(target, env.projects[0])
	targetRow, ok := outlinerView.RowIndexForTask(targetIdentity)
	if !ok {
		t.Fatal("initial All projection does not contain target")
	}
	outlinerView.Cursor = targetRow
	originRow := targetRow

	result := model.operations.executor.Execute(operation.OperationExecutionRequest{
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
	if outlinerView.RelocationHistory.Pending == nil {
		t.Fatal("committed direct state update did not arm relocation history")
	}
	if got := outlinerView.CurrentTaskIdentity(); got != targetIdentity {
		t.Fatalf("refresh cursor = %+v, want relocated target %+v", got, targetIdentity)
	}
	if outlinerView.Cursor == originRow {
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
	loaded, ok := loadedMsg.(outliner.CrossProjectsLoadedMsg)
	if !ok {
		t.Fatalf("nested reload command = %T, want crossProjectsLoadedMsg", loadedMsg)
	}
	outlinerView.HandleCrossProjectsLoaded(loaded)
	if len(outlinerView.RelocationHistory.Back) != 1 {
		t.Fatalf("committed reload recorded %d jump entries, want 1", len(outlinerView.RelocationHistory.Back))
	}
	requireCurrentJumpTask(t, outlinerView, targetIdentity)

	outlinerView.JumpBackward()
	if outlinerView.CurrentTaskIdentity() == targetIdentity {
		t.Fatal("Ctrl+O equivalent did not leave the relocated task")
	}
	outlinerView.JumpForward()
	requireCurrentJumpTask(t, outlinerView, targetIdentity)
}
