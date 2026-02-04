package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderFormLabelShared renders a form field label with fixed width alignment.
// When focusIdx matches currentFocus, the label is rendered with the focused style.
// Use focusIdx=-1 for non-focusable labels.
func renderFormLabelShared(label string, focusIdx int, currentFocus int) string {
	paddedLabel := fmt.Sprintf("%-*s", FormLabelWidth, label)
	if focusIdx >= 0 && currentFocus == focusIdx {
		return FieldLabelFocusedStyle.Render(paddedLabel)
	}
	return FieldLabelStyle.Render(paddedLabel)
}

// renderFormLabelWithWidth renders a form field label with a custom width and focus arrow indicator.
// The arrow prefix (▸/space) is prepended to indicate the focused field.
func renderFormLabelWithWidth(label string, labelWidth int, focused bool) string {
	padded := fmt.Sprintf("%-*s", labelWidth, label)
	if focused {
		return FieldLabelFocusedStyle.Render("▸ " + padded)
	}
	return FieldLabelStyle.Render("  " + padded)
}

// renderFormFieldDesc renders a dim description below a field, only when focused.
func renderFormFieldDesc(desc string, indent int, focused bool) string {
	if !focused {
		return ""
	}
	pad := strings.Repeat(" ", indent)
	return pad + FieldDescriptionStyle.Render(desc) + "\n"
}

// renderFormToggle renders a toggle switch with filled/empty circle indicators.
func renderFormToggle(optionA, optionB string, aSelected bool, focused bool) string {
	var s string
	if aSelected {
		s = BoldStyle.Foreground(ColorPrimary).Render("◉ "+optionA) + MutedStyle.Render("  ○ "+optionB)
	} else {
		s = MutedStyle.Render("○ "+optionA) + "  " + BoldStyle.Foreground(ColorPrimary).Render("◉ "+optionB)
	}
	if focused {
		s += HelpStyle.Render("  (Space to toggle)")
	}
	return s
}

// renderTypeSelectorShared renders the task type selector options.
func renderTypeSelectorShared(types []models.TaskType, typeCursor int, focused bool) string {
	var b strings.Builder
	for i, t := range types {
		icon := GetTypeBadge(string(t), t.Icon())
		if i == typeCursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s %s]", icon, t.HeadingPrefix())))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s %s]", icon, t.HeadingPrefix())))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s %s ", icon, t.HeadingPrefix())))
		}
	}
	return b.String()
}

// renderParentSelectorShared renders the parent task selector options.
func renderParentSelectorShared(validParents []*models.Task, parentCursor int, focused bool) string {
	var b strings.Builder

	// "None" option
	if parentCursor == 0 {
		if focused {
			b.WriteString(SelectorOptionSelectedStyle.Render("[None]"))
		} else {
			b.WriteString(BoldStyle.Render("[None]"))
		}
	} else {
		b.WriteString(SelectorOptionStyle.Render(" None "))
	}

	// Show a few valid parents
	maxShow := 5
	startIdx := 0
	if parentCursor > maxShow {
		startIdx = parentCursor - maxShow
	}

	for i := startIdx; i < len(validParents) && i < startIdx+maxShow+1; i++ {
		p := validParents[i]
		display := util.Truncate(fmt.Sprintf("#%d %s", p.ShortID, p.Title), 18)
		if parentCursor == i+1 {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf(" [%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf(" [%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	if len(validParents) > maxShow+1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" +%d", len(validParents)-maxShow-1)))
	}
	return b.String()
}

// renderPrioritySelectorShared renders the priority selector options with stars.
func renderPrioritySelectorShared(priority int, focused bool) string {
	var b strings.Builder
	for i := 1; i <= 5; i++ {
		stars := renderPriorityStarsShared(i)
		label := fmt.Sprintf("%d %s", i, stars)
		if i == priority {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", label)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", label)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", label)))
		}
	}
	return b.String()
}

// renderPriorityStarsShared returns star characters for a priority level.
func renderPriorityStarsShared(priority int) string {
	if priority <= 0 {
		return ""
	}
	return strings.Repeat("★", priority)
}

// renderSectionSelectorShared renders the section selector options.
func renderSectionSelectorShared(sections []string, sectionCursor int, focused bool) string {
	var b strings.Builder
	for i, s := range sections {
		if i == sectionCursor {
			if focused {
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

// renderRecurrenceSelectorShared renders the recurrence pattern selector options.
func renderRecurrenceSelectorShared(recurrenceOpts []string, recurrenceCursor int, focused bool, recurrence *models.RecurrencePattern) string {
	var b strings.Builder
	for i, opt := range recurrenceOpts {
		display := opt
		// For "Custom..." option, show the actual pattern if we have one
		if i == 6 && recurrenceCursor == 6 && recurrence != nil {
			display = recurrence.String()
		}
		if i == recurrenceCursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", opt)))
		}
	}
	if focused {
		b.WriteString("  ")
		b.WriteString(HelpStyle.Render("r: edit"))
	}
	return b.String()
}

// renderFlagsSelectorShared renders the mutually exclusive flag options: None, Pin, Active.
func renderFlagsSelectorShared(flagCursor int, focused bool) string {
	var b strings.Builder
	options := []string{"None", "Pin", "Active"}

	for i, opt := range options {
		if i == flagCursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", opt)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", opt)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", opt)))
		}
	}
	return b.String()
}

// renderDueDateFieldShared renders the due date input field.
func renderDueDateFieldShared(dueDateInput string, focused bool) string {
	var b strings.Builder
	if focused {
		b.WriteString(SelectorOptionSelectedStyle.Render(dueDateInput + "█"))
	} else if dueDateInput == "" {
		b.WriteString(MutedStyle.Render("(none)"))
	} else {
		b.WriteString(dueDateInput)
	}
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("tomorrow, +3d, 2024-01-15"))
	return b.String()
}

// renderContextSelectorShared renders a hybrid context selector (predefined list + custom typing).
// It uses the same [selected] / unselected pattern as other form selectors.
func renderContextSelectorShared(contexts []string, contextCursor int, focused bool) string {
	var b strings.Builder
	for i, ctx := range contexts {
		if i == contextCursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", ctx)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", ctx)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", ctx)))
		}
	}
	if focused {
		b.WriteString("  ")
		b.WriteString(HelpStyle.Render("←/→ select · type custom"))
	}
	return b.String()
}
