package tui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// clipboardSelectionEnv is one wired shell over two seeded projects, with the
// Outliner rows in a deliberately non-identity order so the copied text can
// only match by following the displayed rows.
type clipboardSelectionEnv struct {
	env      *appBulkPickerEnv
	model    *AppModel
	outliner *outliner.OutlinerModel
	tasks    map[string]*models.Task
}

// newClipboardSelectionEnv seeds three tasks in the first project and one in the
// second, then draws only three of them: "hidden" stands for the member a fold,
// a filter, or another mode is not showing.
func newClipboardSelectionEnv(t *testing.T) *clipboardSelectionEnv {
	t.Helper()
	first := newBulkPickerTestTask(t, 1, "First")
	first.Description = "First body"
	second := newBulkPickerTestTask(t, 2, "Second")
	second.Description = "Second body"
	hidden := newBulkPickerTestTask(t, 3, "Hidden")
	hidden.Description = "Hidden body"
	other := newBulkPickerTestTask(t, 1, "Other")
	other.Description = "Other body"

	env := newAppBulkPickerEnv(t,
		[]*models.Task{first, second, hidden},
		[]*models.Task{other},
	)
	model := env.appModel()
	model.operations.Registry = operation.NewOperationRegistry()
	model.operations.Executor = operation.NewOperationExecutor(model.operations.Registry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}

	view := outliner.NewOutlinerModel(nil, model.domain.manager, model.domain.runtime)
	view.Population.Projects = env.projects
	view.Rows = []outliner.OutlinerRow{
		{Task: second, ProjectID: env.projects[0].ID},
		{Task: other, ProjectID: env.projects[1].ID},
		{Task: first, ProjectID: env.projects[0].ID},
	}
	model.crossOutliner = view
	model.viewNav.views.Register(presentation.ViewCrossProjectOutliner, view)

	return &clipboardSelectionEnv{
		env: env, model: model, outliner: view,
		tasks: map[string]*models.Task{
			"first": first, "second": second, "hidden": hidden, "other": other,
		},
	}
}

func (c *clipboardSelectionEnv) identity(t *testing.T, name string) taskref.TaskIdentity {
	t.Helper()
	task, ok := c.tasks[name]
	if !ok {
		t.Fatalf("fixture has no task %q", name)
	}
	project := c.env.projects[0]
	if name == "other" {
		project = c.env.projects[1]
	}
	return taskref.For(task, project)
}

func (c *clipboardSelectionEnv) selectTasks(t *testing.T, names ...string) {
	t.Helper()
	c.outliner.MultiSelectMode = true
	for _, name := range names {
		c.outliner.SelectTaskIdentity(c.identity(t, name))
	}
}

// copyThroughShell runs one copy operation the whole way a key press does:
// executor, application projection command, and the shell's clipboard handler.
// It returns the status-bearing command so a caller can drain it.
func (c *clipboardSelectionEnv) copyThroughShell(t *testing.T, id operation.OperationID) tea.Cmd {
	t.Helper()
	result := c.model.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:      id,
		Context: c.model.operationCommandContext(),
	})
	if result.Err != nil {
		t.Fatalf("execute %s: %v", id, result.Err)
	}
	command, ok := result.Message.(operation.OperationCommandResultMsg)
	if !ok {
		t.Fatalf("%s message = %T, want OperationCommandResultMsg", id, result.Message)
	}
	if command.Cmd == nil {
		t.Fatalf("%s produced no application command", id)
	}
	projected, ok := command.Cmd().(coordinator.TaskClipboardProjectedMsg)
	if !ok {
		t.Fatalf("%s produced %T, want a clipboard projection", id, command.Cmd())
	}
	handled, cmd := c.model.handleClipboardMessages(projected)
	if !handled {
		t.Fatalf("the shell did not handle %T", projected)
	}
	return cmd
}

// drainCopiedStatus follows the handler's own message chain to the status line,
// which is where the counts a user reads are written.
func (c *clipboardSelectionEnv) drainCopiedStatus(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		t.Fatal("the clipboard handler produced no follow-up command")
	}
	copied, ok := cmd().(TaskCopiedMsg)
	if !ok {
		t.Fatalf("follow-up message = %T, want TaskCopiedMsg", cmd())
	}
	if handled, _ := c.model.handleClipboardMessages(copied); !handled {
		t.Fatal("the shell did not handle TaskCopiedMsg")
	}
	return c.model.feedback.message
}

