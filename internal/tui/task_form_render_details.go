package tui

import (
	"strings"
)

// formDetailsLabelWidth is the fixed label column width for all Details-tab rows.
// "Description:" (12 chars) is the widest; +1 pad = 13.
const formDetailsLabelWidth = 13

// renderDetailsTab renders the Details tab: Description, URL, Source.
func (m *TaskFormModel) renderDetailsTab() string {
	var b strings.Builder
	const lw = formDetailsLabelWidth

	// 0: Description — read-only row; Enter opens TextEditorModel.
	b.WriteString(renderFormLabelWithWidth("Description:", lw, m.focusIndex == 0))
	{
		val := m.draft.desc
		var display string
		if val == "" {
			display = MutedStyle.Render("(none)")
		} else {
			preview := truncateToVisualWidth(val, 40)
			if m.focusIndex == 0 {
				display = SelectorOptionSelectedStyle.Render(preview)
			} else {
				display = preview
			}
		}
		b.WriteString(display)
	}
	b.WriteString(MutedStyle.Render("  Enter"))
	b.WriteString("\n")

	// 1: URL — inline textinput when focused.
	b.WriteString(renderFormLabelWithWidth("URL:", lw, m.focusIndex == 1))
	if m.focusIndex == 1 {
		applyThemedTextInput(&m.urlTextInput)
		b.WriteString(m.urlTextInput.View())
	} else {
		val := m.draft.url
		if val == "" {
			b.WriteString(MutedStyle.Render("(none)"))
		} else {
			b.WriteString(truncateToVisualWidth(val, 50))
		}
		b.WriteString(MutedStyle.Render("  Enter"))
	}
	b.WriteString("\n")

	// 2: Source — inline textinput when focused.
	b.WriteString(renderFormLabelWithWidth("Source:", lw, m.focusIndex == 2))
	if m.focusIndex == 2 {
		applyThemedTextInput(&m.sourceTextInput)
		b.WriteString(m.sourceTextInput.View())
	} else {
		val := m.draft.source
		if val == "" {
			b.WriteString(MutedStyle.Render("(none)"))
		} else {
			b.WriteString(truncateToVisualWidth(val, 50))
		}
		b.WriteString(MutedStyle.Render("  Enter"))
	}
	b.WriteString("\n")

	return b.String()
}
