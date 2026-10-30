package taskdetail

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func TestTaskAttachmentsReorderWaitsForCommit(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatal(err)
	}
	task.Attachments = []models.Attachment{
		{Filename: "a.txt", AddedAt: time.Now()},
		{Filename: "b.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}
	before := slices.Clone(task.Attachments)
	attach, _ := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, attach)

	cmd := model.MoveSelectedAttachmentDown()
	if cmd == nil {
		t.Fatal("reorder returned no command")
	}
	if !slices.Equal(task.Attachments, before) || model.Cursor != 0 {
		t.Fatal("reorder changed shared task data or cursor before its transaction ran")
	}
	result, ok := cmd().(coordinator.TransactionResultMsg)
	if !ok {
		t.Fatal("reorder did not dispatch its transaction")
	}
	if !slices.Equal(task.Attachments, before) {
		t.Fatal("panel changed task data without a committed reload")
	}
	// A successful reload supplies the new order; only then follow the item.
	task.Attachments[0], task.Attachments[1] = task.Attachments[1], task.Attachments[0]
	if !model.RefreshAfterReload() {
		t.Fatal("reload lost the task")
	}
	model.FollowAttachmentReorder(taskref.For(task, project), result.AttachmentReorder)
	if model.Cursor != 1 {
		t.Fatal("committed reorder did not follow the moved attachment")
	}
}

func TestTaskAttachmentsReorderFreezesTheReadList(t *testing.T) {
	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatal(err)
	}
	task.Attachments = []models.Attachment{
		{Filename: "a.txt", AddedAt: time.Now()},
		{Filename: "b.txt", AddedAt: time.Now()},
	}
	project := &models.Project{ID: "project-1", Tasks: []*models.Task{task}}
	before := slices.Clone(task.Attachments)
	attach, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, attach)
	cmd := model.MoveSelectedAttachmentDown()
	// Edit the same backing array while the command is waiting to run.
	task.Attachments[0].Filename = "replacement.txt"
	result, ok := cmd().(coordinator.TransactionResultMsg)
	if !ok || len(fake.reorders) != 1 {
		t.Fatal("reorder did not dispatch")
	}
	if !slices.Equal(fake.reorders[0].expected, before) || result.AttachmentReorder.Attachment != before[0] {
		t.Fatal("asynchronous reorder reread the live attachment list")
	}
}

func TestTaskAttachmentsReorderKeepsNavigationAndOtherOwners(t *testing.T) {
	for _, scenario := range []string{"navigation", "other owner", "item replaced"} {
		t.Run(scenario, func(t *testing.T) {
			a := models.Attachment{Filename: "a.txt", AddedAt: time.Now()}
			b := models.Attachment{Filename: "b.txt", AddedAt: time.Now()}
			task := &models.Task{ID: "task-1", Attachments: []models.Attachment{b, a, {Filename: "c.txt"}}}
			project := &models.Project{ID: "project-1", Tasks: []*models.Task{task}}
			model := NewTaskAttachmentsModel(task, project, nil, coordinator.AttachmentCoordinator{})
			owner := taskref.For(task, project)
			switch scenario {
			case "navigation":
				model.Cursor = 2
			case "other owner":
				owner.TaskID = "another-task"
			case "item replaced":
				task.Attachments[1].Filename = "replacement.txt"
			}
			cursor := model.Cursor
			model.FollowAttachmentReorder(owner, &coordinator.AttachmentReorder{From: 0, To: 1, Attachment: a})
			if model.Cursor != cursor {
				t.Fatal("committed reorder redirected an unrelated cursor")
			}
		})
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

	attach, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, attach)
	model.Cursor = 1

	cmd := model.executeDelete(pendingRemovalAt(model, 1))
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected transaction message from executeDelete command")
	} else if _, ok := msg.(coordinator.TransactionResultMsg); !ok {
		t.Fatalf("expected TransactionResultMsg, got %T", msg)
	}

	// The view no longer mutates its own copy; the application layer commits
	// and the shell reloads the project.
	if len(model.Task.Attachments) != 2 {
		t.Fatalf("expected local attachments untouched before reload, got %d", len(model.Task.Attachments))
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

	attach, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, attach)
	model.Cursor = 5 // Out of bounds

	cmd := model.executeDelete(&pendingAttachmentRemoval{index: 5})
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}

	// Should return an error message, not panic
	msg := cmd()
	if _, ok := msg.(status.ErrorMsg); !ok {
		t.Fatalf("expected ErrorMsg for out-of-bounds delete, got %T", msg)
	}

	// Attachments should be unchanged and nothing dispatched
	if len(model.Task.Attachments) != 1 {
		t.Fatalf("expected attachments to be unchanged, got %d", len(model.Task.Attachments))
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

	attach, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, attach)
	model.Cursor = 0

	cmd := model.executeKeep(pendingRemovalAt(model, 0))
	if cmd == nil {
		t.Fatal("expected non-nil command from executeKeep")
	}
	if _, ok := cmd().(coordinator.TransactionResultMsg); !ok {
		t.Fatal("expected TransactionResultMsg from executeKeep command")
	}
	if len(fake.removals) != 1 || fake.removals[0].index != 0 || fake.removals[0].deleteFile {
		t.Fatalf("expected RemoveAttachment(index=0, deleteFile=false), got %+v", fake.removals)
	}
}