// workspaceFingerprint hashes every file under the workspace, which is how a
// read-only claim is tested rather than asserted: a rewritten tasks.md changes
// it even when only a timestamp inside it moved.
func workspaceFingerprint(t *testing.T, root string) map[string]string {
	t.Helper()
	fingerprint := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		bytes, readErr := os.ReadFile(path) //nolint:gosec // test-owned temp workspace
		if readErr != nil {
			return readErr
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		fingerprint[relative] = fmt.Sprintf("%x", sha256.Sum256(bytes))
		return nil
	})
	if err != nil {
		t.Fatalf("fingerprint %s: %v", root, err)
	}
	return fingerprint
}

func assertWorkspaceUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	changed := make([]string, 0)
	for path, hash := range after {
		if before[path] != hash {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, path)
		}
	}
	if len(changed) != 0 {
		sort.Strings(changed)
		t.Errorf("a copy changed workspace bytes: %v", changed)
	}
}

// TestCopySelectionWritesOneOrderedBlockAndTouchesNothing is the session's
// end-to-end guarantee: three copy keys, a four-task selection with one member
// the view is not showing, one clipboard write in displayed-then-hidden order,
// and a workspace that comes out byte-identical with no undo entry.
func TestCopySelectionWritesOneOrderedBlockAndTouchesNothing(t *testing.T) {
	fixture := newClipboardSelectionEnv(t)
	fixture.selectTasks(t, "first", "second", "hidden", "other")
	fixture.outliner.Cursor = 2
	before := workspaceFingerprint(t, fixture.env.dataDir)
	undoBefore := clipboardUndoCount(t, fixture.env)

	for _, testCase := range []struct {
		id     operation.OperationID
		want   string
		status string
	}{
		{
			id:     operation.OperationTaskCopyTitle,
			want:   "Second\nOther\nFirst\nHidden",
			status: "Copied title for 4 tasks to clipboard · 1 hidden",
		},
		{
			id:     operation.OperationTaskCopyDetails,
			want:   "Second body\n\nOther body\n\nFirst body\n\nHidden body",
			status: "Copied description for 4 tasks to clipboard · 1 hidden",
		},
		{
			id: operation.OperationTaskCopyTitleDescription,
			want: "Second\n\nSecond body\n\nOther\n\nOther body\n\n" +
				"First\n\nFirst body\n\nHidden\n\nHidden body",
			status: "Copied title and description for 4 tasks to clipboard · 1 hidden",
		},
	} {
		var copied string
		withFakeClipboard(t, nil, &copied)
		status := fixture.drainCopiedStatus(t, fixture.copyThroughShell(t, testCase.id))
		if copied != testCase.want {
			t.Errorf("%s clipboard = %q, want %q", testCase.id, copied, testCase.want)
		}
		if status != testCase.status {
			t.Errorf("%s status = %q, want %q", testCase.id, status, testCase.status)
		}
	}

	if !fixture.outliner.MultiSelectMode || fixture.outliner.SelectedCount() != 4 {
		t.Errorf("selection after copy = active %v count %d, want active/4",
			fixture.outliner.MultiSelectMode, fixture.outliner.SelectedCount())
	}
	if fixture.outliner.Cursor != 2 {
		t.Errorf("cursor after copy = %d, want the untouched 2", fixture.outliner.Cursor)
	}
	assertWorkspaceUnchanged(t, before, workspaceFingerprint(t, fixture.env.dataDir))
	if got := clipboardUndoCount(t, fixture.env); got != undoBefore {
		t.Errorf("undo entries = %d, want the seeded %d: a copy is not a mutation", got, undoBefore)
	}
}

// clipboardUndoCount reads the workspace ledger depth. The seed commit leaves
// one entry behind, so the copies are measured against that baseline rather
// than against zero.
func clipboardUndoCount(t *testing.T, env *appBulkPickerEnv) int {
	t.Helper()
	count, err := env.ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("undo count: %v", err)
	}
	return count
}

