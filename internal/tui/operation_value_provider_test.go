package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestStateValueProviderAcceptsOnlyCanonicalPresets(t *testing.T) {
	for _, state := range models.AllWorkflowStates() {
		if !ValueProviderState.acceptsPresetValue(state) {
			t.Errorf("canonical state preset %q was rejected", state)
		}
	}
	for _, retired := range []models.WorkflowState{"ready", "next", "in_progress", "review"} {
		if ValueProviderState.acceptsPresetValue(retired) {
			t.Errorf("retired state preset %q was accepted", retired)
		}
	}
}

func TestTaskStatePickerContainsExactCanonicalStatesAndTransitionReasons(t *testing.T) {
	request := OperationExecutionRequest{
		ID:      operationTaskSetState,
		Context: taskActionTestContext(models.StateDone),
	}
	picker, err := NewTaskStatePickerForTargets(request, []models.WorkflowState{models.StateDone}, "Completed task")
	if err != nil {
		t.Fatalf("NewTaskStatePickerForTargets: %v", err)
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

func newOperationTagPickerForTest(t *testing.T, tags ...string) *OperationValuePickerModel {
	t.Helper()
	infos := make([]TagInfo, 0, len(tags))
	for _, tag := range tags {
		infos = append(infos, TagInfo{Name: tag})
	}
	source := ListSourceFromTags(infos)
	for index := range source.Items {
		source.Items[index].Data = source.Items[index].PrimaryText
	}
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationTaskSetTags},
		ValueProviderTags,
		"Set Tags",
		"Task",
		source,
		true,
		nil,
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}
	return picker
}

func TestOperationTagPickerCreatesSelectsAndAppliesNewTag(t *testing.T) {
	picker := newOperationTagPickerForTest(t, "zeta")
	picker.surface.SetQuery("stale")
	if rendered := picker.render(90); !strings.Contains(ansi.Strip(rendered), "a add tag") {
		t.Fatalf("Set Tags help does not advertise tag creation:\n%s", rendered)
	}

	_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: 'a'})
	if dismissed || result != nil || !picker.tagAdder.AddTagMode {
		t.Fatalf("a result = dismissed %v, message %T, add mode %v", dismissed, result, picker.tagAdder.AddTagMode)
	}
	if rendered := picker.render(90); !strings.Contains(rendered, "Add New Tag") {
		t.Fatalf("add-tag overlay is missing its title:\n%s", rendered)
	}

	picker.tagAdder.AddTagInput.SetValue("  #alpha  ")
	_, dismissed, result = picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed || result != nil || picker.tagAdder.AddTagMode {
		t.Fatalf("tag submit = dismissed %v, message %T, add mode %v", dismissed, result, picker.tagAdder.AddTagMode)
	}
	if got := picker.surface.Query().Raw; got != "" {
		t.Fatalf("query after tag creation = %q, want cleared", got)
	}
	if !picker.surface.IsSelected("tag:alpha") {
		t.Fatal("new tag is not selected")
	}
	items := picker.surface.Items()
	if len(items) != 2 || items[0].ID != "tag:alpha" || items[1].ID != "tag:zeta" {
		t.Fatalf("sorted tag items = %+v, want alpha then zeta", items)
	}

	_, dismissed, result = picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !dismissed {
		t.Fatal("Enter did not apply the completed tag selection")
	}
	selected, ok := result.(OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("result = %T, want OperationValueSelectedMsg", result)
	}
	if got, want := selected.Value, []string{"alpha"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected tags = %#v, want %#v", got, want)
	}
}

