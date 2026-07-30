package tui

import (
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// formScheduleLabelWidth is the fixed label column width for all Schedule-tab rows.
const formScheduleLabelWidth = 12

// renderScheduleTab renders the Schedule tab: Due (inline textinput) and Recurrence.
func (m *TaskFormModel) renderScheduleTab() string {
	if m.collapsedSchedule {
		return m.renderScheduleTabCollapsed()
	}
	return m.renderScheduleTabExpanded()
}

// renderScheduleTabCollapsed renders the Schedule tab with only Due + expand row.
func (m *TaskFormModel) renderScheduleTabCollapsed() string {
	var b strings.Builder
	const lw = formScheduleLabelWidth

	// 0: Due
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

	// 1: Expand row
	expandLabel := MutedStyle.Render("+ more fields")
	if m.focusIndex == 1 {
		expandLabel = SelectorOptionSelectedStyle.Render("+ more fields")
	}
	b.WriteString(renderFormLabelWithWidth("", lw, false))
	b.WriteString(expandLabel)
	if m.focusIndex == 1 {
		b.WriteString(HelpStyle.Render("  (Enter to expand)"))
	}
	b.WriteString("\n")

	return b.String()
}

// renderScheduleTabExpanded renders all Schedule tab fields.
func (m *TaskFormModel) renderScheduleTabExpanded() string {
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
