package taskform

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// formBasicsLabelWidth is the fixed label column width for all Basics-tab rows.
const formBasicsLabelWidth = 13 // "Description:" (12 chars) + 1 pad = 13

// formBasicsPreviewWidth is how much of a read-only value the row would rather
// show. It is a preference, not a reservation: the row's own budget only ever
// shortens it, so a wide terminal previews exactly what it always previewed.
const formBasicsPreviewWidth = 40

// renderBasicsTab renders the generic task fields.
func (m *TaskFormModel) renderBasicsTab(b *formBodyBuilder, bodyWidth int) {
	if m.collapsedBasics {
		m.renderBasicsTabCollapsed(b, bodyWidth)
		return
	}
	m.renderBasicsTabExpanded(b, bodyWidth)
}

// renderBasicsTabCollapsed renders Title, Priority, and the expand row.
func (m *TaskFormModel) renderBasicsTabCollapsed(b *formBodyBuilder, bodyWidth int) {
	const lw = formBasicsLabelWidth

	// 0: Title
	b.field(0)
	b.WriteString(chrome.RenderFormLabelWithWidth("Title:", lw, m.focusIndex == 0))
	b.WriteString("\n")
	m.titleInput.SetWidth(bodyWidth)
	theme.ApplyThemedTextarea(&m.titleInput)
	b.WriteString(m.titleInput.View())
	b.WriteString("\n")

	// 1: Priority
	b.field(1)
	b.WriteString(chrome.RenderFormLabelWithWidth("Priority:", lw, m.focusIndex == 1))
	b.WriteString(renderFormPriorityValue(m.draft.priority, m.focusIndex == 1))
	b.WriteString("\n")

	// 2: Expand row
	b.field(2)
	expandLabel := theme.MutedStyle.Render("+ more fields")
	if m.focusIndex == 2 {
		expandLabel = theme.SelectorOptionSelectedStyle.Render("+ more fields")
	}
	b.WriteString(chrome.RenderFormLabelWithWidth("", lw, false))
	b.WriteString(expandLabel)
	if m.focusIndex == 2 {
		b.WriteString(theme.HelpStyle.Render("  (Enter to expand)"))
	}
	b.WriteString("\n")
}

