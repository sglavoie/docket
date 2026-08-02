package tui

import (
	"strings"
	"time"
)

// formScheduleLabelWidth is the fixed label column width for all Schedule-tab rows.
const formScheduleLabelWidth = 12

// renderScheduleTab renders the retained Due field.
func (m *TaskFormModel) renderScheduleTab() string {
	var b strings.Builder
	const lw = formScheduleLabelWidth

	// 0: Due — inline textinput when focused, plain text otherwise.
	b.WriteString(renderFormLabelWithWidth("Due:", lw, m.focusIndex == 0))
	if m.focusIndex == 0 {
		applyThemedTextInput(&m.dueTextInput)
		b.WriteString(m.dueTextInput.View())
	} else {
		val := m.draft.dueInput
		if val == "" && m.draft.dueDate == nil {
			b.WriteString(MutedStyle.Render("(none)"))
		} else if val != "" {
			b.WriteString(val)
		} else {
			b.WriteString(scheduleDueDateDisplay(m.draft.dueDate))
		}
	}
	b.WriteString("\n")
	return b.String()
}

// scheduleDueDateDisplay formats a due date for display in the Schedule tab row.
func scheduleDueDateDisplay(d *time.Time) string {
	if d == nil {
		return MutedStyle.Render("(none)")
	}
	return d.Format("2006-01-02")
}
