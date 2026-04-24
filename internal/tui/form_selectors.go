package tui

import (
	"fmt"
	"strings"
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

// renderEnumSelector renders a generic enum selector with (none) as first option.
func renderEnumSelector(options []string, cursor int, focused bool) string {
	var b strings.Builder
	for i, opt := range options {
		if i == cursor {
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

// renderTextAttrPreview renders a text attribute field as a truncated preview.
func renderTextAttrPreview(value string, focused bool) string {
	if value == "" {
		display := MutedStyle.Render("(Enter to edit)")
		if focused {
			return SelectorOptionSelectedStyle.Render(display)
		}
		return display
	}
	// Show truncated preview
	display := strings.ReplaceAll(value, "\n", " ")
	display = truncateToVisualWidth(display, 40)
	if focused {
		return SelectorOptionSelectedStyle.Render(display)
	}
	return display
}
