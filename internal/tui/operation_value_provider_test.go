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

func TestContextActionSheetValuePickerCancelReturnsToSheet(t *testing.T) {
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
	sheet, err := NewTaskActionSheetModel(
		model.operationRegistry,
		model.operationExecutor,
		taskActionTestContext(models.StateReady),
		"Task",
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	model.contextActionSheet = sheet
	if !sheet.surface.SetCursorToID(ListItemID("task-action:" + string(operationTaskSetSnooze))) {
		t.Fatal("task sheet is missing the snooze action")
	}

	_, dismissed, raw := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed {
		t.Fatal("a pending value picker dismissed the sheet")
	}
	handled, cmd := model.handleOperationOverlayMessage(raw)
	if !handled || cmd != nil {
		t.Fatalf("sheet selection = handled %v, cmd %T; want picker opened synchronously", handled, cmd)
	}
	if model.operationValuePicker == nil {
		t.Fatal("snooze action did not open its ListSurface value picker")
	}
	if model.contextActionSheet == nil {
		t.Fatal("opening a value picker discarded the action sheet beneath it")
	}

	handled, cmd = model.handleOperationOverlayMessage(OperationValueCanceledMsg{})
	if !handled || cmd != nil {
		t.Fatalf("picker cancel = handled %v, cmd %T; want handled with no command", handled, cmd)
	}
	if model.operationValuePicker != nil {
		t.Fatal("cancel did not close the value picker")
	}
	if model.contextActionSheet == nil {
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

// newTaskStructureTestProject lays out one epic with a story child followed by a
// loose root task, which is the smallest shape covering every legal and illegal
// indent/outdent/reparent case.
func newTaskStructureTestProject() *models.Project {
	epic := &models.Task{
		ID: "epic", ShortID: 1, Title: "Epic",
		Type: models.TaskTypeEpic, WorkflowState: models.StateReady,
	}
	story := &models.Task{
		ID: "story", ShortID: 2, Title: "Story", ParentID: strPtr("epic"),
		Type: models.TaskTypeStory, WorkflowState: models.StateReady,
	}
	loose := &models.Task{
		ID: "loose", ShortID: 3, Title: "Loose",
		Type: models.TaskTypeTask, WorkflowState: models.StateReady,
	}
	project := &models.Project{
		ID: "alpha", Name: "Alpha", Slug: "alpha",
		Tasks: []*models.Task{epic, story, loose},
	}
	project.BuildTaskTree()
	return project
}

func taskOperationTestContext(task *models.Task, project *models.Project) CommandContext {
	context := PresentationCommandContext(ViewCrossProjectOutliner)
	context.Capabilities.Add(CapabilitySimplifiedInput)
	context.Entity = taskOperationEntity(task, project)
	return context
}

func taskOperationApplicable(
	t *testing.T,
	model *AppModel,
	id OperationID,
	task *models.Task,
	project *models.Project,
) bool {
	t.Helper()
	descriptor, ok := model.operationRegistry.Descriptor(id)
	if !ok {
		t.Fatalf("operation %s is not registered", id)
	}
	return descriptor.Applicable(taskOperationTestContext(task, project))
}

func TestTaskOperationIndentAndOutdentApplicabilityFollowsHierarchyLegality(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project)

	for _, testCase := range []struct {
		taskID string
		id     OperationID
		want   bool
	}{
		{taskID: "loose", id: operationTaskIndent, want: true},
		{taskID: "epic", id: operationTaskIndent, want: false},
		{taskID: "story", id: operationTaskOutdent, want: true},
		{taskID: "loose", id: operationTaskOutdent, want: false},
	} {
		task := project.GetTaskByID(testCase.taskID)
		if task == nil {
			t.Fatalf("fixture is missing task %s", testCase.taskID)
		}
		if got := taskOperationApplicable(t, model, testCase.id, task, project); got != testCase.want {
			t.Errorf("%s applicable for %s = %v, want %v", testCase.id, testCase.taskID, got, testCase.want)
		}
	}
}

func TestTaskOperationParentAndMoveApplicabilityRequireCandidates(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project)
	loose := project.GetTaskByID("loose")
	epic := project.GetTaskByID("epic")

	if !taskOperationApplicable(t, model, operationTaskSetParent, loose, project) {
		t.Error("set-parent is unavailable for a task with valid parent candidates")
	}
	if taskOperationApplicable(t, model, operationTaskSetParent, epic, project) {
		t.Error("set-parent is offered for a root epic that nothing can parent")
	}
	if taskOperationApplicable(t, model, operationTaskMoveProject, loose, project) {
		t.Error("move-project is offered with no other active project")
	}

	other := &models.Project{ID: "beta", Name: "Beta", Slug: "beta"}
	setTestAppProjects(model, project, other)
	if !taskOperationApplicable(t, model, operationTaskMoveProject, loose, project) {
		t.Error("move-project stayed unavailable after a destination project appeared")
	}
	if got := model.moveProjectCandidates(project); len(got) != 1 || got[0].ID != other.ID {
		t.Errorf("move destinations = %v, want only the other project", got)
	}
}

func TestTaskOperationManifestOrdersTheSingleTaskActionSheet(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	loose := project.GetTaskByID("loose")

	descriptors := TaskActionSheetDescriptors(
		model.operationRegistry,
		taskOperationTestContext(loose, project),
	)
	ordered := make([]OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if _, manifested := taskOperationManifestEntryFor(descriptor.ID); manifested {
			ordered = append(ordered, descriptor.ID)
		}
	}
	// Unpark, unpin, and undim are absent because the loose task is none of
	// those; Outdent is absent because a top-level task has no parent to leave;
	// both child creators are absent because a leaf type cannot hold a child.
	// State, priority, due date, and tags are absent because they are direct-only.
	want := []OperationID{
		operationTaskSetSnooze,
		operationTaskScore,
		operationTaskPark,
		operationTaskMoveProject,
		operationTaskPin,
		operationTaskDim,
		operationCoreEdit,
		operationCoreDetail,
		operationTaskChecklist,
		operationTaskComments,
		operationTaskQuickNote,
		operationTaskDecisions,
		operationTaskAttachments,
		operationTaskActivity,
		operationTaskCopyTitle,
		operationTaskCopyTitleDescription,
		operationTaskCopyDetails,
		operationTaskCopyReference,
		operationTaskSetParent,
		operationTaskIndent,
		operationTaskSetDependencies,
		operationTaskLinkedTasks,
		operationTaskNewLinkedSubtask,
		operationTaskDelete,
	}
	if !reflect.DeepEqual(ordered, want) {
		t.Fatalf("manifest order = %v, want %v", ordered, want)
	}
	for _, id := range []OperationID{operationTaskIndent, operationTaskOutdent} {
		descriptor, ok := model.operationRegistry.Descriptor(id)
		if !ok {
			t.Fatalf("operation %s is not registered", id)
		}
		if got := taskActionCategory(descriptor); got != taskActionCategoryRelationships {
			t.Errorf("%s category = %s, want %s", id, got, taskActionCategoryRelationships)
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
