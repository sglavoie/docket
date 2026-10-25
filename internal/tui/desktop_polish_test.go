package tui

import (
	"strings"
	"testing"
	"unicode/utf16"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/tui/grammar"
)

func polishDriver(t *testing.T) *UITestDriver {
	t.Helper()
	d, ws, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })
	return d
}

func desktopRequest(t *testing.T, d *UITestDriver, op, text string) control.Reply {
	t.Helper()
	replies := make(chan control.Reply, 1)
	d.dispatch(controlRequestMsg{request: control.Request{Op: op, Text: text}, reply: replies})
	d.drain()
	select {
	case reply := <-replies:
		return reply
	default:
		t.Fatal("missing desktop reply")
		return control.Reply{}
	}
}

func TestDesktopDraftUndoRedoAndQuit(t *testing.T) {
	d := polishDriver(t)
	m := d.App()
	d.dispatch(m.captureComposerMessage(m.currentProject, m.taskWorkspaceProjects()))
	d.drain()
	if got := m.quitBlockReason(); got != "" {
		t.Fatalf("empty capture is dirty: %s", got)
	}
	// Paste is one history entry, and undo restores the empty draft without
	// consulting or changing durable workspace history.
	d.dispatch(tea.PasteMsg{Content: "Buy 🥛 milk"})
	d.drain()
	if got := m.desktopEditing().Undo; got != "Typing" {
		t.Fatalf("undo label: %q", got)
	}
	if reply := desktopRequest(t, d, control.OpQuit, ""); reply.OK || !strings.Contains(reply.Message, "capture draft") {
		t.Fatalf("dirty quit: %+v", reply)
	}
	if reply := desktopRequest(t, d, control.OpEdit, "undo"); !reply.OK {
		t.Fatal(reply.Message)
	}
	if got := m.taskComposer.Input.Value(); got != "" {
		t.Fatalf("undo: %q", got)
	}
	if reason := m.quitBlockReason(); reason != "" {
		t.Fatalf("undo did not clear dirtiness: %s", reason)
	}
	if reply := desktopRequest(t, d, control.OpEdit, "undo"); reply.OK {
		t.Fatal("empty draft fell through to workspace undo")
	}
	if reply := desktopRequest(t, d, control.OpEdit, "redo"); !reply.OK {
		t.Fatal(reply.Message)
	}
	if got := m.taskComposer.Input.Value(); got != "Buy 🥛 milk" {
		t.Fatalf("redo: %q", got)
	}
	// Help covers the draft without making it disposable or deleting history.
	if err := d.runAction(UITestAction{Type: UITestActionKey, Key: "?"}); err != nil {
		t.Fatal(err)
	}
	if reason := m.quitBlockReason(); reason == "" {
		t.Fatal("covered draft lost its quit protection")
	}
}

func TestDesktopSearchAndExplicitPeekSize(t *testing.T) {
	d := polishDriver(t)
	if reply := desktopRequest(t, d, control.OpEdit, "search"); !reply.OK {
		t.Fatal(reply.Message)
	}
	m := d.App()
	if !m.crossOutliner.IsSearchMode() {
		t.Fatal("native Find did not open search")
	}
	if err := d.runAction(UITestAction{Type: UITestActionText, Text: "m"}); err != nil {
		t.Fatal(err)
	}
	if reason := m.quitBlockReason(); reason != "" {
		t.Fatalf("search is not unsaved content: %s", reason)
	}
	if reply := desktopRequest(t, d, control.OpEdit, "undo"); !reply.OK {
		t.Fatal(reply.Message)
	}
	if got := m.crossOutliner.SearchInputValue(); got != "" {
		t.Fatalf("search undo: %q", got)
	}
	for _, key := range []string{"esc", "b", "B"} {
		if err := d.runAction(UITestAction{Type: UITestActionKey, Key: key}); err != nil {
			t.Fatal(err)
		}
	}
	if !m.crossOutliner.PeekMode || !m.crossOutliner.PeekExpanded {
		t.Fatal("b then B did not expand Peek")
	}
	for _, key := range []string{"j", "k", "b", "b"} {
		_ = d.runAction(UITestAction{Type: UITestActionKey, Key: key})
	}
	if !m.crossOutliner.PeekExpanded {
		t.Fatal("cursor/toggle forgot expanded Peek")
	}
	_ = d.runAction(UITestAction{Type: UITestActionKey, Key: "B"})
	if m.crossOutliner.PeekExpanded {
		t.Fatal("B did not compact Peek")
	}
}

