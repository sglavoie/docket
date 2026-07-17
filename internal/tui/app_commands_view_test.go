package tui

import "testing"

func registerCommandGroupsForTest(t *testing.T, groups ...[]Command) *CommandRegistry {
	t.Helper()
	registry := NewCommandRegistry()
	for _, commands := range groups {
		if err := registry.RegisterBatch(commands); err != nil {
			t.Fatalf("RegisterBatch failed: %v", err)
		}
	}
	return registry
}

func assertPickerCommandPurpose(t *testing.T, cmd *Command, want PickerPurpose) {
	t.Helper()
	if cmd == nil {
		t.Fatal("command is nil")
	}
	msg := cmd.Handler()
	pickerMsg, ok := msg.(ShowGlobalProjectPickerMsg)
	if !ok {
		t.Fatalf("expected ShowGlobalProjectPickerMsg, got %T", msg)
	}
	if pickerMsg.Purpose != want {
		t.Fatalf("picker purpose = %v, want %v", pickerMsg.Purpose, want)
	}
}

func TestViewCommands_AllTasks(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCommand(cmds, "all tasks")
	if cmd == nil {
		t.Fatal("expected 'all tasks' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowAllTasksMsg); !ok {
		t.Errorf("expected ShowAllTasksMsg, got %T", msg)
	}
}

func TestProjectViewToggle_CommandMetadataAndScopes(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	cmd := findCommand(app.viewCommands(), "toggle project tree/outliner")
	if cmd == nil {
		t.Fatal("expected 'toggle project tree/outliner' command")
	}
	if cmd.Description != "Toggle project tree/outliner" {
		t.Fatalf("description = %q", cmd.Description)
	}
	if cmd.Shortcut != "ctrl+p" {
		t.Fatalf("shortcut = %q, want ctrl+p", cmd.Shortcut)
	}
	if len(cmd.Views) != 2 || !containsView(cmd.Views, ViewProjectTree) || !containsView(cmd.Views, ViewOutliner) {
		t.Fatalf("views = %v, want project tree and outliner", cmd.Views)
	}
	for _, alias := range []string{
		"project tree",
		"project outliner",
		"tree view",
		"outliner view",
		"toggle project view",
		"switch project view",
	} {
		if !commandHasAlias(cmd, alias) {
			t.Errorf("aliases missing %q: %v", alias, cmd.Aliases)
		}
	}

	registry := registerCommandGroupsForTest(t, app.viewCommands())
	if findCommand(registry.GetForView(ViewProjectTree), cmd.Name) == nil {
		t.Fatal("toggle command missing from project-tree scope")
	}
	if findCommand(registry.GetForView(ViewOutliner), cmd.Name) == nil {
		t.Fatal("toggle command missing from single-project outliner scope")
	}
	if findCommand(registry.GetForView(ViewCrossProjectOutliner), cmd.Name) != nil {
		t.Fatal("toggle command must not be exposed in cross-project outliner scope")
	}

	crossCmd := findCommand(registry.GetForView(ViewCrossProjectOutliner), "project tree")
	if crossCmd == nil {
		t.Fatal("cross-project outliner missing 'project tree' command")
	}
	if crossCmd.Description != "Open highlighted project's tree" || crossCmd.Shortcut != "ctrl+p" {
		t.Fatalf("cross-project metadata = description %q shortcut %q", crossCmd.Description, crossCmd.Shortcut)
	}
	if findCommand(registry.GetForView(ViewProjectTree), "project tree") != nil ||
		findCommand(registry.GetForView(ViewOutliner), "project tree") != nil {
		t.Fatal("cross-project tree command leaked into a single-project scope")
	}
}

func TestProjectViewToggle_CommandRoutesByNonPaletteOrigin(t *testing.T) {
	app, project := newTestAppModelWithProject()
	cmd := findCommand(app.viewCommands(), "toggle project tree/outliner")
	if cmd == nil {
		t.Fatal("expected 'toggle project tree/outliner' command")
	}

	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewCommandPalette}
	msg := cmd.Handler()
	showOutliner, ok := msg.(ShowOutlinerMsg)
	if !ok {
		t.Fatalf("tree origin returned %T, want ShowOutlinerMsg", msg)
	}
	if showOutliner.Project != project {
		t.Fatalf("outliner project = %v, want %v", showOutliner.Project, project)
	}

	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.viewStack = []ViewType{ViewDashboard, ViewOutliner, ViewCommandPalette}
	msg = cmd.Handler()
	showTree, ok := msg.(ShowProjectTreeMsg)
	if !ok {
		t.Fatalf("outliner origin returned %T, want ShowProjectTreeMsg", msg)
	}
	if showTree.Project != project {
		t.Fatalf("tree project = %v, want %v", showTree.Project, project)
	}
}