func TestOperationTagPickerValidatesInputAndEscapesBackToPicker(t *testing.T) {
	picker := newOperationTagPickerForTest(t)
	if rendered := picker.render(90); !strings.Contains(ansi.Strip(rendered), "a add tag") {
		t.Fatalf("empty Set Tags help does not advertise tag creation:\n%s", rendered)
	}

	picker.handleKey(tea.KeyPressMsg{Code: 'a'})
	for _, input := range []string{" ", "!!!"} {
		picker.tagAdder.AddTagInput.SetValue(input)
		_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		if dismissed || result != nil || !picker.tagAdder.AddTagMode || picker.tagAdder.AddTagError == "" {
			t.Fatalf(
				"invalid input %q = dismissed %v, message %T, add mode %v, error %q",
				input,
				dismissed,
				result,
				picker.tagAdder.AddTagMode,
				picker.tagAdder.AddTagError,
			)
		}
	}
	if len(picker.surface.Items()) != 0 {
		t.Fatalf("invalid input added tag rows: %+v", picker.surface.Items())
	}

	_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if dismissed || result != nil || picker.tagAdder.AddTagMode {
		t.Fatalf("first Esc = dismissed %v, message %T, add mode %v", dismissed, result, picker.tagAdder.AddTagMode)
	}
	_, dismissed, result = picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !dismissed {
		t.Fatal("second Esc did not dismiss Set Tags")
	}
	if _, ok := result.(OperationValueCanceledMsg); !ok {
		t.Fatalf("second Esc result = %T, want OperationValueCanceledMsg", result)
	}
}

func TestBulkOperationTagPickerCreatesAndPromotesTagsAsAddPatch(t *testing.T) {
	picker := newOperationTagPickerForTest(t, "partial")
	if err := picker.configureTagMembership(map[ListItemID]operationTagMembership{
		"tag:partial": operationTagSome,
	}); err != nil {
		t.Fatalf("configureTagMembership: %v", err)
	}

	for _, input := range []string{" #partial ", "fresh"} {
		picker.handleKey(tea.KeyPressMsg{Code: 'a'})
		picker.tagAdder.AddTagInput.SetValue(input)
		_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		if dismissed || result != nil || picker.tagAdder.AddTagMode {
			t.Fatalf("add %q = dismissed %v, message %T, add mode %v", input, dismissed, result, picker.tagAdder.AddTagMode)
		}
	}
	if items := picker.surface.Items(); len(items) != 2 {
		t.Fatalf("duplicate tag created an extra row: %+v", items)
	}

	value, err := picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue: %v", err)
	}
	patch, ok := value.(OperationTagsValue)
	if !ok {
		t.Fatalf("tag value = %T, want OperationTagsValue", value)
	}
	if !reflect.DeepEqual(patch.Add, []string{"fresh", "partial"}) || len(patch.Remove) != 0 {
		t.Fatalf("tag patch = %+v, want add fresh and partial only", patch)
	}
}

