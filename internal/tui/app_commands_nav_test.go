package tui

import "testing"

func TestNavCommands_Dashboard(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCommand(cmds, "dashboard")
	if cmd == nil {
		t.Fatal("expected 'dashboard' command")
	}
	if cmd.Shortcut != "ctrl+esc" {
		t.Fatalf("dashboard shortcut = %q, want ctrl+esc", cmd.Shortcut)
	}
	msg := cmd.Handler()
	if _, ok := msg.(JumpDashboardMsg); !ok {
		t.Errorf("expected JumpDashboardMsg, got %T", msg)
	}
}

func TestNavCommands_ProjectCycle(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCommand(cmds, "project cycle")
	if cmd == nil {
		t.Fatal("expected 'project cycle' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowProjectCycleManagerMsg); !ok {
		t.Errorf("expected ShowProjectCycleManagerMsg, got %T", msg)
	}
}

func TestNavCommands_ProjectPicker(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCommand(cmds, "project picker")
	if cmd == nil {
		t.Fatal("expected 'project picker' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowProjectCyclePickerMsg); !ok {
		t.Errorf("expected ShowProjectCyclePickerMsg, got %T", msg)
	}
}

func TestNavCommands_PreviousProject(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCommand(cmds, "previous project")
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
	cmd := findCommand(cmds, "history back")
	if cmd == nil {
		t.Fatal("expected 'history back' command")
	}
	cmd.Handler()
}

func TestNavCommands_HistoryForward(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCommand(cmds, "history forward")
	if cmd == nil {
		t.Fatal("expected 'history forward' command")
	}
	cmd.Handler()
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
