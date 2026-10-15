package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func taskActionTestContext() operation.CommandContext {
	context := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	context.Entity = operation.EntityContext{ProjectID: "project", TaskID: "task"}
	return context
}

// bulkTaskActionTestContext freezes a visual selection over live tasks, the same
// shape openTaskActionSheet builds from the Outliner's selected rows.
func bulkTaskActionTestContext(project *models.Project, tasks ...*models.Task) operation.CommandContext {
	context := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	context.Selection.Visual = true
	context.Selection.Tasks = make([]taskref.TaskIdentity, 0, len(tasks))
	context.Selection.Entities = make([]operation.EntityContext, 0, len(tasks))
	for _, task := range tasks {
		context.Selection.Tasks = append(context.Selection.Tasks, taskref.For(task, project))
		context.Selection.Entities = append(context.Selection.Entities, outliner.TaskOperationEntity(task, project))
	}
	context.Entity = context.Selection.Entities[0]
	return context
}

// taskActionTestDescriptors builds the production planning/structure
// descriptors. The app carries no projects, so structure-dependent applicability
// falls back to structural eligibility.
func taskActionTestDescriptors() []operation.OperationDescriptor {
	return newUITestApp().taskActionOperationDescriptors()
}

func registerTaskActionTestDescriptor(
	t *testing.T,
	registry *operation.OperationRegistry,
	descriptor operation.OperationDescriptor,
) {
	t.Helper()
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("register %s: %v", descriptor.ID, err)
	}
}

func TestTaskActionSheetMissingSelectionSchedulesStatusClear(t *testing.T) {
	model := newUITestApp()
	result := model.executeCoreInputOperation(operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationCoreActions},
	})

	if result.Err != nil {
		t.Fatalf("open task actions: %v", result.Err)
	}
	if model.feedback.message != "select a task to open actions" {
		t.Fatalf("status = %q, want missing-selection guidance", model.feedback.message)
	}
	timeout, ok := result.Message.(operation.OperationCommandResultMsg)
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
	registry := model.operations.Registry
	registerTaskActionTestDescriptor(t, registry, operation.OperationDescriptor{
		ID:                   operation.OperationID("test.task.unlisted"),
		Label:                "Unlisted Task Tool",
		Category:             operation.OperationCategoryTask,
		TargetSource:         operation.OperationTargetCurrentTask,
		TargetCardinality:    operation.TargetCardinalityOne,
		RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
		Risk:                 operation.OperationRiskReadOnly,
	})

	descriptors := TaskActionSheetDescriptors(registry, context)
	ids := make([]operation.OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	wantIDs := []operation.OperationID{
		operation.OperationTaskSetCheckIn,
		operation.OperationTaskDelete,
	}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("action sheet IDs = %v, want %v", ids, wantIDs)
	}
	wantLabels := []string{
		"Set Check-In",
		"Delete",
	}
	for index, descriptor := range descriptors {
		if descriptor.Label != wantLabels[index] {
			t.Errorf("action %d label = %q, want %q", index, descriptor.Label, wantLabels[index])
		}
	}
	// Direct-key core navigation, capture, and the sheet's own opener are not
	// task actions; the metadata operations the input strategy binds directly
	// leave the sheet rather than repeating their key as a row. The registered
	// project operations are listed too: the one-path gate reads both manifests
	// concatenated, so only the two sheets asking their own manifest keeps a
	// project row out of the task menu.
	for _, excluded := range []operation.OperationID{
		operation.OperationProjectActions,
		operation.OperationProjectDelete,
		operation.OperationScopeRenameProject,
		operation.OperationScopePinProject,
		operation.OperationScopeArchiveProject,
		operation.OperationScopeRestoreProject,
		operation.OperationProjectSetTags,
		operation.OperationCoreActions,
		operation.OperationCoreCapture,
		operation.OperationCoreMultiSelect,
		operation.OperationCoreCursorDown,
		operation.OperationTaskSetState,
		operation.OperationTaskSetPriority,
		operation.OperationTaskSetDueDate,
		operation.OperationTaskSetTags,
		operation.OperationTaskTogglePin,
		operation.OperationTaskSetParent,
		operation.OperationTaskAdoptChildren,
		operation.OperationTaskIndent,
		operation.OperationTaskOutdent,
		operation.OperationCoreEdit,
		operation.OperationCoreEditDescription,
		operation.OperationCoreExternalEditor,
		operation.OperationCorePeek,
		operation.OperationTaskCopyTitle,
		operation.OperationTaskCopyDetails,
		operation.OperationTaskCopyTitleDescription,
		operation.OperationTaskNewChild,
		operation.OperationTaskComments,
		operation.OperationTaskAttachments,
	} {
		if slices.Contains(ids, excluded) {
			t.Errorf("action sheet exposed non-contextual %s: %v", excluded, ids)
		}
	}

	sheet, err := NewTaskActionSheetModel(
		registry,
		operation.NewOperationExecutor(registry),
		context,
		"Selected task",
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	rows := sheet.Surface.Rows()
	seenCategories := make(map[listsurface.ListCategoryID]bool)
	for _, row := range rows {
		if row.Kind == listsurface.ListRowCategory {
			seenCategories[row.Category.ID] = true
		}
	}
	// The reusable schedule editor and destructive action keep their distinct
	// Plan and Danger groups.
	if !seenCategories[taskActionCategoryPlan] {
		t.Errorf("grouped sheet missing category %s", taskActionCategoryPlan)
	}
	if !seenCategories[taskActionCategoryDanger] {
		t.Errorf("grouped sheet missing category %s", taskActionCategoryDanger)
	}
	if seenCategories[taskActionCategoryContent] {
		t.Errorf("grouped sheet still renders the retired %s header", taskActionCategoryContent)
	}

	// Printable letters filter only after the search key. The query includes k,
	// which is navigation while the sheet is browsing.
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if query := sheet.Surface.Query().Raw; query != "" {
		t.Fatalf("a browsing letter wrote the query %q", query)
	}
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, letter := range "task" {
		_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: letter, Text: string(letter)})
	}
	matches := sheet.Surface.Matches()
	wantAliasMatches := []listsurface.ListItemID{listsurface.ListItemID("task-action:" + string(operation.OperationTaskDelete))}
	gotMatches := make([]listsurface.ListItemID, 0, len(matches))
	for _, match := range matches {
		gotMatches = append(gotMatches, match.Item.ID)
	}
	if !slices.Equal(gotMatches, wantAliasMatches) {
		t.Fatalf("search results = %v, want the alias matches %v", gotMatches, wantAliasMatches)
	}
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: 'l', Text: "l"})
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: 't', Text: "t"})
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	matches = sheet.Surface.Matches()
	wantDelete := listsurface.ListItemID("task-action:" + string(operation.OperationTaskDelete))
	if len(matches) != 1 || matches[0].Item.ID != wantDelete {
		t.Fatalf("search results = %+v, want only %s", matches, wantDelete)
	}
	if got := matches[0].Item.SecondaryText; got != "confirmation required" {
		t.Errorf("delete secondary text = %q, want the confirmation hint", got)
	}
}

