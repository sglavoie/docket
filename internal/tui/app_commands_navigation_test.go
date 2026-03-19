package tui

import "testing"

// findCmdByName locates a command by name in a slice. Defined locally so
// compilation does not depend on helpers in other test files.
func findCmdByName(cmds []Command, name string) *Command {
	for i := range cmds {
		if cmds[i].Name == name {
			return &cmds[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// globalCommands
// ---------------------------------------------------------------------------

func TestGlobalCommands_Help(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "help")
	if cmd == nil {
		t.Fatal("expected 'help' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowHelpMsg); !ok {
		t.Errorf("expected ShowHelpMsg, got %T", msg)
	}
}

func TestGlobalCommands_Quit(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "quit")
	if cmd == nil {
		t.Fatal("expected 'quit' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestGlobalCommands_GlobalSearch(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "global search")
	if cmd == nil {
		t.Fatal("expected 'global search' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowGlobalSearchMsg); !ok {
		t.Errorf("expected ShowGlobalSearchMsg, got %T", msg)
	}
}

func TestGlobalCommands_Settings(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "settings")
	if cmd == nil {
		t.Fatal("expected 'settings' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(SettingsMsg); !ok {
		t.Errorf("expected SettingsMsg, got %T", msg)
	}
}

func TestGlobalCommands_Notifications(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "notifications")
	if cmd == nil {
		t.Fatal("expected 'notifications' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowNotificationCenterMsg); !ok {
		t.Errorf("expected ShowNotificationCenterMsg, got %T", msg)
	}
}

func TestGlobalCommands_Tutor(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "tutor")
	if cmd == nil {
		t.Fatal("expected 'tutor' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTutorMsg); !ok {
		t.Errorf("expected ShowTutorMsg, got %T", msg)
	}
}

func TestGlobalCommands_AskAI(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "ask ai")
	if cmd == nil {
		t.Fatal("expected 'ask ai' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowAskMsg); !ok {
		t.Errorf("expected ShowAskMsg, got %T", msg)
	}
}

func TestGlobalCommands_StagesSettings(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "stages settings")
	if cmd == nil {
		t.Fatal("expected 'stages settings' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(SettingsStagesMsg); !ok {
		t.Errorf("expected SettingsStagesMsg, got %T", msg)
	}
}

func TestGlobalCommands_Back(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "back")
	if cmd == nil {
		t.Fatal("expected 'back' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestGlobalCommands_SwitchProject(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "switch project")
	if cmd == nil {
		t.Fatal("expected 'switch project' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowProjectPickerMsg); !ok {
		t.Errorf("expected ShowProjectPickerMsg, got %T", msg)
	}
}

func TestGlobalCommands_WorkflowRules(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "workflow rules")
	if cmd == nil {
		t.Fatal("expected 'workflow rules' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowWorkflowRulesMsg); !ok {
		t.Errorf("expected ShowWorkflowRulesMsg, got %T", msg)
	}
}

func TestGlobalCommands_CreateWorkflowRule(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "create workflow rule")
	if cmd == nil {
		t.Fatal("expected 'create workflow rule' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowWorkflowRuleCreatorMsg); !ok {
		t.Errorf("expected ShowWorkflowRuleCreatorMsg, got %T", msg)
	}
}

func TestGlobalCommands_TaskTemplates(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "task templates")
	if cmd == nil {
		t.Fatal("expected 'task templates' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTaskTemplatesMsg); !ok {
		t.Errorf("expected ShowTaskTemplatesMsg, got %T", msg)
	}
}

func TestGlobalCommands_CreateTaskTemplate(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCmdByName(cmds, "create task template")
	if cmd == nil {
		t.Fatal("expected 'create task template' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTaskTemplateCreatorMsg); !ok {
		t.Errorf("expected ShowTaskTemplateCreatorMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// filterCommands
// ---------------------------------------------------------------------------

func TestFilterCommands_FilterByTag_WithProjectTree(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "filter by tag")
	if cmd == nil {
		t.Fatal("expected 'filter by tag' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTagFilterMsg); !ok {
		t.Errorf("expected ShowTagFilterMsg, got %T", msg)
	}
}

func TestFilterCommands_FilterByTag_WithOutliner(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.pushView(ViewOutliner)

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "filter by tag")
	if cmd == nil {
		t.Fatal("expected 'filter by tag' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTagFilterMsg); !ok {
		t.Errorf("expected ShowTagFilterMsg, got %T", msg)
	}
}

func TestFilterCommands_FilterByTag_NilBoth(t *testing.T) {
	app := newTestAppModel()

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "filter by tag")
	if cmd == nil {
		t.Fatal("expected 'filter by tag' command")
	}
	msg := cmd.Handler()
	if msg != nil {
		t.Errorf("expected nil, got %T", msg)
	}
}

func TestFilterCommands_FilterBySection(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "filter by section")
	if cmd == nil {
		t.Fatal("expected 'filter by section' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowSectionFilterSelectorMsg); !ok {
		t.Errorf("expected ShowSectionFilterSelectorMsg, got %T", msg)
	}
}

func TestFilterCommands_ClearFilters_ProjectTree(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	// Set a non-zero priority so we can verify it gets cleared.
	app.projectTree.filter.Priority = 1

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "clear filters")
	if cmd == nil {
		t.Fatal("expected 'clear filters' command")
	}
	cmd.Handler()

	if app.projectTree.filter.Priority != 0 {
		t.Errorf("expected Priority to be reset to 0, got %d", app.projectTree.filter.Priority)
	}
}

func TestFilterCommands_ClearFilters_Outliner(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.pushView(ViewOutliner)

	// Set a non-zero priority so we can verify it gets cleared.
	app.outliner.filter.Priority = 1

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "clear filters")
	if cmd == nil {
		t.Fatal("expected 'clear filters' command")
	}
	cmd.Handler()

	if app.outliner.filter.Priority != 0 {
		t.Errorf("expected Priority to be reset to 0, got %d", app.outliner.filter.Priority)
	}
}

func TestFilterCommands_JumpToTask(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "jump to task")
	if cmd == nil {
		t.Fatal("expected 'jump to task' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTaskJumperMsg); !ok {
		t.Errorf("expected ShowTaskJumperMsg, got %T", msg)
	}
}

func TestFilterCommands_JumpToTask_NilProject(t *testing.T) {
	app := newTestAppModel()

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "jump to task")
	if cmd == nil {
		t.Fatal("expected 'jump to task' command")
	}
	msg := cmd.Handler()
	if msg != nil {
		t.Errorf("expected nil, got %T", msg)
	}
}

func TestFilterCommands_TagManager(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "tag manager")
	if cmd == nil {
		t.Fatal("expected 'tag manager' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTagManagerMsg); !ok {
		t.Errorf("expected ShowTagManagerMsg, got %T", msg)
	}
}

func TestFilterCommands_GlobalTagManager(t *testing.T) {
	app := newTestAppModel()

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "global tag manager")
	if cmd == nil {
		t.Fatal("expected 'global tag manager' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowGlobalTagManagerMsg); !ok {
		t.Errorf("expected ShowGlobalTagManagerMsg, got %T", msg)
	}
}

func TestFilterCommands_SectionManager(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "custom section manager")
	if cmd == nil {
		t.Fatal("expected 'custom section manager' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowSectionManagerMsg); !ok {
		t.Errorf("expected ShowSectionManagerMsg, got %T", msg)
	}
}

func TestFilterCommands_OpenDataFolder(t *testing.T) {
	app := newTestAppModel()

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "open data folder")
	if cmd == nil {
		t.Fatal("expected 'open data folder' command")
	}
	msg := cmd.Handler()
	m, ok := msg.(OpenDataFolderMsg)
	if !ok {
		t.Errorf("expected OpenDataFolderMsg, got %T", msg)
	}
	if m.Project != nil {
		t.Errorf("expected nil project, got %v", m.Project)
	}
}

func TestFilterCommands_OpenProjectFolder(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCmdByName(cmds, "open project folder")
	if cmd == nil {
		t.Fatal("expected 'open project folder' command")
	}
	msg := cmd.Handler()
	m, ok := msg.(OpenDataFolderMsg)
	if !ok {
		t.Errorf("expected OpenDataFolderMsg, got %T", msg)
	}
	if m.Project != project {
		t.Errorf("expected project to match current project")
	}
}

// ---------------------------------------------------------------------------
// viewCommands
// ---------------------------------------------------------------------------

func TestViewCommands_AllTasks(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "all tasks")
	if cmd == nil {
		t.Fatal("expected 'all tasks' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowAllTasksMsg); !ok {
		t.Errorf("expected ShowAllTasksMsg, got %T", msg)
	}
}

func TestViewCommands_DueDateOverview(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "due date overview")
	if cmd == nil {
		t.Fatal("expected 'due date overview' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowDueDateOverviewMsg); !ok {
		t.Errorf("expected ShowDueDateOverviewMsg, got %T", msg)
	}
}

func TestViewCommands_DailyReview(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "daily review")
	if cmd == nil {
		t.Fatal("expected 'daily review' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowDailyReviewMsg); !ok {
		t.Errorf("expected ShowDailyReviewMsg, got %T", msg)
	}
}

func TestViewCommands_WeeklyReview(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "weekly review")
	if cmd == nil {
		t.Fatal("expected 'weekly review' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowWeeklyReviewMsg); !ok {
		t.Errorf("expected ShowWeeklyReviewMsg, got %T", msg)
	}
}

func TestViewCommands_LatestActivity(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "latest activity")
	if cmd == nil {
		t.Fatal("expected 'latest activity' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowRecentActivityMsg); !ok {
		t.Errorf("expected ShowRecentActivityMsg, got %T", msg)
	}
}

func TestViewCommands_WhatsNext(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "what's next")
	if cmd == nil {
		t.Fatal("expected \"what's next\" command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowFocusViewMsg); !ok {
		t.Errorf("expected ShowFocusViewMsg, got %T", msg)
	}
}

func TestViewCommands_FocusBoard(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "focus board")
	if cmd == nil {
		t.Fatal("expected 'focus board' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowFocusBoardMsg); !ok {
		t.Errorf("expected ShowFocusBoardMsg, got %T", msg)
	}
}

func TestViewCommands_PriorityView(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "priority view")
	if cmd == nil {
		t.Fatal("expected 'priority view' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowPriorityViewMsg); !ok {
		t.Errorf("expected ShowPriorityViewMsg, got %T", msg)
	}
}

func TestViewCommands_PriorityView_NilProject(t *testing.T) {
	app := newTestAppModel()

	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "priority view")
	if cmd == nil {
		t.Fatal("expected 'priority view' command")
	}
	msg := cmd.Handler()
	if msg != nil {
		t.Errorf("expected nil, got %T", msg)
	}
}

func TestViewCommands_InboxProcessor(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "inbox processor")
	if cmd == nil {
		t.Fatal("expected 'inbox processor' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowInboxProcessorMsg); !ok {
		t.Errorf("expected ShowInboxProcessorMsg, got %T", msg)
	}
}

func TestViewCommands_GlobalInboxProcessor(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "global inbox processor")
	if cmd == nil {
		t.Fatal("expected 'global inbox processor' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowGlobalInboxProcessorMsg); !ok {
		t.Errorf("expected ShowGlobalInboxProcessorMsg, got %T", msg)
	}
}

func TestViewCommands_GlobalNotes(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "global notes")
	if cmd == nil {
		t.Fatal("expected 'global notes' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowGlobalNotesMsg); !ok {
		t.Errorf("expected ShowGlobalNotesMsg, got %T", msg)
	}
}

func TestViewCommands_GlobalBlockers(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "global blockers")
	if cmd == nil {
		t.Fatal("expected 'global blockers' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowGlobalBlockersMsg); !ok {
		t.Errorf("expected ShowGlobalBlockersMsg, got %T", msg)
	}
}

func TestViewCommands_MetricsDashboard(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "metrics dashboard")
	if cmd == nil {
		t.Fatal("expected 'metrics dashboard' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowMetricsDashboardMsg); !ok {
		t.Errorf("expected ShowMetricsDashboardMsg, got %T", msg)
	}
}

func TestViewCommands_UndoHistory(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "undo history")
	if cmd == nil {
		t.Fatal("expected 'undo history' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowUndoHistoryMsg); !ok {
		t.Errorf("expected ShowUndoHistoryMsg, got %T", msg)
	}
}

func TestViewCommands_UndoHistory_NilProject(t *testing.T) {
	app := newTestAppModel()

	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "undo history")
	if cmd == nil {
		t.Fatal("expected 'undo history' command")
	}
	msg := cmd.Handler()
	if msg != nil {
		t.Errorf("expected nil, got %T", msg)
	}
}

func TestViewCommands_FilterPopup(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	cmds := app.viewCommands()
	cmd := findCmdByName(cmds, "filter popup")
	if cmd == nil {
		t.Fatal("expected 'filter popup' command")
	}
	cmd.Handler()

	if app.projectTree.pendingPrefix != "F" {
		t.Errorf("expected pendingPrefix to be \"F\", got %q", app.projectTree.pendingPrefix)
	}
}

// ---------------------------------------------------------------------------
// navCommands
// ---------------------------------------------------------------------------

func TestNavCommands_Dashboard(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCmdByName(cmds, "dashboard")
	if cmd == nil {
		t.Fatal("expected 'dashboard' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestNavCommands_ProjectCycle(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCmdByName(cmds, "project cycle")
	if cmd == nil {
		t.Fatal("expected 'project cycle' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowProjectCycleManagerMsg); !ok {
		t.Errorf("expected ShowProjectCycleManagerMsg, got %T", msg)
	}
}

func TestNavCommands_NextProject(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCmdByName(cmds, "next project")
	if cmd == nil {
		t.Fatal("expected 'next project' command")
	}
	msg := cmd.Handler()
	m, ok := msg.(CycleProjectMsg)
	if !ok {
		t.Errorf("expected CycleProjectMsg, got %T", msg)
	}
	if !m.Forward {
		t.Error("expected Forward to be true")
	}
}

func TestNavCommands_PreviousProject(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCmdByName(cmds, "previous project")
	if cmd == nil {
		t.Fatal("expected 'previous project' command")
	}
	msg := cmd.Handler()
	m, ok := msg.(CycleProjectMsg)
	if !ok {
		t.Errorf("expected CycleProjectMsg, got %T", msg)
	}
	if m.Forward {
		t.Error("expected Forward to be false")
	}
}

func TestNavCommands_HistoryBack(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCmdByName(cmds, "history back")
	if cmd == nil {
		t.Fatal("expected 'history back' command")
	}
	// Should not panic.
	cmd.Handler()
}

func TestNavCommands_HistoryForward(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCmdByName(cmds, "history forward")
	if cmd == nil {
		t.Fatal("expected 'history forward' command")
	}
	// Should not panic.
	cmd.Handler()
}
