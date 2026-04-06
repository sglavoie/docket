package tui

import "testing"

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

func TestViewCommands_WhatsNext(t *testing.T) {
	app := newTestAppModel()
	cmds := app.viewCommands()
	cmd := findCommand(cmds, "what's next")
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
	cmd := findCommand(cmds, "focus board")
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

func TestViewCommands_InboxProcessor(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.viewCommands()
	cmd := findCommand(cmds, "inbox processor")
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

func TestViewCommands_UndoHistory(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.viewCommands()
	cmd := findCommand(cmds, "undo history")
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
	cmd := findCommand(cmds, "undo history")
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
	cmd := findCommand(cmds, "filter popup")
	if cmd == nil {
		t.Fatal("expected 'filter popup' command")
	}
	cmd.Handler()

	if app.projectTree.pendingPrefix != "F" {
		t.Errorf("expected pendingPrefix to be \"F\", got %q", app.projectTree.pendingPrefix)
	}
}