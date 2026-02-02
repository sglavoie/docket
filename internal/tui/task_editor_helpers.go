package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// updateValidParents filters tasks that can be parents of the current type.
func (m *TaskEditorModel) updateValidParents() {
	m.validParents = []*models.Task{}

	for _, task := range m.project.Tasks {
		// Can't be parent of itself or its descendants
		if task.ID == m.task.ID {
			continue
		}
		if m.isDescendant(task, m.task) {
			continue
		}
		if task.CanAddChild(m.taskType) {
			m.validParents = append(m.validParents, task)
		}
	}

	// Find current parent cursor position
	m.parentCursor = 0
	if m.parentID != nil {
		for i, p := range m.validParents {
			if p.ID == *m.parentID {
				m.parentCursor = i + 1 // +1 because 0 is "None"
				break
			}
		}
		// If parent no longer valid, clear it
		if m.parentCursor == 0 {
			m.parentID = nil
		}
	}
}

// isDescendant checks if candidate is a descendant of task.
func (m *TaskEditorModel) isDescendant(candidate, task *models.Task) bool {
	for _, child := range task.Children {
		if child.ID == candidate.ID {
			return true
		}
		if m.isDescendant(candidate, child) {
			return true
		}
	}
	return false
}

// checkChanges detects if any field has been modified.
func (m *TaskEditorModel) checkChanges() bool {
	// Normalize title by replacing newlines with spaces (same as saveTask)
	currentTitle := strings.TrimSpace(strings.ReplaceAll(m.titleInput.Value(), "\n", " "))
	if currentTitle != m.originalTitle {
		return true
	}
	if strings.Join(m.tags, ", ") != m.originalTags {
		return true
	}
	if m.taskType != m.originalType {
		return true
	}
	if m.priority != m.originalPriority {
		return true
	}
	// Compare parent IDs
	if m.parentID == nil && m.originalParentID != nil {
		return true
	}
	if m.parentID != nil && m.originalParentID == nil {
		return true
	}
	if m.parentID != nil && m.originalParentID != nil && *m.parentID != *m.originalParentID {
		return true
	}
	// Check section
	if m.section != m.originalSection {
		return true
	}
	// Check due date
	if m.dueDate == nil && m.originalDueDate != nil {
		return true
	}
	if m.dueDate != nil && m.originalDueDate == nil {
		return true
	}
	if m.dueDate != nil && m.originalDueDate != nil && !m.dueDate.Equal(*m.originalDueDate) {
		return true
	}
	// Check flags
	if m.pinned != m.originalPinned {
		return true
	}
	if m.active != m.originalActive {
		return true
	}
	// Check recurrence
	if !recurrenceEqual(m.recurrence, m.originalRecurrence) {
		return true
	}
	return false
}

// recurrenceEqual compares two recurrence patterns for equality.
func recurrenceEqual(a, b *models.RecurrencePattern) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Interval != b.Interval || a.Unit != b.Unit {
		return false
	}
	// Compare weekdays
	if len(a.Weekdays) != len(b.Weekdays) {
		return false
	}
	for i, wd := range a.Weekdays {
		if wd != b.Weekdays[i] {
			return false
		}
	}
	return true
}

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

// parseDueDate parses the due date input string and updates m.dueDate.
func (m *TaskEditorModel) parseDueDate() error {
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

// saveTask saves the current task changes to the project.
func (m *TaskEditorModel) saveTask() error {
	// Replace newlines with spaces since titles should be single logical lines
	title := strings.ReplaceAll(m.titleInput.Value(), "\n", " ")
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("task title cannot be empty")
	}

	// Parse due date before saving
	if err := m.parseDueDate(); err != nil {
		return err
	}

	// Update the task in the project via MutateTask (auto-sets UpdatedAt)
	m.project.MutateTask(m.task.ID, func(t *models.Task) {
		t.Title = title
		t.Type = m.taskType
		t.Priority = m.priority
		t.ParentID = m.parentID
		if models.IsGTDSection(m.section) || m.section == "" {
			t.Section = m.section
		} else {
			t.CustomSection = m.section
		}
		t.DueDate = m.dueDate
		t.Tags = append([]string{}, m.tags...)
		t.Active = m.active
		t.Recurrence = m.recurrence

		// Handle pinned status change
		if m.pinned != t.Pinned {
			t.Pinned = m.pinned
			if m.pinned {
				now := time.Now()
				t.PinnedAt = &now
			} else {
				t.PinnedAt = nil
			}
		}
	})

	// Rebuild task tree
	m.project.BuildTaskTree()

	// Save project with backup for undo support
	if err := saveProjectSync(m.project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	return nil
}
