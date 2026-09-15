package taskform

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// formSupportedWidths spans the two regimes the body has: below
// formBodyPreferredWidth the controls shorten, at and above it they are laid out
// exactly as they always were. The floor and the ceiling are both included
// because the arithmetic that fails at 60 and the arithmetic that fails at 68
// are different mistakes.
var formSupportedWidths = []int{formMinWidth, 64, formBodyPreferredWidth, 72, 100}

// TestTaskFormBodyFitsEverySupportedWidth is the width half of the session's
// done-when: no row of any tab, at any focus, may be wider than the terminal it
// is drawn into. A form that overflows is worse than one that refuses, because
// the terminal wraps the overflow and every row below it is then drawn in the
// wrong place.
func TestTaskFormBodyFitsEverySupportedWidth(t *testing.T) {
	for _, width := range formSupportedWidths {
		for _, tab := range []int{formTabBasics, formTabSchedule, formTabChildren, formTabExtras} {
			m := newLoadedTaskForm()
			m.SetSize(width, formMinHeight)
			m.activeTab = tab
			m.focusIndex = 0
			m.syncInputFocus()

			for focus := range m.fieldCountForActiveTab() {
				rendered := m.View().Content
				if strings.Contains(ansi.Strip(rendered), "Terminal too small") {
					t.Fatalf("width %d tab %d: the form refuses a size the app itself accepts", width, tab)
				}
				for _, line := range strings.Split(ansi.Strip(rendered), "\n") {
					if w := lipgloss.Width(line); w > width {
						t.Fatalf("width %d tab %d focus %d: row is %d columns: %q",
							width, tab, focus, w, line)
					}
				}
				m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
			}
		}
	}
}

// formOverlayOpeners is every overlay that opens inside the form. They are
// listed once so a tenth one joins the frame guarantee by being added here
// rather than by someone remembering to write a test for it.
func formOverlayOpeners() map[string]func(m *TaskFormModel) {
	return map[string]func(m *TaskFormModel){
		"priority": func(m *TaskFormModel) {
			m.activeOverlay = formOverlayPriority
			m.priorityPicker = newFormPriorityPicker(m.draft.priority)
		},
		"state": func(m *TaskFormModel) {
			m.activeOverlay = formOverlayState
			m.statePicker = newFormStatePicker(m.draft.state)
		},
		"flags": func(m *TaskFormModel) {
			m.activeOverlay = formOverlayFlags
			m.flagsPicker = newFormFlagsPicker(m.draft.pinned, m.draft.review)
		},
		"due": func(m *TaskFormModel) {
			m.activeOverlay = formOverlayDue
			m.duePicker = newFormDueDatePicker(m.draft.dueDate)
		},
		"checkIn": func(m *TaskFormModel) {
			m.activeOverlay = formOverlayCheckIn
			m.checkInEditor = NewCheckInEditorModel(
				operation.OperationExecutionRequest{},
				"A subtitle far longer than any narrow overlay could ever hold on one line",
				models.CheckInSchedule{},
				false,
			)
		},
		"checklistEdit": func(m *TaskFormModel) {
			m.activeOverlay = formOverlayChecklistEdit
			m.initChecklistItemInput("an item")
		},
		"subtaskEdit": func(m *TaskFormModel) {
			m.activeOverlay = formOverlaySubtaskEdit
			m.initSubtaskItemInput("a subtask")
		},
		"attachmentEdit": func(m *TaskFormModel) {
			m.activeOverlay = formOverlayAttachmentEdit
			m.initAttachmentItemInput("https://example.com/spec.pdf")
		},
		"commentEdit": func(m *TaskFormModel) {
			m.activeOverlay = formOverlayCommentEdit
			m.initCommentItemInput("a comment")
		},
	}
}

// TestTaskFormOverlaysKeepACompleteFrameAtEveryWidth is why an open overlay is
// the whole view rather than a block inside it. A frame missing its bottom
// border reads as a rendering bug and, on the list pickers, silently hides the
// options underneath it — which is the difference between a narrow terminal and
// a broken one.
func TestTaskFormOverlaysKeepACompleteFrameAtEveryWidth(t *testing.T) {
	for name, open := range formOverlayOpeners() {
		for _, width := range formSupportedWidths {
			m := newTestTaskForm()
			m.SetSize(width, formMinHeight)
			open(m)

			rendered := m.View().Content
			assertFitsWithin(t, rendered, width, formMinHeight)
			plain := ansi.Strip(rendered)
			if !strings.Contains(plain, "╭") || !strings.Contains(plain, "╰") {
				t.Fatalf("%s overlay lost part of its frame at width %d:\n%s", name, width, plain)
			}
		}
	}
}

// TestTaskFormStatePickerKeepsEveryStateAtTheFloor is the consequence that
// motivated the whole-view overlay: the list a picker offers is not decoration,
// so a state that cannot be seen is a state that cannot be chosen.
func TestTaskFormStatePickerKeepsEveryStateAtTheFloor(t *testing.T) {
	m := newTestTaskForm()
	m.SetSize(formMinWidth, formMinHeight)
	m.activeOverlay = formOverlayState
	m.statePicker = newFormStatePicker(m.draft.state)

	plain := ansi.Strip(m.View().Content)
	for _, state := range models.AllWorkflowStates() {
		if !strings.Contains(plain, string(state)) {
			t.Fatalf("state %q is unreachable at the floor:\n%s", state, plain)
		}
	}
}

