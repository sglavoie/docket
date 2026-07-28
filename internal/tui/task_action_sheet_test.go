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

// bulkTaskActionTestContext freezes a visual selection over live tasks, the same
// shape openTaskActionSheet builds from the Outliner's selected rows.
func bulkTaskActionTestContext(project *models.Project, tasks ...*models.Task) CommandContext {
	context := PresentationCommandContext(ViewCrossProjectOutliner)
	context.Capabilities.Add(CapabilitySimplifiedInput)
	context.Selection.Visual = true
	context.Selection.Tasks = make([]TaskIdentity, 0, len(tasks))
	context.Selection.Entities = make([]EntityContext, 0, len(tasks))
	for _, task := range tasks {
		context.Selection.Tasks = append(context.Selection.Tasks, taskIdentityFor(task, project))
		context.Selection.Entities = append(context.Selection.Entities, taskOperationEntity(task, project))
	}
	context.Entity = context.Selection.Entities[0]
	return context
}

// taskActionTestDescriptors builds the production planning/structure
// descriptors. The app carries no projects, so structure-dependent applicability
// falls back to structural eligibility.
func taskActionTestDescriptors() []OperationDescriptor {
	return newUITestApp().taskActionOperationDescriptors()
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

func TestTaskActionSheetProjectsTheProductionSelectedTaskCatalog(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	story := project.GetTaskByID("story")
	context := taskOperationTestContext(story, project)
	registry := model.operationRegistry

	descriptors := TaskActionSheetDescriptors(registry, context)
	ids := make([]OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	for _, want := range []OperationID{
		operationTaskSetSnooze,
		operationTaskScore,
		operationTaskSetDependencies,
		operationTaskSetParent,
		operationTaskIndent,
		operationTaskOutdent,
		operationCoreEdit,
		operationCoreDetail,
		operationTaskDelete,
	} {
		if !slices.Contains(ids, want) {
			t.Errorf("action sheet missing %s: %v", want, ids)
		}
	}
	// Direct-key core navigation, capture, and the sheet's own opener are not
	// task actions; the metadata operations the input strategy binds directly
	// leave the sheet rather than repeating their key as a row.
	for _, excluded := range []OperationID{
		operationCoreStart,
		operationCoreDone,
		operationCoreActions,
		operationCoreCapture,
		operationCoreMultiSelect,
		operationCoreCursorDown,
		operationTaskSetState,
		operationTaskSetPriority,
		operationTaskSetDueDate,
		operationTaskSetTags,
	} {
		if slices.Contains(ids, excluded) {
			t.Errorf("action sheet exposed non-contextual %s: %v", excluded, ids)
		}
	}

	sheet, err := NewTaskActionSheetModel(
		registry,
		NewOperationExecutor(registry),
		context,
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
	matches := sheet.surface.Matches()
	wantDelete := ListItemID("task-action:" + string(operationTaskDelete))
	if len(matches) != 1 || matches[0].Item.ID != wantDelete {
		t.Fatalf("search results = %+v, want only %s", matches, wantDelete)
	}
	if got := matches[0].Item.SecondaryText; got != "confirmation required" {
		t.Errorf("delete secondary text = %q, want the confirmation hint", got)
	}
}

func TestTaskActionSheetSnoozeShowsUsefulUnavailableReason(t *testing.T) {
	registry := NewOperationRegistry()
	for _, descriptor := range taskActionTestDescriptors() {
		registerTaskActionTestDescriptor(t, registry, descriptor)
	}
	sheet, err := NewTaskActionSheetModel(
		registry,
		NewOperationExecutor(registry),
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
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	registry := model.operationRegistry

	// The in-progress member is what removes snooze from the intersection.
	story := project.GetTaskByID("story")
	loose := project.GetTaskByID("loose")
	loose.WorkflowState = models.StateInProgress
	context := bulkTaskActionTestContext(project, story, loose)

	descriptors := TaskActionSheetDescriptors(registry, context)
	ids := make([]OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	if !slices.Contains(ids, operationTaskDelete) {
		t.Errorf("bulk action sheet missing %s: %v", operationTaskDelete, ids)
	}
	for _, excluded := range []OperationID{
		// Snooze is bulk-capable but the in-progress member removes it.
		operationTaskSetSnooze,
		// The four metadata operations reach a selection through their direct key.
		operationTaskSetState,
		operationTaskSetPriority,
		operationTaskSetDueDate,
		operationTaskSetTags,
		operationTaskMoveProject,
		operationTaskSetDependencies,
		operationTaskSetParent,
		operationTaskIndent,
		operationTaskOutdent,
		operationCoreEdit,
		operationCoreDetail,
		operationTaskCopyTitle,
		operationTaskNewChild,
	} {
		if slices.Contains(ids, excluded) {
			t.Errorf("bulk action sheet exposed non-common operation %s: %v", excluded, ids)
		}
	}
	// The manifest Bulk flag is the only membership rule, so nothing can reach a
	// multi-target sheet without a coordinator that carries every frozen target.
	for _, id := range ids {
		if !taskOperationSupportsBulk(id) {
			t.Errorf("bulk action sheet advertised %s, which has no bulk coordinator", id)
		}
	}

	sheet, err := NewTaskActionSheetModel(registry, NewOperationExecutor(registry), context, "2 tasks")
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	if !sheet.surface.SetCursorToID("task-action:" + ListItemID(operationTaskDelete)) {
		t.Fatal("delete action is missing from the bulk sheet")
	}
	_, _, result := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	emitted, ok := result.(ContextActionSheetResultMsg)
	if !ok {
		t.Fatalf("bulk Enter returned %T, want ContextActionSheetResultMsg", result)
	}
	if emitted.Host != ContextActionHostOutliner {
		t.Fatalf("bulk result host = %q, want %q", emitted.Host, ContextActionHostOutliner)
	}
	if emitted.Request.Source != OperationSourceBulk {
		t.Fatalf("bulk request source = %q, want %q", emitted.Request.Source, OperationSourceBulk)
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
	if app.contextActionSheet == nil {
		t.Fatal("Space did not open the selected-task action sheet")
	}
	if app.operationValuePicker != nil {
		t.Fatal("Space skipped the action sheet and opened a value picker")
	}
	if got := len(app.contextActionSheet.surface.Items()); got == 0 {
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
	sheet := app.contextActionSheet
	if sheet == nil || !sheet.config.Bulk || sheet.config.Targets != 2 {
		t.Fatalf("Enter did not open a two-target bulk action sheet: %#v", sheet)
	}
	// Priority now belongs to the p chord, so the bulk sheet must not repeat it.
	if sheet.surface.SetCursorToID(
		ListItemID("task-action:" + string(operationTaskSetPriority)),
	) {
		t.Fatal("bulk action sheet still lists the direct-only set-priority action")
	}
	if !sheet.surface.SetCursorToID(
		ListItemID("task-action:" + string(operationTaskSetSnooze)),
	) {
		t.Fatal("bulk action sheet is missing set-snooze")
	}
	_, dismissed, rawResult := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed {
		t.Fatal("a pending value picker dismissed the bulk sheet")
	}
	emitted, ok := rawResult.(ContextActionSheetResultMsg)
	if !ok {
		t.Fatalf("snooze selection returned %T, want ContextActionSheetResultMsg", rawResult)
	}
	handled, cmd := app.handleOperationOverlayMessage(emitted)
	if !handled || cmd != nil {
		t.Fatalf("snooze execution = handled %v cmd %T, want value picker", handled, cmd)
	}
	if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderSnooze {
		t.Fatalf("snooze action did not open the canonical value picker: %#v", app.operationValuePicker)
	}
}
