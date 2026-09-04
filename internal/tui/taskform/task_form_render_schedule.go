package taskform

import (
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// formScheduleLabelWidth is the fixed label column width for all Schedule-tab rows.
const formScheduleLabelWidth = 12

// renderScheduleTab renders the retained Due field.
func (m *TaskFormModel) renderScheduleTab() string {
	var b strings.Builder
	const lw = formScheduleLabelWidth

	// 0: Due — inline textinput when focused, plain text otherwise.
	b.WriteString(chrome.RenderFormLabelWithWidth("Due:", lw, m.focusIndex == 0))
	if m.focusIndex == 0 {
		theme.ApplyThemedTextInput(&m.dueTextInput)
		b.WriteString(m.dueTextInput.View())
	} else {
		val := m.draft.dueInput
		if val == "" && m.draft.dueDate == nil {
			b.WriteString(theme.MutedStyle.Render("(none)"))
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
		return theme.MutedStyle.Render("(none)")
	}
	if !d.Equal(util.StartOfDay(*d)) {
		return d.Format("2006-01-02 15:04")
	}
	return d.Format("2006-01-02")
}
