package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskForm_Save_EmptyTitle_SetsErrorJumpsToBasics(t *testing.T) {
	m := newTestTaskForm()
	m.draft.title = ""
	m.titleInput.SetValue("")
	_, cmd := m.Update(syntheticCtrlKey('s'))
	if cmd != nil {
		t.Error("expected nil Cmd on validation failure")
	}
	if m.err == nil {
		t.Error("expected m.err to be set after empty title save")
	}
	if m.activeTab != formTabBasics {
		t.Errorf("expected activeTab=Basics on title error, got %d", m.activeTab)
	}
	if m.focusIndex != 0 {
		t.Errorf("expected focusIndex=0 on title error, got %d", m.focusIndex)
	}
}

func TestTaskForm_Save_InvalidDueDate_SetsErrorJumpsToSchedule(t *testing.T) {
	m := newTestTaskForm()
	m.draft.title = "valid title"
	m.titleInput.SetValue("valid title")
	m.draft.dueInput = "not-a-date-xyz"
	m.dueTextInput.SetValue("not-a-date-xyz")
	cmd := m.handleSave()
	if cmd != nil {
		t.Error("expected nil Cmd on due date parse failure")
	}
	if m.err == nil {
		t.Error("expected m.err to be set after bad due date")
	}
	if m.activeTab != formTabSchedule {
		t.Errorf("expected activeTab=Schedule on due-date error, got %d", m.activeTab)
	}
	if m.focusIndex != 0 {
		t.Errorf("expected focusIndex=0 on due-date error, got %d", m.focusIndex)
	}
}

func TestTaskForm_Save_CommitsTextInputsBeforeSave(t *testing.T) {
	m := newTestTaskForm()
	m.titleInput.SetValue("committed title")
	m.handleSave()
	if m.draft.title != "committed title" {
		t.Errorf("handleSave should commit titleInput; got %q", m.draft.title)
	}
}

func TestTaskForm_Save_ClearsDueDateOnNoneInput(t *testing.T) {
	m := newTestTaskForm()
	m.draft.title = "some task"
	m.draft.dueInput = "none"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.dueDate != nil {
		t.Error("parseDraftDueDate('none') should clear dueDate")
	}
}

func TestTaskForm_Save_ParsesDueDateFromInput(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "2026-06-15"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error parsing ISO date: %v", err)
	}
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate to be set")
	}
	if m.draft.dueDate.Year() != 2026 || int(m.draft.dueDate.Month()) != 6 || m.draft.dueDate.Day() != 15 {
		t.Errorf("parsed wrong date: got %v", m.draft.dueDate)
	}
}

func TestTaskForm_Save_ParsesDueDateWithTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "2026-06-15T14:30"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error parsing date+time: %v", err)
	}
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate to be set")
	}
	if !m.draft.hasTime {
		t.Error("expected hasTime=true for input with time component")
	}
	if m.draft.dueDate.Hour() != 14 || m.draft.dueDate.Minute() != 30 {
		t.Errorf("expected 14:30, got %02d:%02d", m.draft.dueDate.Hour(), m.draft.dueDate.Minute())
	}
}

func TestTaskForm_Save_ParsesDueDateWithTimeSuffix(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "tomorrow 5pm"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error parsing 'tomorrow 5pm': %v", err)
	}
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate to be set")
	}
	if !m.draft.hasTime {
		t.Error("expected hasTime=true for 'tomorrow 5pm'")
	}
	if m.draft.dueDate.Hour() != 17 {
		t.Errorf("expected hour=17, got %d", m.draft.dueDate.Hour())
	}
}

func TestTaskForm_Save_DateOnlyDoesNotSetHasTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "2026-06-15"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.hasTime {
		t.Error("expected hasTime=false for date-only input")
	}
}

func TestTaskForm_Save_ClearDueDateClearsHasTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.hasTime = true
	m.draft.dueInput = "none"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.dueDate != nil {
		t.Error("expected dueDate=nil")
	}
	if m.draft.hasTime {
		t.Error("expected hasTime=false after clearing due date")
	}
}

