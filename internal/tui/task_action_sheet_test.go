package tui

import (
	"errors"
	"slices"
	"strings"
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
	wantIDs := []OperationID{
		operationTaskRecurrence,
		operationTaskMoveProject,
		operationTaskSetParent,
		operationTaskSetDependencies,
		operationTaskChecklist,
		operationTaskComments,
		operationTaskDecisions,
		operationTaskAttachments,
		operationTaskDelete,
	}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("action sheet IDs = %v, want exact nine %v", ids, wantIDs)
	}
	wantLabels := []string{
		"Recurrence", "Move Project", "Set Parent", "Dependencies", "Checklist",
		"Comments", "Decisions", "Attachments", "Delete",
	}
	for index, descriptor := range descriptors {
		if descriptor.Label != wantLabels[index] {
			t.Errorf("action %d label = %q, want %q", index, descriptor.Label, wantLabels[index])
		}
	}
	// Direct-key core navigation, capture, and the sheet's own opener are not
	// task actions; the metadata operations the input strategy binds directly
	// leave the sheet rather than repeating their key as a row.
	for _, excluded := range []OperationID{
		operationCoreActions,
		operationCoreCapture,
		operationCoreMultiSelect,
		operationCoreCursorDown,
		operationTaskSetState,
		operationTaskSetPriority,
		operationTaskSetDueDate,
		operationTaskSetTags,
		operationTaskSetSnooze,
		operationTaskTogglePin,
		operationTaskIndent,
		operationTaskOutdent,
		operationCoreEdit,
		operationCoreEditDescription,
		operationCoreExternalEditor,
		operationCoreDetail,
		operationTaskCopyTitle,
		operationTaskCopyDetails,
		operationTaskCopyTitleDescription,
		operationTaskNewChild,
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
		taskActionCategoryOrganize,
		taskActionCategoryRelationships,
		taskActionCategoryContent,
		taskActionCategoryDanger,
	} {
		if !seenCategories[category] {
			t.Errorf("grouped sheet missing category %s", category)
		}
	}

	// Printable letters filter immediately. This query includes k, which is no
	// longer a navigation alias inside the task menu.
	for _, letter := range "check" {
		_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: letter, Text: string(letter)})
	}
	matches := sheet.surface.Matches()
	wantChecklist := ListItemID("task-action:" + string(operationTaskChecklist))
	if len(matches) != 1 || matches[0].Item.ID != wantChecklist {
		t.Fatalf("immediate filter results = %+v, want only %s", matches, wantChecklist)
	}
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'l', Text: "l"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 't', Text: "t"})
	_, _, _ = sheet.handleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	matches = sheet.surface.Matches()
	wantDelete := ListItemID("task-action:" + string(operationTaskDelete))
	if len(matches) != 1 || matches[0].Item.ID != wantDelete {
		t.Fatalf("search results = %+v, want only %s", matches, wantDelete)
	}
	if got := matches[0].Item.SecondaryText; got != "confirmation required" {
		t.Errorf("delete secondary text = %q, want the confirmation hint", got)
	}
}

func TestTaskActionSheetMovesWithCtrlJAndCtrlK(t *testing.T) {
	registry := NewOperationRegistry()
	for _, descriptor := range taskActionTestDescriptors() {
		registerTaskActionTestDescriptor(t, registry, descriptor)
	}
	sheet, err := NewTaskActionSheetModel(
		registry,
		NewOperationExecutor(registry),
		taskActionTestContext(models.StateReady),
		"Selected task",
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}

	first, ok := sheet.surface.CurrentItem()
	if !ok {
		t.Fatal("task action sheet has no initial item")
	}
	_, _, _ = sheet.handleKey(syntheticCtrlKey('j'))
	second, ok := sheet.surface.CurrentItem()
	if !ok || second.ID == first.ID {
		t.Fatalf("Ctrl+J did not move down: first = %q, current = %q", first.ID, second.ID)
	}
	_, _, _ = sheet.handleKey(syntheticCtrlKey('k'))
	current, ok := sheet.surface.CurrentItem()
	if !ok || current.ID != first.ID {
		t.Fatalf("Ctrl+K did not move back up: current = %q, want %q", current.ID, first.ID)
	}
	if query := sheet.surface.Query().Raw; query != "" {
		t.Fatalf("Ctrl+J/Ctrl+K changed the immediate-filter query to %q", query)
	}
	if view := sheet.render(100); !strings.Contains(view, "↑↓/Ctrl+j/k move") {
		t.Fatalf("task action sheet footer does not advertise Ctrl+j/k navigation:\n%s", view)
	}
}

func TestTaskActionSheetOmitsDirectSnoozeOperation(t *testing.T) {
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
			t.Fatal("direct snooze operation is repeated in the task menu")
		}
	}
}