func TestBulkOperationTagPickerKeepsSemanticTagsIndependent(t *testing.T) {
	picker := newOperationTagPickerForTest(t, "bug", "idea", "keep")
	if err := picker.configureTagMembership(map[ListItemID]operationTagMembership{
		"tag:bug":  operationTagSome,
		"tag:idea": operationTagNone,
		"tag:keep": operationTagAll,
	}); err != nil {
		t.Fatalf("configureTagMembership: %v", err)
	}
	if !picker.surface.SetCursorToID("tag:idea") {
		t.Fatal("idea tag is missing")
	}
	if err := picker.toggleCurrentMultiValue(); err != nil {
		t.Fatalf("toggle idea: %v", err)
	}

	value, err := picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue: %v", err)
	}
	patch, ok := value.(OperationTagsValue)
	if !ok {
		t.Fatalf("tag value = %T, want OperationTagsValue", value)
	}
	if !reflect.DeepEqual(patch.Add, []string{"idea"}) || len(patch.Remove) != 0 {
		t.Fatalf("tag patch = %+v, want add idea without implicit removals", patch)
	}
	if picker.tagCurrent["tag:keep"] != operationTagAll {
		t.Fatal("ordinary tag selection changed while adding a semantic tag")
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

func TestBulkTagPickerRepresentsAllSomeNoneAndEmitsPatch(t *testing.T) {
	common := newBulkPickerTestTask(t, 1, "first", "common", "first-only")
	other := newBulkPickerTestTask(t, 1, "second", "common", "second-only")
	env := newAppBulkPickerEnv(t, []*models.Task{common}, []*models.Task{other})
	model := env.appModel()
	context := PresentationCommandContext(ViewCrossProjectOutliner)
	context.Capabilities.Add(CapabilitySimplifiedInput)
	context.Selection.Visual = true
	for index, task := range []*models.Task{common, other} {
		project := env.projects[index]
		context.Selection.Tasks = append(context.Selection.Tasks, taskIdentityFor(task, project))
		context.Selection.Entities = append(context.Selection.Entities, taskOperationEntity(task, project))
	}
	context.Entity = context.Selection.Entities[0]
	request := OperationExecutionRequest{
		ID:      operationTaskSetTags,
		Context: context,
	}
	picker, err := model.newOperationValuePicker(request, ValueProviderTags)
	if err != nil {
		t.Fatalf("newOperationValuePicker: %v", err)
	}
	if !picker.surface.IsSelected("tag:common") {
		t.Fatal("tag on every task is not rendered as selected")
	}
	items := picker.surface.Items()
	indeterminate := map[ListItemID]bool{}
	for _, item := range items {
		indeterminate[item.ID] = item.Indeterminate
	}
	if !indeterminate["tag:first-only"] || !indeterminate["tag:second-only"] {
		t.Fatalf("partial tag membership = %v, want both task-specific tags indeterminate", indeterminate)
	}
	if rendered := picker.render(90); !strings.Contains(rendered, "[-]") {
		t.Fatalf("tri-state picker did not render an indeterminate marker:\n%s", rendered)
	}

	if !picker.surface.SetCursorToID("tag:first-only") {
		t.Fatal("first-only tag is missing")
	}
	if toggleErr := picker.toggleCurrentMultiValue(); toggleErr != nil {
		t.Fatalf("promote partial tag to all: %v", toggleErr)
	}
	if !picker.surface.SetCursorToID("tag:common") {
		t.Fatal("common tag is missing")
	}
	if toggleErr := picker.toggleCurrentMultiValue(); toggleErr != nil {
		t.Fatalf("remove common tag: %v", toggleErr)
	}
	value, err := picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue: %v", err)
	}
	patch, ok := value.(OperationTagsValue)
	if !ok {
		t.Fatalf("tag value = %T, want OperationTagsValue", value)
	}
	if !reflect.DeepEqual(patch.Add, []string{"first-only"}) ||
		!reflect.DeepEqual(patch.Remove, []string{"common"}) {
		t.Fatalf("tag patch = %+v, want add first-only and remove common", patch)
	}
}

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
	outliner.SelectTaskIdentity(taskIdentityFor(first, env.projects[0]))
	outliner.SelectTaskIdentity(taskIdentityFor(second, env.projects[1]))
	model.crossOutliner = outliner
	model.views.Register(ViewCrossProjectOutliner, outliner)

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
	context := taskActionTestContext(models.StateBacklog)
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

