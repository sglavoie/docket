package taskform

import (
	"time"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// formScheduleLabelWidth is the fixed label column width for all Schedule-tab rows.
const formScheduleLabelWidth = 16

// renderScheduleTab keeps the real deadline independent from the paired
// check-in schedule. The latter two rows both open the shared schedule editor,
// so the form and existing-task action cannot drift on parsing or repeat rules.
func (m *TaskFormModel) renderScheduleTab(b *formBodyBuilder, bodyWidth int) {
	const lw = formScheduleLabelWidth
	valueWidth := formValueWidth(bodyWidth, lw)

	// 0: Due — inline textinput when focused, plain text otherwise.
	b.field(0)
	b.WriteString(chrome.RenderFormLabelWithWidth("Due:", lw, m.focusIndex == 0))
	if m.focusIndex == 0 {
		m.dueTextInput.SetWidth(formInputWidth(formDueInputWidth, valueWidth))
		theme.ApplyThemedTextInput(&m.dueTextInput)
		b.WriteString(m.dueTextInput.View())
	} else {
		val := m.draft.dueInput
		if val == "" && m.draft.dueDate == nil {
			b.WriteString(theme.MutedStyle.Render("(none)"))
		} else if val != "" {
			b.WriteString(textfit.TruncateToVisualWidth(val, valueWidth))
		} else {
			b.WriteString(scheduleDueDateDisplay(m.draft.dueDate))
		}
	}
	b.WriteString("\n")

	// 1: Next check-in — the shared editor owns both this row and Repeat.
	b.field(1)
	b.WriteString(chrome.RenderFormLabelWithWidth("Next check-in:", lw, m.focusIndex == 1))
	if m.draft.checkIn.Next == nil {
		b.WriteString(theme.MutedStyle.Render("(none)"))
	} else {
		b.WriteString(m.draft.checkIn.Next.Format(time.DateOnly))
	}
	b.WriteString("\n")

	// 2: Repeat — opening from here starts the shared editor on its repeat row.
	b.field(2)
	b.WriteString(chrome.RenderFormLabelWithWidth("Repeat:", lw, m.focusIndex == 2))
	if m.draft.checkIn.Next == nil {
		b.WriteString(theme.MutedStyle.Render("Off"))
	} else {
		b.WriteString(repeatDisplay(m.draft.checkIn.RepeatDays))
	}
	b.WriteString("\n")
	m.renderRelationField(b, 3, formRelationDefer, bodyWidth)
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