func TestTaskRecurrenceCancelRestoresFrozenActionSheet(t *testing.T) {
	model := newUITestApp()
	target := &models.Task{
		ID: "target", ShortID: 1, Title: "Target",
		WorkflowState: models.StateReady,
		Recurrence:    &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly},
	}
	other := &models.Task{
		ID: "other", ShortID: 2, Title: "Other",
		WorkflowState: models.StateReady,
	}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{target, other}}
	project.BuildTaskTree()
	setTestAppProjects(model, project)
	model.crossOutliner.loadedProjects = []*models.Project{project}
	model.crossOutliner.rows = []outlinerRow{
		{task: target, ProjectID: project.ID},
		{task: other, ProjectID: project.ID},
	}

	sheet, err := NewTaskActionSheetModel(
		model.operationRegistry,
		model.operationExecutor,
		taskOperationTestContext(target, project),
		target.Title,
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	model.contextActionSheet = sheet
	if !sheet.surface.SetCursorToID(ListItemID("task-action:" + string(operationTaskRecurrence))) {
		t.Fatal("task sheet is missing Recurrence")
	}
	_, dismissed, raw := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed {
		t.Fatal("Recurrence dismissed the sheet before its value step")
	}
	handled, cmd := model.handleOperationOverlayMessage(raw)
	if !handled || cmd != nil {
		t.Fatalf("Recurrence selection = handled %v cmd %T; want synchronous editor open", handled, cmd)
	}
	if model.currentView() != ViewRecurrenceEditor || model.pendingTaskRecurrenceRequest == nil {
		t.Fatal("Recurrence did not open the canonical editor with a pending frozen request")
	}
	if model.contextActionSheet != nil || model.suspendedTaskActionSheet != sheet {
		t.Fatal("Recurrence did not suspend the original action sheet")
	}

	// Moving the underlying cursor cannot retarget the suspended request.
	model.crossOutliner.cursor = 1
	handled, cmd = model.handleEditorMessages(RecurrenceEditorDoneMsg{Canceled: true})
	if !handled || cmd != nil {
		t.Fatalf("Recurrence cancel = handled %v cmd %T; want unwind without mutation", handled, cmd)
	}
	if model.contextActionSheet != sheet || model.suspendedTaskActionSheet != nil {
		t.Fatal("Recurrence cancel did not restore the same action sheet")
	}
	if got := model.contextActionSheet.context.Entity.TaskID; got != target.ID {
		t.Fatalf("restored frozen target = %q, want %q", got, target.ID)
	}
}

func TestRecurrenceEditorUsesCanonicalIntervalModelAndExplicitClear(t *testing.T) {
	editor := NewRecurrenceEditorModel(&models.RecurrencePattern{
		Interval: 3,
		Unit:     models.RecurrenceWeekly,
	}, nil)
	if got := editor.units; !slices.Equal(got, []models.RecurrenceUnit{
		models.RecurrenceDaily,
		models.RecurrenceWeekly,
		models.RecurrenceMonthly,
	}) {
		t.Fatalf("recurrence units = %v, want day/week/month", got)
	}
	rendered := editor.View().Content
	for _, retired := range []string{"Weekdays:", "Mode:", "Streak:", "year"} {
		if strings.Contains(rendered, retired) {
			t.Errorf("canonical recurrence editor still renders %q", retired)
		}
	}
	_, cmd := editor.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if cmd == nil {
		t.Fatal("c did not emit an explicit recurrence clear")
	}
	raw := cmd()
	cleared, ok := raw.(RecurrenceEditorDoneMsg)
	if !ok || cleared.Canceled || cleared.Pattern != nil {
		t.Fatalf("clear result = %#v, want non-canceled nil pattern", raw)
	}
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
	if !slices.Equal(ids, []OperationID{operationTaskDelete}) {
		t.Fatalf("bulk action sheet IDs = %v, want Delete only", ids)
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
	if !app.crossOutliner.IsTaskIdentitySelected(taskIdentityFor(first, project)) {
		t.Fatal("v did not toggle the cursor row into visual selection")
	}
	send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if app.crossOutliner.multiSelectMode || app.crossOutliner.SelectedCount() != 0 {
		t.Fatal("Esc did not clear visual selection")
	}

	send(tea.KeyPressMsg{Code: 'v', Text: "v"})
	app.crossOutliner.cursor = 1
	send(tea.KeyPressMsg{Code: 'v', Text: "v"})
	send(tea.KeyPressMsg{Code: tea.KeySpace})
	sheet := app.contextActionSheet
	if sheet == nil || !sheet.config.Bulk || sheet.config.Targets != 2 {
		t.Fatalf("Space did not open a two-target bulk action sheet: %#v", sheet)
	}
	// Priority now belongs to the p chord, so the bulk sheet must not repeat it.
	if sheet.surface.SetCursorToID(
		ListItemID("task-action:" + string(operationTaskSetPriority)),
	) {
		t.Fatal("bulk action sheet still lists the direct-only set-priority action")
	}
	if sheet.surface.SetCursorToID(
		ListItemID("task-action:" + string(operationTaskSetSnooze)),
	) {
		t.Fatal("bulk action sheet repeats direct-only set-snooze")
	}
	if !sheet.surface.SetCursorToID(
		ListItemID("task-action:" + string(operationTaskDelete)),
	) {
		t.Fatal("bulk action sheet is missing delete")
	}
}

func TestVisualSelectionRejectsPeekWithoutCursorFallback(t *testing.T) {
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
	app.crossOutliner.toggleCursorSelection()
	app.crossOutliner.cursor = 1

	result := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationCoreDetail,
		Context: app.operationCommandContext(),
		Source:  OperationSourceDirectKey,
	})
	if !errors.Is(result.Err, ErrOperationUnavailable) {
		t.Fatalf("visual-selection Peek error = %v, want unavailable", result.Err)
	}
	if result.Message != nil || app.contextActionSheet != nil {
		t.Fatal("visual-selection Peek fell back to the cursor or action sheet")
	}
}
