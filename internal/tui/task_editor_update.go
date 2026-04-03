package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	if model, cmd, handled := m.handleEditorMessage(msg); handled {
		return model, cmd
	}

	cmd := m.updateFocusedInput(msg)
	m.hasChanges = m.checkChanges()

	return m, cmd
}

func (m *TaskEditorModel) handleEditorMessage(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case TagSelectorDoneMsg:
		if !msg.Canceled {
			m.tags = msg.SelectedTags
			m.hasChanges = m.checkChanges()
		}
		return m, nil, true
	case RecurrenceEditorDoneMsg:
		if !msg.Canceled && msg.Pattern != nil {
			m.recurrence = msg.Pattern
			m.recurrenceCursor = 6
			m.hasChanges = m.checkChanges()
		}
		return m, nil, true
	case ParentSelectedMsg:
		if msg.Source == ViewTaskEditor {
			m.applyParentSelection(msg.ParentID)
			m.hasChanges = m.checkChanges()
		}
		return m, nil, true
	case TextEditorDoneMsg:
		if !msg.Canceled {
			m.applyTextEdit(msg.FieldName, msg.Value)
			m.hasChanges = m.checkChanges()
		}
		return m, nil, true
	case tea.KeyPressMsg:
		model, cmd, handled := m.handleKeyPress(msg)
		if handled {
			return model, cmd, true
		}
	}

	return nil, nil, false
}

func (m *TaskEditorModel) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if model, cmd, handled := m.handleConfirmationKey(msg.String()); handled {
		return model, cmd, true
	}

	switch msg.String() {
	case "esc":
		if m.checkChanges() {
			m.confirmDiscard = true
			return m, nil, true
		}
		return m, func() tea.Msg { return BackMsg{} }, true
	case "tab", "shift+tab":
		m.moveFocus(msg.String() == "tab")
		return m, nil, true
	case "enter":
		return m.handleEnterKey()
	case "ctrl+e":
		return m, func() tea.Msg { return EditTaskContentMsg{Task: m.task, Project: m.project} }, true
	case "ctrl+s":
		if err := m.saveTask(); err != nil {
			return m, func() tea.Msg { return ErrorMsg{Err: err} }, true
		}
		task := m.task
		return m, func() tea.Msg { return TaskUpdatedMsg{Task: task} }, true
	case "ctrl+t":
		return m, func() tea.Msg {
			return ShowTagSelectorMsg{Task: m.task, Project: m.project, CurrentTags: m.tags}
		}, true
	case "/":
		if m.focusIndex == 2 {
			return m, func() tea.Msg {
				return OpenParentSelectorMsg{
					ValidParents: m.validParents,
					CurrentID:    m.parentID,
					AllowNone:    true,
					Source:       ViewTaskEditor,
				}
			}, true
		}
	case "1", "2", "3", "4", "5":
		if m.focusIndex == 3 {
			m.priority = int(msg.String()[0] - '0')
			m.hasChanges = m.checkChanges()
			return m, nil, true
		}
	case "left", "h":
		return m.handleHorizontalMove(-1)
	case "right", "l":
		return m.handleHorizontalMove(1)
	case "space":
		if m.handleSpaceKey() {
			return m, nil, true
		}
	case "r":
		if m.focusIndex == 7 {
			return m, m.showRecurrenceEditor(), true
		}
	}

	return nil, nil, false
}

func (m *TaskEditorModel) handleConfirmationKey(key string) (tea.Model, tea.Cmd, bool) {
	if m.confirmDiscard {
		switch key {
		case "y", "Y":
			m.confirmDiscard = false
			return m, func() tea.Msg { return BackMsg{} }, true
		case "n", "N", "esc":
			m.confirmDiscard = false
			return m, nil, true
		default:
			return m, nil, true
		}
	}

	if m.confirmTypeChange {
		switch key {
		case "y", "Y":
			m.confirmTypeChange = false
			m.clearCurrentAttrs()
			m.typeCursor = m.pendingTypeCursor
			m.taskType = m.pendingTaskType
			m.updateValidParents()
			m.hasChanges = m.checkChanges()
			return m, nil, true
		case "n", "N", "esc":
			m.confirmTypeChange = false
			return m, nil, true
		default:
			return m, nil, true
		}
	}

	return nil, nil, false
}

func (m *TaskEditorModel) handleEnterKey() (tea.Model, tea.Cmd, bool) {
	if m.focusIndex == 5 {
		if err := m.parseDueDate(); err != nil {
			m.err = err
			return m, nil, true
		}
		m.err = nil
	}

	if m.focusIndex >= m.attrsStartIndex() {
		fieldIdx := m.focusIndex - m.attrsStartIndex()
		if msgFn := m.handleAttrEnter(fieldIdx); msgFn != nil {
			return m, func() tea.Msg { return msgFn().(tea.Msg) }, true
		}
	}

	if m.focusIndex < m.totalFieldCount()-1 {
		m.moveFocus(true)
		return m, nil, true
	}

	return nil, nil, false
}

