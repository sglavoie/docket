package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskForm_Children_FieldCount_DefaultTask(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	got := m.fieldCountForActiveTab()
	if got != 2 {
		t.Errorf("Children tab field count: expected 2 section headers, got %d", got)
	}
}

func TestTaskForm_Children_FieldCount_ExpandedChecklist(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m.draft.checklist = []models.ChecklistItem{{Text: "a"}, {Text: "b"}}
	m.checklistExpanded = true
	got := m.fieldCountForActiveTab()
	if got != 4 {
		t.Errorf("Children tab field count with 2 checklist items: expected 4, got %d", got)
	}
}

func TestTaskForm_Children_FieldCountWithSubtasks(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m.draft.subtasks = []formSubtaskDraft{{title: "s1"}}
	m.subtasksExpanded = true
	got := m.fieldCountForActiveTab()
	if got != 3 {
		t.Errorf("Children tab with 1 subtask expanded: expected 3, got %d", got)
	}
}

func TestTaskForm_Children_CKey_TogglesChecklistExpand(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	if m.checklistExpanded {
		t.Fatal("checklist should start collapsed")
	}
	m = sendFormKey(m, syntheticKey('c'))
	if !m.checklistExpanded {
		t.Error("c should expand checklist section")
	}
	m = sendFormKey(m, syntheticKey('c'))
	if m.checklistExpanded {
		t.Error("c again should collapse checklist section")
	}
}

func TestTaskForm_Children_SKey_TogglesSubtasksExpand(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticKey('S'))
	if !m.subtasksExpanded {
		t.Error("S should expand subtasks section")
	}
}

func TestTaskForm_Children_CtrlA_AddsChecklistItemAndOpensOverlay(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('a'))
	if len(m.draft.checklist) != 1 {
		t.Fatalf("expected 1 checklist item after Ctrl+A, got %d", len(m.draft.checklist))
	}
	if m.activeOverlay != formOverlayChecklistEdit {
		t.Errorf("expected formOverlayChecklistEdit after Ctrl+A, got %v", m.activeOverlay)
	}
	if !m.checklistExpanded {
		t.Error("checklist should be expanded after Ctrl+A")
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after Ctrl+A")
	}
}

func TestTaskForm_Children_ChecklistEdit_Commit_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('a'))
	m = sendFormKey(m, syntheticKey('H'))
	m = sendFormKey(m, syntheticKey('i'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayNone {
		t.Error("overlay should be dismissed after Enter")
	}
	if m.draft.checklist[0].Text == "" {
		t.Error("checklist item text should be set after committing overlay")
	}
}

func TestTaskForm_Children_ChecklistEdit_Cancel_RemovesNewItem(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('a'))
	if len(m.draft.checklist) != 1 {
		t.Fatalf("precondition: expected 1 checklist item")
	}
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.activeOverlay != formOverlayNone {
		t.Error("overlay should be dismissed after Esc")
	}
	if len(m.draft.checklist) != 0 {
		t.Errorf("canceled new checklist item should be removed; got %d items", len(m.draft.checklist))
	}
}

func TestTaskForm_Children_ChecklistEdit_CancelExisting_PreservesItem(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "existing"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayChecklistEdit {
		t.Fatalf("expected overlay open, got %v", m.activeOverlay)
	}
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if len(m.draft.checklist) != 1 || m.draft.checklist[0].Text != "existing" {
		t.Errorf("cancel edit of existing item should preserve it; got %v", m.draft.checklist)
	}
}

func TestTaskForm_Children_CtrlD_DeletesChecklistItem(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "a"}, {Text: "b"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1
	m = sendFormKey(m, syntheticCtrlKey('d'))
	if len(m.draft.checklist) != 1 {
		t.Errorf("expected 1 checklist item after Ctrl+D, got %d", len(m.draft.checklist))
	}
	if m.draft.checklist[0].Text != "b" {
		t.Errorf("expected remaining item 'b', got %q", m.draft.checklist[0].Text)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after Ctrl+D")
	}
}

func TestTaskForm_Children_CtrlJ_ReordersChecklistDown(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "first"}, {Text: "second"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1
	m = sendFormKey(m, syntheticCtrlKey('j'))
	if m.draft.checklist[0].Text != "second" {
		t.Errorf("after Ctrl+J: expected 'second' at index 0, got %q", m.draft.checklist[0].Text)
	}
	if m.draft.checklist[1].Text != "first" {
		t.Errorf("after Ctrl+J: expected 'first' at index 1, got %q", m.draft.checklist[1].Text)
	}
	if m.focusIndex != 2 {
		t.Errorf("focusIndex should advance to 2, got %d", m.focusIndex)
	}
}

func TestTaskForm_Children_CtrlK_ReordersChecklistUp(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "first"}, {Text: "second"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 2
	m = sendFormKey(m, syntheticCtrlKey('k'))
	if m.draft.checklist[0].Text != "second" {
		t.Errorf("after Ctrl+K: expected 'second' at index 0, got %q", m.draft.checklist[0].Text)
	}
	if m.focusIndex != 1 {
		t.Errorf("focusIndex should move to 1, got %d", m.focusIndex)
	}
}

func TestTaskForm_Children_Space_TogglesChecklistDone(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "item", Done: false}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1
	m = sendFormKey(m, syntheticSpecialKey(tea.KeySpace))
	if !m.draft.checklist[0].Done {
		t.Error("Space should toggle checklist item Done=true")
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after toggling Done")
	}
}

