package tui

// renderFormLabel renders a form field label with fixed width alignment.
func (m *TaskEditorModel) renderFormLabel(label string, focusIdx int) string {
	return renderFormLabelShared(label, focusIdx, m.focusIndex)
}

// renderTypeSelector renders the task type selector options.
func (m *TaskEditorModel) renderTypeSelector() string {
	return renderTypeSelectorShared(m.types, m.typeCursor, m.focusIndex == 1)
}

// renderParentSelector renders the parent task selector options.
func (m *TaskEditorModel) renderParentSelector() string {
	return renderParentSelectorShared(m.validParents, m.parentCursor, m.focusIndex == 2)
}

// renderPrioritySelector renders the priority selector options.
func (m *TaskEditorModel) renderPrioritySelector() string {
	return renderPrioritySelectorShared(m.priority, m.focusIndex == 3)
}

// renderSectionSelector renders the section selector options.
func (m *TaskEditorModel) renderSectionSelector() string {
	return renderSectionSelectorShared(m.sections, m.sectionCursor, m.focusIndex == 4)
}

// renderDueDateField renders the due date input field.
func (m *TaskEditorModel) renderDueDateField() string {
	return renderDueDateFieldShared(m.dueDateInput, m.focusIndex == 5)
}

// renderFlagsField renders the mutually exclusive flag options.
func (m *TaskEditorModel) renderFlagsField() string {
	return renderFlagsSelectorShared(m.flagCursor, m.focusIndex == 6)
}

// renderRecurrenceSelector renders the recurrence pattern selector options.
func (m *TaskEditorModel) renderRecurrenceSelector() string {
	return renderRecurrenceSelectorShared(m.recurrenceOpts, m.recurrenceCursor, m.focusIndex == 7, m.recurrence)
}
