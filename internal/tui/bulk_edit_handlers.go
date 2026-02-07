package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleTagAddInput handles input when adding a tag.
func (m *BulkEditModel) handleTagAddInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.tagMode = 0
		m.tagAddInput.Blur()
		return m, nil

	case "enter":
		tag := strings.TrimSpace(m.tagAddInput.Value())
		if tag != "" {
			// Check if already in add list
			found := false
			for _, t := range m.addTags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				m.addTags = append(m.addTags, tag)
				sort.Strings(m.addTags)
			}
		}
		m.tagMode = 0
		m.tagAddInput.Blur()
		return m, nil

	default:
		var cmd tea.Cmd
		m.tagAddInput, cmd = m.tagAddInput.Update(msg)
		return m, cmd
	}
}

// handleDueDateInput handles input when setting a due date.
func (m *BulkEditModel) handleDueDateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dueDateInputMode = false
		m.dueDateInput.Blur()
		m.dueDateError = nil
		return m, nil

	case "enter":
		value := strings.TrimSpace(m.dueDateInput.Value())
		if value == "" {
			// Empty input, stay in input mode
			return m, nil
		}

		// Parse the date using the existing ParseDate function
		date, err := ParseDate(value)
		if err != nil {
			m.dueDateError = err
			return m, nil
		}

		// Success - store the date and exit input mode
		m.setDueDate = &date
		m.clearDueDate = false
		m.dueDateInputMode = false
		m.dueDateInput.Blur()
		m.dueDateError = nil
		return m, nil

	default:
		// Clear error when user types
		m.dueDateError = nil
		var cmd tea.Cmd
		m.dueDateInput, cmd = m.dueDateInput.Update(msg)
		return m, cmd
	}
}

// handleEnter handles the enter key based on current field.
func (m *BulkEditModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focusIndex {
	case fieldStatus:
		if m.statusCursor == 0 {
			m.statusValue = nil
		} else {
			statuses := []models.TaskStatus{
				models.StatusTodo,
				models.StatusInProgress,
				models.StatusReview,
				models.StatusDone,
				models.StatusBlocked,
				models.StatusCanceled,
			}
			if m.statusCursor <= len(statuses) {
				s := statuses[m.statusCursor-1]
				m.statusValue = &s
			}
		}

	case fieldPriority:
		if m.priorityCursor == 0 {
			m.priorityValue = nil
		} else {
			p := m.priorityCursor
			m.priorityValue = &p
		}

	case fieldSection:
		if m.sectionCursor == 0 {
			m.sectionValue = nil
		} else {
			idx := m.sectionCursor - 1
			if idx < len(m.sectionOptions) {
				s := m.sectionOptions[idx]
				m.sectionValue = &s
			}
		}

	case fieldDueDate:
		switch m.dueDateCursor {
		case 0:
			// Keep current - clear any set date
			m.clearDueDate = false
			m.setDueDate = nil
		case 1:
			// Set date - enter input mode
			m.dueDateInputMode = true
			m.dueDateInput.Reset()
			m.dueDateInput.Focus()
			m.dueDateError = nil
			return m, textinput.Blink
		case 2:
			// Clear due date
			m.clearDueDate = true
			m.setDueDate = nil
		}

	case fieldActions:
		if m.actionCursor == 0 {
			return m, m.apply()
		}
		return m, m.cancel()
	}

	return m, nil
}

// handleSpace handles the space key for toggling.
func (m *BulkEditModel) handleSpace() (tea.Model, tea.Cmd) {
	switch m.focusIndex {
	case fieldTags:
		if m.tagMode == 2 && m.tagRemoveCursor < len(m.allTags) {
			tag := m.allTags[m.tagRemoveCursor]
			m.removeTags[tag] = !m.removeTags[tag]
			if !m.removeTags[tag] {
				delete(m.removeTags, tag)
			}
		}
	}
	return m, nil
}

// handleRight moves cursor right within current field.
func (m *BulkEditModel) handleRight() {
	switch m.focusIndex {
	case fieldStatus:
		if m.statusCursor < 5 {
			m.statusCursor++
		}
	case fieldPriority:
		if m.priorityCursor < 5 {
			m.priorityCursor++
		}
	case fieldSection:
		if m.sectionCursor < len(m.sectionOptions) {
			m.sectionCursor++
		}
	case fieldDueDate:
		if m.dueDateCursor < 2 {
			m.dueDateCursor++
		}
	case fieldActions:
		if m.actionCursor < 1 {
			m.actionCursor++
		}
	}
}

// handleLeft moves cursor left within current field.
func (m *BulkEditModel) handleLeft() {
	switch m.focusIndex {
	case fieldStatus:
		if m.statusCursor > 0 {
			m.statusCursor--
		}
	case fieldPriority:
		if m.priorityCursor > 0 {
			m.priorityCursor--
		}
	case fieldSection:
		if m.sectionCursor > 0 {
			m.sectionCursor--
		}
	case fieldDueDate:
		if m.dueDateCursor > 0 {
			m.dueDateCursor--
		}
	case fieldActions:
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	}
}

// apply returns a command that applies the changes.
func (m *BulkEditModel) apply() tea.Cmd {
	changes := m.buildChanges()
	return func() tea.Msg {
		return BulkEditDoneMsg{
			Changes:   changes,
			Tasks:     m.tasks,
			Project:   m.project,
			TaskCount: len(m.tasks),
			Applied:   true,
		}
	}
}

// cancel returns a command that cancels the bulk edit.
func (m *BulkEditModel) cancel() tea.Cmd {
	return func() tea.Msg {
		return BulkEditDoneMsg{
			Applied: false,
		}
	}
}

// buildChanges creates a PendingChanges struct from current selections.
func (m *BulkEditModel) buildChanges() PendingChanges {
	changes := PendingChanges{
		SetStatus:    m.statusValue,
		SetPriority:  m.priorityValue,
		AddTags:      m.addTags,
		SetSection:   m.sectionValue,
		ClearDueDate: m.clearDueDate,
		SetDueDate:   m.setDueDate,
	}

	// Collect remove tags
	for tag := range m.removeTags {
		changes.RemoveTags = append(changes.RemoveTags, tag)
	}
	sort.Strings(changes.RemoveTags)

	return changes
}
