package taskform

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func TestTaskForm_ScheduleTab_RendersLabels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m)
	view := m.View().Content
	for _, label := range []string{"Due:", "Next check-in:", "Repeat:"} {
		if !strings.Contains(view, label) {
			t.Errorf("Schedule tab should contain %s; got:\n%s", label, view)
		}
	}
	if strings.Contains(view, "Recurrence:") {
		t.Errorf("Schedule tab still exposes recurrence; got:\n%s", view)
	}
}

func TestTaskFormScheduleCheckInRowsUseTheSharedEditor(t *testing.T) {
	m := navigateToScheduleField(newTestTaskForm())
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	if m.focusIndex != 1 {
		t.Fatalf("focus after Due = %d, want Next check-in", m.focusIndex)
	}
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.activeOverlay != formOverlayCheckIn || m.checkInEditor == nil || m.checkInEditor.focus != checkInFieldNext {
		t.Fatalf("Next check-in opened overlay=%v editor=%#v", m.activeOverlay, m.checkInEditor)
	}

	// Select today's date in the shared editor, then save its draft into the
	// creation form. No due date is involved in this path.
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m = sendFormKey(m, syntheticCtrlKey('s'))
	want := time.Now().Format(time.DateOnly)
	if m.activeOverlay != formOverlayNone || m.draft.checkIn.Next == nil || m.draft.checkIn.Next.Format(time.DateOnly) != want {
		t.Fatalf("applied schedule = %+v overlay=%v, want %s", m.draft.checkIn, m.activeOverlay, want)
	}
	if m.draft.dueDate != nil || m.draft.dueInput != "" {
		t.Fatalf("check-in editor changed due = %v / %q", m.draft.dueDate, m.draft.dueInput)
	}

	m.focusIndex = 2
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if m.checkInEditor == nil || m.checkInEditor.focus != checkInFieldRepeat {
		t.Fatalf("Repeat row did not open the shared editor on Repeat: %#v", m.checkInEditor)
	}
}

func TestTaskFormScheduleDueShortcutDoesNotCollideWithCheckInRows(t *testing.T) {
	m := navigateToScheduleField(newTestTaskForm())
	active := func() bool {
		for _, entry := range m.buildKeyMap().Entries {
			if entry.Binding != nil && entry.Binding.Key == "ctrl+d" {
				return entry.Binding.Gate == nil || entry.Binding.Gate()
			}
		}
		return false
	}
	if !active() {
		t.Fatal("Due row does not activate its preset shortcut")
	}
	for _, focus := range []int{1, 2} {
		m.focusIndex = focus
		if active() {
			t.Errorf("schedule field %d activates Due's Ctrl+D shortcut", focus)
		}
		m = sendFormKey(m, syntheticCtrlKey('d'))
		if m.activeOverlay != formOverlayNone || m.duePicker != nil {
			t.Fatalf("Ctrl+D on schedule field %d opened Due: overlay=%v", focus, m.activeOverlay)
		}
	}
}

func TestTaskFormCheckInOverlayCancelPreservesCreationDraft(t *testing.T) {
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	m := navigateToScheduleField(newTestTaskForm())
	m.draft.checkIn = models.CheckInSchedule{Next: &day, RepeatDays: 14}
	m.focusIndex = 1
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	m.checkInEditor.Restore(operation.OperationCheckInValue{DateInput: "tomorrow", RepeatInput: "30"}, "")
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))
	if m.activeOverlay != formOverlayNone || m.draft.checkIn.Next == nil || !m.draft.checkIn.Next.Equal(day) || m.draft.checkIn.RepeatDays != 14 {
		t.Fatalf("cancel changed creation schedule: %+v", m.draft.checkIn)
	}
}

func TestTaskForm_ScheduleTab_Due_ShowsPlaceholder_WhenEmpty(t *testing.T) {
	m := newTestTaskForm()
	// The contextual help pane is only shown two-column when the body, a gap,
	// and the pane all fit within the terminal width; widen so it renders.
	m.SetSize(100, 24)
	m = navigateToScheduleField(m)
	view := m.View().Content
	if !strings.Contains(view, "Ctrl+D") || !strings.Contains(view, "presets") {
		t.Errorf("Due field help pane should show 'Ctrl+D' and 'presets' when focused; got:\n%s", view)
	}
}

func TestTaskForm_ScheduleTab_Due_TypedValue_SetsHasChanges(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m)
	m = sendFormKey(m, syntheticKey('t'))
	if !m.hasChanges {
		t.Error("typing in Due field should set hasChanges=true")
	}
}

func TestTaskForm_ScheduleTab_Due_DKey_OpensDuePicker(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m)
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
	m = navigateToScheduleField(m)
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
	m = navigateToScheduleField(m)
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
	m = navigateToScheduleField(m)
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
	m = navigateToScheduleField(m)
	m = sendFormKey(m, syntheticCtrlKey('d'))
	view := m.View().Content
	if !strings.Contains(view, "Set Due Date") {
		t.Errorf("view should show 'Set Due Date' when due picker is active; got:\n%s", view)
	}
}

func TestTaskForm_ScheduleTab_NLDateRoundTrip(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToScheduleField(m)
	m = sendFormKey(m, syntheticCtrlKey('d'))
	m = sendFormKey(m, syntheticKey('m'))
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate set after picking tomorrow")
	}
	if m.draft.dueInput == "" {
		t.Error("expected dueInput non-empty after picking tomorrow")
	}
}
