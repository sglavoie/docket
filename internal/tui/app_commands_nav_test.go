package tui

import "testing"

func TestNavCommands_OutlinerRoot(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCommand(cmds, "outliner root")
	if cmd == nil {
		t.Fatal("expected 'outliner root' command")
	}
	if cmd.Shortcut != "ctrl+esc" {
		t.Fatalf("outliner root shortcut = %q, want ctrl+esc", cmd.Shortcut)
	}
	msg := cmd.Handler()
	if _, ok := msg.(JumpDashboardMsg); !ok {
		t.Errorf("expected JumpDashboardMsg, got %T", msg)
	}
}

func TestNavCommands_RecentTasksEmptyShowsStatus(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCommand(cmds, "recent tasks")
	if cmd == nil {
		t.Fatal("expected 'recent tasks' command")
	}

	msg := cmd.Handler()
	result, ok := msg.(PaletteCommandResultMsg)
	if !ok {
		t.Fatalf("recent tasks should return PaletteCommandResultMsg, got %T", msg)
	}
	if result.Cmd == nil {
		t.Fatal("recent tasks should preserve the status timeout command")
	}
	if app.showQuickSwitch {
		t.Fatal("quick switch should close again when there are no recent tasks")
	}
	if app.statusMessage != "No recent tasks for quick-switch" {
		t.Fatalf("statusMessage = %q, want %q", app.statusMessage, "No recent tasks for quick-switch")
	}
}
