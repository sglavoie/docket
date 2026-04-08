package tui

import "testing"

func TestNavCommands_Dashboard(t *testing.T) {
	app := newTestAppModel()
	cmds := app.navCommands()
	cmd := findCommand(cmds, "dashboard")
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
	cmd := findCommand(cmds, "project cycle")
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
	cmd := findCommand(cmds, "next project")
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
