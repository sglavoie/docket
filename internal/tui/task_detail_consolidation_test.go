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
	task, err := models.NewGenericTask("detail-task", 1, "Consolidated detail")
	if err != nil {
		t.Fatal(err)
	}
	task.Description = "Overview description"
	task.Checklist = []models.ChecklistItem{{Text: "Verify the unified surface"}}
	task.AddComment("A preserved comment")
	task.AddDecision("Use tabs", "One route keeps parent state stable")
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

func TestTaskDetailHostsEachDedicatedTaskToolWithoutCrossToolTabs(t *testing.T) {
	model, _, _ := newConsolidatedTaskDetailFixture(t)
	model.SetSize(80, 28)
	model.SetPlanningMembership(true, true)

	wantContent := map[TaskDetailTab]string{
		TaskOverview:    "Today: planned",
		TaskChecklist:   "Verify the unified surface",
		TaskComments:    "A preserved comment",
		TaskDecisions:   "Use tabs",
		TaskAttachments: "No attachments",
	}
	for _, tab := range taskDetailTabOrder {
		t.Run(tab.String(), func(t *testing.T) {
			model.SetTab(tab)
			content := ansi.Strip(model.View().Content)
			if !strings.Contains(content, wantContent[tab]) {
				t.Fatalf("tab %s missing preserved content %q:\n%s", tab, wantContent[tab], content)
			}
			if tab != TaskOverview && strings.Contains(content, "[Overview]") {
				t.Fatalf("subordinate %s tool exposes cross-tool tab navigation:\n%s", tab, content)
			}
		})
	}
}

func TestTaskDetailLegacyMessagesChangeOneTypedRouteMode(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	task := project.Tasks[0]
	app.pushView(ViewCrossProjectOutliner)
	parent := app.CurrentRoute()
	parentPath := app.presentationPath()

	messages := []struct {
		msg tea.Msg
		tab TaskDetailTab
	}{
		{ShowTaskDetailMsg{Task: task, Project: project}, TaskOverview},
		{ShowTaskChecklistMsg{Task: task, Project: project}, TaskChecklist},
		{ShowTaskCommentsMsg{Task: task, Project: project}, TaskComments},
		{ShowTaskDecisionsMsg{Task: task, Project: project}, TaskDecisions},
		{ShowTaskAttachmentsMsg{Task: task, Project: project}, TaskAttachments},
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
		if app.navigation.Primary.Depth() != 2 {
			t.Fatalf("%T grew primary stack to depth %d; child tabs must replace one mode above the sole Outliner root", tc.msg, app.navigation.Primary.Depth())
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
	if got := app.presentationPath(); len(got) != len(parentPath) || got[len(got)-1] != parentPath[len(parentPath)-1] {
		t.Fatalf("parent renderer path was not restored: got %v want %v", got, parentPath)
	}
}

func TestTaskMenuToolsCloseToSameWorkspaceCursorFiltersAndSelection(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	if len(project.Tasks) < 2 {
		t.Fatal("workspace fixture needs at least two tasks")
	}
	outliner := NewOutlinerModel(project, app.configManager, app.runtime)
	outliner.rows = []outlinerRow{{task: project.Tasks[0]}, {task: project.Tasks[1]}}
	outliner.cursor = 1
	outliner.SetSearchQuery(project.Tasks[0].Title)
	outliner.multiSelectMode = true
	outliner.SelectTaskIdentity(taskIdentityFor(project.Tasks[0], project))
	outliner.SelectTaskIdentity(taskIdentityFor(project.Tasks[1], project))
	app.crossOutliner = outliner
	app.views.Register(ViewCrossProjectOutliner, outliner)
	app.pushView(ViewCrossProjectOutliner)

	beforeSearch := outliner.SearchQuery()
	beforeCursor := outliner.cursor
	beforeSelected := make(map[TaskIdentity]bool, len(outliner.selectedTaskIdentities))
	for identity, selected := range outliner.selectedTaskIdentities {
		beforeSelected[identity] = selected
	}
	for _, tool := range []TaskDetailTab{TaskChecklist, TaskComments, TaskDecisions, TaskAttachments} {
		_ = app.activateTaskDetail(project.Tasks[0], project, tool)
		if !app.popTypedPresentation() {
			t.Fatalf("failed to close %s task tool", tool)
		}
		if outliner.SearchQuery() != beforeSearch || outliner.cursor != beforeCursor {
			t.Fatalf("%s changed Outliner return state: search=%q cursor=%d", tool, outliner.SearchQuery(), outliner.cursor)
		}
		if len(outliner.selectedTaskIdentities) != len(beforeSelected) {
			t.Fatalf("%s changed selection size: got %+v want %+v", tool, outliner.selectedTaskIdentities, beforeSelected)
		}
		for identity, selected := range beforeSelected {
			if outliner.selectedTaskIdentities[identity] != selected {
				t.Fatalf("%s changed selection: got %+v want %+v", tool, outliner.selectedTaskIdentities, beforeSelected)
			}
		}
	}
}

func TestTaskDetailSubordinateToolsDoNotCycleOnTab(t *testing.T) {
	model, _, _ := newConsolidatedTaskDetailFixture(t)
	model.SetSize(80, 28)
	model.SetTab(TaskChecklist)

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if updated != model || model.Tab() != TaskChecklist || cmd != nil {
		t.Fatalf("Tab result model=%T tab=%s cmd=%T", updated, model.Tab(), cmd)
	}

	_, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if model.Tab() != TaskChecklist || cmd != nil {
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
