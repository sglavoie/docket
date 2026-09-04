package taskform

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// formBasicsLabelWidth is the fixed label column width for all Basics-tab rows.
const formBasicsLabelWidth = 13 // "Description:" (12 chars) + 1 pad = 13

// renderBasicsTab renders the generic task fields.
func (m *TaskFormModel) renderBasicsTab() string {
	if m.collapsedBasics {
		return m.renderBasicsTabCollapsed()
	}
	return m.renderBasicsTabExpanded()
}

// renderBasicsTabCollapsed renders Title, Priority, and the expand row.
func (m *TaskFormModel) renderBasicsTabCollapsed() string {
	var b strings.Builder
	const lw = formBasicsLabelWidth

	// 0: Title
	b.WriteString(chrome.RenderFormLabelWithWidth("Title:", lw, m.focusIndex == 0))
	b.WriteString("\n")
	theme.ApplyThemedTextarea(&m.titleInput)
	b.WriteString(m.titleInput.View())
	b.WriteString("\n")

	// 1: Priority
	b.WriteString(chrome.RenderFormLabelWithWidth("Priority:", lw, m.focusIndex == 1))
	{
		icon := theme.GetPriorityStyle(m.draft.priority).Render(theme.PriorityIcon(m.draft.priority))
		lbl := formPriorityLabel(m.draft.priority)
		val := fmt.Sprintf("%s %s", icon, lbl)
		if m.focusIndex == 1 {
			b.WriteString(theme.SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString("\n")

	// 2: Expand row
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

	return b.String()
}

// renderBasicsTabExpanded renders the full generic task form.
func (m *TaskFormModel) renderBasicsTabExpanded() string {
	var b strings.Builder
	const lw = formBasicsLabelWidth

	// 0: Title (inline textarea)
	b.WriteString(chrome.RenderFormLabelWithWidth("Title:", lw, m.focusIndex == 0))
	b.WriteString("\n")
	theme.ApplyThemedTextarea(&m.titleInput)
	b.WriteString(m.titleInput.View())
	b.WriteString("\n")

	// 1: Description — read-only row; Enter opens TextEditorModel.
	b.WriteString(chrome.RenderFormLabelWithWidth("Description:", lw, m.focusIndex == 1))
	{
		val := m.draft.desc
		var display string
		if val == "" {
			display = theme.MutedStyle.Render("(none)")
		} else {
			preview := textfit.TruncateToVisualWidth(val, 40)
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
	b.WriteString(chrome.RenderFormLabelWithWidth("Priority:", lw, m.focusIndex == 2))
	{
		icon := theme.GetPriorityStyle(m.draft.priority).Render(theme.PriorityIcon(m.draft.priority))
		lbl := formPriorityLabel(m.draft.priority)
		val := fmt.Sprintf("%s %s", icon, lbl)
		if m.focusIndex == 2 {
			b.WriteString(theme.SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString("\n")

	// 3: Parent
	b.WriteString(chrome.RenderFormLabelWithWidth("Parent:", lw, m.focusIndex == 3))
	{
		var val string
		if m.draft.parentID != nil {
			if p := formFindTask(m.project.Tasks, *m.draft.parentID); p != nil {
				val = fmt.Sprintf("#%d %s", p.ShortID, textfit.TruncateToVisualWidth(p.Title, 40))
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
	b.WriteString(chrome.RenderFormLabelWithWidth("State:", lw, m.focusIndex == 4))
	{
		val := m.draft.state.String()
		if m.focusIndex == 4 {
			b.WriteString(theme.SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString("\n")

	// 5: Flags
	b.WriteString(chrome.RenderFormLabelWithWidth("Flags:", lw, m.focusIndex == 5))
	b.WriteString(renderFormFlagsValue(m.draft.pinned, m.draft.review, m.focusIndex == 5))
	b.WriteString("\n")

	// 6: Tags
	b.WriteString(chrome.RenderFormLabelWithWidth("Tags:", lw, m.focusIndex == 6))
	{
		var val string
		if len(m.draft.tags) == 0 {
			val = theme.MutedStyle.Render("(none)")
		} else {
			val = strings.Join(m.draft.tags, ", ")
		}
		if m.focusIndex == 6 {
			b.WriteString(theme.SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString("\n")

	return b.String()
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

// formPriorityLabel returns the human-readable label for a priority value.
func formPriorityLabel(p int) string {
	for _, e := range formPriorityEntries {
		if e.priority == p {
			return e.label
		}
	}
	return fmt.Sprintf("%d", p)
}
