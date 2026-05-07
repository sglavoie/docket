package tui

import (
	"slices"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleTagAddInput handles input when adding a tag.
func (m *BulkEditModel) handleTagAddInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.tagMode = 0
		m.tagAddInput.Blur()
		return m, nil

	case "enter":
		tag := strings.TrimSpace(m.tagAddInput.Value())
		if tag != "" {
			// Check if already in add list
			found := slices.Contains(m.addTags, tag)
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
func (m *BulkEditModel) handleDueDateInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

// handleParentListInput handles input when browsing the parent candidate list.
func (m *BulkEditModel) handleParentListInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.parentListMode = false
		return m, nil

	case "j", "down":
		if m.parentListCursor < len(m.parentCandidates)-1 {
			m.parentListCursor++
			// Scroll if cursor goes below visible window
			maxVisible := 8
			if m.parentListCursor >= m.parentListOffset+maxVisible {
				m.parentListOffset = m.parentListCursor - maxVisible + 1
			}
		}
		m.ensureFocusedFieldVisible()
		return m, nil

	case "k", "up":
		if m.parentListCursor > 0 {
			m.parentListCursor--
			if m.parentListCursor < m.parentListOffset {
				m.parentListOffset = m.parentListCursor
			}
		}
		m.ensureFocusedFieldVisible()
		return m, nil

	case "enter":
		if m.parentListCursor < len(m.parentCandidates) {
			id := m.parentCandidates[m.parentListCursor].ID
			m.parentValue = &id
			m.clearParent = false
		}
		m.parentListMode = false
		return m, nil
	}

	return m, nil
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
				models.StatusStandby,
				models.StatusMaybe,
				models.StatusDone,
				models.StatusBlocked,
				models.StatusCanceled,
			}
			if m.statusCursor <= len(statuses) {
				s := statuses[m.statusCursor-1]
				m.statusValue = &s
			}
		}

	case fieldType:
		if m.typeCursor == 0 {
			m.typeValue = nil
		} else {
			types := []models.TaskType{
				models.TaskTypeEpic,
				models.TaskTypeStory,
				models.TaskTypeTask,
				models.TaskTypeBug,
				models.TaskTypeIdea,
			}
			if m.typeCursor <= len(types) {
				tp := types[m.typeCursor-1]
				m.typeValue = &tp
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

	case fieldRecurrence:
		presets := []models.RecurrencePattern{
			{Interval: 1, Unit: models.RecurrenceDaily},
			{Interval: 1, Unit: models.RecurrenceWeekly},
			{Interval: 2, Unit: models.RecurrenceWeekly},
			{Interval: 1, Unit: models.RecurrenceMonthly},
			{Interval: 1, Unit: models.RecurrenceYearly},
		}
		switch {
		case m.recurrenceCursor == 0:
			// Keep current
			m.recurrenceValue = nil
			m.clearRecurrence = false
		case m.recurrenceCursor >= 1 && m.recurrenceCursor <= len(presets):
			p := presets[m.recurrenceCursor-1]
			m.recurrenceValue = &p
			m.clearRecurrence = false
		case m.recurrenceCursor == len(presets)+1:
			// Clear recurrence
			m.recurrenceValue = nil
			m.clearRecurrence = true
		}

	case fieldParent:
		switch m.parentCursor {
		case 0:
			// Keep current
			m.parentValue = nil
			m.clearParent = false
		case 1:
			// Set parent - enter list mode
			if len(m.parentCandidates) > 0 {
				m.parentListMode = true
				m.parentListCursor = 0
				m.parentListOffset = 0
			}
		case 2:
			// Move to root
			m.clearParent = true
			m.parentValue = nil
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
		if m.statusCursor < 7 {
			m.statusCursor++
		}
	case fieldType:
		if m.typeCursor < 5 {
			m.typeCursor++
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
	case fieldRecurrence:
		if m.recurrenceCursor < 6 {
			m.recurrenceCursor++
		}
	case fieldParent:
		if m.parentCursor < 2 {
			m.parentCursor++
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
	case fieldType:
		if m.typeCursor > 0 {
			m.typeCursor--
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
	case fieldRecurrence:
		if m.recurrenceCursor > 0 {
			m.recurrenceCursor--
		}
	case fieldParent:
		if m.parentCursor > 0 {
			m.parentCursor--
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
	tasks := m.tasks
	project := m.project
	taskCount := len(m.tasks)
	return func() tea.Msg {
		return BulkEditDoneMsg{
			Changes:   changes,
			Tasks:     tasks,
			Project:   project,
			TaskCount: taskCount,
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
		SetStatus:       m.statusValue,
		SetType:         m.typeValue,
		SetPriority:     m.priorityValue,
		AddTags:         m.addTags,
		SetSection:      m.sectionValue,
		ClearDueDate:    m.clearDueDate,
		SetDueDate:      m.setDueDate,
		SetRecurrence:   m.recurrenceValue,
		ClearRecurrence: m.clearRecurrence,
		SetParentID:     m.parentValue,
		ClearParent:     m.clearParent,
	}

	// Collect remove tags
	for tag := range m.removeTags {
		changes.RemoveTags = append(changes.RemoveTags, tag)
	}
	sort.Strings(changes.RemoveTags)

	return changes
}
