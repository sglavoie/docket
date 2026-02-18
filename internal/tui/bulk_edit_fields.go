package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
)

// renderStatusField renders the status selection field.
func (m *BulkEditModel) renderStatusField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldStatus
	label := "Status:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.statusSummary)))
	b.WriteString("\n")

	options := []string{"No change", "Todo", "In Progress", "Review", "Done", "Blocked", "Canceled"}
	for i, opt := range options {
		prefix := "  "
		if focused {
			prefix = "    "
		}

		selected := (m.statusValue == nil && i == 0) ||
			(m.statusValue != nil && i > 0 && getStatusIndex(*m.statusValue) == i-1)

		cursor := "[ ]"
		if selected {
			cursor = "[x]"
		}
		if focused && m.statusCursor == i {
			cursor = "[>]"
		}

		line := fmt.Sprintf("%s%s %s", prefix, cursor, opt)
		if focused && m.statusCursor == i {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderPriorityField renders the priority selection field.
func (m *BulkEditModel) renderPriorityField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldPriority
	label := "Priority:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.prioritySummary)))
	b.WriteString("\n")

	options := []string{"Keep current", "1 - Critical", "2 - High", "3 - Normal", "4 - Low", "5 - Someday"}
	for i, opt := range options {
		prefix := "  "
		if focused {
			prefix = "    "
		}

		selected := (m.priorityValue == nil && i == 0) ||
			(m.priorityValue != nil && *m.priorityValue == i)

		cursor := "[ ]"
		if selected {
			cursor = "[x]"
		}
		if focused && m.priorityCursor == i {
			cursor = "[>]"
		}

		line := fmt.Sprintf("%s%s %s", prefix, cursor, opt)
		if focused && m.priorityCursor == i {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderTagsField renders the tags field.
func (m *BulkEditModel) renderTagsField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldTags
	label := "Tags:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("\n")

	prefix := "    "

	// Show common tags
	if len(m.commonTags) > 0 {
		b.WriteString(prefix)
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Common: %s", strings.Join(m.commonTags, ", "))))
		b.WriteString("\n")
	}

	// Tag add input mode
	if m.tagMode == 1 {
		b.WriteString(prefix)
		b.WriteString("Add: ")
		b.WriteString(m.tagAddInput.View())
		b.WriteString("\n")
	} else {
		// Show tags to add
		if len(m.addTags) > 0 {
			b.WriteString(prefix)
			b.WriteString(SuccessStyle.Render(fmt.Sprintf("+ %s", strings.Join(m.addTags, ", "))))
			b.WriteString("\n")
		}

		// Show tags to remove
		if len(m.removeTags) > 0 {
			tags := []string{}
			for t := range m.removeTags {
				tags = append(tags, t)
			}
			sort.Strings(tags)
			b.WriteString(prefix)
			b.WriteString(ValidationErrorStyle.Render(fmt.Sprintf("- %s", strings.Join(tags, ", "))))
			b.WriteString("\n")
		}
	}

	// Tag removal mode
	if m.tagMode == 2 {
		b.WriteString(prefix)
		b.WriteString("Select tags to remove:\n")
		for i, tag := range m.allTags {
			cursor := "[ ]"
			if m.removeTags[tag] {
				cursor = "[x]"
			}
			if m.tagRemoveCursor == i {
				cursor = "[>]"
			}
			line := fmt.Sprintf("%s  %s %s", prefix, cursor, tag)
			if m.tagRemoveCursor == i {
				b.WriteString(TaskJumperSelectedStyle.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	// Show controls
	if focused && m.tagMode == 0 {
		b.WriteString(prefix)
		b.WriteString(MutedStyle.Render("[a] add tag  [r] remove tag"))
		b.WriteString("\n")
	}

	return b.String()
}

// renderSectionField renders the section selection field.
func (m *BulkEditModel) renderSectionField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldSection
	label := "Section:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.sectionSummary)))
	b.WriteString("\n")

	prefix := "  "
	if focused {
		prefix = "    "
	}

	// Keep current option
	cursor := "[ ]"
	if m.sectionValue == nil {
		cursor = "[x]"
	}
	if focused && m.sectionCursor == 0 {
		cursor = "[>]"
	}
	line := fmt.Sprintf("%s%s Keep current", prefix, cursor)
	if focused && m.sectionCursor == 0 {
		b.WriteString(TaskJumperSelectedStyle.Render(line))
	} else {
		b.WriteString(line)
	}
	b.WriteString("\n")

	// Section options
	for i, section := range m.sectionOptions {
		displayName := section
		if displayName == "" {
			displayName = "Active (clear section)"
		}

		cursor := "[ ]"
		if m.sectionValue != nil && *m.sectionValue == section {
			cursor = "[x]"
		}
		if focused && m.sectionCursor == i+1 {
			cursor = "[>]"
		}

		line := fmt.Sprintf("%s%s %s", prefix, cursor, displayName)
		if focused && m.sectionCursor == i+1 {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderDueDateField renders the due date field.
func (m *BulkEditModel) renderDueDateField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldDueDate
	label := "Due Date:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.dueDateSummary)))
	b.WriteString("\n")

	prefix := "  "
	if focused {
		prefix = "    "
	}

	// Determine which option is selected
	// Priority: setDueDate > clearDueDate > keep current
	keepSelected := m.setDueDate == nil && !m.clearDueDate
	setSelected := m.setDueDate != nil
	clearSelected := m.clearDueDate

	// Option 0: Keep current
	cursor := "[ ]"
	if keepSelected {
		cursor = "[x]"
	}
	if focused && m.dueDateCursor == 0 {
		cursor = "[>]"
	}
	line := fmt.Sprintf("%s%s Keep current", prefix, cursor)
	if focused && m.dueDateCursor == 0 {
		b.WriteString(TaskJumperSelectedStyle.Render(line))
	} else {
		b.WriteString(line)
	}
	b.WriteString("\n")

	// Option 1: Set date
	if m.dueDateInputMode {
		// Show input mode
		b.WriteString(prefix)
		b.WriteString("    Set date: ")
		b.WriteString(m.dueDateInput.View())
		b.WriteString("\n")
		if m.dueDateError != nil {
			b.WriteString(prefix)
			b.WriteString("    ")
			b.WriteString(ValidationErrorStyle.Render(m.dueDateError.Error()))
			b.WriteString("\n")
		}
	} else {
		cursor = "[ ]"
		if setSelected {
			cursor = "[x]"
		}
		if focused && m.dueDateCursor == 1 {
			cursor = "[>]"
		}
		var setLabel string
		if m.setDueDate != nil {
			setLabel = fmt.Sprintf("Set: %s (%s)", m.setDueDate.Format("2006-01-02"), m.setDueDate.Format("Monday"))
		} else {
			setLabel = "Set date..."
		}
		line = fmt.Sprintf("%s%s %s", prefix, cursor, setLabel)
		if focused && m.dueDateCursor == 1 {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Option 2: Clear due date
	cursor = "[ ]"
	if clearSelected {
		cursor = "[x]"
	}
	if focused && m.dueDateCursor == 2 {
		cursor = "[>]"
	}
	line = fmt.Sprintf("%s%s Clear due date", prefix, cursor)
	if focused && m.dueDateCursor == 2 {
		b.WriteString(TaskJumperSelectedStyle.Render(line))
	} else {
		b.WriteString(line)
	}
	b.WriteString("\n")

	return b.String()
}

// renderPreview renders the preview of pending changes.
func (m *BulkEditModel) renderPreview() string {
	changes := m.buildChanges()
	if !changes.HasChanges() {
		return MutedStyle.Render("  Preview: No changes pending")
	}

	var parts []string

	if changes.SetStatus != nil {
		parts = append(parts, fmt.Sprintf("Status → %s", *changes.SetStatus))
	}
	if changes.SetPriority != nil {
		parts = append(parts, fmt.Sprintf("Priority → P%d", *changes.SetPriority))
	}
	for _, tag := range changes.AddTags {
		parts = append(parts, fmt.Sprintf("+%s", tag))
	}
	for _, tag := range changes.RemoveTags {
		parts = append(parts, fmt.Sprintf("-%s", tag))
	}
	if changes.SetSection != nil {
		if *changes.SetSection == "" {
			parts = append(parts, "Section → Active")
		} else {
			parts = append(parts, fmt.Sprintf("Section → %s", *changes.SetSection))
		}
	}
	if changes.SetDueDate != nil {
		parts = append(parts, fmt.Sprintf("Due → %s", changes.SetDueDate.Format("2006-01-02")))
	} else if changes.ClearDueDate {
		parts = append(parts, "Clear due dates")
	}

	previewStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	return previewStyle.Render(fmt.Sprintf("  Preview: %s", strings.Join(parts, ", ")))
}

// renderActions renders the action buttons.
func (m *BulkEditModel) renderActions() string {
	var b strings.Builder

	focused := m.focusIndex == fieldActions

	applyStyle := NormalStyle
	cancelStyle := NormalStyle

	if focused {
		if m.actionCursor == 0 {
			applyStyle = TaskJumperSelectedStyle
		} else {
			cancelStyle = TaskJumperSelectedStyle
		}
	}

	changes := m.buildChanges()
	applyLabel := "Apply Changes"
	if !changes.HasChanges() {
		applyLabel = "(No changes)"
	}

	b.WriteString("  ")
	b.WriteString(applyStyle.Render(fmt.Sprintf("[%s]", applyLabel)))
	b.WriteString("   ")
	b.WriteString(cancelStyle.Render("[Cancel]"))

	return b.String()
}

// renderFooter renders the help footer.
func (m *BulkEditModel) renderFooter() string {
	if m.tagMode == 1 {
		return MutedStyle.Render("  Enter: add tag · Esc: cancel")
	}
	if m.tagMode == 2 {
		return MutedStyle.Render("  Space: toggle · Tab: next · Esc: done")
	}
	if m.dueDateInputMode {
		return MutedStyle.Render("  Enter: set date · Esc: cancel · Formats: tomorrow, +3d, fri, 2024-01-15")
	}
	return MutedStyle.Render("  Tab/j/k: navigate · Enter: select · h/l: options · Esc: cancel")
}

// fieldLabel formats a field label.
func (m *BulkEditModel) fieldLabel(label string, focused bool) string {
	if focused {
		return BoldStyle.Render(label)
	}
	return label
}

// getStatusIndex returns the index of a status in the status list.
func getStatusIndex(s models.TaskStatus) int {
	switch s {
	case models.StatusTodo:
		return 0
	case models.StatusInProgress:
		return 1
	case models.StatusReview:
		return 2
	case models.StatusDone:
		return 3
	case models.StatusBlocked:
		return 4
	case models.StatusCanceled:
		return 5
	default:
		return 0
	}
}