// renderBasicsTabExpanded renders the full generic task form.
func (m *TaskFormModel) renderBasicsTabExpanded(b *formBodyBuilder, bodyWidth int) {
	const lw = formBasicsLabelWidth
	// Every labeled row spends the focus marker and the label column before its
	// own value starts, so one budget answers for all of them.
	valueWidth := formValueWidth(bodyWidth, lw)

	// 0: Title (inline textarea)
	b.field(0)
	b.WriteString(chrome.RenderFormLabelWithWidth("Title:", lw, m.focusIndex == 0))
	b.WriteString("\n")
	m.titleInput.SetWidth(bodyWidth)
	theme.ApplyThemedTextarea(&m.titleInput)
	b.WriteString(m.titleInput.View())
	b.WriteString("\n")

	// 1: Description — read-only row; Enter opens TextEditorModel.
	b.field(1)
	b.WriteString(chrome.RenderFormLabelWithWidth("Description:", lw, m.focusIndex == 1))
	{
		val := m.draft.desc
		var display string
		if val == "" {
			display = theme.MutedStyle.Render("(none)")
		} else {
			preview := textfit.TruncateToVisualWidth(val, min(formBasicsPreviewWidth, valueWidth))
			if m.focusIndex == 1 {
				display = theme.SelectorOptionSelectedStyle.Render(preview)
			} else {
				display = preview
			}
		}
		b.WriteString(display)
	}
	b.WriteString("\n")

	// 2: Priority
	b.field(2)
	b.WriteString(chrome.RenderFormLabelWithWidth("Priority:", lw, m.focusIndex == 2))
	b.WriteString(renderFormPriorityValue(m.draft.priority, m.focusIndex == 2))
	b.WriteString("\n")

	// 3: Parent
	b.field(3)
	b.WriteString(chrome.RenderFormLabelWithWidth("Parent:", lw, m.focusIndex == 3))
	{
		var val string
		if m.draft.parentID != nil {
			if p := formFindTask(m.project.Tasks, *m.draft.parentID); p != nil {
				ref := fmt.Sprintf("#%d ", p.ShortID)
				titleWidth := min(formBasicsPreviewWidth, valueWidth-lipgloss.Width(ref))
				val = ref + textfit.TruncateToVisualWidth(p.Title, max(titleWidth, formMinValueWidth))
			} else {
				val = theme.MutedStyle.Render("(not found)")
			}
		} else {
			val = theme.MutedStyle.Render("(none)")
		}
		if m.focusIndex == 3 {
			b.WriteString(theme.SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString("\n")

	// 4: State
	b.field(4)
	b.WriteString(chrome.RenderFormLabelWithWidth(chrome.StatusFieldLabel+":", lw, m.focusIndex == 4))
	{
		val := chrome.PlanStatusValue(m.draft.state.String())
		if m.focusIndex == 4 {
			b.WriteString(theme.SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString("\n")

	// 5: Flags
	b.field(5)
	b.WriteString(chrome.RenderFormLabelWithWidth("Flags:", lw, m.focusIndex == 5))
	b.WriteString(renderFormFlagsValue(m.draft.pinned, m.draft.review, m.focusIndex == 5))
	b.WriteString("\n")

	// 6: Tags
	b.field(6)
	b.WriteString(chrome.RenderFormLabelWithWidth("Tags:", lw, m.focusIndex == 6))
	{
		var val string
		if len(m.draft.tags) == 0 {
			val = theme.MutedStyle.Render("(none)")
		} else {
			// The whole set is one Enter away in the tag selector, so the row
			// shows what it can rather than widening the body to show all of it.
			val = textfit.TruncateToVisualWidth(strings.Join(m.draft.tags, ", "), valueWidth)
		}
		if m.focusIndex == 6 {
			b.WriteString(theme.SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString("\n")

	// 7: Context — inline textinput when focused, plain text otherwise, exactly
	// as the Schedule tab's Due row behaves.
	focused := m.focusIndex == formBasicsContextField
	b.field(formBasicsContextField)
	b.WriteString(chrome.RenderFormLabelWithWidth("Context:", lw, focused))
	if focused {
		m.contextInput.SetWidth(formInputWidth(formContextInputWidth, valueWidth))
		theme.ApplyThemedTextInput(&m.contextInput)
		b.WriteString(m.contextInput.View())
	} else if m.draft.context == "" {
		b.WriteString(theme.MutedStyle.Render("(none)"))
	} else {
		b.WriteString(textfit.TruncateToVisualWidth(m.draft.context, valueWidth))
	}
	b.WriteString("\n")
}

// renderFormFlagsValue renders the flag badges for a row value.
func renderFormFlagsValue(pinned, review, focused bool) string {
	var parts []string
	if pinned {
		parts = append(parts, "Pinned")
	}
	if review {
		parts = append(parts, "Review")
	}
	if len(parts) == 0 {
		return theme.MutedStyle.Render("(none)")
	}
	val := strings.Join(parts, "  ")
	if focused {
		return theme.SelectorOptionSelectedStyle.Render(val)
	}
	return val
}

// renderFormPriorityValue renders a priority as a row value. Unset has no icon,
// so it is the muted placeholder every other empty row shows rather than a
// label standing one column in behind the icon it lacks.
func renderFormPriorityValue(priority int, focused bool) string {
	val := formPriorityValue(priority)
	if priority == models.PriorityUnset || !focused {
		return val
	}
	return theme.SelectorOptionSelectedStyle.Render(val)
}

// formPriorityValue is the one spelling of a priority the form and its picker
// share: muted (none) for unset, else the priority's icon and label.
func formPriorityValue(priority int) string {
	if priority == models.PriorityUnset {
		return theme.MutedStyle.Render("(none)")
	}
	icon := theme.GetPriorityStyle(priority).Render(theme.PriorityIcon(priority))
	return icon + " " + formPriorityLabel(priority)
}

// formPriorityLabel returns the human-readable label for a priority value.
func formPriorityLabel(p int) string {
	for _, e := range formPriorityEntries {
		if e.priority == p {
			return e.label
		}
	}
	return fmt.Sprintf("%d", p)
}
