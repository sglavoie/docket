package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskForm_ScheduleTab_RendersLabels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	view := m.View().Content
	for _, label := range []string{"Due:", "Recurrence:"} {
		if !strings.Contains(view, label) {
			t.Errorf("Schedule tab should contain label %q; got:\n%s", label, view)
		}
	}
}

func TestTaskForm_ScheduleTab_Due_ShowsPlaceholder_WhenEmpty(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	view := m.View().Content
	if !strings.Contains(view, "Ctrl+D") || !strings.Contains(view, "presets") {
		t.Errorf("Due field help pane should show 'Ctrl+D' and 'presets' when focused; got:\n%s", view)
	}
}

func TestTaskForm_ScheduleTab_Due_TypedValue_SetsHasChanges(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticKey('t'))
	if !m.hasChanges {
		t.Error("typing in Due field should set hasChanges=true")
	}
}

func TestTaskForm_ScheduleTab_Due_DKey_OpensDuePicker(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d'))
	if m.activeOverlay != formOverlayDue {
		t.Errorf("expected formOverlayDue, got %v", m.activeOverlay)
	}
	if m.duePicker == nil {
		t.Error("expected duePicker to be non-nil")
	}
}

func TestTaskForm_ScheduleTab_DuePicker_TodayKey_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d'))
	m = sendFormKey(m, syntheticKey('t'))
	if m.draft.dueDate == nil {
		t.Error("expected draft.dueDate to be set after picking today")
	}
	if m.draft.dueInput == "" {
		t.Error("expected draft.dueInput to be non-empty after picking today")
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after due date picked")
	}
	if m.activeOverlay != formOverlayNone {
		t.Error("expected overlay dismissed after selection")
	}
}

func TestTaskForm_ScheduleTab_DuePicker_ClearKey_ClearsDraft(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m.draft.dueInput = "2026-01-15"
	m = sendFormKey(m, syntheticCtrlKey('d'))
	m = sendFormKey(m, syntheticKey('0'))
	if m.draft.dueDate != nil {
		t.Error("expected draft.dueDate=nil after clear")
	}
	if m.draft.dueInput != "" {
		t.Errorf("expected draft.dueInput empty after clear, got %q", m.draft.dueInput)
	}
}

func TestTaskForm_ScheduleTab_DuePicker_EscCancels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.activeOverlay != formOverlayNone {
		t.Error("expected overlay dismissed after Esc")
	}
	if m.draft.dueDate != nil {
		t.Error("cancel should not set dueDate")
	}
}

func TestTaskForm_ScheduleTab_EnterOnRecurrence_EmitsShowRecurrenceEditorMsg(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 3)
	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected Cmd from Enter on Recurrence, got nil")
	}
	msg := cmd()
	if _, ok := msg.(ShowRecurrenceEditorMsg); !ok {
		t.Errorf("expected ShowRecurrenceEditorMsg, got %T", msg)
	}
}

func TestTaskForm_ScheduleTab_RecurrenceEditorDoneMsg_UpdatesDraft(t *testing.T) {
	m := newTestTaskForm()
	pattern := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	m = sendFormMsg(m, RecurrenceEditorDoneMsg{Pattern: pattern, Canceled: false})
	if m.draft.recurrence == nil {
		t.Fatal("expected draft.recurrence to be set")
	}
	if m.draft.recurrence.Interval != 1 {
		t.Errorf("expected recurrence interval=1, got %d", m.draft.recurrence.Interval)
	}
	if !m.hasChanges {
		t.Error("expected hasChanges=true after recurrence updated")
	}
}

func TestTaskForm_ScheduleTab_RecurrenceEditorDoneMsg_Canceled_NoChange(t *testing.T) {
	m := newTestTaskForm()
	m.draft.recurrence = nil
	m = sendFormMsg(m, RecurrenceEditorDoneMsg{Pattern: &models.RecurrencePattern{Interval: 7}, Canceled: true})
	if m.draft.recurrence != nil {
		t.Error("canceled RecurrenceEditorDoneMsg should not update draft")
	}
}

func TestTaskForm_ScheduleTab_DuePicker_Render_ContainsTitleText(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d'))
	view := m.View().Content
	if !strings.Contains(view, "Set Due Date") {
		t.Errorf("view should show 'Set Due Date' when due picker is active; got:\n%s", view)
	}
}

func TestTaskForm_ScheduleTab_NLDateRoundTrip(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m, 0)
	m = sendFormKey(m, syntheticCtrlKey('d'))
	m = sendFormKey(m, syntheticKey('m'))
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate set after picking tomorrow")
	}
	if m.draft.dueInput == "" {
		t.Error("expected dueInput non-empty after picking tomorrow")
	}
}

func TestTaskForm_Edit_PreservesHasTime(t *testing.T) {
	project := &models.Project{
		ID:   "test",
		Name: "Test",
		Path: t.TempDir(),
	}
	dueDate := time.Date(2026, 6, 15, 14, 30, 0, 0, time.Local)
	task := &models.Task{
		ID:      "task-1",
		ShortID: 1,
		Type:    models.TaskTypeTask,
		Title:   "Test task",
		Status:  models.StatusTodo,
		DueDate: &dueDate,
		HasTime: true,
	}
	project.Tasks = []*models.Task{task}

	m := NewTaskFormModelEdit(project, task, nil, nil)
	if !m.draft.hasTime {
		t.Error("expected draft.hasTime=true when editing task with HasTime=true")
	}
	if m.draft.dueInput != "2026-06-15T14:30" {
		t.Errorf("expected dueInput='2026-06-15T14:30', got %q", m.draft.dueInput)
	}
}

func TestTaskForm_DueDatePresetPickerClearsHasTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.hasTime = true
	tomorrow := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	pickedMsg := TaskFormDueDatePickedMsg{Date: &tomorrow}
	m.activeOverlay = formOverlayDue
	m.duePicker = newFormDueDatePicker(nil)
	m.applyFormOverlayResult(pickedMsg)
	if m.draft.hasTime {
		t.Error("expected hasTime=false after preset picker selection")
	}
	if m.draft.dueDate == nil {
		t.Error("expected dueDate to be set")
	}
}

func TestDateInput_PrefillPreservesHasTime(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	dueWithTime := time.Date(2026, 6, 15, 14, 30, 0, 0, time.Local)

	task := &models.Task{
		ID:      "test-due-time",
		ShortID: 99,
		Title:   "Test task",
		DueDate: &dueWithTime,
		HasTime: true,
	}

	m := NewDateInputModel(task, project)
	got := m.input.Value()
	want := "2026-06-15T14:30"
	if got != want {
		t.Errorf("DateInputModel prefill with HasTime=true: got %q, want %q", got, want)
	}
}

func TestDateInput_PrefillDateOnly(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	due := time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)

	task := &models.Task{
		ID:      "test-due-date",
		ShortID: 98,
		Title:   "Test task",
		DueDate: &due,
		HasTime: false,
	}

	m := NewDateInputModel(task, project)
	got := m.input.Value()
	want := "2026-06-15"
	if got != want {
		t.Errorf("DateInputModel prefill with HasTime=false: got %q, want %q", got, want)
	}
}
