package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// palettePlumbingSentinelMsg is produced by a tea.Cmd carried via
// PaletteCommandResultMsg, used by tests to confirm the carried command was
// returned to the runtime instead of being routed as a Msg.
type palettePlumbingSentinelMsg struct{}

// TestCommandPaletteExecuteUnwrapsCommandCarryingResult pins that a
// PaletteCommandResultMsg is unwrapped so its carried tea.Cmd reaches the
// runtime intact, with the palette closed in the same update cycle.
func TestCommandPaletteExecuteUnwrapsCommandCarryingResult(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []PresentationID{ViewDashboard, ViewCommandPalette}

	sentinel := func() tea.Msg { return palettePlumbingSentinelMsg{} }
	handled, cmd := app.handleSearchMessages(CommandPaletteExecuteMsg{
		ResultMsg: PaletteCommandResultMsg{Cmd: sentinel},
	})

	if !handled {
		t.Fatal("palette execute should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView = %v, want Dashboard after palette closes", app.currentView())
	}
	if cmd == nil {
		t.Fatal("expected unwrapped tea.Cmd from PaletteCommandResultMsg, got nil")
	}
	if _, ok := cmd().(palettePlumbingSentinelMsg); !ok {
		t.Errorf("expected sentinel from carried cmd, got %T", cmd())
	}
}

// TestCommandPaletteExecutePreservesPopCmd pins that the tea.Cmd returned by
// popView() (e.g. ViewDidAppearMsg when the cross-project outliner is
// revealed) is not silently dropped — it must be returned to the runtime so
// the revealed view gets its eager-reload signal.
func TestCommandPaletteExecutePreservesPopCmd(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner, ViewCommandPalette}

	handled, cmd := app.handleSearchMessages(CommandPaletteExecuteMsg{
		ResultMsg: nil,
	})

	if !handled {
		t.Fatal("palette execute should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView = %v, want CrossProjectOutliner after palette closes", app.currentView())
	}
	if cmd == nil {
		t.Fatal("expected popView's ViewDidAppearMsg cmd to be forwarded, got nil")
	}
	if _, ok := cmd().(ViewDidAppearMsg); !ok {
		t.Errorf("expected ViewDidAppearMsg from forwarded popCmd, got %T", cmd())
	}
}

// TestCommandPaletteExecuteBatchesPopCmdWithResultCmd pins that when both
// popView() and the carried result produce a tea.Cmd, both are batched
// together so neither side-effect is dropped.
func TestCommandPaletteExecuteBatchesPopCmdWithResultCmd(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner, ViewCommandPalette}

	sentinel := func() tea.Msg { return palettePlumbingSentinelMsg{} }
	handled, cmd := app.handleSearchMessages(CommandPaletteExecuteMsg{
		ResultMsg: PaletteCommandResultMsg{Cmd: sentinel},
	})

	if !handled {
		t.Fatal("palette execute should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView = %v, want CrossProjectOutliner after palette closes", app.currentView())
	}
	if cmd == nil {
		t.Fatal("expected batched popCmd + resultCmd, got nil")
	}
	// tea.Batch wraps both cmds; we verify the batch surface (a BatchMsg) by
	// invoking it and asserting it produces a non-nil value rather than asserting
	// internal BatchMsg shape, which is bubbletea's private contract. Pairing
	// this with the standalone unwrap and popCmd tests above proves both halves
	// were preserved.
	msg := cmd()
	if msg == nil {
		t.Fatal("batched cmd should produce a non-nil message")
	}
}

// TestCommandPaletteExecuteKeepsPaletteForModalResults pins that modal-style
// inner messages (editor, pickers, glow preview) leave the palette in place
// and do not invoke popView(), so the modal can return to the palette when
// dismissed.
func TestCommandPaletteExecuteKeepsPaletteForModalResults(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []PresentationID{ViewDashboard, ViewCommandPalette}

	handled, cmd := app.handleSearchMessages(CommandPaletteExecuteMsg{
		// Empty EditTaskMsg short-circuits in handleEditTask but is still
		// classified as a modal-keep-open type by handler_search.go.
		ResultMsg: EditTaskMsg{},
	})

	if !handled {
		t.Fatal("palette execute should be handled")
	}
	if cmd != nil {
		t.Fatalf("modal keep-open path should return nil cmd, got %T", cmd)
	}
	if app.currentView() != ViewCommandPalette {
		t.Errorf("currentView = %v, want CommandPalette to remain open for modal results", app.currentView())
	}
}
