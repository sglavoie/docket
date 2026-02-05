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

	case tea.KeyMsg:
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
					m.activeTab = creatorTabAttrs
				}
			}
			m.focusIndex = m.getFirstFocusIndexForTab(m.activeTab)
			m.scrollOffset = 0
			cmdFocus := m.focusCurrentField()
			return m, cmdFocus

		case "]":
			// Switch to next tab
			m.blurCurrentField()
			maxTab := creatorTabAttrs
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
			cmdFocus := m.focusCurrentField()
			return m, cmdFocus

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Navigate within current tab
			tabStart := m.getFirstFocusIndexForTab(m.activeTab)
			tabCount := m.getFocusCountForTab(m.activeTab)

			if tabCount == 0 {
				// Empty tab, stay at current position
				cmdFocus := m.focusCurrentField()
				return m, cmdFocus
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
			cmdFocus := m.focusCurrentField()
			return m, cmdFocus

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

		case " ":
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

// handleAttrLeft handles left arrow key for attribute fields.
func (m *TaskCreatorModel) handleAttrLeft(fieldIdx int) {
	switch m.taskType {
	case models.TaskTypeBug:
		switch fieldIdx {
		case 0: // Reproducible
			if m.bugReproducibleCursor > 0 {
				m.bugReproducibleCursor--
			}
		case 1: // Severity
			if m.bugSeverityCursor > 0 {
				m.bugSeverityCursor--
			}
		}
	case models.TaskTypeIdea:
		switch fieldIdx {
		case 0: // Category
			if m.ideaCategoryCursor > 0 {
				m.ideaCategoryCursor--
			}
		case 1: // Effort
			if m.ideaEffortCursor > 0 {
				m.ideaEffortCursor--
			}
		case 2: // Value
			if m.ideaValueCursor > 0 {
				m.ideaValueCursor--
			}
		}
	case models.TaskTypeTask:
		switch fieldIdx {
		case 0: // Effort
			if m.taskEffortCursor > 0 {
				m.taskEffortCursor--
			}
		case 1: // WorkType
			if m.taskWorkTypeCursor > 0 {
				m.taskWorkTypeCursor--
			}
		}
	case models.TaskTypeStory:
		switch fieldIdx {
		case 0: // Story Points
			if m.storyPointsCursor > 0 {
				m.storyPointsCursor--
			}
		}
	case models.TaskTypeEpic:
		switch fieldIdx {
		case 0: // Scope
			if m.epicScopeCursor > 0 {
				m.epicScopeCursor--
			}
		}
	}
}

// handleAttrRight handles right arrow key for attribute fields.
func (m *TaskCreatorModel) handleAttrRight(fieldIdx int) {
	switch m.taskType {
	case models.TaskTypeBug:
		switch fieldIdx {
		case 0: // Reproducible
			if m.bugReproducibleCursor < len(bugReproducibleOptions)-1 {
				m.bugReproducibleCursor++
			}
		case 1: // Severity
			if m.bugSeverityCursor < len(bugSeverityOptions)-1 {
				m.bugSeverityCursor++
			}
		}
	case models.TaskTypeIdea:
		switch fieldIdx {
		case 0: // Category
			if m.ideaCategoryCursor < len(ideaCategoryOptions)-1 {
				m.ideaCategoryCursor++
			}
		case 1: // Effort
			if m.ideaEffortCursor < len(effortOptions)-1 {
				m.ideaEffortCursor++
			}
		case 2: // Value
			if m.ideaValueCursor < len(ideaValueOptions)-1 {
				m.ideaValueCursor++
			}
		}
	case models.TaskTypeTask:
		switch fieldIdx {
		case 0: // Effort
			if m.taskEffortCursor < len(effortOptions)-1 {
				m.taskEffortCursor++
			}
		case 1: // WorkType
			if m.taskWorkTypeCursor < len(workTypeOptions)-1 {
				m.taskWorkTypeCursor++
			}
		}
	case models.TaskTypeStory:
		switch fieldIdx {
		case 0: // Story Points
			if m.storyPointsCursor < len(storyPointsOptions)-1 {
				m.storyPointsCursor++
			}
		}
	case models.TaskTypeEpic:
		switch fieldIdx {
		case 0: // Scope
			if m.epicScopeCursor < len(epicScopeOptions)-1 {
				m.epicScopeCursor++
			}
		}
	}
}

// handleAttrEnter handles enter key for text attribute fields.
func (m *TaskCreatorModel) handleAttrEnter(fieldIdx int) tea.Cmd {
	switch m.taskType {
	case models.TaskTypeBug:
		if fieldIdx == 2 { // Workaround
			return func() tea.Msg {
				return ShowTextEditorMsg{
					FieldName:    "bugWorkaround",
					CurrentValue: m.bugWorkaround,
				}
			}
		}
	case models.TaskTypeStory:
		if fieldIdx == 1 { // Persona
			return func() tea.Msg {
				return ShowTextEditorMsg{
					FieldName:    "storyPersona",
					CurrentValue: m.storyPersona,
				}
			}
		}
	case models.TaskTypeEpic:
		if fieldIdx == 1 { // Milestone
			return func() tea.Msg {
				return ShowTextEditorMsg{
					FieldName:    "epicMilestone",
					CurrentValue: m.epicMilestone,
				}
			}
		}
		if fieldIdx == 2 { // Success Metrics
			return func() tea.Msg {
				return ShowTextEditorMsg{
					FieldName:    "epicSuccessMetrics",
					CurrentValue: m.epicSuccessMetrics,
				}
			}
		}
	}
	return nil
}

// buildTaskAttrs creates the appropriate attrs struct from form state.
func (m *TaskCreatorModel) buildTaskAttrs() (bugAttrs *models.BugAttrs, ideaAttrs *models.IdeaAttrs, taskAttrs *models.TaskAttrs, storyAttrs *models.StoryAttrs, epicAttrs *models.EpicAttrs) {
	switch m.taskType {
	case models.TaskTypeBug:
		attrs := &models.BugAttrs{}
		if m.bugReproducibleCursor > 0 {
			attrs.Reproducible = models.ValidBugReproducible[m.bugReproducibleCursor-1]
		}
		if m.bugSeverityCursor > 0 {
			attrs.Severity = models.ValidBugSeverity[m.bugSeverityCursor-1]
		}
		attrs.Workaround = m.bugWorkaround
		if !attrs.IsEmpty() {
			bugAttrs = attrs
		}
	case models.TaskTypeIdea:
		attrs := &models.IdeaAttrs{}
		if m.ideaCategoryCursor > 0 {
			attrs.Category = models.ValidIdeaCategory[m.ideaCategoryCursor-1]
		}
		if m.ideaEffortCursor > 0 {
			attrs.Effort = models.ValidEffortEstimate[m.ideaEffortCursor-1]
		}
		if m.ideaValueCursor > 0 {
			attrs.Value = models.ValidIdeaValue[m.ideaValueCursor-1]
		}
		attrs.Validated = m.ideaValidated
		if !attrs.IsEmpty() {
			ideaAttrs = attrs
		}
	case models.TaskTypeTask:
		attrs := &models.TaskAttrs{}
		if m.taskEffortCursor > 0 {
			attrs.Effort = models.ValidEffortEstimate[m.taskEffortCursor-1]
		}
		if m.taskWorkTypeCursor > 0 {
			attrs.WorkType = models.ValidWorkType[m.taskWorkTypeCursor-1]
		}
		if !attrs.IsEmpty() {
			taskAttrs = attrs
		}
	case models.TaskTypeStory:
		attrs := &models.StoryAttrs{}
		if m.storyPointsCursor > 0 {
			attrs.StoryPoints = storyPointsValues[m.storyPointsCursor]
		}
		attrs.Persona = m.storyPersona
		if !attrs.IsEmpty() {
			storyAttrs = attrs
		}
	case models.TaskTypeEpic:
		attrs := &models.EpicAttrs{}
		if m.epicScopeCursor > 0 {
			attrs.Scope = models.ValidEpicScope[m.epicScopeCursor-1]
		}
		attrs.Milestone = m.epicMilestone
		attrs.SuccessMetrics = m.epicSuccessMetrics
		if !attrs.IsEmpty() {
			epicAttrs = attrs
		}
	}
	return
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

		// Build type-specific attributes
		bugAttrs, ideaAttrs, taskAttrs, storyAttrs, epicAttrs := m.buildTaskAttrs()

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
			BugAttrs:    bugAttrs,
			IdeaAttrs:   ideaAttrs,
			TaskAttrs:   taskAttrs,
			StoryAttrs:  storyAttrs,
			EpicAttrs:   epicAttrs,
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