// TestFormValueWidthChargesTheSelectionPadAtEveryWidth pins the one piece of the
// arithmetic that looks like dead weight: theme.SelectorOptionSelectedStyle pads
// a focused value by two columns, so a budget that ignored it would let a row
// grow the moment the focus landed on it.
func TestFormValueWidthChargesTheSelectionPadAtEveryWidth(t *testing.T) {
	const labelWidth = 13
	got := formValueWidth(formBodyPreferredWidth, labelWidth)
	want := formBodyPreferredWidth - labelWidth - formLabelMarkerWidth - formSelectedValuePadding
	if got != want {
		t.Fatalf("formValueWidth(%d, %d) = %d, want %d",
			formBodyPreferredWidth, labelWidth, got, want)
	}
	if narrow := formValueWidth(20, 16); narrow != formMinValueWidth {
		t.Fatalf("a body with nothing to spare must floor at formMinValueWidth, got %d", narrow)
	}
}

// TestFormInputWidthLeavesRoomForTextInputChrome guards the measured cost of a
// bubbles textinput, which renders three columns more than the width it is set
// to. Setting it to the whole budget is the natural mistake and overflows by
// exactly those three.
func TestFormInputWidthLeavesRoomForTextInputChrome(t *testing.T) {
	if got := formInputWidth(40, 20); got != 20-formTextInputChrome {
		t.Fatalf("formInputWidth(40, 20) = %d, want %d", got, 20-formTextInputChrome)
	}
	if got := formInputWidth(12, 80); got != 12 {
		t.Fatalf("a budget larger than the preference must not widen the input: %d", got)
	}
	if got := formInputWidth(40, 4); got != formMinValueWidth {
		t.Fatalf("formInputWidth must floor at formMinValueWidth, got %d", got)
	}
}

// TestFormFitHintDropsWholeEntries keeps the hint honest: half a key name is
// worse than no key name, and every key a hint sheds is still spelled in the
// tab's guidance footer.
func TestFormFitHintDropsWholeEntries(t *testing.T) {
	const hint = "Enter edit · Ctrl+D delete · Ctrl+J/K move"
	if got := formFitHint(hint, 100); got != hint {
		t.Fatalf("a hint that fits must be returned whole: %q", got)
	}
	got := formFitHint(hint, 28)
	if got != "Enter edit · Ctrl+D delete" {
		t.Fatalf("formFitHint(28) = %q, want the first two entries", got)
	}
	if got := formFitHint(hint, 4); got != "" {
		t.Fatalf("a hint with no room must vanish rather than truncate: %q", got)
	}
}

// TestFormOverlayInnerWidthNeverExceedsTheTerminal is the shared replacement for
// the per-picker frame arithmetic. The title floor is the interesting half: a
// picker whose title is wider than its preferred body used to widen past the
// terminal instead of letting the title be the thing that gives.
func TestFormOverlayInnerWidthNeverExceedsTheTerminal(t *testing.T) {
	for _, width := range formSupportedWidths {
		inner := formOverlayInnerWidth(width, max(width*2/3, 40), formOverlayTitleMinInner("Set Due Date"))
		if inner+formOverlayFrameColumns > width {
			t.Fatalf("width %d: inner %d plus frame %d exceeds the terminal",
				width, inner, formOverlayFrameColumns)
		}
	}
	if got := formOverlayInnerWidth(200, 40, 80); got != 80 {
		t.Fatalf("a title wider than the preference must raise the inner width, got %d", got)
	}
}

// TestTaskFormResizePreservesValuesAndCaret covers the other half of the brief:
// resizing is a layout event, so it may move where a control is drawn and never
// what it holds or where the caret sits inside it.
func TestTaskFormResizePreservesValuesAndCaret(t *testing.T) {
	m := newTestTaskForm()
	m.SetSize(100, 40)
	m.titleInput.SetValue("Ship the release notes")
	m.contextInput.SetValue("client-acme")
	m.contextInput.SetCursor(6)
	m.dueTextInput.SetValue("2026-03-04")
	m.dueTextInput.SetCursor(4)
	m.draft.checklist = []models.ChecklistItem{{Text: "Draft the announcement"}}

	for _, size := range [][2]int{{formMinWidth, formMinHeight}, {100, 40}, {formMinWidth, formMinHeight}} {
		m.SetSize(size[0], size[1])
		if got := m.titleInput.Value(); got != "Ship the release notes" {
			t.Fatalf("resize to %v lost the title: %q", size, got)
		}
		if got := m.contextInput.Value(); got != "client-acme" {
			t.Fatalf("resize to %v lost the context: %q", size, got)
		}
		if got := m.contextInput.Position(); got != 6 {
			t.Fatalf("resize to %v moved the context caret to %d", size, got)
		}
		if got := m.dueTextInput.Value(); got != "2026-03-04" {
			t.Fatalf("resize to %v lost the due text: %q", size, got)
		}
		if got := m.dueTextInput.Position(); got != 4 {
			t.Fatalf("resize to %v moved the due caret to %d", size, got)
		}
		if len(m.draft.checklist) != 1 {
			t.Fatalf("resize to %v lost the checklist draft", size)
		}
	}
}
