package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// withFakeClipboard swaps copyTextToClipboard for the duration of the test,
// capturing the copied text (or forcing an error) so clipboard-dependent code
// paths are exercised without touching the real system clipboard.
func withFakeClipboard(t *testing.T, err error, captured *string) {
	t.Helper()
	orig := copyTextToClipboard
	t.Cleanup(func() { copyTextToClipboard = orig })
	copyTextToClipboard = func(text string) error {
		if captured != nil {
			*captured = text
		}
		return err
	}
}

func TestFormatNumberedMarkdownList(t *testing.T) {
	got := formatNumberedMarkdownList([]string{
		"first",
		"second line one\nsecond line two",
		"third",
	})
	want := strings.Join([]string{
		"1. first",
		"2. second line one",
		"   second line two",
		"3. third",
	}, "\n")
	if got != want {
		t.Fatalf("numbered markdown mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func newChecklistFixture(t *testing.T, items ...models.ChecklistItem) *TaskChecklistModel {
	t.Helper()
	project, err := models.NewProject("cl-project", "Checklist", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("cl-task", 1, models.TaskTypeTask, "Checklist task")
	if err != nil {
		t.Fatal(err)
	}
	task.Checklist = items
	project.Tasks = []*models.Task{task}
	m := NewTaskChecklistModel(task, project)
	m.coordinator = newTestSecondaryCoordinators(project).Subview
	return m
}

func TestChecklistNormalEnterToggles(t *testing.T) {
	m := newChecklistFixture(t, models.ChecklistItem{Text: "toggle me"})
	updated, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	cl := updated.(*TaskChecklistModel)
	if cmd == nil || !cl.task.Checklist[0].Done {
		t.Fatal("expected Enter in normal mode to toggle completion")
	}
	if cl.editMode {
		t.Fatal("Enter must not open the editor in normal mode")
	}
}

func TestChecklistSpaceStillToggles(t *testing.T) {
	m := newChecklistFixture(t, models.ChecklistItem{Text: "toggle me"})
	updated, cmd := m.Update(syntheticSpecialKey(tea.KeySpace))
	cl := updated.(*TaskChecklistModel)
	if cmd == nil || !cl.task.Checklist[0].Done {
		t.Fatal("expected Space to toggle completion")
	}
	if cl.editMode {
		t.Fatal("Space must not open the editor")
	}
}

func TestChecklistAddModeEnterSaves(t *testing.T) {
	m := newChecklistFixture(t)
	m.addMode = true
	m.textInput.SetValue("brand new item")

	updated, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	cl := updated.(*TaskChecklistModel)
	if cmd == nil {
		t.Fatal("expected Enter in add mode to return a save command")
	}
	if cl.addMode {
		t.Fatal("expected add mode to close after Enter save")
	}
	if len(cl.task.Checklist) != 1 || cl.task.Checklist[0].Text != "brand new item" {
		t.Fatalf("expected the new item saved, got %+v", cl.task.Checklist)
	}
}

func TestChecklistEditModeEnterSaves(t *testing.T) {
	m := newChecklistFixture(t, models.ChecklistItem{Text: "old"})
	m.editMode = true
	m.editIndex = 0
	m.editTextInput.SetValue("updated")

	updated, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	cl := updated.(*TaskChecklistModel)
	if cmd == nil {
		t.Fatal("expected Enter in edit mode to return a save command")
	}
	if cl.editMode {
		t.Fatal("expected edit mode to close after Enter save")
	}
	if cl.task.Checklist[0].Text != "updated" {
		t.Fatalf("expected item text updated, got %q", cl.task.Checklist[0].Text)
	}
}

func TestChecklistNoCtrlSItemSave(t *testing.T) {
	// Ctrl+S must no longer save an item; the add-mode text stays uncommitted.
	m := newChecklistFixture(t)
	m.addMode = true
	m.textInput.SetValue("uncommitted")

	updated, _ := m.Update(syntheticCtrlKey('s'))
	cl := updated.(*TaskChecklistModel)
	if len(cl.task.Checklist) != 0 {
		t.Fatalf("Ctrl+S must not save a checklist item, got %+v", cl.task.Checklist)
	}
}

func TestChecklistCopySelectedItem(t *testing.T) {
	var captured string
	withFakeClipboard(t, nil, &captured)
	m := newChecklistFixture(t, models.ChecklistItem{Text: "copy this"}, models.ChecklistItem{Text: "not this"})

	if cmd := m.copyItemText(); cmd != nil {
		t.Fatalf("expected nil cmd, got %v", cmd)
	}
	if captured != "copy this" {
		t.Fatalf("expected selected item text copied, got %q", captured)
	}
	if m.statusMessage == "" {
		t.Fatal("expected a success status message")
	}
}

func TestChecklistCopyAllIncludesCompletedInOrder(t *testing.T) {
	var captured string
	withFakeClipboard(t, nil, &captured)
	m := newChecklistFixture(t,
		models.ChecklistItem{Text: "alpha", Done: true},
		models.ChecklistItem{Text: "beta"},
		models.ChecklistItem{Text: "line one\nline two"},
	)

	m.copyAllItemsText()
	want := strings.Join([]string{"1. alpha", "2. beta", "3. line one", "   line two"}, "\n")
	if captured != want {
		t.Fatalf("copy-all mismatch:\n got: %q\nwant: %q", captured, want)
	}
	if m.err != nil {
		t.Fatalf("unexpected error: %v", m.err)
	}
}

func TestChecklistCopyAllEmptyNothingToCopy(t *testing.T) {
	called := false
	orig := copyTextToClipboard
	t.Cleanup(func() { copyTextToClipboard = orig })
	copyTextToClipboard = func(string) error { called = true; return nil }

	m := newChecklistFixture(t)
	m.copyAllItemsText()
	if called {
		t.Fatal("empty list must not invoke the clipboard")
	}
	if m.statusMessage != "Nothing to copy" {
		t.Fatalf("expected 'Nothing to copy', got %q", m.statusMessage)
	}
}

func TestChecklistCopyAllUnsupportedClipboard(t *testing.T) {
	withFakeClipboard(t, errClipboardUnsupported, nil)
	m := newChecklistFixture(t, models.ChecklistItem{Text: "x"})
	m.copyAllItemsText()
	if m.err != nil {
		t.Fatalf("unsupported clipboard must not set err, got %v", m.err)
	}
	if m.statusMessage != errClipboardUnsupported.Error() {
		t.Fatalf("expected unsupported message, got %q", m.statusMessage)
	}
}

func TestChecklistCopyAllClipboardError(t *testing.T) {
	withFakeClipboard(t, errors.New("boom"), nil)
	m := newChecklistFixture(t, models.ChecklistItem{Text: "x"})
	m.copyAllItemsText()
	if m.err == nil {
		t.Fatal("a real clipboard error must surface on m.err")
	}
}

func TestCommentsAndDecisionsCopyAllIncludeResolved(t *testing.T) {
	project, err := models.NewProject("cd-project", "CD", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("cd-task", 1, models.TaskTypeTask, "CD task")
	if err != nil {
		t.Fatal(err)
	}
	task.AddComment("first comment")
	task.AddComment("second comment")
	task.ToggleCommentResolved(0)
	task.AddDecision("first decision", "why", models.DecisionCategoryDesign)
	task.AddDecision("second decision", "why2", models.DecisionCategoryDesign)
	task.ToggleDecisionResolved(0)
	project.Tasks = []*models.Task{task}

	coords := newTestSecondaryCoordinators(project)

	t.Run("comments", func(t *testing.T) {
		var captured string
		withFakeClipboard(t, nil, &captured)
		cm := NewTaskCommentsModelWithEditor(task, project, "", coords.Subview)
		cm.copyAllCommentsText()
		want := strings.Join([]string{"1. first comment", "2. second comment"}, "\n")
		if captured != want {
			t.Fatalf("comments copy-all mismatch:\n got: %q\nwant: %q", captured, want)
		}
	})

	t.Run("decisions", func(t *testing.T) {
		var captured string
		withFakeClipboard(t, nil, &captured)
		dm := NewTaskDecisionsModelWithEditor(task, project, "", coords.Subview)
		dm.copyAllDecisionsText()
		want := strings.Join([]string{"1. first decision", "2. second decision"}, "\n")
		if captured != want {
			t.Fatalf("decisions copy-all mismatch:\n got: %q\nwant: %q", captured, want)
		}
	})
}

func TestCommentsCopyAllEmptyNothingToCopy(t *testing.T) {
	project, _ := models.NewProject("empty-project", "Empty", t.TempDir())
	task, _ := models.NewTask("empty-task", 1, models.TaskTypeTask, "Empty task")
	project.Tasks = []*models.Task{task}
	coords := newTestSecondaryCoordinators(project)

	called := false
	orig := copyTextToClipboard
	t.Cleanup(func() { copyTextToClipboard = orig })
	copyTextToClipboard = func(string) error { called = true; return nil }

	cm := NewTaskCommentsModelWithEditor(task, project, "", coords.Subview)
	cm.copyAllCommentsText()
	if called {
		t.Fatal("empty comment list must not invoke the clipboard")
	}
	if cm.statusMessage != "Nothing to copy" {
		t.Fatalf("expected 'Nothing to copy', got %q", cm.statusMessage)
	}
}

// TestTaskDetailDelegatesChildConfirmDialog verifies that a child tab's active
// confirmation is surfaced through the TaskDetail wrapper so the App shell
// renders it, and that IsConfirmActive reports the child's active dialog.
func TestTaskDetailDelegatesChildConfirmDialog(t *testing.T) {
	model, _, _ := newConsolidatedTaskDetailFixture(t)
	model.SetSize(80, 28)
	model.SetTab(TaskChecklist)

	if model.IsConfirmActive() {
		t.Fatal("no confirmation should be active initially")
	}
	if cd := model.GetConfirmDialog(); cd != nil && cd.IsActive() {
		t.Fatal("no confirmation dialog should be active initially")
	}

	// Trigger the checklist delete confirmation through the detail wrapper.
	model.Update(syntheticKey('d'))

	if !model.IsConfirmActive() {
		t.Fatal("expected child checklist delete confirmation to be active")
	}
	cd := model.GetConfirmDialog()
	if cd == nil || !cd.IsActive() {
		t.Fatal("expected GetConfirmDialog to surface the active child dialog")
	}
	if cd.Action() != "delete" {
		t.Fatalf("expected delete action, got %q", cd.Action())
	}

	// Esc dismisses the child confirmation without popping the detail view.
	model.Update(syntheticSpecialKey(tea.KeyEscape))
	if model.IsConfirmActive() {
		t.Fatal("expected Esc to cancel the child confirmation")
	}
}
