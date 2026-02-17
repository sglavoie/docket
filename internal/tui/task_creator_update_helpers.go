package tui

import (
	"strings"

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

// updateFlagsFromCursor updates pinned/active based on cursor position.
func (m *TaskCreatorModel) updateFlagsFromCursor() {
	switch m.flagCursor {
	case 0: // None
		m.pinned = false
		m.active = false
	case 1: // Pin
		m.pinned = true
		m.active = false
	case 2: // Active
		m.pinned = false
		m.active = true
	}
}
