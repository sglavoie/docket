package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// updateRecurrenceFromCursor updates the recurrence pattern based on cursor position.
func (m *TaskEditorModel) updateRecurrenceFromCursor() {
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
func (m *TaskEditorModel) updateFlagsFromCursor() {
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

// flagCursorFromFlags returns the cursor position based on pinned/active/dimmed flags.
func flagCursorFromFlags(pinned, active, dimmed bool) int {
	if pinned {
		return 1 // Pin
	}
	if active {
		return 2 // Active
	}
	if dimmed {
		return 3 // Dim
	}
	return 0 // None
}

// parseDueDate parses the due date input string and updates m.dueDate.
func (m *TaskEditorModel) parseDueDate() error {
	value := strings.TrimSpace(m.dueDateInput)

	if value == "" || strings.ToLower(value) == "none" || strings.ToLower(value) == "clear" {
		m.dueDate = nil
		return nil
	}

	date, err := ParseDate(value)
	if err != nil {
		return err
	}

	m.dueDate = &date
	return nil
}

// blurCurrentField removes focus from the currently focused field.
func (m *TaskEditorModel) blurCurrentField() {
	if m.focusIndex == 0 {
		m.titleInput.Blur()
	}
}

// focusCurrentField sets focus to the currently focused field.
func (m *TaskEditorModel) focusCurrentField() {
	if m.focusIndex == 0 {
		m.titleInput.Focus()
	}
}
