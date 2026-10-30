package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/tui/entityops"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
)

// fakeAttachmentMutator records the application use cases the attachments view
// dispatches through its coordinator (the view no longer mutates or saves the
// project itself).
type fakeAttachmentMutator struct {
	urls     []string
	codeRefs []string
	files    []string
	removals []fakeRemoval
	reorders []fakeReorder
}

type fakeRemoval struct {
	index      int
	expected   models.Attachment
	deleteFile bool
}

type fakeReorder struct {
	from int
	to   int
}

func (f *fakeAttachmentMutator) AttachURL(locator app.TaskLocator, url string) (app.TaskResult, error) {
	f.urls = append(f.urls, url)
	return app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}, nil
}

func (f *fakeAttachmentMutator) AttachCodeReference(locator app.TaskLocator, path string, line int) (app.TaskResult, error) {
	f.codeRefs = append(f.codeRefs, path)
	return app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}, nil
}

func (f *fakeAttachmentMutator) AttachFile(locator app.TaskLocator, sourcePath string) (app.AttachFileResult, error) {
	f.files = append(f.files, sourcePath)
	return app.AttachFileResult{Task: app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}}, nil
}

func (f *fakeAttachmentMutator) RemoveAttachmentExpecting(locator app.TaskLocator, index int, expected models.Attachment, deleteFile bool) (app.RemoveAttachmentResult, error) {
	f.removals = append(f.removals, fakeRemoval{index: index, expected: expected, deleteFile: deleteFile})
	return app.RemoveAttachmentResult{Task: app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}}, nil
}

func (f *fakeAttachmentMutator) ReorderAttachment(locator app.TaskLocator, from, to int, expected []models.Attachment) (app.TaskResult, error) {
	f.reorders = append(f.reorders, fakeReorder{from: from, to: to})
	return app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}, nil
}

func newFakeAttachmentCoordinator() (coordinator.AttachmentCoordinator, *fakeAttachmentMutator) {
	fake := &fakeAttachmentMutator{}
	return coordinator.NewAttachmentCoordinator(fake), fake
}

