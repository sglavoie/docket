package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if msg.FieldName == "description" {
				m.descValue = msg.Value
			}
		}
		return m, nil

	case tea.KeyMsg:
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

		case "[":
			// Switch to previous tab
			m.blurCurrentField()
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				// Wrap to last available tab
				if m.currentTypeCanHaveChildren() {
					m.activeTab = creatorTabSubtasks
				} else {
					m.activeTab = creatorTabChecklist
				}
			}
			m.focusIndex = m.getFirstFocusIndexForTab(m.activeTab)
			m.scrollOffset = 0
			cmd := m.focusCurrentField()
			return m, cmd

		case "]":
			// Switch to next tab
			m.blurCurrentField()
			maxTab := creatorTabChecklist
			if m.currentTypeCanHaveChildren() {
				maxTab = creatorTabSubtasks
			}
			if m.activeTab < maxTab {
				m.activeTab++
			} else {
				m.activeTab = creatorTabFields
			}
			m.focusIndex = m.getFirstFocusIndexForTab(m.activeTab)
			m.scrollOffset = 0
			cmd := m.focusCurrentField()
			return m, cmd

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Navigate within current tab
			tabStart := m.getFirstFocusIndexForTab(m.activeTab)
			tabCount := m.getFocusCountForTab(m.activeTab)

			if tabCount == 0 {
				// Empty tab, stay at current position
				cmd := m.focusCurrentField()
				return m, cmd
			}

			// Calculate relative position within tab
			relIdx := m.focusIndex - tabStart
			if relIdx < 0 {
				relIdx = 0
			}

			if msg.String() == "tab" {
				relIdx = (relIdx + 1) % tabCount
			} else {
				relIdx = (relIdx + tabCount - 1) % tabCount
			}

			m.focusIndex = tabStart + relIdx

			// Update active tab based on new focus index
			m.activeTab = m.getTabForFocusIndex(m.focusIndex)

			// Focus new field
			cmd := m.focusCurrentField()
			return m, cmd

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
			cmd := m.focusCurrentField()
			return m, cmd

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
				cmd := m.focusCurrentField()
				return m, cmd
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
				cmd := m.focusCurrentField()
				return m, cmd
			}
			if section == "subtask" && len(m.subtasks) > 0 {
				m.subtasks = append(m.subtasks[:idx], m.subtasks[idx+1:]...)
				if m.focusIndex >= m.focusFieldCount() {
					m.focusIndex = m.focusFieldCount() - 1
				}
				if m.focusIndex < 0 {
					m.focusIndex = 0
				}
				cmd := m.focusCurrentField()
				return m, cmd
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
					m.typeCursor--
					m.taskType = m.types[m.typeCursor]
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
				if m.flagFocus > 0 {
					m.flagFocus--
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
			}

		case "right", "l":
			switch section {
			case "type":
				if m.typeCursor < len(m.types)-1 {
					m.typeCursor++
					m.taskType = m.types[m.typeCursor]
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
				if m.flagFocus < 1 {
					m.flagFocus++
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
			}

		case " ":
			// Toggle flags with space
			if m.focusIndex == 6 {
				if m.flagFocus == 0 {
					m.pinned = !m.pinned
				} else {
					m.active = !m.active
				}
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
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
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

func (m *TaskCreatorModel) blurCurrentField() {
	section, idx := m.getFocusSection()
	switch section {
	case "title":
		m.titleInput.Blur()
	case "checklist":
		if idx >= 0 && idx < len(m.checklist) {
			m.checklist[idx].Blur()
		}
	case "subtask":
		if idx >= 0 && idx < len(m.subtasks) {
			m.subtasks[idx].titleInput.Blur()
		}
	}
}

func (m *TaskCreatorModel) focusCurrentField() tea.Cmd {
	section, idx := m.getFocusSection()
	switch section {
	case "title":
		m.titleInput.Focus()
	case "checklist":
		if idx >= 0 && idx < len(m.checklist) {
			m.checklist[idx].Focus()
		}
	case "subtask":
		if idx >= 0 && idx < len(m.subtasks) {
			m.subtasks[idx].titleInput.Focus()
		}
	}
	return nil
}

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

func (m *TaskCreatorModel) createTask() tea.Cmd {
	return func() tea.Msg {
		// Replace newlines with spaces since titles should be single logical lines
		title := strings.ReplaceAll(m.titleInput.Value(), "\n", " ")
		title = strings.TrimSpace(title)
		if title == "" {
			return ErrorMsg{Err: fmt.Errorf("task title cannot be empty")}
		}

		// Parse due date before saving
		if err := m.parseDueDate(); err != nil {
			return ErrorMsg{Err: err}
		}

		now := time.Now()

		// Create task
		task := &models.Task{
			ID:          uuid.New().String(),
			ShortID:     m.project.GetNextShortID(),
			Type:        m.taskType,
			Title:       title,
			Description: strings.TrimSpace(m.descValue),
			Status:      models.StatusTodo,
			Priority:    m.priority,
			ParentID:    m.parentID,
			Section:     m.section,
			DueDate:     m.dueDate,
			Pinned:      m.pinned,
			Active:      m.active,
			Recurrence:  m.recurrence,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		// Set PinnedAt if pinned
		if m.pinned {
			task.PinnedAt = &now
		}

		// Set tags from the selector
		task.Tags = append([]string{}, m.tags...)

		// Add task to project
		m.project.Tasks = append(m.project.Tasks, task)

		// Build checklist from form-edited checklist items
		for _, item := range m.checklist {
			text := strings.TrimSpace(item.Value())
			if text != "" {
				task.Checklist = append(task.Checklist, models.ChecklistItem{
					Text: text,
					Done: false,
				})
			}
		}

		// Create subtasks from form (Epic/Story only)
		if m.currentTypeCanHaveChildren() {
			allowedTypes := m.getAllowedSubtaskTypes()
			for _, st := range m.subtasks {
				subtaskTitle := strings.TrimSpace(st.titleInput.Value())
				// Skip optional subtasks with empty titles
				if subtaskTitle == "" {
					if st.optional {
						continue
					}
					// Non-optional subtasks with empty titles are skipped too
					continue
				}

				// Get the subtask type, clamping to valid range
				typeCursor := st.typeCursor
				if typeCursor >= len(allowedTypes) {
					typeCursor = 0
				}

				subtask := &models.Task{
					ID:        uuid.New().String(),
					ShortID:   m.project.GetNextShortID(),
					Type:      allowedTypes[typeCursor],
					Title:     subtaskTitle,
					Status:    models.StatusTodo,
					Priority:  m.priority, // Inherit from parent
					ParentID:  &task.ID,
					Section:   m.section, // Inherit from parent
					CreatedAt: now,
					UpdatedAt: now,
				}

				// Inherit tags from parent
				subtask.Tags = append([]string{}, m.tags...)

				m.project.Tasks = append(m.project.Tasks, subtask)
			}
		}

		// Increment template use count if from template
		if m.fromTemplate != nil && m.config != nil && m.configManager != nil {
			m.config.IncrementTemplateUseCount(m.fromTemplate.ID)
			_ = m.configManager.Save(m.config)
		}

		m.project.BuildTaskTree()
		m.project.ReorderRootTasksByCompletion()

		// Save project with backup for undo support
		if err := saveProjectSync(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TaskCreatedMsg{Task: task}
	}
}
