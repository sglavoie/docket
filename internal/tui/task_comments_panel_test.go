package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/entityops"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
)

// TestCommentsPanelWorkspaceUndoRedoRefreshesAndNamesTheLedgerEntry walks the
// promise the panel used to break. A comment mutation commits through
// commitProject with an UndoAction and installs a live `u undo` toast, so the
// panel that made it has to answer both ledger keys itself: the open list
// rebuilds rather than keeping a reversed row on screen, and the status strip
// names the transaction that moved rather than restoring in silence.
func TestCommentsPanelWorkspaceUndoRedoRefreshesAndNamesTheLedgerEntry(t *testing.T) {
	driver, press, typeText := mountLiveCommentsPanel(t)
	comments := func() []string {
		t.Helper()
		panel := driver.App().entities.ActiveCommentsPanel(driver.App())
		if panel == nil {
			t.Fatal("the Comments panel left the route")
		}
		entries := panel.Entries()
		texts := make([]string, 0, len(entries))
		for _, entry := range entries {
			texts = append(texts, entry.Text)
		}
		return texts
	}

	const added = "checked the redirect handler"
	before := len(comments())
	press("a")
	typeText(added)
	press("ctrl+s")
	driver.Wait(defaultWaitTimeout)
	if got := comments(); len(got) != before+1 || got[len(got)-1] != added {
		t.Fatalf("after the add the panel lists %v, want %q appended", got, added)
	}
	if !strings.Contains(driver.App().feedback.message, "u undo") {
		t.Fatalf("the add left status %q, want the recovery toast that offers u", driver.App().feedback.message)
	}

	press("u")
	driver.Wait(defaultWaitTimeout)
	if driver.App().currentView() != presentation.ViewTaskComments {
		t.Fatalf("u left the panel for %s", driver.App().currentView())
	}
	if got := comments(); len(got) != before || slices.Contains(got, added) {
		t.Fatalf("after u the panel lists %v, want the added comment gone", got)
	}
	if got := driver.App().feedback.message; !strings.Contains(got, "Undid: Add Comment") {
		t.Fatalf("u left status %q, want it to name the restored transaction", got)
	}

	press("ctrl+r")
	driver.Wait(defaultWaitTimeout)
	if driver.App().currentView() != presentation.ViewTaskComments {
		t.Fatalf("Ctrl+R left the panel for %s", driver.App().currentView())
	}
	if got := comments(); len(got) != before+1 || !slices.Contains(got, added) {
		t.Fatalf("after Ctrl+R the panel lists %v, want the comment back", got)
	}
	if got := driver.App().feedback.message; !strings.Contains(got, "Redo: Add Comment") {
		t.Fatalf("Ctrl+R left status %q, want it to name the replayed transaction", got)
	}

	if !strings.Contains(driver.Render(), added) {
		t.Fatal("the replayed comment is absent from the rendered list")
	}
}

// mountLiveCommentsPanel opens the panel the way a user does, over a workspace
// whose commits reach the durable ledger. mountCommentsPanel's task is built in
// memory beside the fixture, so a mutation there records nothing to undo.
func mountLiveCommentsPanel(t *testing.T) (*UITestDriver, func(string), func(string)) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	press := func(key string) {
		t.Helper()
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("SendKey(%q): %v", key, err)
		}
		driver.drain()
	}
	typeText := func(text string) {
		t.Helper()
		for _, r := range text {
			driver.dispatch(tea.KeyPressMsg{Code: r, Text: string(r)})
			driver.drain()
		}
	}

	// No navigation is needed to reach work: the launch seats the cursor on the
	// first task row the projection draws, past the lane header an expanded
	// project group spends no second row on.
	press(";")
	if driver.App().entities.ActiveCommentsPanel(driver.App()) == nil {
		t.Fatalf("the comment sigil key did not open the Comments panel; view = %s", driver.App().currentView())
	}
	return driver, press, typeText
}

// TestCommentsSpaceSheetReachesDelete walks the whole new path for Delete, which
// is the retirement this session is really about: neither d nor x removes a
// comment now, so the Space sheet has to, and the confirmation behind it has to
// still commit the removal.
func TestCommentsSpaceSheetReachesDelete(t *testing.T) {
	// The removal is asserted over a live workspace rather than mountCommentsPanel's
	// in-memory task, because the panel no longer edits the stored comments itself:
	// the row leaves the list when the transaction confirms, and only a task the
	// application service knows can confirm one.
	driver, press, typeText := mountLiveCommentsPanel(t)
	panel := func() *taskdetail.CommentsModel {
		t.Helper()
		mounted := driver.App().entities.ActiveCommentsPanel(driver.App())
		if mounted == nil {
			t.Fatal("the Comments panel left the route")
		}
		return mounted
	}
	const doomed = "comment to remove"
	press("a")
	typeText(doomed)
	press("ctrl+s")
	driver.Wait(defaultWaitTimeout)
	before := panel().Entries()
	if len(before) == 0 || before[len(before)-1].Text != doomed {
		t.Fatalf("the add left %v, want %q appended for the delete to remove", before, doomed)
	}
	press("G")

	press("space")
	sheet := driver.App().operations.ActionSheet
	if sheet == nil {
		t.Fatal("Space did not open the comment action sheet")
	}
	want := []operation.OperationID{entityops.OperationCommentDelete}
	if got := sheet.OperationIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("comment sheet lists %v, want exactly %v", got, want)
	}

	press("enter")
	if driver.App().operations.ActionSheet != nil {
		t.Fatal("choosing Delete left the sheet open over the confirmation")
	}
	if !panel().Confirm.IsActive() || panel().Confirm.Action() != "delete" {
		t.Fatalf("the sheet's Delete row raised confirm %q, want the delete prompt", panel().Confirm.Action())
	}

	press("y")
	driver.Wait(defaultWaitTimeout)
	after := panel().Entries()
	if len(after) != len(before)-1 {
		t.Fatalf("confirming the sheet's Delete left %d comment(s), want %d", len(after), len(before)-1)
	}
	for _, entry := range after {
		if entry.Text == doomed {
			t.Fatalf("the comment under the cursor (%q) survived the confirmed delete", doomed)
		}
	}
}

