package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/util"
)

// renderFormLabel renders a form field label with fixed width alignment.
func (m *TaskEditorModel) renderFormLabel(label string, focusIdx int) string {
	// Pad label to fixed width
	paddedLabel := fmt.Sprintf("%-*s", FormLabelWidth, label)
	if focusIdx >= 0 && m.focusIndex == focusIdx {
		return FieldLabelFocusedStyle.Render(paddedLabel)
	}
	return FieldLabelStyle.Render(paddedLabel)
}

// renderTypeSelector renders the task type selector options.
func (m *TaskEditorModel) renderTypeSelector() string {
	var b strings.Builder
	for i, t := range m.types {
		var style = SelectorOptionStyle
		if i == m.typeCursor {
			if m.focusIndex == 1 {
				style = SelectorOptionSelectedStyle
			} else {
				style = BoldStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("[%s]", t.HeadingPrefix())))
		} else {
			b.WriteString(style.Render(fmt.Sprintf(" %s ", t.HeadingPrefix())))
		}
	}
	return b.String()
}

// renderParentSelector renders the parent task selector options.
func (m *TaskEditorModel) renderParentSelector() string {
	var b strings.Builder

	// "None" option
	if m.parentCursor == 0 {
		if m.focusIndex == 2 {
			b.WriteString(SelectorOptionSelectedStyle.Render("[None]"))
		} else {
			b.WriteString(BoldStyle.Render("[None]"))
		}
	} else {
		b.WriteString(SelectorOptionStyle.Render(" None "))
	}

	// Show a few valid parents
	maxShow := 3
	startIdx := 0
	if m.parentCursor > maxShow {
		startIdx = m.parentCursor - maxShow
	}

	for i := startIdx; i < len(m.validParents) && i < startIdx+maxShow+1; i++ {
		p := m.validParents[i]
		display := util.Truncate(fmt.Sprintf("#%d %s", p.ShortID, p.Title), 18)
		if m.parentCursor == i+1 {
			if m.focusIndex == 2 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf(" [%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf(" [%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	if len(m.validParents) > maxShow+1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" +%d", len(m.validParents)-maxShow-1)))
	}
	return b.String()
}

// renderPrioritySelector renders the priority selector options.
func (m *TaskEditorModel) renderPrioritySelector() string {
	var b strings.Builder
	for i := 1; i <= 5; i++ {
		if i == m.priority {
			if m.focusIndex == 3 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%d]", i)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%d]", i)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %d ", i)))
		}
	}
	b.WriteString(MutedStyle.Render(" (1=high)"))
	return b.String()
}

// renderSectionSelector renders the section selector options.
func (m *TaskEditorModel) renderSectionSelector() string {
	var b strings.Builder
	for i, s := range m.sections {
		if i == m.sectionCursor {
			if m.focusIndex == 4 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", s)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", s)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", s)))
		}
	}
	return b.String()
}

// renderDueDateField renders the due date input field.
func (m *TaskEditorModel) renderDueDateField() string {
	var b strings.Builder
	if m.focusIndex == 5 {
		b.WriteString(SelectorOptionSelectedStyle.Render(m.dueDateInput + "█"))
	} else if m.dueDateInput == "" {
		b.WriteString(MutedStyle.Render("(none)"))
	} else {
		b.WriteString(m.dueDateInput)
	}
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("tomorrow, +3d, 2024-01-15"))
	return b.String()
}

// renderFlagsField renders the pinned and active flag toggles.
func (m *TaskEditorModel) renderFlagsField() string {
	var b strings.Builder

	// Pinned toggle
	pinnedText := "Pin"
	if m.pinned {
		pinnedText = "Pin ✓"
	}
	if m.focusIndex == 6 && m.flagFocus == 0 {
		b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", pinnedText)))
	} else if m.pinned {
		b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", pinnedText)))
	} else {
		b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", pinnedText)))
	}

	b.WriteString(" ")

	// Active toggle
	activeText := "Active"
	if m.active {
		activeText = "Active ✓"
	}
	if m.focusIndex == 6 && m.flagFocus == 1 {
		b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", activeText)))
	} else if m.active {
		b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", activeText)))
	} else {
		b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", activeText)))
	}
	return b.String()
}

// renderRecurrenceSelector renders the recurrence pattern selector options.
func (m *TaskEditorModel) renderRecurrenceSelector() string {
	var b strings.Builder
	for i, opt := range m.recurrenceOpts {
		display := opt
		// For "Custom..." option, show the actual pattern if we have one
		if i == 6 && m.recurrenceCursor == 6 && m.recurrence != nil {
			display = m.recurrence.String()
		}
		if i == m.recurrenceCursor {
			if m.focusIndex == 7 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", opt)))
		}
	}
	// Add hint for 'r' keybind
	if m.focusIndex == 7 {
		b.WriteString("  ")
		b.WriteString(HelpStyle.Render("r: edit"))
	}
	return b.String()
}
