package tui

import "github.com/sglavoie/tt/internal/models"

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
		for i, parent := range m.validParents {
			if parent.ID == *m.parentID {
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