func TestTaskForm_Save_BuildAttrs_BugType(t *testing.T) {
	m := newBugTaskForm()
	m.draft.bugAttrs.Severity = models.BugSeverityMajor
	m.draft.bugAttrs.Reproducible = models.BugReproducibleYes
	bug, idea, task, story, epic := m.buildDraftTaskAttrs()
	if bug == nil {
		t.Fatal("expected bugAttrs to be non-nil")
	}
	if bug.Severity != models.BugSeverityMajor {
		t.Errorf("expected Severity=major, got %v", bug.Severity)
	}
	if idea != nil || task != nil || story != nil || epic != nil {
		t.Error("other attrs should be nil for Bug type")
	}
}

func TestTaskForm_Save_BuildAttrs_EmptyBug_ReturnsNil(t *testing.T) {
	m := newBugTaskForm()
	bug, _, _, _, _ := m.buildDraftTaskAttrs()
	if bug != nil {
		t.Error("expected bugAttrs=nil when bug attrs are empty")
	}
}

func TestTaskForm_Save_CreateHappyPath_EmitsTaskCreatedMsg(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.title = "My New Task"
	m.titleInput.SetValue("My New Task")
	m.draft.priority = 2
	m.draft.tags = []string{"go", "tui"}
	m.draft.desc = "Task description"

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd on successful create; err=%v", m.err)
	}
	msg := cmd()
	created, ok := msg.(TaskCreatedMsg)
	if !ok {
		t.Fatalf("expected TaskCreatedMsg, got %T", msg)
	}
	if created.Task == nil {
		t.Fatal("TaskCreatedMsg.Task should be non-nil")
	}
	if created.Task.Title != "My New Task" {
		t.Errorf("expected Title='My New Task', got %q", created.Task.Title)
	}
	if created.Task.Priority != 2 {
		t.Errorf("expected Priority=2, got %d", created.Task.Priority)
	}
	if len(created.Task.Tags) != 2 {
		t.Errorf("expected 2 tags, got %v", created.Task.Tags)
	}
	if created.Task.Description != "Task description" {
		t.Errorf("expected Description='Task description', got %q", created.Task.Description)
	}
}

func TestTaskForm_Save_CreateInitializesCollectionFields(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.title = "Field init test"
	m.titleInput.SetValue("Field init test")

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	created := cmd().(TaskCreatedMsg)
	task := created.Task

	if task.DependsOn == nil {
		t.Error("DependsOn should be initialized to empty slice, got nil")
	}
	if task.RelatedTo == nil {
		t.Error("RelatedTo should be initialized to empty slice, got nil")
	}
	if task.ScoreDimensions == nil {
		t.Error("ScoreDimensions should be initialized to empty map, got nil")
	}
	if task.Children == nil {
		t.Error("Children should be initialized to empty slice, got nil")
	}
}

func TestTaskForm_Save_CreateSubtasksInitializeCollectionFields(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.taskType = models.TaskTypeEpic
	m.draft.title = "Parent Epic"
	m.titleInput.SetValue("Parent Epic")
	m.draft.subtasks = []formSubtaskDraft{{title: "child one", taskType: models.TaskTypeTask}}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	_ = cmd()

	var subtask *models.Task
	for _, t2 := range m.project.Tasks {
		if t2.Title == "child one" {
			subtask = t2
			break
		}
	}
	if subtask == nil {
		t.Fatal("subtask 'child one' not found in project tasks")
	}

	if subtask.DependsOn == nil {
		t.Error("subtask DependsOn should be initialized to empty slice, got nil")
	}
	if subtask.RelatedTo == nil {
		t.Error("subtask RelatedTo should be initialized to empty slice, got nil")
	}
	if subtask.ScoreDimensions == nil {
		t.Error("subtask ScoreDimensions should be initialized to empty map, got nil")
	}
	if subtask.Children == nil {
		t.Error("subtask Children should be initialized to empty slice, got nil")
	}
}

func TestTaskForm_Save_CreateHappyPath_AddsChecklistItems(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.title = "Task with checklist"
	m.titleInput.SetValue("Task with checklist")
	m.draft.checklist = []models.ChecklistItem{
		{Text: "step one", Done: false},
		{Text: "  ", Done: false},
		{Text: "step two", Done: true},
	}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	created := cmd().(TaskCreatedMsg)
	if len(created.Task.Checklist) != 2 {
		t.Errorf("expected 2 checklist items (blank filtered), got %d", len(created.Task.Checklist))
	}
	if created.Task.Checklist[0].Text != "step one" {
		t.Errorf("expected first item 'step one', got %q", created.Task.Checklist[0].Text)
	}
	if !created.Task.Checklist[1].Done {
		t.Error("expected second checklist item Done=true")
	}
}

