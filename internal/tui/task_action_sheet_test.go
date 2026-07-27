package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func taskActionTestContext(state models.WorkflowState) CommandContext {
	context := PresentationCommandContext(ViewCrossProjectOutliner)
	context.Capabilities.Add(CapabilitySimplifiedInput)
	context.Entity = EntityContext{
		ProjectID: "project",
		TaskID:    "task",
		State:     state.String(),
		Mutable:   !state.IsTerminal(),
	}
	return context
}

func bulkTaskActionTestContext(states ...models.WorkflowState) CommandContext {
	context := taskActionTestContext(states[0])
	context.Selection.Visual = true
	context.Selection.Tasks = make([]TaskIdentity, 0, len(states))
	context.Selection.Entities = make([]EntityContext, 0, len(states))
	for index, state := range states {
		projectID := "project"
		if index > 0 {
			projectID = "project-" + string(rune('a'+index))
		}
		taskID := "task-" + string(rune('a'+index))
		context.Selection.Tasks = append(context.Selection.Tasks, TaskIdentity{
			ProjectID: projectID,
			TaskID:    taskID,
		})
		context.Selection.Entities = append(context.Selection.Entities, EntityContext{
			ProjectID: projectID,
			TaskID:    taskID,
			State:     state.String(),
			Mutable:   !state.IsTerminal(),
		})
	}
	context.Entity = context.Selection.Entities[0]
	return context
}

func registerTaskActionTestDescriptor(
	t *testing.T,
	registry *OperationRegistry,
	descriptor OperationDescriptor,
) {
	t.Helper()
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("register %s: %v", descriptor.ID, err)
	}
}

