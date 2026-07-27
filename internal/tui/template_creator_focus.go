package tui

// focusFieldCount returns total number of focusable fields.
// Base fields: name(1) + type(1) + category(1) + priority(1) + state(1) + tags(1) + description(1) = 7
// Plus checklist items (all types) plus subtasks (Epic/Story only)
func (m *TaskTemplateCreatorModel) focusFieldCount() int {
	count := 7 + len(m.checklist)
	if m.currentTypeCanHaveChildren() {
		count += len(m.subtasks)
	}
	return count
}

// currentTypeCanHaveChildren returns true if the currently selected type can have children.
func (m *TaskTemplateCreatorModel) currentTypeCanHaveChildren() bool {
	return m.types[m.typeCursor].CanHaveChildren()
}

// checklistStartIndex returns the focus index where checklist items begin.
func (m *TaskTemplateCreatorModel) checklistStartIndex() int {
	return 7
}

// subtaskStartIndex returns the focus index where subtask items begin.
func (m *TaskTemplateCreatorModel) subtaskStartIndex() int {
	return 7 + len(m.checklist)
}

// getFirstFocusIndexForTab returns the starting focus index for a given tab.
func (m *TaskTemplateCreatorModel) getFirstFocusIndexForTab(tab int) int {
	switch tab {
	case templateTabFields:
		return 0
	case templateTabChecklist:
		return m.checklistStartIndex()
	case templateTabSubtasks:
		return m.subtaskStartIndex()
	}
	return 0
}

// getTabForFocusIndex returns which tab the given focus index belongs to.
func (m *TaskTemplateCreatorModel) getTabForFocusIndex(idx int) int {
	if idx < m.checklistStartIndex() {
		return templateTabFields
	}
	if idx < m.subtaskStartIndex() {
		return templateTabChecklist
	}
	return templateTabSubtasks
}

// getFocusCountForTab returns the number of focusable fields in a tab.
func (m *TaskTemplateCreatorModel) getFocusCountForTab(tab int) int {
	switch tab {
	case templateTabFields:
		return 7
	case templateTabChecklist:
		return len(m.checklist)
	case templateTabSubtasks:
		if m.currentTypeCanHaveChildren() {
			return len(m.subtasks)
		}
		return 0
	}
	return 0
}

// getFocusSection returns which section the current focus is in.
// Returns section name and index within that section (-1 if not applicable).
func (m *TaskTemplateCreatorModel) getFocusSection() (section string, idx int) {
	switch m.focusIndex {
	case 0:
		return "name", -1
	case 1:
		return "type", -1
	case 2:
		return "category", -1
	case 3:
		return "priority", -1
	case 4:
		return "state", -1
	case 5:
		return "tags", -1
	case 6:
		return "description", -1
	default:
		checklistStart := m.checklistStartIndex()
		subtaskStart := m.subtaskStartIndex()

		if m.focusIndex >= checklistStart && m.focusIndex < subtaskStart {
			return "checklist", m.focusIndex - checklistStart
		}
		if m.currentTypeCanHaveChildren() && m.focusIndex >= subtaskStart {
			subtaskIdx := m.focusIndex - subtaskStart
			if subtaskIdx < len(m.subtasks) {
				return "subtask", subtaskIdx
			}
		}
		return "name", -1
	}
}

// HasTextInput returns true to block : and ? keys in the template creator.
// This prevents Finder and help from opening while editing.
func (m *TaskTemplateCreatorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true to let the creator handle esc internally.
func (m *TaskTemplateCreatorModel) ShouldCaptureEsc() bool {
	return true
}

func (m *TaskTemplateCreatorModel) SupportsHelpShortcut() bool { return false }
