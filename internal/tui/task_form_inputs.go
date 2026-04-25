package tui

import (
	"time"

	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/util"
)

// initTitleInput creates and configures the title textarea with an optional initial value.
func (m *TaskFormModel) initTitleInput(value string) {
	factory := DefaultFormFieldFactory()
	m.titleInput = factory.NewTitleInputWithValue(value)
	// Focus by default (Basics tab, field 0 is selected on open)
	m.titleInput.Focus()
}

// syncInputFocus ensures all tab-specific text inputs have the correct focus state.
// Call this after any navigation change (tab switch, focus move).
func (m *TaskFormModel) syncInputFocus() {
	m.focusTitleInput()
	m.focusDueInput()
	m.focusURLInput()
	m.focusSourceInput()
}

// focusTitleInput focuses the title textarea when field 0 is active.
func (m *TaskFormModel) focusTitleInput() {
	if m.activeTab == formTabBasics && m.focusIndex == 0 {
		m.titleInput.Focus()
	} else {
		m.titleInput.Blur()
	}
}

// initDueInput creates and configures the due date textinput with an optional initial value.
func (m *TaskFormModel) initDueInput(value string) {
	m.dueTextInput = textinput.New()
	m.dueTextInput.Placeholder = "tomorrow 5pm, +3d 14:30, fri, 2026-01-15"
	m.dueTextInput.CharLimit = 40
	m.dueTextInput.SetWidth(40)
	applyThemedTextInput(&m.dueTextInput)
	if value != "" {
		m.dueTextInput.SetValue(value)
	}
}

// focusDueInput focuses the due textinput when field 0 on the Schedule tab is active.
func (m *TaskFormModel) focusDueInput() {
	if m.activeTab == formTabSchedule && m.focusIndex == 0 {
		m.dueTextInput.Focus()
	} else {
		m.dueTextInput.Blur()
	}
}

// commitDueInput syncs the dueTextInput value back into the draft.
func (m *TaskFormModel) commitDueInput() {
	m.draft.dueInput = m.dueTextInput.Value()
	m.dueTextInput.Blur()
}

// initURLInput creates and configures the URL textinput with an optional initial value.
func (m *TaskFormModel) initURLInput(value string) {
	m.urlTextInput = textinput.New()
	m.urlTextInput.Placeholder = "https://..."
	m.urlTextInput.CharLimit = 500
	m.urlTextInput.SetWidth(50)
	applyThemedTextInput(&m.urlTextInput)
	if value != "" {
		m.urlTextInput.SetValue(value)
	}
}

// focusURLInput focuses the URL textinput when field 0 on the Details tab is active.
func (m *TaskFormModel) focusURLInput() {
	if m.activeTab == formTabDetails && m.focusIndex == 0 {
		m.urlTextInput.Focus()
	} else {
		m.urlTextInput.Blur()
	}
}

// commitURLInput syncs the URL textinput value back into the draft.
func (m *TaskFormModel) commitURLInput() {
	m.draft.url = m.urlTextInput.Value()
	m.urlTextInput.Blur()
}

// initSourceInput creates and configures the Source textinput with an optional initial value.
func (m *TaskFormModel) initSourceInput(value string) {
	m.sourceTextInput = textinput.New()
	m.sourceTextInput.Placeholder = "source reference..."
	m.sourceTextInput.CharLimit = 500
	m.sourceTextInput.SetWidth(50)
	applyThemedTextInput(&m.sourceTextInput)
	if value != "" {
		m.sourceTextInput.SetValue(value)
	}
}

// focusSourceInput focuses the Source textinput when field 1 on the Details tab is active.
func (m *TaskFormModel) focusSourceInput() {
	if m.activeTab == formTabDetails && m.focusIndex == 1 {
		m.sourceTextInput.Focus()
	} else {
		m.sourceTextInput.Blur()
	}
}

// commitSourceInput syncs the Source textinput value back into the draft.
func (m *TaskFormModel) commitSourceInput() {
	m.draft.source = m.sourceTextInput.Value()
	m.sourceTextInput.Blur()
}

// initChecklistItemInput creates and configures the checklist item edit textinput.
func (m *TaskFormModel) initChecklistItemInput(value string) {
	m.checklistItemInput = textinput.New()
	m.checklistItemInput.Placeholder = "Checklist item text..."
	m.checklistItemInput.CharLimit = 200
	m.checklistItemInput.SetWidth(50)
	applyThemedTextInput(&m.checklistItemInput)
	if value != "" {
		m.checklistItemInput.SetValue(value)
	}
}

// initSubtaskItemInput creates and configures the subtask title edit textinput.
func (m *TaskFormModel) initSubtaskItemInput(value string) {
	m.subtaskItemInput = textinput.New()
	m.subtaskItemInput.Placeholder = "Subtask title..."
	m.subtaskItemInput.CharLimit = 200
	m.subtaskItemInput.SetWidth(50)
	applyThemedTextInput(&m.subtaskItemInput)
	if value != "" {
		m.subtaskItemInput.SetValue(value)
	}
}

// formatDateInput formats a *time.Time as a "YYYY-MM-DD" string for text inputs.
// When hasTime is true, appends the time in "YYYY-MM-DDTHH:MM" format.
func formatDateInput(value *time.Time, hasTime bool) string {
	if value == nil {
		return ""
	}
	if hasTime {
		return value.Format(util.DateTimeDueFormat)
	}
	return value.Format("2006-01-02")
}
