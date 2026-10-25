package chrome

import (
	"fmt"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// RenderFormLabelShared renders a form field label with fixed width alignment.
// When focusIdx matches currentFocus, the label is rendered with the focused style.
// Use focusIdx=-1 for non-focusable labels.
func RenderFormLabelShared(label string, focusIdx int, currentFocus int) string {
	paddedLabel := fmt.Sprintf("%-*s", theme.FormLabelWidth, label)
	if focusIdx >= 0 && currentFocus == focusIdx {
		return theme.FieldLabelFocusedStyle.Render(paddedLabel)
	}
	return theme.FieldLabelStyle.Render(paddedLabel)
}

// RenderFormLabelWithWidth renders a form field label with a custom width and focus arrow indicator.
// The arrow prefix (▸/space) is prepended to indicate the focused field.
func RenderFormLabelWithWidth(label string, labelWidth int, focused bool) string {
	padded := fmt.Sprintf("%-*s", labelWidth, label)
	if focused {
		return theme.FieldLabelFocusedStyle.Render(theme.IconCursorRight + " " + padded)
	}
	return theme.FieldLabelStyle.Render("  " + padded)
}
