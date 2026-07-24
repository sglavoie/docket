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

func TestOutlinerModeCommands_OmitTabOnlyAll(t *testing.T) {
	cmd := findCommand(outlinerModeCommands(), "All")
	if cmd != nil {
		t.Fatalf("Tab-only All mode unexpectedly registered as a command: %#v", cmd)
	}
}

func TestViewCommands_DashboardOutlinerEntries(t *testing.T) {
	app := newTestAppModel()
	registry := registerCommandGroupsForTest(t, app.viewCommands(), outlinerModeCommands())
	cmds := registry.GetForView(ViewDashboard)

	tests := []struct {
		name     string
		shortcut string
		wantMsg  any
	}{
		{"outliner", "", ShowGlobalOutlinerMsg{}},
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

func TestOutlinerModeCommands_DueDate(t *testing.T) {
	cmd := findCommand(outlinerModeCommands(), "Due Date")
	if cmd == nil {
		t.Fatal("expected canonical 'Due Date' command")
	}
	msg := cmd.Handler()
	at, ok := msg.(ShowGlobalOutlinerMsg)
	if !ok {
		t.Errorf("expected ShowGlobalOutlinerMsg, got %T", msg)
	}
	if !at.Options.SetMode || at.Options.Mode != ModeDueDateLens {
		t.Errorf("expected Due Date mode, got %+v", at)
	}
}

func TestOutlinerModeCommands_DailyReview(t *testing.T) {
	cmd := findCommand(outlinerModeCommands(), "Daily Review")
	if cmd == nil {
		t.Fatal("expected canonical 'Daily Review' command")
	}
	msg, ok := cmd.Handler().(ShowGlobalOutlinerMsg)
	if !ok || !msg.Options.SetMode || msg.Options.Mode != ModeDailyReview {
		t.Errorf("expected Daily Review mode, got %#v", msg)
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

func TestOutlinerModeCommands_Activity(t *testing.T) {
	cmd := findCommand(outlinerModeCommands(), "Activity")
	if cmd == nil {
		t.Fatal("expected canonical 'Activity' command")
	}
	msg, ok := cmd.Handler().(ShowGlobalOutlinerMsg)
	if !ok || !msg.Options.SetMode || msg.Options.Mode != ModeActivity {
		t.Errorf("expected Activity mode, got %#v", msg)
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
	if peekCmd.Shortcut != "shift+enter" {
		t.Fatalf("peek shortcut = %q, want shift+enter", peekCmd.Shortcut)
	}
	if peekCmd.Description != "Peek at the selected task's details" {
		t.Fatalf("peek description = %q", peekCmd.Description)
	}

	notesCmd := findCommand(registry.GetForView(ViewProjectRecentActivity), "show notes")
	if notesCmd == nil {
		t.Fatal("project recent activity should expose show notes command")
	}
	if notesCmd.Shortcut != "" {
		t.Fatalf("show notes shortcut = %q, want palette-only", notesCmd.Shortcut)
	}

	assertPickerCommandPurpose(t, findCommand(registry.GetForView(ViewDashboard), "global priority update"), PickerPurposePriorityUpdate)
	assertPickerCommandPurpose(t, findCommand(registry.GetForView(ViewDashboard), "global due date update"), PickerPurposeDueDate)
	assertPickerCommandPurpose(t, findCommand(registry.GetForView(ViewDashboard), "global section change"), PickerPurposeSectionChange)
}

// TestHighPriorityUsesCatalogMode verifies the canonical command owns the old
// `priority view` phrase while bare `priority` remains exclusive to the
// Priority lens.
func TestHighPriorityUsesCatalogMode(t *testing.T) {
	cmd := findCommand(outlinerModeCommands(), "High Priority")
	if cmd == nil {
		t.Fatal("expected canonical 'High Priority' command")
	}
	msg, ok := cmd.Handler().(ShowGlobalOutlinerMsg)
	if !ok {
		t.Fatalf("expected ShowGlobalOutlinerMsg, got %T", cmd.Handler())
	}
	if !msg.Options.SetMode || msg.Options.Mode != ModeHighPriority {
		t.Errorf("expected global SetMode High Priority catalog mode, got %#v", msg.Options)
	}
	if !commandHasAlias(cmd, "priority view") {
		t.Errorf("High Priority aliases = %v, want legacy phrase 'priority view'", cmd.Aliases)
	}
	for _, alias := range cmd.Aliases {
		if alias == "priority" {
			t.Errorf("High Priority must not carry Priority lens alias %q", alias)
		}
	}
}

func TestOutlinerModeCommands_HighPriorityCrossOutlinerScope(t *testing.T) {
	registry := registerCommandGroupsForTest(t, outlinerModeCommands())
	cmd := findCommand(registry.GetForView(ViewCrossProjectOutliner), "High Priority")
	if cmd == nil {
		t.Fatal("High Priority should be reachable from ViewCrossProjectOutliner")
	}
	if cmd.Shortcut != "" {
		t.Errorf("High Priority shortcut = %q, want palette-only", cmd.Shortcut)
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