// mountAttachmentsPanel opens the panel over a live shell on a task carrying two
// file attachments, and returns the driver, the panel, and a key press helper.
func mountAttachmentsPanel(t *testing.T) (*UITestDriver, *taskdetail.TaskAttachmentsModel, func(string)) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Attachments = []models.Attachment{
		{Filename: "file1.txt", OriginalPath: "/tmp/file1.txt", AddedAt: time.Now()},
		{Filename: "file2.txt", OriginalPath: "/tmp/file2.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	driver.dispatch(taskdetail.ShowTaskAttachmentsMsg{Task: task, Project: project})
	panel := driver.App().entities.ActiveAttachmentsPanel(driver.App())
	if panel == nil {
		t.Fatal("ShowTaskAttachmentsMsg did not mount the Attachments panel")
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

// TestAttachmentsPanelKeysReachTheirOperations presses routed keys through a
// live shell. Neither Attachments scenario presses a key that acts on an item,
// so a dispatch seam the shell forgot to hand down would leave every operation
// silently doing nothing with every other check still green.
func TestAttachmentsPanelKeysReachTheirOperations(t *testing.T) {
	driver, panel, press := mountAttachmentsPanel(t)

	press("space")
	if driver.App().operations.ActionSheet == nil {
		t.Fatal("Space did not reach tui.attachment.actions")
	}
	press("esc")
	if driver.App().operations.ActionSheet != nil {
		t.Fatal("Esc left the attachment action sheet open")
	}

	coordinator, fake := newFakeAttachmentCoordinator()
	panel.Coordinator = coordinator
	press("J")
	if driver.pendingCmd == nil {
		t.Fatal("J did not reach tui.attachment.reorder-down")
	}
	runImmediateTestCommand(driver.pendingCmd)
	if len(fake.reorders) != 1 || fake.reorders[0].from != 0 || fake.reorders[0].to != 1 {
		t.Fatalf("J recorded %+v, want ReorderAttachment(0, 1) on press one", fake.reorders)
	}

	// E is read through the refusal it earns rather than through a command: the
	// fixture names a file that was never written, so the operation reaches the
	// panel, resolves the path, and fails there. Anything that stopped short of
	// the handler would leave the panel with no error at all.
	press("E")
	if panel.Err == nil {
		t.Fatal("E did not reach tui.attachment.open-in-editor")
	}
	if !strings.Contains(panel.Err.Error(), "file not found") {
		t.Fatalf("E raised %v, want the editor's missing-file refusal", panel.Err)
	}
	panel.ClearError()

	press("a")
	if !panel.AddMode {
		t.Fatal("a did not reach tui.attachment.add")
	}
}

// TestAttachmentsRetiredKeysAreSilent holds the keys the panel gave up. Each one
// now means something else app-wide, so the panel must answer none of them
// rather than quietly keeping its old meaning. `e` is the last to join them: the
// external editor answers `E` here the way it does everywhere else, so the key
// the rest of the app spends on edit and rename reaches nothing on a panel with
// nothing to rename. `u` left this list going the other way: retiring unlink
// freed the key, so the panel now answers it with the app-wide undo rather than
// with silence.
func TestAttachmentsRetiredKeysAreSilent(t *testing.T) {
	driver, panel, press := mountAttachmentsPanel(t)

	for _, key := range []string{"e", "d", "x", "o", "O", "ctrl+e", "ctrl+o"} {
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
		if driver.pendingCmd != nil {
			t.Fatalf("retired key %q returned a command, so it still does something", key)
		}
	}
}

// TestAttachmentsUndoKeysAskTheShellForTheWorkspaceLedger pins the two keys the
// panel took back from silence. Adding and removing an attachment are ledger
// transactions, so the panel that made them answers the app-wide undo; the core
// undo descriptors answer only to the Outliner, so the press asks the shell
// rather than dispatching through the executor.
func TestAttachmentsUndoKeysAskTheShellForTheWorkspaceLedger(t *testing.T) {
	driver, _, press := mountAttachmentsPanel(t)

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

// TestAttachmentsAddModeKeepsUAsTypedText holds u inside the add input. The
// panel answers it only in browse, so typing a URL that carries the letter can
// never reverse the transaction the user is in the middle of writing.
func TestAttachmentsAddModeKeepsUAsTypedText(t *testing.T) {
	driver, panel, press := mountAttachmentsPanel(t)

	press("a")
	if !panel.AddMode {
		t.Fatal("a did not enter add mode")
	}
	press("u")
	if driver.pendingCmd != nil {
		if _, ok := runImmediateTestCommand(driver.pendingCmd).(status.WorkspaceUndoMsg); ok {
			t.Fatal("u undid a transaction while the attachment path was being typed")
		}
	}
	if got := panel.TextInput.Value(); got != "u" {
		t.Fatalf("add-mode input = %q, want %q", got, "u")
	}
}

// TestAttachmentsSpaceSheetReachesUnlink walks the whole new path for Unlink,
// which is the retirement this session is really about: `u` no longer unlinks,
// so the Space sheet has to, and the confirmation behind it has to still commit
// the keep-the-file removal the coordinator records.
func TestAttachmentsSpaceSheetReachesUnlink(t *testing.T) {
	driver, panel, press := mountAttachmentsPanel(t)

	press("space")
	sheet := driver.App().operations.ActionSheet
	if sheet == nil {
		t.Fatal("Space did not open the attachment action sheet")
	}
	want := []operation.OperationID{
		entityops.OperationAttachmentRevealOriginal,
		entityops.OperationAttachmentOpenOriginalDirectory,
		entityops.OperationAttachmentOpenFolder,
		entityops.OperationAttachmentUnlink,
		entityops.OperationAttachmentDelete,
	}
	if got := sheet.OperationIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("attachment sheet lists %v, want exactly %v", got, want)
	}

	for range 3 {
		press("down")
	}
	press("enter")

	if driver.App().operations.ActionSheet != nil {
		t.Fatal("choosing Unlink left the sheet open over the confirmation")
	}
	if !panel.Confirm.IsActive() || panel.Confirm.Action() != "unlink" {
		t.Fatalf("the sheet's Unlink row raised confirm %q, want the unlink prompt", panel.Confirm.Action())
	}

	coordinator, fake := newFakeAttachmentCoordinator()
	panel.Coordinator = coordinator
	press("y")
	if driver.pendingCmd == nil {
		t.Fatal("confirming Unlink produced no command")
	}
	runImmediateTestCommand(driver.pendingCmd)

	if len(fake.removals) != 1 || fake.removals[0].index != 0 || fake.removals[0].deleteFile {
		t.Fatalf("confirming the sheet's Unlink recorded %+v, want RemoveAttachment(index=0, deleteFile=false)", fake.removals)
	}
}
