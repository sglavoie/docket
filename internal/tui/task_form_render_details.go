package tui

import (
	"strings"
)

// formDetailsLabelWidth is the fixed label column width for all Details-tab rows.
// "Source:" (7 chars) is the widest; +1 pad = 8.
const formDetailsLabelWidth = 8

// renderDetailsTab renders the Details tab: URL, Source.
func (m *TaskFormModel) renderDetailsTab() string {
	var b strings.Builder
	const lw = formDetailsLabelWidth

	// 0: URL — inline textinput when focused.
	b.WriteString(renderFormLabelWithWidth("URL:", lw, m.focusIndex == 0))
	if m.focusIndex == 0 {
		applyThemedTextInput(&m.urlTextInput)
		b.WriteString(m.urlTextInput.View())
	} else {
		val := m.draft.url
		if val == "" {
			b.WriteString(MutedStyle.Render("(none)"))
		} else {
			b.WriteString(truncateToVisualWidth(val, 50))
		}
	}
	b.WriteString("\n")

	// 1: Source — inline textinput when focused.
	b.WriteString(renderFormLabelWithWidth("Source:", lw, m.focusIndex == 1))
	if m.focusIndex == 1 {
		applyThemedTextInput(&m.sourceTextInput)
		b.WriteString(m.sourceTextInput.View())
	} else {
		val := m.draft.source
		if val == "" {
			b.WriteString(MutedStyle.Render("(none)"))
		} else {
			b.WriteString(truncateToVisualWidth(val, 50))
		}
	}
	b.WriteString("\n")

	return b.String()
}
