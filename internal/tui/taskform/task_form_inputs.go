package taskform

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
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
	m.focusContextInput()
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

// formContextInputWidth and formDueInputWidth are the widths those two inline
// fields would take given the room. The row that renders them resets the width
// from its own budget every frame, so these are the opening preference rather
// than the field's size; the value and the caret survive that resize, which is
// what lets the width follow the terminal without the edit noticing.
const (
	formContextInputWidth = 40
	formDueInputWidth     = 40
)

// initContextInput creates and configures the context textinput with an
// optional initial value. An empty value is the field's cleared state, and the
// bound is the durable tag length the context shares rather than a second
// literal that could truncate a value the canonical writer would have accepted.
func (m *TaskFormModel) initContextInput(value string) {
	m.contextInput = textinput.New()
	m.contextInput.Placeholder = "client-acme (empty clears)"
	m.contextInput.CharLimit = models.MaxTagLength
	m.contextInput.SetWidth(formContextInputWidth)
	theme.ApplyThemedTextInput(&m.contextInput)
	if value != "" {
		m.contextInput.SetValue(value)
	}
}

// focusContextInput focuses the context textinput when the expanded Basics tab
// has field 7 active. The collapsed Basics tab does not render the row, so it
// never focuses the input.
func (m *TaskFormModel) focusContextInput() {
	if m.activeTab == formTabBasics && !m.collapsedBasics && m.focusIndex == formBasicsContextField {
		m.contextInput.Focus()
	} else {
		m.contextInput.Blur()
	}
}

// commitContextInput syncs the contextInput value back into the draft.
func (m *TaskFormModel) commitContextInput() {
	m.draft.context = strings.TrimSpace(m.contextInput.Value())
	m.contextInput.Blur()
}

// initDueInput creates and configures the due date textinput with an optional initial value.
func (m *TaskFormModel) initDueInput(value string) {
	m.dueTextInput = textinput.New()
	m.dueTextInput.Placeholder = "e.g. tomorrow"
	m.dueTextInput.Prompt = ""
	m.dueTextInput.CharLimit = 40
	m.dueTextInput.SetWidth(formDueInputWidth)
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