// TestActionSheetBrowsesWithJAndK covers the shared sheet navigation
// independently from the production task catalog.
func TestActionSheetBrowsesWithJAndK(t *testing.T) {
	registry, executor := contextActionTestEngine(t, []operation.OperationDescriptor{
		contextActionTestDescriptor("test.first", "First"),
		contextActionTestDescriptor("test.second", "Second"),
	}, nil)
	sheet, err := listsurface.NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		[]operation.OperationID{"test.first", "test.second"},
		listsurface.ContextActionSheetConfig{Host: listsurface.ContextActionHostOutliner},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}

	first, ok := sheet.Surface.CurrentItem()
	if !ok {
		t.Fatal("action sheet has no initial item")
	}
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	second, ok := sheet.Surface.CurrentItem()
	if !ok || second.ID == first.ID {
		t.Fatalf("j did not move down: first = %q, current = %q", first.ID, second.ID)
	}
	_, _, _ = sheet.HandleKey(tea.KeyPressMsg{Code: 'k', Text: "k"})
	current, ok := sheet.Surface.CurrentItem()
	if !ok || current.ID != first.ID {
		t.Fatalf("k did not move back up: current = %q, want %q", current.ID, first.ID)
	}
	if query := sheet.Surface.Query().Raw; query != "" {
		t.Fatalf("j/k changed the query to %q", query)
	}
	view := ansi.Strip(sheet.Render(100))
	if !strings.Contains(view, "j/k navigate") {
		t.Fatalf("action sheet footer does not advertise j/k:\n%s", view)
	}
	// A sheet short enough to read whole does not advertise searching it.
	if len(sheet.Surface.Items()) <= 5 && strings.Contains(view, "/ search") {
		t.Fatalf("a short action sheet advertises / search:\n%s", view)
	}
	if strings.Contains(view, "Search:") {
		t.Fatalf("a browsing sheet drew a search row:\n%s", view)
	}
}