func (m *TaskEditorModel) handleHorizontalMove(direction int) (tea.Model, tea.Cmd, bool) {
	if m.focusIndex >= m.attrsStartIndex() {
		fieldIdx := m.focusIndex - m.attrsStartIndex()
		if direction < 0 {
			m.handleAttrLeft(fieldIdx)
		} else {
			m.handleAttrRight(fieldIdx)
		}
		m.hasChanges = m.checkChanges()
		return m, nil, true
	}

	var cmd tea.Cmd
	switch m.focusIndex {
	case 1:
		cmd = m.moveTypeCursor(direction)
	case 2:
		m.moveParentCursor(direction)
	case 3:
		m.movePriority(direction)
	case 4:
		m.moveSectionCursor(direction)
	case 6:
		m.moveFlagCursor(direction)
	case 7:
		cmd = m.moveRecurrenceCursor(direction)
	default:
		return nil, nil, false
	}

	m.hasChanges = m.checkChanges()
	return m, cmd, true
}

func (m *TaskEditorModel) handleSpaceKey() bool {
	if m.focusIndex == 6 {
		return true
	}

	if m.focusIndex < m.attrsStartIndex() || m.taskType != models.TaskTypeIdea {
		return false
	}

	fieldIdx := m.focusIndex - m.attrsStartIndex()
	if fieldIdx != 3 {
		return false
	}

	m.ideaValidated = !m.ideaValidated
	m.hasChanges = m.checkChanges()
	return true
}

func (m *TaskEditorModel) moveFocus(forward bool) {
	m.blurCurrentField()
	totalFields := m.totalFieldCount()
	if forward {
		m.focusIndex = (m.focusIndex + 1) % totalFields
	} else {
		m.focusIndex = (m.focusIndex + totalFields - 1) % totalFields
	}
	m.focusCurrentField()
}

func (m *TaskEditorModel) moveTypeCursor(direction int) tea.Cmd {
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

func (m *TaskEditorModel) moveParentCursor(direction int) {
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

func (m *TaskEditorModel) movePriority(direction int) {
	newPriority := m.priority + direction
	if newPriority < 1 || newPriority > 5 {
		return
	}
	m.priority = newPriority
}

func (m *TaskEditorModel) moveSectionCursor(direction int) {
	newCursor := m.sectionCursor + direction
	if newCursor < 0 || newCursor >= len(m.sections) {
		return
	}
	m.sectionCursor = newCursor
	m.section = m.sections[m.sectionCursor]
}

func (m *TaskEditorModel) moveFlagCursor(direction int) {
	newCursor := m.flagCursor + direction
	if newCursor < 0 || newCursor > 3 {
		return
	}
	m.flagCursor = newCursor
	m.updateFlagsFromCursor()
}

func (m *TaskEditorModel) moveRecurrenceCursor(direction int) tea.Cmd {
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

func (m *TaskEditorModel) showRecurrenceEditor() tea.Cmd {
	return func() tea.Msg {
		return ShowRecurrenceEditorMsg{
			CurrentPattern: m.recurrence,
			DueDate:        m.dueDate,
		}
	}
}

func (m *TaskEditorModel) applyParentSelection(parentID *string) {
	m.parentID = parentID
	if parentID == nil {
		m.parentCursor = 0
		return
	}

	for i, parent := range m.validParents {
		if parent.ID == *parentID {
			m.parentCursor = i + 1
			return
		}
	}
}

func (m *TaskEditorModel) applyTextEdit(fieldName, value string) {
	switch fieldName {
	case "bugWorkaround":
		m.bugWorkaround = value
	case "storyPersona":
		m.storyPersona = value
	case "epicMilestone":
		m.epicMilestone = value
	case "epicSuccessMetrics":
		m.epicSuccessMetrics = value
	}
}

func (m *TaskEditorModel) updateFocusedInput(msg tea.Msg) tea.Cmd {
	switch m.focusIndex {
	case 0:
		var cmd tea.Cmd
		m.titleInput, cmd = m.titleInput.Update(msg)
		return cmd
	case 5:
		m.dueDateInput = updateInlineTextField(m.dueDateInput, msg)
	case 8:
		m.urlInput = updateInlineTextField(m.urlInput, msg)
	case 9:
		m.sourceInput = updateInlineTextField(m.sourceInput, msg)
	}

	return nil
}

func updateInlineTextField(current string, msg tea.Msg) string {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return current
	}

	switch key := keyMsg.String(); {
	case key == "backspace" && len(current) > 0:
		return current[:len(current)-1]
	case len(key) == 1 && key[0] >= 32 && key[0] <= 126:
		return current + key
	default:
		return current
	}
}