func TestTaskForm_Save_CreateHappyPath_AddsSubtasksForEpic(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.taskType = models.TaskTypeEpic
	m.draft.title = "Parent Epic"
	m.titleInput.SetValue("Parent Epic")
	m.draft.subtasks = []formSubtaskDraft{
		{title: "child one", taskType: models.TaskTypeTask},
		{title: "", taskType: models.TaskTypeTask},
		{title: "child two", taskType: models.TaskTypeBug},
	}

	initialCount := len(m.project.Tasks)
	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	got := len(m.project.Tasks) - initialCount
	if got != 3 {
		t.Errorf("expected 3 new tasks (1 epic + 2 subtasks), got %d", got)
	}
}

func TestTaskForm_Save_EditHappyPath_EmitsTaskUpdatedMsg(t *testing.T) {
	m, cleanup := newSaveableEditForm(t)
	defer cleanup()

	m.draft.title = "Updated Title"
	m.titleInput.SetValue("Updated Title")
	m.draft.priority = 1
	m.draft.desc = "Updated description"

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd on successful edit; err=%v", m.err)
	}
	msg := cmd()
	updated, ok := msg.(TaskUpdatedMsg)
	if !ok {
		t.Fatalf("expected TaskUpdatedMsg, got %T", msg)
	}
	if updated.Task == nil {
		t.Fatal("TaskUpdatedMsg.Task should be non-nil")
	}
}

func TestTaskForm_Save_EditHappyPath_MutatesTaskInProject(t *testing.T) {
	m, cleanup := newSaveableEditForm(t)
	defer cleanup()

	taskID := m.task.ID
	m.draft.title = "Mutated Title"
	m.titleInput.SetValue("Mutated Title")
	m.draft.priority = 5
	m.draft.tags = []string{"mutated"}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	_ = cmd()

	var found *models.Task
	for _, t2 := range m.project.Tasks {
		if t2.ID == taskID {
			found = t2
			break
		}
	}
	if found == nil {
		t.Fatal("task not found in project after edit save")
	}
	if found.Title != "Mutated Title" {
		t.Errorf("expected Title='Mutated Title', got %q", found.Title)
	}
	if found.Priority != 5 {
		t.Errorf("expected Priority=5, got %d", found.Priority)
	}
}

func TestTaskForm_Save_EditFiltersBlankChecklistItems(t *testing.T) {
	m, cleanup := newSaveableEditForm(t)
	defer cleanup()

	m.draft.title = "Edited task"
	m.titleInput.SetValue("Edited task")
	m.draft.checklist = []models.ChecklistItem{
		{Text: "keep me", Done: false},
		{Text: "   ", Done: false},
		{Text: "", Done: true},
		{Text: "also keep", Done: true},
	}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	msg := cmd()
	updated, ok := msg.(TaskUpdatedMsg)
	if !ok {
		t.Fatalf("expected TaskUpdatedMsg, got %T", msg)
	}
	if len(updated.Task.Checklist) != 2 {
		t.Errorf("expected 2 checklist items (blanks filtered), got %d", len(updated.Task.Checklist))
	}
	if updated.Task.Checklist[0].Text != "keep me" {
		t.Errorf("expected first item 'keep me', got %q", updated.Task.Checklist[0].Text)
	}
	if updated.Task.Checklist[1].Text != "also keep" {
		t.Errorf("expected second item 'also keep', got %q", updated.Task.Checklist[1].Text)
	}
}

func TestTaskForm_Children_ChecklistEdit_CommitBlank_RemovesItem(t *testing.T) {
	m := newTestTaskForm()
	m.draft.checklist = []models.ChecklistItem{{Text: "existing item"}}
	m.checklistExpanded = true
	m = navigateToChildrenTab(m)
	m.focusIndex = 1

	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayChecklistEdit {
		t.Fatalf("expected checklist edit overlay, got %v", m.activeOverlay)
	}

	m.checklistItemInput.SetValue("")
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))

	if m.activeOverlay != formOverlayNone {
		t.Error("overlay should be dismissed after Enter")
	}
	if len(m.draft.checklist) != 0 {
		t.Errorf("confirming blank text should remove the item; got %d items: %v", len(m.draft.checklist), m.draft.checklist)
	}
}
