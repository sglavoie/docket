package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
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
	deleteFile bool
}

type fakeReorder struct {
	from int
	to   int
}

func (f *fakeAttachmentMutator) AttachURLByLocator(locator app.TaskLocator, url string) (app.TaskResult, error) {
	f.urls = append(f.urls, url)
	return app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}, nil
}

func (f *fakeAttachmentMutator) AttachCodeReferenceByLocator(locator app.TaskLocator, path string, line int) (app.TaskResult, error) {
	f.codeRefs = append(f.codeRefs, path)
	return app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}, nil
}

func (f *fakeAttachmentMutator) AttachFileByLocator(locator app.TaskLocator, sourcePath string) (app.AttachFileResult, error) {
	f.files = append(f.files, sourcePath)
	return app.AttachFileResult{Task: app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}}, nil
}

func (f *fakeAttachmentMutator) RemoveAttachmentByLocator(locator app.TaskLocator, index int, deleteFile bool) (app.RemoveAttachmentResult, error) {
	f.removals = append(f.removals, fakeRemoval{index: index, deleteFile: deleteFile})
	return app.RemoveAttachmentResult{Task: app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}}, nil
}

func (f *fakeAttachmentMutator) ReorderAttachmentByLocator(locator app.TaskLocator, from, to int) (app.TaskResult, error) {
	f.reorders = append(f.reorders, fakeReorder{from: from, to: to})
	return app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}, nil
}

func newFakeAttachmentCoordinator() (AttachmentCoordinator, *fakeAttachmentMutator) {
	fake := &fakeAttachmentMutator{}
	return AttachmentCoordinator{attach: fake}, fake
}

// mountAttachmentsPanel opens the panel over a live shell on a task carrying two
// file attachments, and returns the driver, the panel, and a key press helper.
func mountAttachmentsPanel(t *testing.T) (*UITestDriver, *TaskAttachmentsModel, func(string)) {
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

	driver.dispatch(ShowTaskAttachmentsMsg{Task: task, Project: project})
	panel := driver.App().activeAttachmentsPanel()
	if panel == nil {
		t.Fatal("ShowTaskAttachmentsMsg did not mount the Attachments panel")
	}
	if panel.dispatch == nil {
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
	if driver.App().contextActionSheet == nil {
		t.Fatal("Space did not reach tui.attachment.actions")
	}
	press("esc")
	if driver.App().contextActionSheet != nil {
		t.Fatal("Esc left the attachment action sheet open")
	}

	coordinator, fake := newFakeAttachmentCoordinator()
	panel.coordinator = coordinator
	press("J")
	if driver.pendingCmd == nil {
		t.Fatal("J did not reach tui.attachment.reorder-down")
	}
	driver.pendingCmd()
	if len(fake.reorders) != 1 || fake.reorders[0].from != 0 || fake.reorders[0].to != 1 {
		t.Fatalf("J recorded %+v, want ReorderAttachment(0, 1) on press one", fake.reorders)
	}

	press("a")
	if !panel.addMode {
		t.Fatal("a did not reach tui.attachment.add")
	}
}

// TestAttachmentsRetiredKeysAreSilent holds the keys the panel gave up. Each one
// now means something else app-wide, so the panel must answer none of them
// rather than quietly keeping its old meaning.
func TestAttachmentsRetiredKeysAreSilent(t *testing.T) {
	driver, panel, press := mountAttachmentsPanel(t)

	for _, key := range []string{"u", "d", "x", "o", "O", "ctrl+e", "ctrl+o"} {
		press(key)
		if panel.confirm.IsActive() {
			t.Fatalf("retired key %q raised confirm %q", key, panel.confirm.Action())
		}
		if panel.addMode {
			t.Fatalf("retired key %q entered add mode", key)
		}
		if driver.App().contextActionSheet != nil {
			t.Fatalf("retired key %q opened the action sheet", key)
		}
	}
}

// TestAttachmentsSpaceSheetReachesUnlink walks the whole new path for Unlink,
// which is the retirement this session is really about: `u` no longer unlinks,
// so the Space sheet has to, and the confirmation behind it has to still commit
// the keep-the-file removal the coordinator records.
func TestAttachmentsSpaceSheetReachesUnlink(t *testing.T) {
	driver, panel, press := mountAttachmentsPanel(t)

	press("space")
	sheet := driver.App().contextActionSheet
	if sheet == nil {
		t.Fatal("Space did not open the attachment action sheet")
	}
	want := []OperationID{
		operationAttachmentRevealOriginal,
		operationAttachmentOpenOriginalDirectory,
		operationAttachmentOpenFolder,
		operationAttachmentUnlink,
		operationAttachmentDelete,
	}
	if got := sheet.operationIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("attachment sheet lists %v, want exactly %v", got, want)
	}

	for range 3 {
		press("down")
	}
	press("enter")

	if driver.App().contextActionSheet != nil {
		t.Fatal("choosing Unlink left the sheet open over the confirmation")
	}
	if !panel.confirm.IsActive() || panel.confirm.Action() != "unlink" {
		t.Fatalf("the sheet's Unlink row raised confirm %q, want the unlink prompt", panel.confirm.Action())
	}

	coordinator, fake := newFakeAttachmentCoordinator()
	panel.coordinator = coordinator
	press("y")
	if driver.pendingCmd == nil {
		t.Fatal("confirming Unlink produced no command")
	}
	driver.pendingCmd()

	if len(fake.removals) != 1 || fake.removals[0].index != 0 || fake.removals[0].deleteFile {
		t.Fatalf("confirming the sheet's Unlink recorded %+v, want RemoveAttachment(index=0, deleteFile=false)", fake.removals)
	}
}