func TestTaskForm_Children_CtrlT_AddsSubtaskAndOpensOverlay(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('t'))
	if len(m.draft.subtasks) != 1 {
		t.Fatalf("expected 1 subtask after Ctrl+T, got %d", len(m.draft.subtasks))
	}
	if m.activeOverlay != formOverlaySubtaskEdit {
		t.Errorf("expected formOverlaySubtaskEdit after Ctrl+T, got %v", m.activeOverlay)
	}
	if !m.subtasksExpanded {
		t.Error("subtasks should be expanded after Ctrl+T")
	}
}

func TestTaskForm_Children_SubtaskEdit_Commit_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('t'))
	m = sendFormKey(m, syntheticKey('F'))
	m = sendFormKey(m, syntheticKey('o'))
	m = sendFormKey(m, syntheticKey('o'))
	m = sendFormKey(m, syntheticKey('j'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayNone {
		t.Error("overlay should be dismissed after Enter")
	}
	if len(m.draft.subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(m.draft.subtasks))
	}
	if m.draft.subtasks[0].title != "Fooj" {
		t.Errorf("expected subtask title Fooj, got %q", m.draft.subtasks[0].title)
	}
}

func TestTaskForm_Children_SubtaskEdit_Cancel_RemovesNewSubtask(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	m = sendFormKey(m, syntheticCtrlKey('t'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if len(m.draft.subtasks) != 0 {
		t.Errorf("canceled new subtask should be removed; got %d", len(m.draft.subtasks))
	}
}

func TestTaskForm_Children_CtrlOIsUnbound(t *testing.T) {
	m := newTestTaskForm()
	m.draft.subtasks = []formSubtaskDraft{{title: "s1"}}
	m.subtasksExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = m.childrenSubtasksHeaderIdx() + 1
	m = sendFormKey(m, syntheticCtrlKey('o'))
	if m.hasChanges || m.draft.subtasks[0].title != "s1" {
		t.Fatal("Ctrl+O changed a subtask after optional/create-and-open removal")
	}
	for _, key := range m.HandledKeys() {
		if key == "ctrl+o" {
			t.Fatal("task form still advertises Ctrl+O")
		}
	}
}

func TestTaskForm_Children_SubtaskReorder(t *testing.T) {
	m := newTestTaskForm()
	m.draft.subtasks = []formSubtaskDraft{
		{title: "first"},
		{title: "second"},
	}
	m.subtasksExpanded = true
	m = navigateToChildrenTab(m)
	stStart := m.childrenSubtasksHeaderIdx() + 1
	m.focusIndex = stStart
	m = sendFormKey(m, syntheticCtrlKey('j'))
	if m.draft.subtasks[0].title != "second" {
		t.Errorf("after Ctrl+J: expected 'second' at index 0, got %q", m.draft.subtasks[0].title)
	}
	if m.draft.subtasks[1].title != "first" {
		t.Errorf("after Ctrl+J: expected 'first' at index 1, got %q", m.draft.subtasks[1].title)
	}
}

func TestTaskForm_Children_RendersChecklistSectionHeader(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	view := m.View().Content
	if !strings.Contains(view, "Checklist") {
		t.Errorf("Children tab should render Checklist section header; got:\n%s", view)
	}
}

func TestTaskForm_Children_RendersChecklistItems_WhenExpanded(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "buy milk", Done: false}, {Text: "done thing", Done: true}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	view := m.View().Content
	if !strings.Contains(view, "buy milk") {
		t.Errorf("Children tab should show checklist item text; got:\n%s", view)
	}
	if !strings.Contains(view, "☑") {
		t.Errorf("Children tab should show ☑ for done item; got:\n%s", view)
	}
}

func TestTaskForm_Children_RendersSubtasksSection(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToChildrenTab(m)
	view := m.View().Content
	if !strings.Contains(view, "Subtasks") {
		t.Errorf("Children tab should render Subtasks section; got:\n%s", view)
	}
}

func TestSubtaskEditEmptyTitleRemovesSubtask(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test")
	cfg := newTestConfig()

	m := NewTaskFormModelCreate(project, testRuntimeAt(cfg.DataDir(), cfg), nil)
	m.draft.subtasks = []formSubtaskDraft{
		{title: "Sub 1"},
		{title: "Sub 2"},
	}

	msg := TaskFormSubtaskEditedMsg{Idx: 0, Title: ""}
	m.applyFormOverlayResult(msg)

	if len(m.draft.subtasks) != 1 {
		t.Fatalf("expected 1 subtask after empty title edit, got %d", len(m.draft.subtasks))
	}
	if m.draft.subtasks[0].title != "Sub 2" {
		t.Errorf("expected remaining subtask to be 'Sub 2', got %q", m.draft.subtasks[0].title)
	}
}

func TestSubtaskEditNonEmptyTitlePreserved(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test")
	cfg := newTestConfig()

	m := NewTaskFormModelCreate(project, testRuntimeAt(cfg.DataDir(), cfg), nil)
	m.draft.subtasks = []formSubtaskDraft{
		{title: "Old Title"},
	}

	msg := TaskFormSubtaskEditedMsg{Idx: 0, Title: "New Title"}
	m.applyFormOverlayResult(msg)

	if len(m.draft.subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(m.draft.subtasks))
	}
	if m.draft.subtasks[0].title != "New Title" {
		t.Errorf("expected title 'New Title', got %q", m.draft.subtasks[0].title)
	}
}
