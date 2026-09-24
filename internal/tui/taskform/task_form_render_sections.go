package taskform

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// The Children and Extras tabs are four instances of one shape: a collapsible
// section header, then a draft row per item. They are rendered here once so the
// width arithmetic that keeps them inside the frame exists once too — a fifth
// section inherits the fitting rather than restating it.

const (
	// formSectionLeadRule is the short rule drawn before a section's label.
	formSectionLeadRule = 3
	// formSectionTrailRule is the rule drawn after the label when the row can
	// afford it. It is decoration, so it is what gives way first.
	formSectionTrailRule = 16
	// formSectionTrailRuleMin keeps the header legible as a header once the trail
	// rule starts giving up columns.
	formSectionTrailRuleMin = 3
)

// renderFormSectionHeader writes one collapsible section header. The label and
// the hint are information and the trailing rule is not, so the rule shrinks to
// formSectionTrailRuleMin before formFitHint sheds a single key.
func (m *TaskFormModel) renderFormSectionHeader(
	b *formBodyBuilder,
	icon, name string,
	count int,
	focused bool,
	hint string,
	bodyWidth int,
) {
	label := fmt.Sprintf(" %s %s (%d) ", icon, name, count)
	spent := formLabelMarkerWidth + formSectionLeadRule + lipgloss.Width(label)
	remaining := max(bodyWidth-spent, 0)

	fitted := ""
	if focused {
		fitted = formFitHint(hint, remaining-formSectionTrailRuleMin-formHintGap)
		if fitted != "" {
			fitted = strings.Repeat(" ", formHintGap) + fitted
		}
	}
	trail := max(min(formSectionTrailRule, remaining-lipgloss.Width(fitted)), 0)

	if focused {
		b.WriteString(theme.FieldLabelFocusedStyle.Render(theme.IconCursorRight + " "))
	} else {
		b.WriteString(theme.FieldLabelStyle.Render("  "))
	}
	b.WriteString(theme.MutedStyle.Render(strings.Repeat("─", formSectionLeadRule)))
	b.WriteString(theme.BoldStyle.Render(label))
	b.WriteString(theme.MutedStyle.Render(strings.Repeat("─", trail)))
	if fitted != "" {
		b.WriteString(theme.MutedStyle.Render(fitted))
	}
	b.WriteString("\n")
}

// renderFormItemRow renders one draft row of a section. The value wins the
// width and the hint disappears before the value is cut, because the keys a
// shed hint drops are still spelled in the tab's guidance while the value is the
// row's whole point. A value too long even for the whole row is shortened here
// and recovered by Enter, which opens it in an editor at full length.
func renderFormItemRow(line string, focused bool, hint string, bodyWidth int) string {
	budget := max(bodyWidth-formItemIndent-formSelectedValuePadding, formMinValueWidth)
	value := textfit.TruncateToVisualWidth(line, budget)
	if !focused {
		return strings.Repeat(" ", formItemIndent) + value + "\n"
	}
	fitted := formFitHint(hint, budget-lipgloss.Width(value)-formHintGap)
	if fitted != "" {
		fitted = theme.MutedStyle.Render(strings.Repeat(" ", formHintGap) + fitted)
	}
	return theme.FieldLabelFocusedStyle.Render("  "+theme.IconCursorRight+" ") +
		theme.SelectorOptionSelectedStyle.Render(value) + fitted + "\n"
}

// renderFormSectionEmptyState writes a section's empty row, shortened to the
// body rather than allowed to widen it.
func renderFormSectionEmptyState(b *formBodyBuilder, text string, bodyWidth int) {
	b.WriteString(theme.MutedStyle.Render(textfit.TruncateToVisualWidth(
		strings.Repeat(" ", formItemIndent)+text, bodyWidth)))
	b.WriteString("\n")
}
