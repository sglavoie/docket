package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func taskActionTestContext(state models.WorkflowState) CommandContext {
	context := PresentationCommandContext(ViewCrossProjectOutliner)
	context.Capabilities.Add(CapabilitySimplifiedInput)
	context.Entity = EntityContext{
		ProjectID: "project",
		TaskID:    "task",
		State:     state.String(),
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

func TestTaskActionSheetMissingSelectionSchedulesStatusClear(t *testing.T) {
	model := newUITestApp()
	result := model.executeCoreInputOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationCoreActions},
	})

	if result.Err != nil {
		t.Fatalf("open task actions: %v", result.Err)
	}
	if model.statusMessage != "select a task to open actions" {
		t.Fatalf("status = %q, want missing-selection guidance", model.statusMessage)
	}
	timeout, ok := result.Message.(OperationCommandResultMsg)
	if !ok || timeout.Cmd == nil {
		t.Fatalf("result message = %T, want a status timeout command", result.Message)
	}
}

func TestTaskActionSheetProjectsTheProductionSelectedTaskCatalog(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	story := project.GetTaskByID("story")
	context := taskOperationTestContext(story, project)
	registry := model.operationRegistry
	registerTaskActionTestDescriptor(t, registry, OperationDescriptor{
		ID:                   OperationID("test.task.unlisted"),
		Label:                "Unlisted Task Tool",
		Category:             OperationCategoryTask,
		TargetSource:         OperationTargetCurrentTask,
		TargetCardinality:    TargetCardinalityOne,
		RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
		Risk:                 OperationRiskReadOnly,
	})

	descriptors := TaskActionSheetDescriptors(registry, context)
	ids := make([]OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	wantIDs := []OperationID{
		operationTaskMoveProject,
		operationTaskSetParent,
		operationTaskChecklist,
		operationTaskComments,
		operationTaskAttachments,
		operationTaskDelete,
	}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("action sheet IDs = %v, want exact six %v", ids, wantIDs)
	}
	wantLabels := []string{
		"Move Project", "Set Parent", "Checklist", "Comments", "Attachments", "Delete",
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
		operationCorePeek,
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
		taskActionTestContext(models.StateBacklog),
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
	if view := sheet.render(100); !strings.Contains(ansi.Strip(view), "↑↓/Ctrl+j/k move") {
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
		taskActionTestContext(models.StateDoing),
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

func TestBulkActionSheetIntersectsApplicableOperationsForEverySelectedTask(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	registry := model.operationRegistry

	// The in-progress member is what removes snooze from the intersection.
	story := project.GetTaskByID("story")
	loose := project.GetTaskByID("loose")
	loose.WorkflowState = models.StateDoing
	context := bulkTaskActionTestContext(project, story, loose)

	descriptors := TaskActionSheetDescriptors(registry, context)
	ids := make([]OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	if !slices.Equal(ids, []OperationID{operationTaskMoveProject, operationTaskDelete}) {
		t.Fatalf("bulk action sheet IDs = %v, want Move Project and Delete", ids)
	}
	for _, excluded := range []OperationID{
		// Snooze is bulk-capable but the in-progress member removes it.
		operationTaskSetSnooze,
		// The four metadata operations reach a selection through their direct key.
		operationTaskSetState,
		operationTaskSetPriority,
		operationTaskSetDueDate,
		operationTaskSetTags,
		operationTaskSetParent,
		operationTaskIndent,
		operationTaskOutdent,
		operationCoreEdit,
		operationCorePeek,
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
	_, ok := result.(ContextActionSheetResultMsg)
	if !ok {
		t.Fatalf("bulk Enter returned %T, want ContextActionSheetResultMsg", result)
	}
}

func TestSimplifiedSpaceOpensSelectedTaskActionSheet(t *testing.T) {
	app := newUITestApp()
	task := &models.Task{
		ID:            "task",
		Title:         "Selected task",
		WorkflowState: models.StateBacklog,
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

func TestSimplifiedVisualSelectionKeysOpenBulkActionsAndEscFreezes(t *testing.T) {
	app := newUITestApp()
	first := &models.Task{ID: "first", Title: "First", WorkflowState: models.StateBacklog}
	second := &models.Task{ID: "second", Title: "Second", WorkflowState: models.StateBacklog}
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
		t.Fatal("v did not anchor a fresh visual range at the cursor row")
	}
	send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !app.crossOutliner.multiSelectMode || app.crossOutliner.SelectedCount() != 1 ||
		app.crossOutliner.IsRangeSelectionActive() {
		t.Fatal("Esc did not freeze the visual range while preserving selection")
	}

	app.crossOutliner.cursor = 0
	send(tea.KeyPressMsg{Code: 'v', Text: "v"})
	send(tea.KeyPressMsg{Code: 'j', Text: "j"})
	send(tea.KeyPressMsg{Code: 'v', Text: "v"}) // freeze the two-row range
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
	first := &models.Task{ID: "first", Title: "First", WorkflowState: models.StateBacklog}
	second := &models.Task{ID: "second", Title: "Second", WorkflowState: models.StateBacklog}
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
		ID:      operationCorePeek,
		Context: app.operationCommandContext(),
	})
	if !errors.Is(result.Err, ErrOperationUnavailable) {
		t.Fatalf("visual-selection Peek error = %v, want unavailable", result.Err)
	}
	if result.Message != nil || app.contextActionSheet != nil {
		t.Fatal("visual-selection Peek fell back to the cursor or action sheet")
	}
}