func TestViewCommands_DashboardSemicolonRoutes(t *testing.T) {
	app := newTestAppModel()
	registry := registerCommandGroupsForTest(t, app.viewCommands())
	cmds := registry.GetForView(ViewDashboard)

	tests := []struct {
		name     string
		shortcut string
		wantMsg  any
	}{
		{"all tasks", ";a", ShowAllTasksMsg{}},
		{"outliner", ";o", ShowGlobalOutlinerMsg{}},
		{"kanban", ";k", ShowCrossProjectKanbanMsg{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := findCommand(cmds, tt.name)
			if cmd == nil {
				t.Fatalf("%q should be reachable from ViewDashboard", tt.name)
			}
			if cmd.Shortcut != tt.shortcut {
				t.Fatalf("%q shortcut = %q, want %q", tt.name, cmd.Shortcut, tt.shortcut)
			}
			if got := cmd.Handler(); got != tt.wantMsg {
				t.Fatalf("%q handler returned %T, want %T", tt.name, got, tt.wantMsg)
			}
		})
	}
}

func TestViewCommands_DueDateOverview(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCommand(cmds, "due date overview")
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
	cmd := findCommand(cmds, "daily review")
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
	cmd := findCommand(cmds, "weekly review")
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
	cmd := findCommand(cmds, "latest activity")
	if cmd == nil {
		t.Fatal("expected 'latest activity' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowRecentActivityMsg); !ok {
		t.Errorf("expected ShowRecentActivityMsg, got %T", msg)
	}
}

func TestViewCommands_Backlog(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCommand(cmds, "backlog")
	if cmd == nil {
		t.Fatal("expected \"backlog\" command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowFocusViewMsg); !ok {
		t.Errorf("expected ShowFocusViewMsg, got %T", msg)
	}
}

func TestViewCommands_FocusBoard(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCommand(cmds, "today")
	if cmd == nil {
		t.Fatal("expected 'today' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowFocusBoardMsg); !ok {
		t.Errorf("expected ShowFocusBoardMsg, got %T", msg)
	}
}

func TestCommandRegistry_AppLevelShortcutScopes(t *testing.T) {
	app := newTestAppModel()
	registry := registerCommandGroupsForTest(t, app.navCommands(), app.notesCommands(), app.taskCommands())

	if cmd := findCommand(registry.GetForView(ViewDashboard), "peek task detail"); cmd != nil {
		t.Fatalf("dashboard should not expose task peek command: %+v", cmd)
	}
	if cmd := findCommand(registry.GetForView(ViewTaskDetail), "peek task detail"); cmd != nil {
		t.Fatalf("task detail should not expose task peek command: %+v", cmd)
	}
	peekCmd := findCommand(registry.GetForView(ViewOutliner), "peek task detail")
	if peekCmd == nil {
		t.Fatal("outliner should expose task peek command")
	}
	if peekCmd.Shortcut != "space" {
		t.Fatalf("peek shortcut = %q, want space", peekCmd.Shortcut)
	}
	if peekCmd.Description != "Toggle peek for the selected task's details" {
		t.Fatalf("peek description = %q", peekCmd.Description)
	}

	notesCmd := findCommand(registry.GetForView(ViewProjectRecentActivity), "show notes")
	if notesCmd == nil {
		t.Fatal("project recent activity should expose show notes command")
	}
	if notesCmd.Shortcut != "ctrl+n" {
		t.Fatalf("show notes shortcut = %q, want ctrl+n", notesCmd.Shortcut)
	}

	assertPickerCommandPurpose(t, findCommand(registry.GetForView(ViewDashboard), "global priority update"), PickerPurposePriorityUpdate)
	assertPickerCommandPurpose(t, findCommand(registry.GetForView(ViewDashboard), "global due date update"), PickerPurposeDueDate)
	assertPickerCommandPurpose(t, findCommand(registry.GetForView(ViewDashboard), "global section change"), PickerPurposeSectionChange)
}

func TestViewCommands_PriorityView(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.viewCommands()
	cmd := findCommand(cmds, "priority view")
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
	cmd := findCommand(cmds, "priority view")
	if cmd == nil {
		t.Fatal("expected 'priority view' command")
	}
	msg := cmd.Handler()
	if msg != nil {
		t.Errorf("expected nil, got %T", msg)
	}
}

// TestViewCommands_PriorityView_CrossOutlinerScope verifies that the
// `priority view` command is exposed in the cross-project outliner scope so
// the palette can reach the Priority-mode toggle from there.
func TestViewCommands_PriorityView_CrossOutlinerScope(t *testing.T) {
	app := newTestAppModel()
	registry := registerCommandGroupsForTest(t, app.viewCommands())

	cmd := findCommand(registry.GetForView(ViewCrossProjectOutliner), "priority view")
	if cmd == nil {
		t.Fatal("`priority view` command should be reachable from ViewCrossProjectOutliner")
	}
	if cmd.Shortcut != "p" {
		t.Errorf("priority view shortcut = %q, want p", cmd.Shortcut)
	}
}

// TestViewCommands_PriorityView_Aliases verifies that the `priority view`
// command advertises the cross-project alias set so palette search surfaces
// it under "priority mode" / "toggle priority mode" / "cross-project priority".
func TestViewCommands_PriorityView_Aliases(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCommand(cmds, "priority view")
	if cmd == nil {
		t.Fatal("expected 'priority view' command")
	}

	wantAliases := []string{"priority mode", "toggle priority mode", "cross-project priority"}
	for _, want := range wantAliases {
		found := false
		for _, alias := range cmd.Aliases {
			if alias == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("priority view aliases missing %q, got %v", want, cmd.Aliases)
		}
	}
}

// TestViewCommands_PriorityView_TogglesCrossMode verifies that running the
// `priority view` command handler when baseProjectView() == ViewCrossProjectOutliner
// flips m.crossOutliner.crossPriorityMode rather than emitting ShowPriorityViewMsg.
func TestViewCommands_PriorityView_TogglesCrossMode(t *testing.T) {
	app := newTestAppModel()
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.viewStack = []ViewType{ViewDashboard, ViewCrossProjectOutliner}

	cmds := app.viewCommands()
	cmd := findCommand(cmds, "priority view")
	if cmd == nil {
		t.Fatal("expected 'priority view' command")
	}

	if app.crossOutliner.crossPriorityMode {
		t.Fatal("crossPriorityMode should default to false")
	}

	msg := cmd.Handler()
	if msg != nil {
		t.Errorf("expected nil message when toggling in-place, got %T", msg)
	}
	if !app.crossOutliner.crossPriorityMode {
		t.Error("priority view command should toggle crossPriorityMode on in cross-outliner scope")
	}

	// Second invocation toggles off.
	msg2 := cmd.Handler()
	if msg2 != nil {
		t.Errorf("second invocation should still return nil, got %T", msg2)
	}
	if app.crossOutliner.crossPriorityMode {
		t.Error("priority view command should toggle crossPriorityMode off on the second invocation")
	}
}

func TestViewCommands_InboxProcessor(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.viewCommands()
	cmd := findCommand(cmds, "inbox processor")
	if cmd == nil {
		t.Fatal("expected 'inbox processor' command")
	}
	if cmd.Shortcut != "W" {
		t.Fatalf("inbox processor shortcut = %q, want W", cmd.Shortcut)
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowInboxProcessorMsg); !ok {
		t.Errorf("expected ShowInboxProcessorMsg, got %T", msg)
	}
}

func TestViewCommands_GlobalInboxProcessor(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCommand(cmds, "global inbox processor")
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
	cmd := findCommand(cmds, "global notes")
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
	cmd := findCommand(cmds, "global blockers")
	if cmd == nil {
		t.Fatal("expected 'global blockers' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowGlobalBlockersMsg); !ok {
		t.Errorf("expected ShowGlobalBlockersMsg, got %T", msg)
	}
	if !commandHasAlias(cmd, "all blockers") {
		t.Errorf("expected global blockers aliases to include all blockers, got %v", cmd.Aliases)
	}
}

func commandHasAlias(cmd *Command, want string) bool {
	for _, alias := range cmd.Aliases {
		if alias == want {
			return true
		}
	}
	return false
}

func TestViewCommands_MetricsDashboard(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCommand(cmds, "metrics dashboard")
	if cmd == nil {
		t.Fatal("expected 'metrics dashboard' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowMetricsDashboardMsg); !ok {
		t.Errorf("expected ShowMetricsDashboardMsg, got %T", msg)
	}
}

func TestViewCommands_UndoHistoryRemoved(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.viewCommands()
	if cmd := findCommand(cmds, "undo history"); cmd != nil {
		t.Fatalf("undo history command should not be registered: %+v", cmd)
	}
	removedShortcut := "ctrl+" + "z"
	for _, cmd := range cmds {
		if cmd.Shortcut == removedShortcut {
			t.Fatalf("view command %q should not advertise removed undo history shortcut", cmd.Name)
		}
	}
}

func TestViewCommands_FilterPopup(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, nil, app.runtime)
	app.pushView(ViewProjectTree)

	cmds := app.viewCommands()
	cmd := findCommand(cmds, "filter popup")
	if cmd == nil {
		t.Fatal("expected 'filter popup' command")
	}
	cmd.Handler()

	if app.projectTree.pendingPrefix != "F" {
		t.Errorf("expected pendingPrefix to be \"F\", got %q", app.projectTree.pendingPrefix)
	}
}