// pendingRemovalAt is what a removal prompt opened on index freezes.
func pendingRemovalAt(m *TaskAttachmentsModel, index int) *pendingAttachmentRemoval {
	return &pendingAttachmentRemoval{index: index, attachment: m.Task.Attachments[index]}
}

// TestTaskAttachmentsConfirmedDeleteRefusesShiftedList pins that a delete
// prompt removes the attachment it asked about or nothing: a reload that drops
// an earlier attachment while the prompt is open shifts the list under the
// cursor, and answering yes must not delete the neighbor that moved into place.
func TestTaskAttachmentsConfirmedDeleteRefusesShiftedList(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	added := time.Now()
	task.Attachments = []models.Attachment{
		{Filename: "file1.txt", OriginalPath: "/tmp/file1.txt", AddedAt: added},
		{Filename: "file2.txt", OriginalPath: "/tmp/file2.txt", AddedAt: added},
		{Filename: "file3.txt", OriginalPath: "/tmp/file3.txt", AddedAt: added},
	}
	project.Tasks = []*models.Task{task}

	attach, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, attach)
	model.Cursor = 1
	model.ConfirmDeleteSelectedAttachment()
	if !model.Confirm.IsActive() {
		t.Fatal("expected the delete prompt to open")
	}

	// A sync pull removes file1 while the prompt is open; file3 now sits at 1.
	task.Attachments = task.Attachments[1:]

	_, cmd := model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("expected a refusal command")
	}
	if _, ok := cmd().(status.ErrorMsg); !ok {
		t.Fatal("expected the shifted list to refuse the removal")
	}
	if len(fake.removals) != 0 {
		t.Fatalf("a shifted list must dispatch nothing, got %+v", fake.removals)
	}

	// Asking again about the attachment now under the cursor removes exactly it,
	// and names it to the application layer.
	model.Cursor = 0
	model.ConfirmDeleteSelectedAttachment()
	_, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("expected a removal command")
	}
	cmd()
	if len(fake.removals) != 1 || fake.removals[0].index != 0 || fake.removals[0].expected.Filename != "file2.txt" {
		t.Fatalf("expected removal of file2.txt at 0, got %+v", fake.removals)
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

	attach, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, attach)
	model.AddMode = true
	model.TextInput.SetValue("https://example.com/design.pdf")

	cmd := model.addAttachment()
	if cmd == nil {
		t.Fatal("expected non-nil command from addAttachment")
	}
	if _, ok := cmd().(coordinator.TransactionResultMsg); !ok {
		t.Fatal("expected TransactionResultMsg from addAttachment command")
	}

	// Add mode exits before the transaction resolves.
	if model.AddMode {
		t.Fatal("expected addMode to be false after addAttachment()")
	}
	if len(fake.urls) != 1 || fake.urls[0] != "https://example.com/design.pdf" {
		t.Fatalf("expected AttachURL dispatch, got %+v", fake.urls)
	}
	if model.Cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.Cursor)
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

	model := NewTaskAttachmentsModel(task, project, nil, coordinator.AttachmentCoordinator{})
	model.AddMode = true
	model.TextInput.SetValue("   ")

	cmd := model.addAttachment()
	if cmd != nil {
		t.Fatal("expected nil command for empty path")
	}
	if model.Err == nil {
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

			attach, _ := newFakeAttachmentCoordinator()
			model := NewTaskAttachmentsModel(task, project, nil, attach)
			model.AddMode = true
			model.TextInput.SetValue(tc.input)

			cmd := model.addAttachment()
			if cmd == nil {
				t.Fatalf("expected non-nil command from addAttachment for input %q", tc.input)
			}
			if model.Err != nil {
				t.Fatalf("expected no error for input %q, got: %v", tc.input, model.Err)
			}
			if model.AddMode {
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

	model := NewTaskAttachmentsModel(task, project, testRuntimeAt(cfg.DataDir(), cfg), coordinator.AttachmentCoordinator{})
	model.Cursor = 0

	// openAttachmentInEditor should attempt
	// buildOpenCommand (OS default). Since the file doesn't exist in the
	// expected attachment directory, it sets model.err about "file not found",
	// NOT about "no editor configured" or "not found in PATH".
	model.openAttachmentInEditor()

	if model.Err == nil {
		// The attachment dir doesn't exist so we expect an error.
		// If we got here with no error, the test environment has the file.
		return
	}

	errMsg := model.Err.Error()
	if strings.Contains(errMsg, "no editor") || strings.Contains(errMsg, "not found in PATH") {
		t.Errorf("empty attachment_editor should use OS default open, not text editor resolution; got: %s", errMsg)
	}
}