func TestCaptureErrorSelectionUsesUTF16SourceOffsets(t *testing.T) {
	for _, tc := range []struct {
		text, prefix, token string
		line, column        int
	}{
		{"  Buy 🥛 milk %missing", "  Buy 🥛 milk ", "%missing", 1, 13},
		{"First\n  Next 🐈 @invalid", "First\n  Next 🐈 ", "@invalid", 2, 9},
	} {
		selection := captureErrorSelection(tc.text, grammar.LineError{Line: tc.line, Column: tc.column})
		if selection == nil {
			t.Fatalf("no selection: %q", tc.text)
		}
		if selection.Start != len(utf16.Encode([]rune(tc.prefix))) || selection.Length != len(utf16.Encode([]rune(tc.token))) {
			t.Fatalf("selection = %+v for %q", selection, tc.text)
		}
	}
	if captureErrorSelection("no title", grammar.LineError{Line: 1, Column: 0}) != nil {
		t.Fatal("non-token error selected text")
	}
}

func TestCaptureRefusalSelectsUnknownProjectWithoutSaving(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))
	reply := env.request(t, control.Request{Op: control.OpCapture, Text: "  Buy 🥛 milk %missing"})
	if reply.OK || reply.Selection == nil || *reply.Selection != (control.TextSelection{Start: 14, Length: 8}) {
		t.Fatalf("refusal: %+v", reply)
	}
	if env.hasTitle(t, "Buy 🥛 milk") {
		t.Fatal("invalid capture committed")
	}
}

func TestDesktopDraftHistorySurvivesHelpAndClearsRedoOnTyping(t *testing.T) {
	d := polishDriver(t)
	m := d.App()
	d.dispatch(m.captureComposerMessage(m.currentProject, m.taskWorkspaceProjects()))
	d.drain()
	d.dispatch(tea.PasteMsg{Content: "draft"})
	d.drain()
	for _, key := range []string{"?", "esc"} {
		if err := d.runAction(UITestAction{Type: UITestActionKey, Key: key}); err != nil {
			t.Fatal(err)
		}
	}
	if reply := desktopRequest(t, d, control.OpEdit, "undo"); !reply.OK {
		t.Fatal(reply.Message)
	}
	d.dispatch(tea.PasteMsg{Content: "replacement"})
	d.drain()
	if reply := desktopRequest(t, d, control.OpEdit, "redo"); reply.OK {
		t.Fatal("typing retained stale redo")
	}
	if m.taskComposer.Input.Value() != "replacement" {
		t.Fatal("stale redo changed the draft")
	}
}

func TestDesktopBrowsingUndoRedoUsesWorkspaceHistory(t *testing.T) {
	d := polishDriver(t)
	m := d.App()
	d.dispatch(m.captureComposerMessage(m.currentProject, m.taskWorkspaceProjects()))
	d.drain()
	d.dispatch(tea.PasteMsg{Content: "desktop saved task"})
	d.drain()
	if err := d.runAction(UITestAction{Type: UITestActionKey, Key: "enter"}); err != nil {
		t.Fatal(err)
	}
	hasTask := func() bool {
		for _, project := range m.taskWorkspaceProjects() {
			for _, task := range project.Tasks {
				if task.Title == "desktop saved task" {
					return true
				}
			}
		}
		return false
	}
	if !hasTask() {
		t.Fatal("capture was not saved")
	}
	if state := m.desktopEditing(); state.Undo == "" || state.Undo == "Typing" {
		t.Fatalf("workspace undo label: %+v", state)
	}
	if reply := desktopRequest(t, d, control.OpEdit, "undo"); !reply.OK {
		t.Fatal(reply.Message)
	}
	if hasTask() {
		t.Fatal("native workspace undo did not remove the captured task")
	}
	if reply := desktopRequest(t, d, control.OpEdit, "redo"); !reply.OK {
		t.Fatal(reply.Message)
	}
	if !hasTask() {
		t.Fatal("native workspace redo did not restore the captured task")
	}
}
