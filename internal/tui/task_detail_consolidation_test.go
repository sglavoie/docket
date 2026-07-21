package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func newConsolidatedTaskDetailFixture(t *testing.T) (*TaskDetailModel, *models.Task, *models.Project) {
	t.Helper()
	project, err := models.NewProject("detail-project", "Detail", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("detail-task", 1, models.TaskTypeTask, "Consolidated detail")
	if err != nil {
		t.Fatal(err)
	}
	task.Description = "Overview description"
	task.Checklist = []models.ChecklistItem{{Text: "Verify the unified surface"}}
	task.AddComment("A preserved comment")
	task.AddDecision("Use tabs", "One route keeps parent state stable", models.DecisionCategoryDesign)
	project.Tasks = []*models.Task{task}
	model := NewTaskDetailModelForTab(
		task,
		project,
		nil,
		TaskOverview,
		"",
		SubviewCoordinator{},
		AttachmentCoordinator{},
	)
	return model, task, project
}

func TestTaskDetailContainsEveryConsolidatedTab(t *testing.T) {
	model, _, _ := newConsolidatedTaskDetailFixture(t)
	model.SetSize(80, 28)
	model.SetPlanningMembership(true, true)

	wantContent := map[TaskDetailTab]string{
		TaskOverview:    "Today: planned",
		TaskChecklist:   "Verify the unified surface",
		TaskComments:    "A preserved comment",
		TaskDecisions:   "Use tabs",
		TaskAttachments: "No attachments",
		TaskActivity:    "Activity Log",
	}
	for _, tab := range taskDetailTabOrder {
		t.Run(tab.String(), func(t *testing.T) {
			model.SetTab(tab)
			content := ansi.Strip(model.View().Content)
			if !strings.Contains(content, "["+tab.String()+"]") {
				t.Fatalf("active tab %s is not visible:\n%s", tab, content)
			}
			if !strings.Contains(content, wantContent[tab]) {
				t.Fatalf("tab %s missing preserved content %q:\n%s", tab, wantContent[tab], content)
			}
		})
	}
}

func TestTaskDetailLegacyMessagesChangeOneTypedRouteMode(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	task := project.Tasks[0]
	app.pushView(ViewOutliner)
	parent := app.CurrentRoute()
	parentMirror := append([]PresentationID(nil), app.viewStack...)

	messages := []struct {
		msg tea.Msg
		tab TaskDetailTab
	}{
		{ShowTaskDetailMsg{Task: task, Project: project}, TaskOverview},
		{ShowTaskChecklistMsg{Task: task, Project: project}, TaskChecklist},
		{ShowTaskCommentsMsg{Task: task, Project: project}, TaskComments},
		{ShowTaskDecisionsMsg{Task: task, Project: project}, TaskDecisions},
		{ShowTaskAttachmentsMsg{Task: task, Project: project}, TaskAttachments},
		{OpenTaskActivityMsg{Task: task, Project: project}, TaskActivity},
	}
	for _, tc := range messages {
		if _, cmd := app.Update(tc.msg); cmd != nil {
			_ = cmd()
		}
		route := app.CurrentRoute()
		params, ok := route.Params().(TaskDetailParams)
		if route.ID() != TaskDetail || !ok || params.Tab != tc.tab {
			t.Fatalf("%T route = %s/%+v, want TaskDetail/%s", tc.msg, route.ID(), route.Params(), tc.tab)
		}
		if app.navigation.Primary.Depth() != 3 {
			t.Fatalf("%T grew primary stack to depth %d; child tabs must replace one mode", tc.msg, app.navigation.Primary.Depth())
		}
		if app.activeView() != app.taskDetail {
			t.Fatalf("%T selected %T instead of unified TaskDetailModel", tc.msg, app.activeView())
		}
	}

	handled, cmd := app.handleEscKey()
	if !handled {
		t.Fatal("Esc did not close Task Detail")
	}
	if cmd != nil {
		_ = cmd()
	}
	if !app.CurrentRoute().Equal(parent) {
		t.Fatalf("parent route changed after Detail close: got %+v want %+v", app.CurrentRoute(), parent)
	}
	if got := app.viewStack; len(got) != len(parentMirror) || got[len(got)-1] != parentMirror[len(parentMirror)-1] {
		t.Fatalf("parent presentation mirror was not restored: got %v want %v", got, parentMirror)
	}
}

func TestTaskDetailClosePreservesWorkspaceCursorFiltersAndSelection(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	if len(project.Tasks) < 2 {
		t.Fatal("workspace fixture needs at least two tasks")
	}
	params := TaskWorkspaceParams{
		Scope:  ProjectEntityScope(project.ID),
		Layout: WorkspaceOutline,
		Focus:  WorkspaceActive,
	}
	controller, err := NewTaskWorkspaceController(
		params,
		NewTaskWorkspaceQuery(staticTaskWorkspaceSource{data: TaskWorkspaceData{Projects: []*models.Project{project}}}),
		NewFeatureTaskWorkspaceMutationAdapter(TaskItemCoordinator{}, BulkCoordinator{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	view, err := NewTaskWorkspaceView(controller)
	if err != nil {
		t.Fatal(err)
	}
	app.taskWorkspace = controller
	app.taskWorkspaceView = view
	route, err := NewRoute(TaskWorkspace, params)
	if err != nil {
		t.Fatal(err)
	}
	if cmd := app.activateTaskWorkspaceRoute(route); cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("activate workspace returned %T", msg)
		}
	}

	first := TaskIdentity{ProjectID: project.ID, TaskID: project.Tasks[0].ID}
	second := TaskIdentity{ProjectID: project.ID, TaskID: project.Tasks[1].ID}
	controller.State.SetSearchQuery(project.Tasks[0].Title)
	if !controller.State.SetCursor(first) || !controller.State.ToggleSelection(first) || !controller.State.ToggleSelection(second) {
		t.Fatal("failed to seed workspace return state")
	}

	beforeParams := controller.State.RouteParams()
	beforeSearch := controller.State.SearchQuery
	beforeCursor, _ := controller.State.Cursor()
	beforeSelected := controller.State.Selected()
	_ = app.activateTaskDetail(project.Tasks[0], project, TaskOverview)
	if !app.popTypedPresentation() {
		t.Fatal("failed to close Task Detail")
	}

	afterCursor, _ := controller.State.Cursor()
	afterSelected := controller.State.Selected()
	if controller.State.RouteParams() != beforeParams || controller.State.SearchQuery != beforeSearch || afterCursor != beforeCursor {
		t.Fatalf("workspace return state changed: params=%+v search=%q cursor=%+v", controller.State.RouteParams(), controller.State.SearchQuery, afterCursor)
	}
	if len(afterSelected) != len(beforeSelected) {
		t.Fatalf("selection size changed: got %+v want %+v", afterSelected, beforeSelected)
	}
	for i := range beforeSelected {
		if afterSelected[i] != beforeSelected[i] {
			t.Fatalf("selection changed: got %+v want %+v", afterSelected, beforeSelected)
		}
	}
}

func TestTaskDetailTabNavigationAndInlineInputCapture(t *testing.T) {
	model, _, _ := newConsolidatedTaskDetailFixture(t)
	model.SetSize(80, 28)

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if updated != model || model.Tab() != TaskChecklist || cmd == nil {
		t.Fatalf("Tab result model=%T tab=%s cmd=%T", updated, model.Tab(), cmd)
	}
	if msg := cmd(); msg != (TaskDetailTabChangedMsg{Tab: TaskChecklist}) {
		t.Fatalf("Tab message = %#v", msg)
	}

	_, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if model.Tab() != TaskOverview || cmd == nil {
		t.Fatalf("Shift+Tab tab=%s cmd=%T", model.Tab(), cmd)
	}

	model.SetTab(TaskComments)
	model.comments.addMode = true
	model.comments.textInput.Focus()
	_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if model.Tab() != TaskComments {
		t.Fatalf("Tab escaped active comment input to %s", model.Tab())
	}
}

func TestTaskDetailChildOperationsAreNotShadowedByGlobalKeys(t *testing.T) {
	app := typedTestApp(t)
	task := app.currentProject.Tasks[0]

	_ = app.activateTaskDetail(task, app.currentProject, TaskChecklist)
	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter}); handled {
		t.Fatal("global Enter shadowed Checklist toggle")
	}

	_ = app.activateTaskDetail(task, app.currentProject, TaskComments)
	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'y'}); handled {
		t.Fatal("global task-title copy shadowed Comments copy action")
	}

	_ = app.activateTaskDetail(task, app.currentProject, TaskActivity)
	before := app.taskDetail.activity.activeTab
	_, _ = app.taskDetail.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	if app.taskDetail.activity.activeTab == before {
		t.Fatal("Activity filter navigation was not preserved")
	}
}