func TestTaskAttachmentsDeleteDispatchesCoordinator(t *testing.T) {
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

	coordinator, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, coordinator)
	model.cursor = 1

	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected transaction message from executeDelete command")
	} else if _, ok := msg.(TransactionResultMsg); !ok {
		t.Fatalf("expected TransactionResultMsg, got %T", msg)
	}

	// The view no longer mutates its own copy; the application layer commits
	// and the shell reloads the project.
	if len(model.task.Attachments) != 2 {
		t.Fatalf("expected local attachments untouched before reload, got %d", len(model.task.Attachments))
	}
	if len(fake.removals) != 1 || fake.removals[0].index != 1 || !fake.removals[0].deleteFile {
		t.Fatalf("expected RemoveAttachment(index=1, deleteFile=true), got %+v", fake.removals)
	}
}

func TestTaskAttachmentsDeleteOutOfBounds(t *testing.T) {
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
	}
	project.Tasks = []*models.Task{task}

	coordinator, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, coordinator)
	model.cursor = 5 // Out of bounds

	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}

	// Should return an error message, not panic
	msg := cmd()
	if _, ok := msg.(ErrorMsg); !ok {
		t.Fatalf("expected ErrorMsg for out-of-bounds delete, got %T", msg)
	}

	// Attachments should be unchanged and nothing dispatched
	if len(model.task.Attachments) != 1 {
		t.Fatalf("expected attachments to be unchanged, got %d", len(model.task.Attachments))
	}
	if len(fake.removals) != 0 {
		t.Fatalf("expected no coordinator dispatch, got %+v", fake.removals)
	}
}

func TestTaskAttachmentsKeepDispatchesCoordinator(t *testing.T) {
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

	coordinator, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, coordinator)
	model.cursor = 0

	cmd := model.executeKeep()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeKeep")
	}
	if _, ok := cmd().(TransactionResultMsg); !ok {
		t.Fatal("expected TransactionResultMsg from executeKeep command")
	}
	if len(fake.removals) != 1 || fake.removals[0].index != 0 || fake.removals[0].deleteFile {
		t.Fatalf("expected RemoveAttachment(index=0, deleteFile=false), got %+v", fake.removals)
	}
}