// TestCopySingleSelectionMatchesCursorCopyExactly is the parity guard at the
// shell: selecting one task copies the same bytes and reads the same status as
// copying the task under the cursor with no selection at all.
func TestCopySingleSelectionMatchesCursorCopyExactly(t *testing.T) {
	fixture := newClipboardSelectionEnv(t)
	fixture.outliner.Cursor = 2 // the "First" row

	var fromCursor string
	withFakeClipboard(t, nil, &fromCursor)
	cursorStatus := fixture.drainCopiedStatus(
		t, fixture.copyThroughShell(t, operation.OperationTaskCopyTitleDescription),
	)
	if want := "First\n\nFirst body"; fromCursor != want {
		t.Fatalf("cursor copy = %q, want %q", fromCursor, want)
	}
	if want := "Copied title and description to clipboard"; cursorStatus != want {
		t.Errorf("cursor status = %q, want the uncounted %q", cursorStatus, want)
	}

	fixture.selectTasks(t, "first")
	var fromSelection string
	withFakeClipboard(t, nil, &fromSelection)
	selectionStatus := fixture.drainCopiedStatus(
		t, fixture.copyThroughShell(t, operation.OperationTaskCopyTitleDescription),
	)
	if fromSelection != fromCursor {
		t.Errorf("single-selection copy = %q, want the cursor copy %q", fromSelection, fromCursor)
	}
	if selectionStatus != cursorStatus {
		t.Errorf("single-selection status = %q, want %q", selectionStatus, cursorStatus)
	}
}

// TestCopyEmptyVisualSelectionRefusesInsteadOfCopyingTheCursor holds the
// selection rule for the read-only operations: an empty visual selection is a
// refusal, never a silent fall back to the row under the cursor.
func TestCopyEmptyVisualSelectionRefusesInsteadOfCopyingTheCursor(t *testing.T) {
	fixture := newClipboardSelectionEnv(t)
	fixture.outliner.MultiSelectMode = true
	fixture.outliner.Cursor = 0

	copied := "untouched"
	withFakeClipboard(t, nil, &copied)
	for _, id := range []operation.OperationID{
		operation.OperationTaskCopyTitle,
		operation.OperationTaskCopyDetails,
		operation.OperationTaskCopyTitleDescription,
	} {
		result := fixture.model.operations.Executor.Execute(operation.OperationExecutionRequest{
			ID:      id,
			Context: fixture.model.operationCommandContext(),
		})
		if result.Err == nil {
			t.Fatalf("%s copied under an empty visual selection: %+v", id, result.Message)
		}
		if !strings.Contains(result.Err.Error(), "move the cursor to a task") {
			t.Errorf("%s error = %v, want the target rejection", id, result.Err)
		}
	}
	if copied != "untouched" {
		t.Errorf("clipboard = %q, want no write at all", copied)
	}
}

// TestCopySelectionFailureReportsNothingCopied keeps a failing clipboard from
// reading as a partial success: the status is an error and no TaskCopiedMsg,
// which is what the count would have come from, is ever emitted.
func TestCopySelectionFailureReportsNothingCopied(t *testing.T) {
	fixture := newClipboardSelectionEnv(t)
	fixture.selectTasks(t, "first", "second", "hidden", "other")
	withFakeClipboard(t, errors.New("pbcopy exploded"), nil)

	cmd := fixture.copyThroughShell(t, operation.OperationTaskCopyTitle)
	if cmd == nil {
		t.Fatal("a failed copy reported nothing")
	}
	msg := cmd()
	if _, copied := msg.(TaskCopiedMsg); copied {
		t.Fatalf("a failed copy still reported success: %+v", msg)
	}
	text := fmt.Sprint(msg)
	if !strings.Contains(text, "couldn't copy to clipboard") || !strings.Contains(text, "pbcopy exploded") {
		t.Errorf("failure message = %q, want the wrapped clipboard error", text)
	}
	if fixture.model.feedback.message != "" {
		t.Errorf("status = %q, want no success line", fixture.model.feedback.message)
	}
}

// TestCopySelectionRefusesWhenATargetIsGone is the missing-target half of the
// same rule: a selection naming a deleted task copies none of it.
func TestCopySelectionRefusesWhenATargetIsGone(t *testing.T) {
	fixture := newClipboardSelectionEnv(t)
	fixture.selectTasks(t, "first", "second")
	fixture.outliner.SelectTaskIdentity(taskref.TaskIdentity{
		ProjectID: fixture.env.projects[0].ID,
		TaskID:    "deleted-task",
	})

	copied := "untouched"
	withFakeClipboard(t, nil, &copied)
	result := fixture.model.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationTaskCopyTitle,
		Context: fixture.model.operationCommandContext(),
	})
	if result.Err != nil {
		t.Fatalf("execute: %v", result.Err)
	}
	command, ok := result.Message.(operation.OperationCommandResultMsg)
	if !ok || command.Cmd == nil {
		t.Fatalf("result message = %+v, want an application command", result.Message)
	}
	if _, projected := command.Cmd().(coordinator.TaskClipboardProjectedMsg); projected {
		t.Fatal("a selection naming a deleted task still projected clipboard text")
	}
	if copied != "untouched" {
		t.Errorf("clipboard = %q, want no write at all", copied)
	}
}
