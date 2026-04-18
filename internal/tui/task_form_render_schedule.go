package tui

import (
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// formScheduleLabelWidth is the fixed label column width for all Schedule-tab rows.
// "Recurrence:" (11 chars) is the widest; +1 pad = 12.
const formScheduleLabelWidth = 12

// renderScheduleTab renders the Schedule tab: Due (inline textinput) and Recurrence.
func (m *TaskFormModel) renderScheduleTab() string {
	var b strings.Builder
	const lw = formScheduleLabelWidth

	// 0: Due — inline textinput when focused, plain text otherwise.
	b.WriteString(renderFormLabelWithWidth("Due:", lw, m.focusIndex == 0))
	if m.focusIndex == 0 {
		applyThemedTextInput(&m.dueTextInput)
		b.WriteString(m.dueTextInput.View())
		b.WriteString(MutedStyle.Render("  d: presets"))
	} else {
		val := m.draft.dueInput
		if val == "" && m.draft.dueDate == nil {
			b.WriteString(MutedStyle.Render("(none)"))
		} else if val != "" {
			b.WriteString(val)
		} else {
			b.WriteString(scheduleDueDateDisplay(m.draft.dueDate))
		}
		b.WriteString(MutedStyle.Render("  Enter · d"))
	}
	b.WriteString("\n")

	// 1: Recurrence — read-only row; Enter opens RecurrenceEditorModel.
	b.WriteString(renderFormLabelWithWidth("Recurrence:", lw, m.focusIndex == 1))
	{
		val := recurrenceDisplay(m.draft.recurrence)
		if m.focusIndex == 1 {
			b.WriteString(SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString(MutedStyle.Render("  Enter"))
	b.WriteString("\n")

	return b.String()
}

// recurrenceDisplay returns a human-readable label for a recurrence pattern.
func recurrenceDisplay(r *models.RecurrencePattern) string {
	if r == nil {
		return MutedStyle.Render("(none)")
	}
	return r.String()
}

// scheduleDueDateDisplay formats a due date for display in the Schedule tab row.
func scheduleDueDateDisplay(d *time.Time) string {
	if d == nil {
		return MutedStyle.Render("(none)")
	}
	return d.Format("2006-01-02")
}
