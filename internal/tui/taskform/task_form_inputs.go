package taskform

import (
	"time"

	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// initTitleInput creates and configures the title textarea with an optional initial value.
func (m *TaskFormModel) initTitleInput(value string) {
	factory := chrome.DefaultFormFieldFactory()
	m.titleInput = factory.NewTitleInputWithValue(value)
	// Focus by default (Basics tab, field 0 is selected on open)
	m.titleInput.Focus()
}

// syncInputFocus ensures all tab-specific text inputs have the correct focus state.
// Call this after any navigation change (tab switch, focus move).
func (m *TaskFormModel) syncInputFocus() {
	m.focusTitleInput()
	m.focusDueInput()
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
	m.dueTextInput.Placeholder = "tomorrow, +3d, fri 15:30, 2026-01-15 09:00"
	m.dueTextInput.CharLimit = 40
	m.dueTextInput.SetWidth(40)
	theme.ApplyThemedTextInput(&m.dueTextInput)
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

// initChecklistItemInput creates and configures the checklist item edit textinput.
func (m *TaskFormModel) initChecklistItemInput(value string) {
	m.checklistItemInput = textinput.New()
	m.checklistItemInput.Placeholder = "Checklist item text..."
	m.checklistItemInput.CharLimit = 200
	m.checklistItemInput.SetWidth(50)
	theme.ApplyThemedTextInput(&m.checklistItemInput)
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
	theme.ApplyThemedTextInput(&m.subtaskItemInput)
	if value != "" {
		m.subtaskItemInput.SetValue(value)
	}
}

// initAttachmentItemInput creates and configures the attachment value edit
// textinput. The limit is generous because a pasted URL or absolute path is
// routinely longer than a checklist item.
func (m *TaskFormModel) initAttachmentItemInput(value string) {
	m.attachmentItemInput = textinput.New()
	m.attachmentItemInput.Placeholder = "URL, path:line, or file path..."
	m.attachmentItemInput.CharLimit = 500
	m.attachmentItemInput.SetWidth(50)
	theme.ApplyThemedTextInput(&m.attachmentItemInput)
	if value != "" {
		m.attachmentItemInput.SetValue(value)
	}
}

// initCommentItemInput creates and configures the comment text edit textinput.
// It shares the attachment input's generous limit because a comment is free
// prose, not a label.
func (m *TaskFormModel) initCommentItemInput(value string) {
	m.commentItemInput = textinput.New()
	m.commentItemInput.Placeholder = "Comment text..."
	m.commentItemInput.CharLimit = 500
	m.commentItemInput.SetWidth(50)
	theme.ApplyThemedTextInput(&m.commentItemInput)
	if value != "" {
		m.commentItemInput.SetValue(value)
	}
}

// formatDateInput formats a due date for text inputs, appending the hh:mm clock
// when the due date carries an explicit (non-midnight) time so the edit field
// round-trips a timed due date.
func formatDateInput(value *time.Time) string {
	if value == nil {
		return ""
	}
	if !value.Equal(util.StartOfDay(*value)) {
		return value.Format("2006-01-02 15:04")
	}
	return value.Format("2006-01-02")
}
