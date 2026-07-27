package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskStatePickerContainsExactCanonicalStatesAndTransitionReasons(t *testing.T) {
	request := OperationExecutionRequest{
		ID:      operationTaskSetState,
		Context: taskActionTestContext(models.StateDone),
		Source:  OperationSourceActionSheet,
	}
	picker, err := NewTaskStatePickerModel(request, models.StateDone, "Completed task")
	if err != nil {
		t.Fatalf("NewTaskStatePickerModel: %v", err)
	}
	items := picker.surface.Items()
	if len(items) != len(models.AllWorkflowStates()) {
		t.Fatalf("state items = %d, want %d", len(items), len(models.AllWorkflowStates()))
	}
	for index, state := range models.AllWorkflowStates() {
		if got, ok := items[index].Data.(models.WorkflowState); !ok || got != state {
			t.Errorf("state item %d payload = %#v, want %s", index, items[index].Data, state)
		}
		if state == models.StateInbox && items[index].DisabledReason != "" {
			t.Errorf("reopen target Inbox disabled: %q", items[index].DisabledReason)
		}
		if state != models.StateInbox && items[index].DisabledReason == "" {
			t.Errorf("terminal transition to %s has no disabled reason", state)
		}
	}
}

func TestOperationValuePickerMultiSelectionReturnsStableValues(t *testing.T) {
	source := ListSource{Items: []ListItem{
		{ID: "tag:z", PrimaryText: "z", Data: "z"},
		{ID: "tag:a", PrimaryText: "a", Data: "a"},
	}}
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationTaskSetTags},
		ValueProviderTags,
		"Set Tags",
		"Task",
		source,
		true,
		[]ListItemID{"tag:z", "tag:a"},
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}
	_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !dismissed {
		t.Fatal("Enter did not dismiss a completed multi picker")
	}
	selected, ok := result.(OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("result = %T, want OperationValueSelectedMsg", result)
	}
	if got, want := selected.Value, []string{"a", "z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected values = %#v, want %#v", got, want)
	}
}

func TestTaskActionNativeOperationRequestsOneListSurfaceValue(t *testing.T) {
	model := newUITestApp()
	model.operationRegistry = NewOperationRegistry()
	model.operationExecutor = NewOperationExecutor(model.operationRegistry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	context := taskActionTestContext(models.StateReady)
	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationTaskSetState,
		Context: context,
		Source:  OperationSourceActionSheet,
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

func TestTaskActionSelectionOpensValuePickerAndCancelReturnsToSheet(t *testing.T) {
	model := newUITestApp()
	task := &models.Task{
		ID:            "task",
		Title:         "Task",
		WorkflowState: models.StateReady,
	}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	project.BuildTaskTree()
	setTestAppProjects(model, project)
	model.operationRegistry = NewOperationRegistry()
	model.operationExecutor = NewOperationExecutor(model.operationRegistry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	model.taskActionSheet = &TaskActionSheetModel{}

	handled, cmd := model.handleOperationOverlayMessage(TaskActionSheetExecuteMsg{
		Request: OperationExecutionRequest{
			ID:      operationTaskSetState,
			Context: taskActionTestContext(models.StateReady),
			Source:  OperationSourceActionSheet,
		},
	})
	if !handled || cmd != nil {
		t.Fatalf("sheet selection = handled %v, cmd %T; want picker opened synchronously", handled, cmd)
	}
	if model.operationValuePicker == nil {
		t.Fatal("state action did not open its ListSurface value picker")
	}
	if model.taskActionSheet == nil {
		t.Fatal("opening a value picker discarded the action sheet beneath it")
	}

	handled, cmd = model.handleOperationOverlayMessage(OperationValueCanceledMsg{})
	if !handled || cmd != nil {
		t.Fatalf("picker cancel = handled %v, cmd %T; want handled with no command", handled, cmd)
	}
	if model.operationValuePicker != nil {
		t.Fatal("cancel did not close the value picker")
	}
	if model.taskActionSheet == nil {
		t.Fatal("cancel did not return to the action sheet")
	}
}

func TestDependencyValueSourcePreservesExternalExistingEdges(t *testing.T) {
	task := &models.Task{
		ID:            "task",
		Title:         "Task",
		WorkflowState: models.StateReady,
		DependsOn:     []string{"other-project:blocker"},
	}
	local := &models.Task{
		ID:            "local",
		Title:         "Local",
		WorkflowState: models.StateReady,
	}
	project := &models.Project{ID: "project", Tasks: []*models.Task{task, local}}
	project.BuildTaskTree()

	source, selected := dependencyOperationListSource(task, project)
	if !slicesContainsListItemID(selected, "dependency:other-project:blocker") {
		t.Fatalf("selected dependencies = %v, missing external edge", selected)
	}
	found := false
	for _, item := range source.Items {
		if item.ID == "dependency:other-project:blocker" && item.Data == "other-project:blocker" {
			found = true
		}
	}
	if !found {
		t.Fatal("external existing dependency is not represented by the value source")
	}
}

func TestBulkTaskValueOperationUsesQualifiedTargetsAndOneUndoEntry(t *testing.T) {
	taskA := newBulkPickerTestTask(t, 1, "alpha")
	taskB := newBulkPickerTestTask(t, 1, "beta")
	taskA.ID = "shared-task-id"
	taskB.ID = "shared-task-id"
	taskA.Priority = 5
	taskB.Priority = 5
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	model := env.appModel()
	model.operationRegistry = NewOperationRegistry()
	model.operationExecutor = NewOperationExecutor(model.operationRegistry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	context := PresentationCommandContext(ViewCrossProjectOutliner)
	context.Capabilities.Add(CapabilitySimplifiedInput)
	context.Selection.Visual = true
	for index, task := range []*models.Task{taskA, taskB} {
		project := env.projects[index]
		context.Selection.Tasks = append(
			context.Selection.Tasks,
			taskIdentityFor(task, project),
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
		Source:   OperationSourceBulk,
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
		if got := env.reload(t, index).GetTaskByID(task.ID); got == nil || got.Priority != 5 {
			t.Fatalf("undo project %d priority = %v, want 5", index, got)
		}
	}
}

func slicesContainsListItemID(values []ListItemID, want ListItemID) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