func TestTaskDetailDeletedTaskReturnsToParent(t *testing.T) {
	model, task, project := newConsolidatedTaskDetailFixture(t)
	project.Tasks = nil
	_, cmd := model.Update(TaskUpdatedMsg{Task: task})
	if cmd == nil {
		t.Fatal("deleted task did not request return to parent")
	}
	if _, ok := cmd().(BackMsg); !ok {
		t.Fatalf("deleted task returned %T, want BackMsg", cmd())
	}
	if model.task != nil {
		t.Fatal("deleted task remained attached to the detail model")
	}
}

func TestTaskDetailEveryTabFitsMinimumWidth(t *testing.T) {
	model, _, _ := newConsolidatedTaskDetailFixture(t)
	const width = minTerminalWidth
	model.SetSize(width, 24)
	for _, tab := range taskDetailTabOrder {
		model.SetTab(tab)
		content := model.View().Content
		for line := range strings.SplitSeq(content, "\n") {
			if got := lipgloss.Width(ansi.Strip(line)); got > width {
				t.Fatalf("%s line width = %d > %d: %q", tab, got, width, ansi.Strip(line))
			}
		}
	}
}

func TestTaskDetailUndoRemainsAvailableOnEveryTab(t *testing.T) {
	app := typedTestApp(t)
	task := app.currentProject.Tasks[0]
	for _, tab := range taskDetailTabOrder {
		_ = app.activateTaskDetail(task, app.currentProject, tab)
		if !app.currentViewSupportsGlobalTaskUndoRedo() {
			t.Fatalf("undo unavailable on %s tab", tab)
		}
	}
}
