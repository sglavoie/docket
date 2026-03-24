package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ParentSelectedMsg:
		// Handle parent selection from modal
		if msg.Source == ViewTaskCreator {
			m.parentID = msg.ParentID
			// Update cursor position to match
			if msg.ParentID == nil {
				m.parentCursor = 0
			} else {
				for i, p := range m.validParents {
					if p.ID == *msg.ParentID {
						m.parentCursor = i + 1
						break
					}
				}
			}
		}
		return m, nil

	case RecurrenceEditorDoneMsg:
		if !msg.Canceled && msg.Pattern != nil {
			m.recurrence = msg.Pattern
			// Update cursor to "Custom..." since we have a custom pattern
			m.recurrenceCursor = 6
		}
		return m, nil

	case TagSelectorDoneMsg:
		if !msg.Canceled {
			m.tags = msg.SelectedTags
		}
		return m, nil

	case TextEditorDoneMsg:
		if !msg.Canceled {
			switch msg.FieldName {
			case "description":
				m.descValue = msg.Value
			case "bugWorkaround":
				m.bugWorkaround = msg.Value
			case "storyPersona":
				m.storyPersona = msg.Value
			case "epicMilestone":
				m.epicMilestone = msg.Value
			case "epicSuccessMetrics":
				m.epicSuccessMetrics = msg.Value
			}
		}
		return m, nil

	case tea.KeyPressMsg:
		// Handle type change confirmation
		if m.confirmTypeChange {
			switch msg.String() {
			case "y", "Y":
				m.confirmTypeChange = false
				m.clearCurrentAttrs()
				m.typeCursor = m.pendingTypeCursor
				m.taskType = m.pendingTaskType
				m.updateValidParents()
				return m, nil
			case "n", "N", "esc":
				m.confirmTypeChange = false
				return m, nil
			}
			return m, nil
		}

		// Handle discard confirmation
		if m.confirmDiscard {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg {
					return BackMsg{}
				}
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}

		section, idx := m.getFocusSection()

		switch msg.String() {
		case "esc":
			// Show confirmation if title has content
			title := strings.TrimSpace(m.titleInput.Value())
			if title != "" {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "[", "]":
			lastTab := creatorTabAttrs
			if m.currentTypeCanHaveChildren() {
				lastTab = creatorTabSubtasks
			}
			s := formTabState{&m.activeTab, &m.focusIndex, &m.scrollOffset}
			return m, handleFormTabSwitch(m, s, creatorTabFields, lastTab, msg.String() == "]")

		case "tab", "shift+tab":
			s := formTabState{&m.activeTab, &m.focusIndex, &m.scrollOffset}
			return m, handleFormTabKey(m, s, msg.String() == "tab")

		case "enter":
			// For due date field, parse and validate on enter
			if m.focusIndex == 5 {
				if err := m.parseDueDate(); err != nil {
					m.err = err
					return m, nil
				}
				m.err = nil
			}
			if m.focusIndex == 8 {
				// Open tag selector modal
				return m, func() tea.Msg {
					return ShowTagSelectorMsg{
						Task:        nil,
						Project:     m.project,
						CurrentTags: m.tags,
					}
				}
			}
			if m.focusIndex == 9 {
				// Open description editor popup
				return m, func() tea.Msg {
					return ShowTextEditorMsg{
						FieldName:    "description",
						CurrentValue: m.descValue,
					}
				}
			}
			// Handle text attribute fields on enter
			if section == "attrs" {
				if cmdEnter := m.handleAttrEnter(idx); cmdEnter != nil {
					return m, cmdEnter
				}
			}
			// Move to next field for other fields in Fields tab
			if m.activeTab == creatorTabFields && m.focusIndex < creatorFieldCount-1 {
				m.blurCurrentField()
				m.focusIndex++
				m.focusCurrentField()
				return m, nil
			}

		case "ctrl+s":
			// Submit form
			return m, m.createTask()

		case "ctrl+a":
			// Add checklist item (works from any tab)
			m.err = nil
			m.blurCurrentField()
			m.activeTab = creatorTabChecklist
			factory := DefaultFormFieldFactory()
			input := factory.NewTextInput("Checklist item")
			m.checklist = append(m.checklist, input)
			// Move focus to the new checklist item
			m.focusIndex = m.checklistStartIndex() + len(m.checklist) - 1
			m.scrollOffset = 0
			cmdFocus := m.focusCurrentField()
			return m, cmdFocus

		case "ctrl+t":
			// Context-dependent behavior:
			// - On Fields/Checklist tab: open tag selector
			// - On Subtasks tab (Epic/Story only): add new subtask
			if m.activeTab == creatorTabSubtasks && m.currentTypeCanHaveChildren() {
				// Add new subtask
				m.err = nil
				m.blurCurrentField()
				factory := DefaultFormFieldFactory()
				input := factory.NewTextInput("Subtask title")
				// Default to first allowed subtask type (Task)
				m.subtasks = append(m.subtasks, SubtaskFormState{titleInput: input, typeCursor: 0, optional: false})
				// Move focus to the new subtask
				m.focusIndex = m.subtaskStartIndex() + len(m.subtasks) - 1
				m.scrollOffset = 0
				cmdFocus := m.focusCurrentField()
				return m, cmdFocus
			}
			// Open tag selector from any field (Fields or Checklist tab)
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{
					Task:        nil,
					Project:     m.project,
					CurrentTags: m.tags,
				}
			}

		case "ctrl+d":
			// Delete current item (checklist or subtask)
			if section == "checklist" && len(m.checklist) > 0 {
				m.checklist = append(m.checklist[:idx], m.checklist[idx+1:]...)
				if m.focusIndex >= m.focusFieldCount() {
					m.focusIndex = m.focusFieldCount() - 1
				}
				if m.focusIndex < 0 {
					m.focusIndex = 0
				}
				cmdFocus := m.focusCurrentField()
				return m, cmdFocus
			}
			if section == "subtask" && len(m.subtasks) > 0 {
				m.subtasks = append(m.subtasks[:idx], m.subtasks[idx+1:]...)
				if m.focusIndex >= m.focusFieldCount() {
					m.focusIndex = m.focusFieldCount() - 1
				}
				if m.focusIndex < 0 {
					m.focusIndex = 0
				}
				cmdFocus := m.focusCurrentField()
				return m, cmdFocus
			}

		case "ctrl+k":
			// Move item up
			if section == "checklist" && idx > 0 && idx < len(m.checklist) {
				m.checklist[idx], m.checklist[idx-1] = m.checklist[idx-1], m.checklist[idx]
				m.focusIndex--
				return m, nil
			}
			if section == "subtask" && idx > 0 && idx < len(m.subtasks) {
				m.subtasks[idx], m.subtasks[idx-1] = m.subtasks[idx-1], m.subtasks[idx]
				m.focusIndex--
				return m, nil
			}

		case "ctrl+j":
			// Move item down
			if section == "checklist" && idx >= 0 && idx < len(m.checklist)-1 {
				m.checklist[idx], m.checklist[idx+1] = m.checklist[idx+1], m.checklist[idx]
				m.focusIndex++
				return m, nil
			}
			if section == "subtask" && idx >= 0 && idx < len(m.subtasks)-1 {
				m.subtasks[idx], m.subtasks[idx+1] = m.subtasks[idx+1], m.subtasks[idx]
				m.focusIndex++
				return m, nil
			}

		case "ctrl+o":
			// Toggle optional for subtask
			if section == "subtask" && idx >= 0 && idx < len(m.subtasks) {
				m.subtasks[idx].optional = !m.subtasks[idx].optional
				return m, nil
			}

		case "/":
			// Open parent selector modal when on parent field
			if m.focusIndex == 2 {
				return m, func() tea.Msg {
					return OpenParentSelectorMsg{
						ValidParents: m.validParents,
						CurrentID:    m.parentID,
						AllowNone:    true,
						Source:       ViewTaskCreator,
					}
				}
			}

		case "1", "2", "3", "4", "5":
			// Direct priority selection when priority field is focused
			if m.focusIndex == 3 {
				m.priority = int(msg.String()[0] - '0')
				return m, nil
			}
			// Otherwise fall through to let text inputs handle the number

		case "left", "h":
			switch section {
			case "type":
				if m.typeCursor > 0 {
					newCursor := m.typeCursor - 1
					newType := m.types[newCursor]
					// Check if changing type would lose attrs
					if m.hasNonEmptyAttrs() && newType != m.taskType {
						m.confirmTypeChange = true
						m.pendingTypeCursor = newCursor
						m.pendingTaskType = newType
						return m, nil
					}
					m.typeCursor = newCursor
					m.taskType = newType
					m.updateValidParents()
				}
				return m, nil
			case "parent":
				if m.parentCursor > 0 {
					m.parentCursor--
					if m.parentCursor == 0 {
						m.parentID = nil
					} else {
						m.parentID = &m.validParents[m.parentCursor-1].ID
					}
				}
				return m, nil
			case "priority":
				if m.priority > 1 {
					m.priority--
				}
				return m, nil
			case "section":
				if m.sectionCursor > 0 {
					m.sectionCursor--
					m.section = m.sections[m.sectionCursor]
				}
				return m, nil
			case "flags":
				if m.flagCursor > 0 {
					m.flagCursor--
					m.updateFlagsFromCursor()
				}
				return m, nil
			case "recurrence":
				if m.recurrenceCursor > 0 {
					m.recurrenceCursor--
					if m.recurrenceCursor == 6 {
						// "Custom..." selected - open the editor
						return m, func() tea.Msg {
							return ShowRecurrenceEditorMsg{
								CurrentPattern: m.recurrence,
								DueDate:        m.dueDate,
							}
						}
					}
					m.updateRecurrenceFromCursor()
				}
				return m, nil
			case "subtask":
				// Cycle subtask type left
				if idx >= 0 && idx < len(m.subtasks) {
					allowedTypes := m.getAllowedSubtaskTypes()
					if m.subtasks[idx].typeCursor > 0 {
						m.subtasks[idx].typeCursor--
					}
					// Clamp to valid range
					if m.subtasks[idx].typeCursor >= len(allowedTypes) {
						m.subtasks[idx].typeCursor = len(allowedTypes) - 1
					}
				}
				return m, nil
			case "attrs":
				m.handleAttrLeft(idx)
				return m, nil
			}

		case "right", "l":
			switch section {
			case "type":
				if m.typeCursor < len(m.types)-1 {
					newCursor := m.typeCursor + 1
					newType := m.types[newCursor]
					// Check if changing type would lose attrs
					if m.hasNonEmptyAttrs() && newType != m.taskType {
						m.confirmTypeChange = true
						m.pendingTypeCursor = newCursor
						m.pendingTaskType = newType
						return m, nil
					}
					m.typeCursor = newCursor
					m.taskType = newType
					m.updateValidParents()
				}
				return m, nil
			case "parent":
				if m.parentCursor < len(m.validParents) {
					m.parentCursor++
					m.parentID = &m.validParents[m.parentCursor-1].ID
				}
				return m, nil
			case "priority":
				if m.priority < 5 {
					m.priority++
				}
				return m, nil
			case "section":
				if m.sectionCursor < len(m.sections)-1 {
					m.sectionCursor++
					m.section = m.sections[m.sectionCursor]
				}
				return m, nil
			case "flags":
				if m.flagCursor < 2 {
					m.flagCursor++
					m.updateFlagsFromCursor()
				}
				return m, nil
			case "recurrence":
				if m.recurrenceCursor < len(m.recurrenceOpts)-1 {
					m.recurrenceCursor++
					if m.recurrenceCursor == 6 {
						// "Custom..." selected - open the editor
						return m, func() tea.Msg {
							return ShowRecurrenceEditorMsg{
								CurrentPattern: m.recurrence,
								DueDate:        m.dueDate,
							}
						}
					}
					m.updateRecurrenceFromCursor()
				}
				return m, nil
			case "subtask":
				// Cycle subtask type right
				if idx >= 0 && idx < len(m.subtasks) {
					allowedTypes := m.getAllowedSubtaskTypes()
					if m.subtasks[idx].typeCursor < len(allowedTypes)-1 {
						m.subtasks[idx].typeCursor++
					}
				}
				return m, nil
			case "attrs":
				m.handleAttrRight(idx)
				return m, nil
			}

		case "space":
			// Space is a no-op for flags (selection happens via left/right)
			if m.focusIndex == 6 {
				return m, nil
			}
			// Space toggles validated for idea attrs
			if section == "attrs" && m.taskType == models.TaskTypeIdea && idx == 3 {
				m.ideaValidated = !m.ideaValidated
				return m, nil
			}

		case "r":
			// Open recurrence editor when on recurrence field
			if m.focusIndex == 7 {
				return m, func() tea.Msg {
					return ShowRecurrenceEditorMsg{
						CurrentPattern: m.recurrence,
						DueDate:        m.dueDate,
					}
				}
			}
		}
	}

	// Update the focused input
	section, idx := m.getFocusSection()
	switch section {
	case "title":
		m.titleInput, cmd = m.titleInput.Update(msg)
	case "dueDate":
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			key := keyMsg.String()
			// Handle backspace
			if key == "backspace" && len(m.dueDateInput) > 0 {
				m.dueDateInput = m.dueDateInput[:len(m.dueDateInput)-1]
			} else if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
				// Printable ASCII characters
				m.dueDateInput += key
			}
		}
	case "checklist":
		if idx >= 0 && idx < len(m.checklist) {
			m.checklist[idx], cmd = m.checklist[idx].Update(msg)
		}
	case "subtask":
		if idx >= 0 && idx < len(m.subtasks) {
			m.subtasks[idx].titleInput, cmd = m.subtasks[idx].titleInput.Update(msg)
		}
	}

	return m, cmd
}
