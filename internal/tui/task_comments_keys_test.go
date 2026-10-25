package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
)

// mountCommentsPanel opens the panel over a live shell on a task carrying two
// comments, and returns the driver, the panel, and a key press helper.
func mountCommentsPanel(t *testing.T) (*UITestDriver, *taskdetail.CommentsModel, func(string)) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "first comment", CreatedAt: time.Now()},
		{Text: "second comment", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	driver.dispatch(taskdetail.ShowTaskCommentsMsg{Task: task, Project: project})
	panel := driver.App().entities.ActiveCommentsPanel(driver.App())
	if panel == nil {
		t.Fatal("ShowTaskCommentsMsg did not mount the Comments panel")
	}
	if panel.Dispatch == nil {
		t.Fatal("the mounted panel holds no dispatch seam, so every routed key resolves nothing")
	}

	press := func(key string) {
		t.Helper()
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("SendKey(%q): %v", key, err)
		}
	}
	return driver, panel, press
}

// TestCommentsPanelKeysReachTheirOperations presses routed keys through a live
// shell. The Comments scenarios only add and save a comment, so a dispatch seam
// the shell forgot to hand down would leave the acting keys silently doing
// nothing with every other check still green.
func TestCommentsPanelKeysReachTheirOperations(t *testing.T) {
	driver, panel, press := mountCommentsPanel(t)

	press("enter")
	if !panel.ExpandedView {
		t.Fatal("Enter did not reach tui.comment.expand")
	}
	press("esc")

	press("space")
	if driver.App().operations.ActionSheet == nil {
		t.Fatal("Space did not reach tui.comment.actions")
	}
	press("esc")
	if driver.App().operations.ActionSheet != nil {
		t.Fatal("Esc left the comment action sheet open")
	}

	press("e")
	if !panel.EditMode {
		t.Fatal("e did not reach tui.comment.edit")
	}
	press("esc")

	var captured string
	withFakeClipboard(t, nil, &captured)
	press("y")
	if captured != "first comment" {
		t.Fatalf("y copied %q, want the comment under the cursor", captured)
	}
	press("Y")
	if captured != "1. first comment\n2. second comment" {
		t.Fatalf("Y copied %q, want every comment as a numbered list", captured)
	}

	press("a")
	if !panel.AddMode {
		t.Fatal("a did not reach tui.comment.add")
	}
}

// TestCommentsRetiredKeysAreSilent holds the keys the panel gave up. Each one
// means something else app-wide, so the panel must answer none of them rather
// than quietly keeping its old meaning.
func TestCommentsRetiredKeysAreSilent(t *testing.T) {
	driver, panel, press := mountCommentsPanel(t)

	for _, key := range []string{"n", "d", "x"} {
		press(key)
		if panel.Confirm.IsActive() {
			t.Fatalf("retired key %q raised confirm %q", key, panel.Confirm.Action())
		}
		if panel.AddMode {
			t.Fatalf("retired key %q entered add mode", key)
		}
		if driver.App().operations.ActionSheet != nil {
			t.Fatalf("retired key %q opened the action sheet", key)
		}
	}
}

// TestCommentsUndoKeysAskTheShellForTheWorkspaceLedger pins the two keys the
// panel took back from silence. Adding, editing, and removing a comment are
// ledger transactions, so the panel that made them answers the app-wide undo;
// the core undo descriptors answer only to the Outliner, so the press asks the
// shell rather than dispatching through the executor.
func TestCommentsUndoKeysAskTheShellForTheWorkspaceLedger(t *testing.T) {
	driver, _, press := mountCommentsPanel(t)

	press("u")
	if driver.pendingCmd == nil {
		t.Fatal("u emitted no workspace undo request")
	}
	if msg, ok := runImmediateTestCommand(driver.pendingCmd).(status.WorkspaceUndoMsg); !ok || msg.Redo {
		t.Fatalf("u emitted %#v, want an undo request", runImmediateTestCommand(driver.pendingCmd))
	}

	press("ctrl+r")
	if driver.pendingCmd == nil {
		t.Fatal("Ctrl+R emitted no workspace redo request")
	}
	if msg, ok := runImmediateTestCommand(driver.pendingCmd).(status.WorkspaceUndoMsg); !ok || !msg.Redo {
		t.Fatalf("Ctrl+R emitted %#v, want a redo request", runImmediateTestCommand(driver.pendingCmd))
	}
}

// TestCommentsSubStatesKeepUAsTypedText holds u inside every sub-state that owns
// the key stream. The panel answers it only in browse, so a comment carrying the
// letter can never reverse the transaction the user is in the middle of writing,
// and neither the expanded reader nor the delete confirmation turns a stray
// press into a ledger restore.
func TestCommentsSubStatesKeepUAsTypedText(t *testing.T) {
	driver, panel, press := mountCommentsPanel(t)
	refuseLedger := func(state string) {
		t.Helper()
		if driver.pendingCmd == nil {
			return
		}
		if _, ok := runImmediateTestCommand(driver.pendingCmd).(status.WorkspaceUndoMsg); ok {
			t.Fatalf("u reached the workspace ledger while %s owned the key stream", state)
		}
	}

	press("a")
	if !panel.AddMode {
		t.Fatal("a did not enter add mode")
	}
	press("u")
	refuseLedger("the add composer")
	if got := panel.TextInput.Value(); got != "u" {
		t.Fatalf("add-mode input = %q, want %q", got, "u")
	}
	press("esc")
	press("y")

	press("e")
	if !panel.EditMode {
		t.Fatal("e did not enter edit mode")
	}
	press("u")
	refuseLedger("the edit composer")
	if got := panel.EditTextInput.Value(); !strings.HasSuffix(got, "u") {
		t.Fatalf("edit-mode input = %q, want the typed u appended", got)
	}
	press("esc")
	press("y")

	press("enter")
	if !panel.ExpandedView {
		t.Fatal("Enter did not open the expanded comment")
	}
	press("u")
	refuseLedger("the expanded comment")
	press("esc")

	press("space")
	press("enter")
	if !panel.Confirm.IsActive() {
		t.Fatal("the sheet's Delete row raised no confirmation")
	}
	press("u")
	refuseLedger("the delete confirmation")
}
