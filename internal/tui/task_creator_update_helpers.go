package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// parseDueDate parses the due date input string and updates m.dueDate.
func (m *TaskCreatorModel) parseDueDate() error {
	value := strings.TrimSpace(m.dueDateInput)

	// Handle empty or "none" to clear due date
	if value == "" || strings.ToLower(value) == "none" || strings.ToLower(value) == "clear" {
		m.dueDate = nil
		return nil
	}

	// Parse the date using the existing ParseDate function
	date, err := ParseDate(value)
	if err != nil {
		return err
	}

	m.dueDate = &date
	return nil
}

// updateRecurrenceFromCursor updates the recurrence pattern based on cursor position.
func (m *TaskCreatorModel) updateRecurrenceFromCursor() {
	switch m.recurrenceCursor {
	case 0: // None
		m.recurrence = nil
	case 1: // Daily
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	case 2: // Weekly
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly}
	case 3: // Biweekly
		m.recurrence = &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly}
	case 4: // Monthly
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceMonthly}
	case 5: // Yearly
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceYearly}
	}
}

// updateFlagsFromCursor updates pinned/active/dimmed based on cursor position.
func (m *TaskCreatorModel) updateFlagsFromCursor() {
	switch m.flagCursor {
	case 0: // None
		m.pinned = false
		m.active = false
		m.dimmed = false
	case 1: // Pin
		m.pinned = true
		m.active = false
		m.dimmed = false
	case 2: // Active
		m.pinned = false
		m.active = true
		m.dimmed = false
	case 3: // Dim
		m.pinned = false
		m.active = false
		m.dimmed = true
	}
}

// handleHorizontalMove handles left/right navigation for all selector fields.
// direction is -1 for left, +1 for right.
func (m *TaskCreatorModel) handleHorizontalMove(section string, idx, direction int) (tea.Model, tea.Cmd) {
	switch section {
	case "type":
		return m, m.moveTypeCursor(direction)
	case "parent":
		m.moveParentCursor(direction)
	case "priority":
		m.movePriority(direction)
	case "section":
		m.moveSectionCursor(direction)
	case "flags":
		m.moveFlagCursor(direction)
	case "recurrence":
		return m, m.moveRecurrenceCursor(direction)
	case "subtask":
		m.moveSubtaskTypeCursor(idx, direction)
	case "attrs":
		m.handleAttrNav(idx, direction)
	default:
		return nil, nil
	}
	return m, nil
}

func (m *TaskCreatorModel) moveTypeCursor(direction int) tea.Cmd {
	newCursor := m.typeCursor + direction
	if newCursor < 0 || newCursor >= len(m.types) {
		return nil
	}

	newType := m.types[newCursor]
	if m.hasNonEmptyAttrs() && newType != m.taskType {
		m.confirmTypeChange = true
		m.pendingTypeCursor = newCursor
		m.pendingTaskType = newType
		return nil
	}

	m.typeCursor = newCursor
	m.taskType = newType
	m.updateValidParents()
	return nil
}

func (m *TaskCreatorModel) moveParentCursor(direction int) {
	newCursor := m.parentCursor + direction
	if newCursor < 0 || newCursor > len(m.validParents) {
		return
	}

	m.parentCursor = newCursor
	if m.parentCursor == 0 {
		m.parentID = nil
		return
	}

	m.parentID = &m.validParents[m.parentCursor-1].ID
}

func (m *TaskCreatorModel) movePriority(direction int) {
	newPriority := m.priority + direction
	if newPriority < 1 || newPriority > 5 {
		return
	}
	m.priority = newPriority
}

func (m *TaskCreatorModel) moveSectionCursor(direction int) {
	newCursor := m.sectionCursor + direction
	if newCursor < 0 || newCursor >= len(m.sections) {
		return
	}
	m.sectionCursor = newCursor
	m.section = m.sections[m.sectionCursor]
}

func (m *TaskCreatorModel) moveFlagCursor(direction int) {
	newCursor := m.flagCursor + direction
	if newCursor < 0 || newCursor > 3 {
		return
	}
	m.flagCursor = newCursor
	m.updateFlagsFromCursor()
}

func (m *TaskCreatorModel) moveRecurrenceCursor(direction int) tea.Cmd {
	newCursor := m.recurrenceCursor + direction
	if newCursor < 0 || newCursor >= len(m.recurrenceOpts) {
		return nil
	}

	m.recurrenceCursor = newCursor
	if m.recurrenceCursor == 6 {
		return m.showRecurrenceEditor()
	}

	m.updateRecurrenceFromCursor()
	return nil
}

func (m *TaskCreatorModel) showRecurrenceEditor() tea.Cmd {
	return func() tea.Msg {
		return ShowRecurrenceEditorMsg{
			CurrentPattern: m.recurrence,
			DueDate:        m.dueDate,
		}
	}
}

func (m *TaskCreatorModel) moveSubtaskTypeCursor(idx, direction int) {
	if idx < 0 || idx >= len(m.subtasks) {
		return
	}
	allowedTypes := m.getAllowedSubtaskTypes()
	newCursor := m.subtasks[idx].typeCursor + direction
	if newCursor < 0 {
		newCursor = 0
	} else if newCursor >= len(allowedTypes) {
		newCursor = len(allowedTypes) - 1
	}
	m.subtasks[idx].typeCursor = newCursor
}