func TestTaskActionSheetProjectsNativeAndRetainedSelectedTaskActions(t *testing.T) {
	registry := NewOperationRegistry()
	for _, descriptor := range taskActionOperationDescriptors() {
		registerTaskActionTestDescriptor(t, registry, descriptor)
	}
	for _, descriptor := range []OperationDescriptor{
		{
			ID:                "retained.edit-content",
			Label:             "Edit Content",
			Category:          OperationCategoryTask,
			TargetSource:      OperationTargetCurrentTask,
			TargetCardinality: TargetCardinalityOne,
			Risk:              OperationRiskMutation,
		},
		{
			ID:                "retained.delete",
			Label:             "Delete Task",
			Category:          OperationCategoryTask,
			TargetSource:      OperationTargetCurrentTask,
			TargetCardinality: TargetCardinalityOne,
			Risk:              OperationRiskDestructive,
		},
		{
			ID:                "retained.global",
			Label:             "Global Status Update",
			Category:          OperationCategoryTask,
			TargetSource:      OperationTargetSelectedTasks,
			TargetCardinality: TargetCardinalityOneOrMore,
			Risk:              OperationRiskMutation,
		},
		{
			ID:                "retained.status",
			Label:             "Block Task",
			Category:          OperationCategoryTask,
			TargetSource:      OperationTargetCurrentTask,
			TargetCardinality: TargetCardinalityOne,
			Risk:              OperationRiskMutation,
		},
	} {
		registerTaskActionTestDescriptor(t, registry, descriptor)
	}

	descriptors := TaskActionSheetDescriptors(registry, taskActionTestContext(models.StateReady))
	ids := make([]OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	for _, want := range []OperationID{
		operationTaskSetState,
		operationTaskSetPriority,
		operationTaskSetDependencies,
		operationTaskSetParent,
		operationTaskIndent,
		operationTaskOutdent,
		"retained.edit-content",
		"retained.delete",
	} {
		if !slices.Contains(ids, want) {
			t.Errorf("action sheet missing %s: %v", want, ids)
		}
	}
	for _, excluded := range []OperationID{"retained.global", "retained.status"} {
		if slices.Contains(ids, excluded) {
			t.Errorf("action sheet retained superseded/non-contextual %s: %v", excluded, ids)
		}
	}

	sheet, err := NewTaskActionSheetModel(
		registry,
		taskActionTestContext(models.StateReady),
		"Selected task",
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	rows := sheet.surface.Rows()
	seenCategories := make(map[ListCategoryID]bool)
	for _, row := range rows {
		if row.Kind == ListRowCategory {
			seenCategories[row.Category.ID] = true
		}
	}
	for _, category := range []ListCategoryID{
		taskActionCategoryPlan,
		taskActionCategoryContent,
		taskActionCategoryDanger,
	} {
		if !seenCategories[category] {
			t.Errorf("grouped sheet missing category %s", category)
		}
	}

	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: '/', Text: "/"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'l', Text: "l"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 't', Text: "t"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if got := sheet.surface.Matches(); len(got) != 1 || got[0].Item.ID != "task-action:retained.delete" {
		t.Fatalf("search results = %+v, want retained delete", got)
	}
}

func TestTaskActionSheetSnoozeShowsUsefulUnavailableReason(t *testing.T) {
	registry := NewOperationRegistry()
	for _, descriptor := range taskActionOperationDescriptors() {
		registerTaskActionTestDescriptor(t, registry, descriptor)
	}
	sheet, err := NewTaskActionSheetModel(
		registry,
		taskActionTestContext(models.StateInProgress),
		"Active task",
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	for _, item := range sheet.surface.Items() {
		if item.ID == ListItemID("task-action:"+string(operationTaskSetSnooze)) {
			if item.DisabledReason == "" {
				t.Fatal("in-progress snooze action did not explain unavailability")
			}
			return
		}
	}
	t.Fatal("snooze action is missing")
}

func TestBulkActionSheetIntersectsApplicableOperationsForEverySelectedTask(t *testing.T) {
	registry := NewOperationRegistry()
	for _, descriptor := range taskActionOperationDescriptors() {
		registerTaskActionTestDescriptor(t, registry, descriptor)
	}
	registerTaskActionTestDescriptor(t, registry, OperationDescriptor{
		ID:                "retained.delete",
		Label:             "Delete Task",
		Category:          OperationCategoryTask,
		TargetSource:      OperationTargetSelectedTasks,
		TargetCardinality: TargetCardinalityOneOrMore,
		Risk:              OperationRiskDestructive,
	})
	registerTaskActionTestDescriptor(t, registry, OperationDescriptor{
		ID:                "retained.edit",
		Label:             "Edit Content",
		Category:          OperationCategoryTask,
		TargetSource:      OperationTargetCurrentTask,
		TargetCardinality: TargetCardinalityOne,
		Risk:              OperationRiskMutation,
	})

	context := bulkTaskActionTestContext(models.StateReady, models.StateInProgress)
	descriptors := TaskActionSheetDescriptors(registry, context)
	ids := make([]OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	for _, want := range []OperationID{
		operationTaskSetState,
		operationTaskSetPriority,
		operationTaskSetDueDate,
		operationTaskSetTags,
		"retained.delete",
	} {
		if !slices.Contains(ids, want) {
			t.Errorf("bulk action sheet missing %s: %v", want, ids)
		}
	}
	for _, excluded := range []OperationID{
		operationTaskSetSnooze,
		operationTaskMoveProject,
		operationTaskSetDependencies,
		operationTaskSetParent,
		operationTaskIndent,
		operationTaskOutdent,
		"retained.edit",
	} {
		if slices.Contains(ids, excluded) {
			t.Errorf("bulk action sheet exposed non-common operation %s: %v", excluded, ids)
		}
	}

	sheet, err := NewTaskActionSheetModel(registry, context, "2 tasks")
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	if !sheet.surface.SetCursorToID("task-action:" + ListItemID(operationTaskSetPriority)) {
		t.Fatal("set-priority action is missing from the bulk sheet")
	}
	_, _, result := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	execute, ok := result.(TaskActionSheetExecuteMsg)
	if !ok {
		t.Fatalf("bulk Enter returned %T, want TaskActionSheetExecuteMsg", result)
	}
	if execute.Request.Source != OperationSourceBulk {
		t.Fatalf("bulk request source = %q, want %q", execute.Request.Source, OperationSourceBulk)
	}
}

func TestSimplifiedSpaceOpensSelectedTaskActionSheet(t *testing.T) {
	app := newUITestApp()
	task := &models.Task{
		ID:            "task",
		Title:         "Selected task",
		WorkflowState: models.StateReady,
	}
	project := &models.Project{
		ID:    "project",
		Name:  "Project",
		Tasks: []*models.Task{task},
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.rows = []outlinerRow{{task: task, ProjectID: project.ID}}

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeySpace})
	if !handled || cmd != nil {
		t.Fatalf("Space = handled %v, cmd %T; want handled with no command", handled, cmd)
	}
	if app.taskActionSheet == nil {
		t.Fatal("Space did not open the selected-task action sheet")
	}
	if app.operationValuePicker != nil {
		t.Fatal("Space skipped the action sheet and opened a value picker")
	}
	if got := len(app.taskActionSheet.surface.Items()); got == 0 {
		t.Fatal("opened action sheet contains no applicable task actions")
	}
}

func TestSimplifiedVisualSelectionKeysOpenBulkActionsAndEscClears(t *testing.T) {
	app := newUITestApp()
	first := &models.Task{ID: "first", Title: "First", WorkflowState: models.StateReady}
	second := &models.Task{ID: "second", Title: "Second", WorkflowState: models.StateReady}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{first, second}}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.rows = []outlinerRow{
		{task: first, ProjectID: project.ID},
		{task: second, ProjectID: project.ID},
	}

	send := func(msg tea.KeyPressMsg) {
		t.Helper()
		handled, cmd := app.handleGlobalKeyPress(msg)
		if !handled || cmd != nil {
			t.Fatalf("%q = handled %v cmd %T; want synchronous handling", msg.String(), handled, cmd)
		}
	}
	send(tea.KeyPressMsg{Code: 'v', Text: "v"})
	send(tea.KeyPressMsg{Code: tea.KeySpace})
	if !app.crossOutliner.IsTaskIdentitySelected(taskIdentityFor(first, project)) {
		t.Fatal("Space did not toggle the cursor row in visual selection")
	}
	send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if app.crossOutliner.multiSelectMode || app.crossOutliner.SelectedCount() != 0 {
		t.Fatal("Esc did not clear visual selection")
	}

	send(tea.KeyPressMsg{Code: 'v', Text: "v"})
	send(tea.KeyPressMsg{Code: tea.KeySpace})
	app.crossOutliner.cursor = 1
	send(tea.KeyPressMsg{Code: tea.KeySpace})
	send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if app.taskActionSheet == nil || !app.taskActionSheet.bulk || app.taskActionSheet.targets != 2 {
		t.Fatalf("Enter did not open a two-target bulk action sheet: %#v", app.taskActionSheet)
	}
	if !app.taskActionSheet.surface.SetCursorToID(
		ListItemID("task-action:" + string(operationTaskSetPriority)),
	) {
		t.Fatal("bulk action sheet is missing set-priority")
	}
	_, _, rawExecute := app.taskActionSheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	execute, ok := rawExecute.(TaskActionSheetExecuteMsg)
	if !ok {
		t.Fatalf("priority selection returned %T, want TaskActionSheetExecuteMsg", rawExecute)
	}
	handled, cmd := app.handleOperationOverlayMessage(execute)
	if !handled || cmd != nil {
		t.Fatalf("priority execution = handled %v cmd %T, want value picker", handled, cmd)
	}
	if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderPriority {
		t.Fatalf("priority action did not open the canonical value picker: %#v", app.operationValuePicker)
	}
}