func TestTaskActionSheetOmitsDirectDateOperation(t *testing.T) {
	registry := operation.NewOperationRegistry()
	for _, descriptor := range taskActionTestDescriptors() {
		registerTaskActionTestDescriptor(t, registry, descriptor)
	}
	sheet, err := NewTaskActionSheetModel(
		registry,
		operation.NewOperationExecutor(registry),
		taskActionTestContext(),
		"Active task",
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	for _, item := range sheet.Surface.Items() {
		if item.ID == listsurface.ListItemID("task-action:"+string(operation.OperationTaskSetDueDate)) {
			t.Fatal("direct date operation is repeated in the task menu")
		}
	}
}

func TestBulkActionSheetIntersectsApplicableOperationsForEverySelectedTask(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	registry := model.operations.Registry

	story := project.GetTaskByID("story")
	loose := project.GetTaskByID("loose")
	loose.WorkflowState = models.StateDoing
	context := bulkTaskActionTestContext(project, story, loose)

	descriptors := TaskActionSheetDescriptors(registry, context)
	ids := make([]operation.OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	if !slices.Equal(ids, []operation.OperationID{operation.OperationTaskSetCheckIn, operation.OperationTaskDelete}) {
		t.Fatalf("bulk action sheet IDs = %v, want Set Check-In and Delete", ids)
	}
	for _, excluded := range []operation.OperationID{
		// The four metadata operations reach a selection through their direct key.
		operation.OperationTaskSetState,
		operation.OperationTaskSetPriority,
		operation.OperationTaskSetDueDate,
		operation.OperationTaskSetTags,
		operation.OperationTaskSetParent,
		operation.OperationTaskAdoptChildren,
		operation.OperationTaskIndent,
		operation.OperationTaskOutdent,
		operation.OperationCoreEdit,
		operation.OperationCorePeek,
		operation.OperationTaskCopyTitle,
		operation.OperationTaskNewChild,
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

	sheet, err := NewTaskActionSheetModel(registry, operation.NewOperationExecutor(registry), context, "2 tasks")
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	if !sheet.Surface.SetCursorToID("task-action:" + listsurface.ListItemID(operation.OperationTaskSetCheckIn)) {
		t.Fatal("set check-in action is missing from the bulk sheet")
	}
	if !sheet.Surface.SetCursorToID("task-action:" + listsurface.ListItemID(operation.OperationTaskDelete)) {
		t.Fatal("delete action is missing from the bulk sheet")
	}
	_, _, result := sheet.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	_, ok := result.(listsurface.ContextActionSheetResultMsg)
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
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Rows = []outliner.OutlinerRow{{Task: task, ProjectID: project.ID}}

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeySpace})
	if !handled || cmd != nil {
		t.Fatalf("Space = handled %v, cmd %T; want handled with no command", handled, cmd)
	}
	if app.operations.ActionSheet == nil {
		t.Fatal("Space did not open the selected-task action sheet")
	}
	if app.operations.ValuePicker != nil {
		t.Fatal("Space skipped the action sheet and opened a value picker")
	}
	if got := len(app.operations.ActionSheet.Surface.Items()); got == 0 {
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
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Rows = []outliner.OutlinerRow{
		{Task: first, ProjectID: project.ID},
		{Task: second, ProjectID: project.ID},
	}

	send := func(msg tea.KeyPressMsg) {
		t.Helper()
		handled, cmd := app.handleGlobalKeyPress(msg)
		if !handled || cmd != nil {
			t.Fatalf("%q = handled %v cmd %T; want synchronous handling", msg.String(), handled, cmd)
		}
	}
	send(tea.KeyPressMsg{Code: 'V', Text: "V"})
	if !app.crossOutliner.IsTaskIdentitySelected(taskref.For(first, project)) {
		t.Fatal("V did not anchor a fresh visual range at the cursor row")
	}
	send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !app.crossOutliner.MultiSelectMode || app.crossOutliner.SelectedCount() != 1 ||
		app.crossOutliner.IsRangeSelectionActive() {
		t.Fatal("Esc did not freeze the visual range while preserving selection")
	}

	send(tea.KeyPressMsg{Code: tea.KeyEscape}) // drop the frozen selection
	app.crossOutliner.Cursor = 0
	send(tea.KeyPressMsg{Code: 'v', Text: "v"})
	send(tea.KeyPressMsg{Code: 'j', Text: "j"})
	send(tea.KeyPressMsg{Code: 'v', Text: "v"}) // toggle the second row into the selection
	send(tea.KeyPressMsg{Code: tea.KeySpace})
	sheet := app.operations.ActionSheet
	if sheet == nil || !sheet.Config.Bulk || sheet.Config.Targets != 2 {
		t.Fatalf("Space did not open a two-target bulk action sheet: %#v", sheet)
	}
	// Priority now belongs to the p chord, so the bulk sheet must not repeat it.
	if sheet.Surface.SetCursorToID(
		listsurface.ListItemID("task-action:" + string(operation.OperationTaskSetPriority)),
	) {
		t.Fatal("bulk action sheet still lists the direct-only set-priority action")
	}
	if sheet.Surface.SetCursorToID(
		listsurface.ListItemID("task-action:" + string(operation.OperationTaskSetDueDate)),
	) {
		t.Fatal("bulk action sheet repeats direct-only set-due-date")
	}
	if !sheet.Surface.SetCursorToID(
		listsurface.ListItemID("task-action:" + string(operation.OperationTaskDelete)),
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
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Rows = []outliner.OutlinerRow{
		{Task: first, ProjectID: project.ID},
		{Task: second, ProjectID: project.ID},
	}
	app.crossOutliner.ToggleCursorSelection()
	app.crossOutliner.Cursor = 1

	result := app.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationCorePeek,
		Context: app.operationCommandContext(),
	})
	if !errors.Is(result.Err, operation.ErrOperationUnavailable) {
		t.Fatalf("visual-selection Peek error = %v, want unavailable", result.Err)
	}
	if result.Message != nil || app.operations.ActionSheet != nil {
		t.Fatal("visual-selection Peek fell back to the cursor or action sheet")
	}
}