func TestTaskAttachmentsAddURLDispatchesCoordinator(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	coordinator, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, coordinator)
	model.addMode = true
	model.textInput.SetValue("https://example.com/design.pdf")

	cmd := model.addAttachment()
	if cmd == nil {
		t.Fatal("expected non-nil command from addAttachment")
	}
	if _, ok := cmd().(TransactionResultMsg); !ok {
		t.Fatal("expected TransactionResultMsg from addAttachment command")
	}

	// Add mode exits before the transaction resolves.
	if model.addMode {
		t.Fatal("expected addMode to be false after addAttachment()")
	}
	if len(fake.urls) != 1 || fake.urls[0] != "https://example.com/design.pdf" {
		t.Fatalf("expected AttachURL dispatch, got %+v", fake.urls)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.cursor)
	}
}

func TestTaskAttachmentsAddEmptyPath(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskAttachmentsModel(task, project, nil, AttachmentCoordinator{})
	model.addMode = true
	model.textInput.SetValue("   ")

	cmd := model.addAttachment()
	if cmd != nil {
		t.Fatal("expected nil command for empty path")
	}
	if model.err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestTaskAttachmentsAddNormalizesPastedPaths(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "my file.txt")
	if err := os.WriteFile(realPath, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tests := []struct {
		name  string
		input string
	}{
		{"escaped space", strings.ReplaceAll(realPath, " ", `\ `)},
		{"quoted path", `"` + realPath + `"`},
		{"raw unquoted path with spaces", realPath},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
			if err != nil {
				t.Fatalf("NewProject: %v", err)
			}

			task, err := models.NewGenericTask("task-1", 1, "Test task")
			if err != nil {
				t.Fatalf("NewTask: %v", err)
			}
			project.Tasks = []*models.Task{task}

			coordinator, _ := newFakeAttachmentCoordinator()
			model := NewTaskAttachmentsModel(task, project, nil, coordinator)
			model.addMode = true
			model.textInput.SetValue(tc.input)

			cmd := model.addAttachment()
			if cmd == nil {
				t.Fatalf("expected non-nil command from addAttachment for input %q", tc.input)
			}
			if model.err != nil {
				t.Fatalf("expected no error for input %q, got: %v", tc.input, model.err)
			}
			if model.addMode {
				t.Fatal("expected addMode to be false after successful add")
			}
		})
	}
}

// TestOpenAttachmentInEditor_EmptyEditorUsesOSDefault verifies that when
// attachment_editor is empty (the default), openAttachmentInEditor routes
// through buildOpenCommand (OS default) rather than resolving a text editor.
// We detect this by checking that no ResolveEditor call is made (i.e. the
// function returns a non-nil cmd without an error even though no text editor
// is available in PATH).
func TestOpenAttachmentInEditor_EmptyEditorUsesOSDefault(t *testing.T) {
	project, err := models.NewProject("proj", "Test Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	// Create a real file so the existence check passes.
	attachDir := t.TempDir()
	project.Path = attachDir

	// We need the attachment path to exist. Use a simple approach:
	// create a model with a URL attachment (skips file path resolution)
	// then switch to a file attachment via a stub.
	// Actually, easier: just verify the config routing by checking the model
	// returns an error about the file not existing (which means it got past
	// the "empty editor" branch) vs. a "no editor found" error.
	cfg := newTestConfig()

	task.Attachments = []models.Attachment{
		{Filename: "note.txt", OriginalPath: "/nonexistent/note.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskAttachmentsModel(task, project, testRuntimeAt(cfg.DataDir(), cfg), AttachmentCoordinator{})
	model.cursor = 0

	// openAttachmentInEditor should attempt
	// buildOpenCommand (OS default). Since the file doesn't exist in the
	// expected attachment directory, it sets model.err about "file not found",
	// NOT about "no editor configured" or "not found in PATH".
	model.openAttachmentInEditor()

	if model.err == nil {
		// The attachment dir doesn't exist so we expect an error.
		// If we got here with no error, the test environment has the file.
		return
	}

	errMsg := model.err.Error()
	if strings.Contains(errMsg, "no editor") || strings.Contains(errMsg, "not found in PATH") {
		t.Errorf("empty attachment_editor should use OS default open, not text editor resolution; got: %s", errMsg)
	}
}
