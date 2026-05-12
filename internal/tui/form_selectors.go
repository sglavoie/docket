package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
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
		return FieldLabelFocusedStyle.Render(IconCursorRight + " " + padded)
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
		s = SelectedItemTextStyle.Render("◉ "+optionA) + MutedStyle.Render("  ○ "+optionB)
	} else {
		s = MutedStyle.Render("○ "+optionA) + "  " + SelectedItemTextStyle.Render("◉ "+optionB)
	}
	if focused {
		s += HelpStyle.Render("  (Space to toggle)")
	}
	return s
}

// renderPriorityStarsShared returns star characters for a priority level.
func renderPriorityStarsShared(priority int) string {
	if priority <= 0 {
		return ""
	}
	return strings.Repeat("★", priority)
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

// renderEnumSelector renders a generic enum selector with optional windowing.
// When maxWidth > 0 and the rendered options exceed it, a sliding window
// centered on cursor is shown with < > scroll indicators.
func renderEnumSelector(options []string, cursor int, focused bool, maxWidth int) string {
	rendered := make([]string, len(options))
	for i, opt := range options {
		if i == cursor {
			if focused {
				rendered[i] = SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", opt))
			} else {
				rendered[i] = BoldStyle.Render(fmt.Sprintf("[%s]", opt))
			}
		} else {
			rendered[i] = SelectorOptionStyle.Render(fmt.Sprintf(" %s ", opt))
		}
	}

	totalWidth := 0
	for _, r := range rendered {
		totalWidth += lipgloss.Width(r)
	}
	if maxWidth <= 0 || totalWidth <= maxWidth {
		return strings.Join(rendered, "")
	}

	budget := maxWidth - 4 // reserve 2 chars each for "< " and " >"
	if budget < 1 {
		budget = 1
	}

	lo, hi := cursor, cursor+1
	used := lipgloss.Width(rendered[cursor])
	for lo > 0 || hi < len(rendered) {
		if lo > 0 {
			w := lipgloss.Width(rendered[lo-1])
			if used+w <= budget {
				lo--
				used += w
			} else {
				break
			}
		}
		if hi < len(rendered) {
			w := lipgloss.Width(rendered[hi])
			if used+w <= budget {
				hi++
				used += w
			} else {
				break
			}
		}
	}

	var b strings.Builder
	if lo > 0 {
		b.WriteString(MutedStyle.Render("< "))
	} else {
		b.WriteString("  ")
	}
	for i := lo; i < hi; i++ {
		b.WriteString(rendered[i])
	}
	if hi < len(rendered) {
		b.WriteString(MutedStyle.Render(" >"))
	}
	return b.String()
}
