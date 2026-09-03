package tui

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestFrozenBulkTagPatchPreservesSelectionAcrossCursorMoveAndRebuild(t *testing.T) {
	first := newBulkPickerTestTask(t, 1, "first", "common", "first-only")
	second := newBulkPickerTestTask(t, 1, "second", "common", "second-only")
	env := newAppBulkPickerEnv(t, []*models.Task{first}, []*models.Task{second})
	model := env.appModel()
	model.operationRegistry = NewOperationRegistry()
	model.operationExecutor = NewOperationExecutor(model.operationRegistry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	outliner := NewOutlinerModel(nil, model.configManager, model.runtime)
	outliner.loadedProjects = env.projects
	outliner.rows = []outlinerRow{
		{task: first, ProjectID: env.projects[0].ID},
		{task: second, ProjectID: env.projects[1].ID},
	}
	outliner.multiSelectMode = true
	outliner.SelectTaskIdentity(taskref.For(first, env.projects[0]))
	outliner.SelectTaskIdentity(taskref.For(second, env.projects[1]))
	model.crossOutliner = outliner
	model.views.Register(presentation.ViewCrossProjectOutliner, outliner)

	context := model.operationCommandContext()
	outliner.cursor = 1
	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:       operationTaskSetTags,
		Context:  context,
		Value:    OperationTagsValue{Add: []string{"first-only"}, Remove: []string{"common"}},
		HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("execute frozen tag patch: %v", result.Err)
	}
	command, ok := result.Message.(OperationCommandResultMsg)
	if !ok {
		t.Fatalf("result message = %T, want OperationCommandResultMsg", result.Message)
	}
	batch := runBatchResult(t, command.Cmd)
	model.handleBatchTransactionResult(batch)

	if got := env.reload(t, 0).GetTaskByID(first.ID).Tags; !reflect.DeepEqual(got, []string{"first-only"}) {
		t.Fatalf("first tags = %v, want [first-only]", got)
	}
	if got := env.reload(t, 1).GetTaskByID(second.ID).Tags; !reflect.DeepEqual(got, []string{"second-only", "first-only"}) {
		t.Fatalf("second tags = %v, want preserved partial membership plus first-only", got)
	}
	if !outliner.multiSelectMode || outliner.SelectedCount() != 2 {
		t.Fatalf("selection after committed rebuild = active %v count %d, want active/2", outliner.multiSelectMode, outliner.SelectedCount())
	}
}

func TestTaskActionNativeOperationRequestsOneListSurfaceValue(t *testing.T) {
	model := newUITestApp()
	model.operationRegistry = NewOperationRegistry()
	model.operationExecutor = NewOperationExecutor(model.operationRegistry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	context := taskActionTestContext()
	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationTaskSetState,
		Context: context,
	})
	if result.Err != nil {
		t.Fatalf("Execute: %v", result.Err)
	}
	if result.ValueRequired == nil || result.ValueRequired.Provider != ValueProviderState {
		t.Fatalf("value request = %+v, want state provider", result.ValueRequired)
	}
	if result.Message != nil || result.ConfirmationRequired != nil {
		t.Fatalf("unexpected pre-value result: %+v", result)
	}
}

func TestBulkTaskValueOperationUsesQualifiedTargetsAndOneUndoEntry(t *testing.T) {
	taskA := newBulkPickerTestTask(t, 1, "alpha")
	taskB := newBulkPickerTestTask(t, 1, "beta")
	taskA.ID = "shared-task-id"
	taskB.ID = "shared-task-id"
	taskA.Priority = models.PriorityLow
	taskB.Priority = models.PriorityLow
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	model := env.appModel()
	model.operationRegistry = NewOperationRegistry()
	model.operationExecutor = NewOperationExecutor(model.operationRegistry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	context := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(CapabilitySimplifiedInput)
	context.Selection.Visual = true
	for index, task := range []*models.Task{taskA, taskB} {
		project := env.projects[index]
		context.Selection.Tasks = append(
			context.Selection.Tasks,
			taskref.For(task, project),
		)
		context.Selection.Entities = append(
			context.Selection.Entities,
			taskOperationEntity(task, project),
		)
	}
	context.Entity = context.Selection.Entities[0]

	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:       operationTaskSetPriority,
		Context:  context,
		Value:    1,
		HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("Execute: %v", result.Err)
	}
	command, ok := result.Message.(OperationCommandResultMsg)
	if !ok {
		t.Fatalf("result message = %T, want OperationCommandResultMsg", result.Message)
	}
	batch := runBatchResult(t, command.Cmd)
	if len(batch.TouchedProjectIDs) != 2 {
		t.Fatalf("touched projects = %v, want both selected projects", batch.TouchedProjectIDs)
	}
	if len(batch.ClearSelectionTaskLocators) != 0 {
		t.Fatalf("native bulk action unexpectedly cleared selection: %v", batch.ClearSelectionTaskLocators)
	}
	for index, task := range []*models.Task{taskA, taskB} {
		if got := env.reload(t, index).GetTaskByID(task.ID); got == nil || got.Priority != 1 {
			t.Fatalf("project %d priority = %v, want 1", index, got)
		}
	}

	env.undo(t)
	for index, task := range []*models.Task{taskA, taskB} {
		if got := env.reload(t, index).GetTaskByID(task.ID); got == nil || got.Priority != models.PriorityLow {
			t.Fatalf("undo project %d priority = %v, want %d", index, got, models.PriorityLow)
		}
	}
}

func TestNarrowDescriptionEditorCommitsThroughCommonApplicationOperation(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Before")
	task.Description = "Old body"
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	model := env.appModel()
	project := env.projects[0]

	for _, testCase := range []struct {
		operation app.TaskOperation
		value     string
	}{{operation: app.TaskOperationEditDescription, value: "New body"}} {
		handled, _ := model.handleEditorMessages(chrome.ShowTaskTextEditorMsg{
			Task: task, Project: project, Operation: testCase.operation,
		})
		if !handled || model.currentView() != presentation.ViewTextEditor {
			t.Fatalf("%s did not open the narrow editor", testCase.operation)
		}
		handled, cmd := model.handleEditorMessages(chrome.TextEditorDoneMsg{Value: testCase.value})
		if !handled || cmd == nil {
			t.Fatalf("%s did not produce a service command", testCase.operation)
		}
		runBatchResult(t, cmd)
	}

	fresh := env.reload(t, 0).GetTaskByID(task.ID)
	if fresh == nil || fresh.Title != "Before" || fresh.Description != "New body" {
		t.Fatalf("persisted narrow edits = %+v", fresh)
	}
}
