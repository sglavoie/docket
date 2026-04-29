package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// formScheduleLabelWidth is the fixed label column width for all Schedule-tab rows.
// "Estimated Days:" (15 chars) is the widest; +1 pad = 16.
const formScheduleLabelWidth = 16

// renderScheduleTab renders the Schedule tab: Due (inline textinput) and Recurrence.
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

	// 1: Start Date — inline textinput when focused, plain date otherwise.
	b.WriteString(renderFormLabelWithWidth("Start Date:", lw, m.focusIndex == 1))
	if m.focusIndex == 1 {
		applyThemedTextInput(&m.startDateTextInput)
		b.WriteString(m.startDateTextInput.View())
	} else {
		val := m.draft.startDateInput
		if val == "" && m.draft.startDate == nil {
			b.WriteString(MutedStyle.Render("(none)"))
		} else if val != "" {
			b.WriteString(val)
		} else {
			b.WriteString(scheduleDueDateDisplay(m.draft.startDate))
		}
	}
	b.WriteString("\n")

	// 2: Est. Days — inline textinput when focused, plain number otherwise.
	b.WriteString(renderFormLabelWithWidth("Estimated Days:", lw, m.focusIndex == 2))
	if m.focusIndex == 2 {
		applyThemedTextInput(&m.estimatedDaysTextInput)
		b.WriteString(m.estimatedDaysTextInput.View())
	} else {
		if m.draft.estimatedDays == 0 {
			b.WriteString(MutedStyle.Render("(none)"))
		} else {
			b.WriteString(fmt.Sprintf("%g", m.draft.estimatedDays))
		}
	}
	b.WriteString("\n")

	// 3: Recurrence — read-only row; Enter opens RecurrenceEditorModel.
	b.WriteString(renderFormLabelWithWidth("Recurrence:", lw, m.focusIndex == 3))
	{
		val := recurrenceDisplay(m.draft.recurrence)
		if m.focusIndex == 3 {
			b.WriteString(SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
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
