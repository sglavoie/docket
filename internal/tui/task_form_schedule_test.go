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
	if !strings.Contains(view, "Due:") {
		t.Errorf("Schedule tab should contain Due; got:\n%s", view)
	}
	if strings.Contains(view, "Recurrence:") {
		t.Errorf("Schedule tab still exposes recurrence; got:\n%s", view)
	}
}

func TestTaskForm_ScheduleTab_Due_ShowsPlaceholder_WhenEmpty(t *testing.T) {
	m := newTestTaskForm()
	// The contextual help pane is only shown two-column when the body, a gap,
	// and the pane all fit within the terminal width; widen so it renders.
	m.SetSize(100, 24)
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

func TestDateInput_PrefillDateOnly(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	due := time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)

	task := &models.Task{
		ID:      "test-due-date",
		ShortID: 98,
		Title:   "Test task",
		DueDate: &due,
	}

	m := NewDateInputModel(task, project, "")
	got := m.input.Value()
	want := "2026-06-15"
	if got != want {
		t.Errorf("DateInputModel date-only prefill: got %q, want %q", got, want)
	}
}
