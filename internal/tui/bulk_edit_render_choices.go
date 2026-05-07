package tui

import (
	"fmt"
	"strings"

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

	options := []string{"No change", "Todo", "In Progress", "Review", "Standby", "Maybe", "Done", "Blocked", "Canceled"}
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

// renderTypeField renders the type selection field.
func (m *BulkEditModel) renderTypeField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldType
	label := "Type:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.typeSummary)))
	b.WriteString("\n")

	options := []string{"No change", "Epic", "Story", "Task", "Bug", "Idea"}
	for i, opt := range options {
		prefix := "  "
		if focused {
			prefix = "    "
		}

		selected := (m.typeValue == nil && i == 0) ||
			(m.typeValue != nil && i > 0 && getTypeIndex(*m.typeValue) == i-1)

		cursor := "[ ]"
		if selected {
			cursor = "[x]"
		}
		if focused && m.typeCursor == i {
			cursor = "[>]"
		}

		line := fmt.Sprintf("%s%s %s", prefix, cursor, opt)
		if focused && m.typeCursor == i {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// getTypeIndex returns the index of a task type in the type list.
func getTypeIndex(t models.TaskType) int {
	switch t {
	case models.TaskTypeEpic:
		return 0
	case models.TaskTypeStory:
		return 1
	case models.TaskTypeTask:
		return 2
	case models.TaskTypeBug:
		return 3
	case models.TaskTypeIdea:
		return 4
	default:
		return 0
	}
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

// renderRecurrenceField renders the recurrence selection field.
func (m *BulkEditModel) renderRecurrenceField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldRecurrence
	label := "Recurrence:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.recurrenceSummary)))
	b.WriteString("\n")

	options := []string{"Keep current", "Daily", "Weekly", "Every 2 weeks", "Monthly", "Yearly", "Clear recurrence"}
	for i, opt := range options {
		prefix := "  "
		if focused {
			prefix = "    "
		}

		var selected bool
		switch {
		case i == 0:
			selected = m.recurrenceValue == nil && !m.clearRecurrence
		case i == len(options)-1:
			selected = m.clearRecurrence
		default:
			selected = m.recurrenceValue != nil && m.recurrenceCursor == i
		}

		cursor := "[ ]"
		if selected {
			cursor = "[x]"
		}
		if focused && m.recurrenceCursor == i {
			cursor = "[>]"
		}

		line := fmt.Sprintf("%s%s %s", prefix, cursor, opt)
		if focused && m.recurrenceCursor == i {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
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
	case models.StatusStandby:
		return 3
	case models.StatusMaybe:
		return 4
	case models.StatusDone:
		return 5
	case models.StatusBlocked:
		return 6
	case models.StatusCanceled:
		return 7
	default:
		return 0
	}
}