// TestCommentsHelpKeyRaisesTheCommentsGrammar holds `?` to the surface it was
// pressed on. The core help descriptor hardcodes the Outliner, so dispatching it
// through the executor would answer a Comments press with the Outliner's
// grammar; the panel raises its own instead, and the press stays a literal
// character in every sub-state that owns the key stream.
func TestCommentsHelpKeyRaisesTheCommentsGrammar(t *testing.T) {
	driver, panel, sendKey := mountCommentsPanel(t)
	// Help arrives as a command rather than a synchronous mutation, so every press
	// drains the way the scenario runner and the live program do.
	press := func(key string) {
		t.Helper()
		sendKey(key)
		driver.drain()
	}

	press("?")
	if got := driver.App().currentView(); got != presentation.ViewHelp {
		t.Fatalf("? left the panel on %v, want the Help view", got)
	}
	if _, ok := driver.App().viewNav.views.Get(presentation.ViewHelp).(*help.HelpViewModel); !ok {
		t.Fatalf("the Help view is %T, want *help.HelpViewModel", driver.App().viewNav.views.Get(presentation.ViewHelp))
	}
	// The grammar is read off the message the panel raises rather than out of the
	// viewer it produced: ShowHelpMsg is what handler_view_core.go hands to
	// NewHelpViewModel, so the two carry the same rows, and the message says them
	// in the panel's own package.
	raised, ok := panel.HandleNormalKey("?")().(help.ShowHelpMsg)
	if !ok {
		t.Fatalf("? raised %T, want help.ShowHelpMsg", panel.HandleNormalKey("?")())
	}
	if raised.ViewName != "Comments" {
		t.Fatalf("help header = %q, want \"Comments\"", raised.ViewName)
	}

	documented := make(map[string]string, len(raised.Items))
	for _, item := range raised.Items {
		documented[item.Key] = item.Description
	}
	if documented["enter"] != "Expand Comment" {
		t.Fatalf("Comments help describes Enter as %q, want the panel's own grammar: %+v", documented["enter"], raised.Items)
	}
	if documented["space"] != "Comment Actions" {
		t.Fatalf("Comments help omits the Space sheet: %+v", raised.Items)
	}
	// `?` documents itself only because the keymap names it: the core descriptor
	// is inapplicable off the Outliner, so a registry projection would drop it.
	if documented["?"] != "Help" {
		t.Fatalf("Comments help omits ? itself: %+v", raised.Items)
	}
	for _, outlinerOnly := range []string{"tab", "T", "ctrl+p"} {
		if description, listed := documented[outlinerOnly]; listed {
			t.Fatalf("Comments help leaked the Outliner key %q (%q)", outlinerOnly, description)
		}
	}
	press("?")
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("closing help landed on %v, want the panel it came from", got)
	}

	// Every sub-state owns its whole key stream, so help is a browse-only press.
	press("a")
	press("?")
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("? in add mode left the panel for %v", got)
	}
	if got := panel.TextInput.Value(); got != "?" {
		t.Fatalf("add-mode buffer after ? = %q, want a literal %q", got, "?")
	}
	press("esc")
	press("y")
	if panel.AddMode {
		t.Fatal("discarding the add buffer left add mode active")
	}

	press("e")
	press("?")
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("? in edit mode left the panel for %v", got)
	}
	if got := panel.EditTextInput.Value(); !strings.Contains(got, "?") {
		t.Fatalf("edit-mode buffer after ? = %q, want a literal %q in it", got, "?")
	}
	press("esc")
	press("y")

	press("enter")
	if !panel.ExpandedView {
		t.Fatal("Enter did not expand the comment under the cursor")
	}
	press("?")
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("? in the expanded comment left the panel for %v", got)
	}
	press("esc")

	panel.Confirm.Show("delete", "Delete comment [0]?")
	press("?")
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("? under the delete confirmation left the panel for %v", got)
	}
	if !panel.Confirm.IsActive() {
		t.Fatal("? answered the delete confirmation instead of being ignored")
	}
	press("n")
}

// TestCommentsPanelUnboundKeysAreSilent holds the keys the Comments grammar
// never declared. They must stay quiet rather than falling through to some other
// surface's meaning.
func TestCommentsPanelUnboundKeysAreSilent(t *testing.T) {
	driver, panel, press := mountCommentsPanel(t)

	for _, key := range []string{"u", "o", "p", "%"} {
		press(key)
		if panel.Confirm.IsActive() {
			t.Fatalf("unbound key %q raised confirm %q", key, panel.Confirm.Action())
		}
		if panel.AddMode || panel.EditMode || panel.ExpandedView {
			t.Fatalf("unbound key %q changed the panel mode", key)
		}
		if driver.App().operations.ActionSheet != nil {
			t.Fatalf("unbound key %q opened an action sheet", key)
		}
		if got := driver.App().currentView(); got != presentation.ViewTaskComments {
			t.Fatalf("unbound key %q left the panel for %v", key, got)
		}
	}
}