func TestContextActionSheetValuePickerCancelReturnsToSheet(t *testing.T) {
	model := newUITestApp()
	task := &models.Task{
		ID:            "task",
		Title:         "Task",
		WorkflowState: models.StateBacklog,
	}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	other := &models.Project{ID: "other", Name: "Other", Slug: "other"}
	project.BuildTaskTree()
	setTestAppProjects(model, project, other)
	model.operationRegistry = NewOperationRegistry()
	model.operationExecutor = NewOperationExecutor(model.operationRegistry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	sheet, err := NewTaskActionSheetModel(
		model.operationRegistry,
		model.operationExecutor,
		taskOperationTestContext(task, project),
		"Task",
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	model.contextActionSheet = sheet
	if !sheet.surface.SetCursorToID(ListItemID("task-action:" + string(operationTaskMoveProject))) {
		t.Fatal("task sheet is missing the move-project action")
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
		t.Fatal("move-project action did not open its ListSurface value picker")
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

func TestRelationValuePickerEscUnwindsSearchBeforeCancel(t *testing.T) {
	source := ListSource{Items: []ListItem{{
		ID: "parent:none", PrimaryText: "No parent", Data: OperationParentValue{},
	}}}
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationTaskSetParent},
		ValueProviderParent,
		"Set Parent",
		"Task",
		source,
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}
	_, dismissed, _ := picker.handleKey(tea.KeyPressMsg{Code: '/'})
	if dismissed || !picker.searching {
		t.Fatal("search entry unexpectedly dismissed relation picker")
	}
	_, dismissed, _ = picker.handleKey(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if dismissed || picker.surface.Query().Raw != "x" {
		t.Fatalf("search query = %q dismissed=%v", picker.surface.Query().Raw, dismissed)
	}
	_, dismissed, _ = picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if dismissed || picker.surface.Query().Raw != "" || !picker.searching {
		t.Fatal("first Esc did not clear the active relation query")
	}
	_, dismissed, _ = picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if dismissed || picker.searching {
		t.Fatal("second Esc did not leave relation search mode")
	}
	_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !dismissed {
		t.Fatal("third Esc did not cancel the relation picker")
	}
	if _, ok := result.(OperationValueCanceledMsg); !ok {
		t.Fatalf("cancel result = %T, want OperationValueCanceledMsg", result)
	}
}

func TestOperationValuePickerArrowNavigationRemainsActiveDuringSearch(t *testing.T) {
	source := ListSource{Items: []ListItem{
		{ID: "project:alpha", PrimaryText: "Alpha Project", Data: "alpha"},
		{ID: "project:beta", PrimaryText: "Beta Project", Data: "beta"},
		{ID: "project:gamma", PrimaryText: "Gamma Project", Data: "gamma"},
	}}
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationTaskMoveProject},
		ValueProviderProject,
		"Move Project",
		"Task",
		source,
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}

	picker.handleKey(tea.KeyPressMsg{Code: '/'})
	picker.handleKey(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if !picker.searching || picker.surface.Query().Raw != "p" {
		t.Fatalf("search state = active %v query %q, want active query p", picker.searching, picker.surface.Query().Raw)
	}

	picker.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if current, ok := picker.surface.CurrentItem(); !ok || current.ID != "project:beta" {
		t.Fatalf("current item after Down = %+v, %v; want project:beta", current, ok)
	}
	if !picker.searching || picker.surface.Query().Raw != "p" {
		t.Fatalf("Down changed search state = active %v query %q", picker.searching, picker.surface.Query().Raw)
	}

	picker.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	if current, ok := picker.surface.CurrentItem(); !ok || current.ID != "project:alpha" {
		t.Fatalf("current item after Up = %+v, %v; want project:alpha", current, ok)
	}
}

func TestMovedTaskMenuOperationRetargetsAsyncReloadAnchor(t *testing.T) {
	outliner := &OutlinerModel{crossReloadProjectID: "source", crossReloadTaskID: "task"}
	model := &AppModel{crossOutliner: outliner}
	task := &models.Task{ID: "task"}
	destination := &models.Project{ID: "destination"}

	model.preserveMovedOutlinerTarget(TaskMovedToProjectMsg{
		Task: task, DestProject: destination,
	})
	if outliner.crossReloadProjectID != destination.ID || outliner.crossReloadTaskID != task.ID {
		t.Fatalf("reload anchor = %s/%s, want %s/%s",
			outliner.crossReloadProjectID, outliner.crossReloadTaskID, destination.ID, task.ID)
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

func TestNarrowTaskEditorsCommitThroughCommonApplicationOperation(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Before")
	task.Description = "Old body"
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	model := env.appModel()
	project := env.projects[0]

	for _, testCase := range []struct {
		operation app.TaskOperation
		value     string
	}{
		{operation: app.TaskOperationEditTitle, value: "After"},
		{operation: app.TaskOperationEditDescription, value: "New body"},
	} {
		handled, _ := model.handleEditorMessages(ShowTaskTextEditorMsg{
			Task: task, Project: project, Operation: testCase.operation,
		})
		if !handled || model.currentView() != ViewTextEditor {
			t.Fatalf("%s did not open the narrow editor", testCase.operation)
		}
		handled, cmd := model.handleEditorMessages(TextEditorDoneMsg{Value: testCase.value})
		if !handled || cmd == nil {
			t.Fatalf("%s did not produce a service command", testCase.operation)
		}
		runBatchResult(t, cmd)
	}

	fresh := env.reload(t, 0).GetTaskByID(task.ID)
	if fresh == nil || fresh.Title != "After" || fresh.Description != "New body" {
		t.Fatalf("persisted narrow edits = %+v", fresh)
	}
}

// newTaskStructureTestProject lays out one epic with a story child followed by a
// loose root task, which is the smallest shape covering every legal and illegal
// indent/outdent/reparent case.
func newTaskStructureTestProject() *models.Project {
	epic := &models.Task{
		ID: "epic", ShortID: 1, Title: "Epic",
		WorkflowState: models.StateBacklog,
	}
	story := &models.Task{
		ID: "story", ShortID: 2, Title: "Story", ParentID: strPtr("epic"),
		WorkflowState: models.StateBacklog,
	}
	loose := &models.Task{
		ID: "loose", ShortID: 3, Title: "Loose",
		WorkflowState: models.StateBacklog,
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

func TestDirectIndentAndOutdentRemainResolvableAcrossHierarchyPositions(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project)

	for _, testCase := range []struct {
		taskID string
		id     OperationID
		want   bool
	}{
		{taskID: "loose", id: operationTaskIndent, want: true},
		{taskID: "epic", id: operationTaskIndent, want: true},
		{taskID: "story", id: operationTaskOutdent, want: true},
		{taskID: "loose", id: operationTaskOutdent, want: true},
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
	if !taskOperationApplicable(t, model, operationTaskSetParent, epic, project) {
		t.Error("set-parent is unavailable for a generic task with valid lean parent candidates")
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
	// Every direct and retired operation is absent. The remaining projection is
	// the exact five-item task menu in its canonical order.
	want := []OperationID{
		operationTaskMoveProject,
		operationTaskSetParent,
		operationTaskComments,
		operationTaskAttachments,
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

func waitingCheckBackInvocation(id OperationID, value any) OperationInvocation {
	return OperationInvocation{
		Descriptor: OperationDescriptor{ID: id},
		Context:    taskActionTestContext(models.StateBacklog),
		Value:      value,
		HasValue:   true,
	}
}

func TestWaitingCheckBackStepChainsOnlyTheWaitingMove(t *testing.T) {
	committed := func() tea.Msg { return BatchTransactionResultMsg{Action: "Set State"} }

	chained := withWaitingCheckBackStep(
		waitingCheckBackInvocation(operationTaskSetState, models.StateWaiting),
		committed,
	)
	cmds, ok := commandGroupMessage(chained())
	if !ok {
		t.Fatalf("moving to Waiting did not produce a command sequence")
	}
	if len(cmds) != 2 {
		t.Fatalf("sequence length = %d, want 2 (state move, then check-back)", len(cmds))
	}
	if _, isResult := cmds[0]().(BatchTransactionResultMsg); !isResult {
		t.Fatalf("the state move is not the first step of the sequence")
	}
	request, isRequest := cmds[1]().(OperationValueRequestMsg)
	if !isRequest {
		t.Fatalf("the follow-on step is not a value request: %T", cmds[1]())
	}
	if request.Request.ID != operationTaskSetDueDate {
		t.Errorf("follow-on operation = %s, want %s", request.Request.ID, operationTaskSetDueDate)
	}
	if request.Provider != ValueProviderDueDate {
		t.Errorf("follow-on provider = %v, want %v", request.Provider, ValueProviderDueDate)
	}

	unchained := []OperationInvocation{
		waitingCheckBackInvocation(operationTaskSetState, models.StatePaused),
		waitingCheckBackInvocation(operationTaskSetState, models.StateDone),
		waitingCheckBackInvocation(operationTaskSetPriority, models.StateWaiting),
	}
	for _, invocation := range unchained {
		msg := withWaitingCheckBackStep(invocation, committed)()
		if _, isResult := msg.(BatchTransactionResultMsg); !isResult {
			t.Errorf("%s with value %v was wrapped in a check-back chain", invocation.Descriptor.ID, invocation.Value)
		}
	}
}

// The picker is scheduled only after the transition commits, and it targets the
// identities frozen at invocation time rather than whatever the cursor names
// once the committed move has relocated it.
func TestWaitingCheckBackStepSuccessKeepsFrozenTargets(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	first := &models.Task{ID: "first"}
	second := &models.Task{ID: "second"}
	invocation := OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationTaskSetState},
		Context:    bulkTaskActionTestContext(project, first, second),
		Value:      models.StateWaiting,
		HasValue:   true,
	}
	frozen := append([]TaskIdentity(nil), invocation.Context.Selection.Tasks...)

	chained := withWaitingCheckBackStep(invocation, func() tea.Msg {
		return BatchTransactionResultMsg{Action: "Set State"}
	})
	// The commit relocates the cursor, which is what the live selection would
	// follow; the already-built chain must not.
	invocation.Context.Selection.Tasks[0] = TaskIdentity{ProjectID: "project", TaskID: "relocated"}

	cmds, ok := commandGroupMessage(chained())
	if !ok {
		t.Fatalf("a committed Waiting move did not schedule the check-back")
	}
	if len(cmds) != 2 {
		t.Fatalf("sequence length = %d, want 2 (state move, then check-back)", len(cmds))
	}
	request, isRequest := cmds[1]().(OperationValueRequestMsg)
	if !isRequest {
		t.Fatalf("the follow-on step is not a value request: %T", cmds[1]())
	}
	if !reflect.DeepEqual(request.Request.Context.Selection.Tasks, frozen) {
		t.Errorf("check-back targets = %v, want the frozen %v", request.Request.Context.Selection.Tasks, frozen)
	}
}

// A rejected transition leaves the task outside Waiting, so no picker opens and
// no due date can be committed against the refusal.
func TestWaitingCheckBackStepRejectedMoveOpensNoPicker(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	first := &models.Task{ID: "first"}
	second := &models.Task{ID: "second"}
	rejected := func() tea.Msg { return TransactionErrorMsg{Err: errors.New("cannot move to waiting")} }

	contexts := map[string]CommandContext{
		"single":   taskActionTestContext(models.StateBacklog),
		"multiple": bulkTaskActionTestContext(project, first, second),
	}
	for name, context := range contexts {
		t.Run(name, func(t *testing.T) {
			chained := withWaitingCheckBackStep(OperationInvocation{
				Descriptor: OperationDescriptor{ID: operationTaskSetState},
				Context:    context,
				Value:      models.StateWaiting,
				HasValue:   true,
			}, rejected)

			msg := chained()
			if cmds, isGroup := commandGroupMessage(msg); isGroup {
				t.Fatalf("a rejected Waiting move scheduled %d follow-on command(s)", len(cmds))
			}
			failure, isFailure := msg.(TransactionErrorMsg)
			if !isFailure {
				t.Fatalf("rejected move resolved to %T, want TransactionErrorMsg", msg)
			}
			if failure.Err == nil || failure.Err.Error() != "cannot move to waiting" {
				t.Errorf("rejection error = %v, want the underlying refusal", failure.Err)
			}
		})
	}
}
