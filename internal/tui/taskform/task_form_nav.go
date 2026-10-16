package taskform

import tea "charm.land/bubbletea/v2"

func (m *TaskFormModel) Init() tea.Cmd { return nil }

func (m *TaskFormModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// ShouldCaptureEsc returns true so the form handles Esc internally (dirty-check).
func (m *TaskFormModel) ShouldCaptureEsc() bool { return true }

// HasTextInput returns true because the form always has text inputs.
func (m *TaskFormModel) HasTextInput() bool { return true }

// IsFullHeight returns false; the form renders its own footer but not the
// entire terminal (same as TaskCreatorModel).
func (m *TaskFormModel) IsFullHeight() bool { return false }

func (m *TaskFormModel) StatusBarLabel() string {
	return "New Task"
}

// SupportsHelpShortcut is false while the form is a scaffold.
func (m *TaskFormModel) SupportsHelpShortcut() bool { return false }

// HandledKeys returns all keys TaskFormModel's Update method can process.
func (m *TaskFormModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *TaskFormModel) nextTab() {
	m.activeTab = (m.activeTab + 1) % formTabCount
	m.focusIndex = 0
}

func (m *TaskFormModel) prevTab() {
	m.activeTab = (m.activeTab + formTabCount - 1) % formTabCount
	m.focusIndex = 0
}

// fieldCountForActiveTab returns the number of focusable fields on the current tab.
// The Children and Extras tabs are computed dynamically.
func (m *TaskFormModel) fieldCountForActiveTab() int {
	if m.activeTab == formTabBasics {
		if m.collapsedBasics {
			return 3 // Title, Priority, "+ more fields"
		}
		return formBasicsFieldCount
	}
	if m.activeTab == formTabChildren {
		return m.fieldCountForChildren()
	}
	if m.activeTab == formTabExtras {
		return m.fieldCountForExtras()
	}
	return formStaticFieldCount(m.activeTab)
}

// fieldCountForExtras returns the dynamic field count for the Extras tab.
func (m *TaskFormModel) fieldCountForExtras() int {
	count := 3 // attachments section header (always present)
	if m.attachmentsExpanded {
		count += len(m.draft.attachments)
	}
	count++ // comments section header
	if m.commentsExpanded {
		count += len(m.draft.comments)
	}
	return count
}

// extrasCommentsHeaderIdx returns the focusIndex of the Comments section header.
func (m *TaskFormModel) extrasCommentsHeaderIdx() int {
	idx := 1
	if m.attachmentsExpanded {
		idx += len(m.draft.attachments)
	}
	return idx
}

// focusedExtrasItem resolves the current focusIndex on the Extras tab into a
// (kind, localIndex) pair. Returns extrasFocusNone when the focus is on a
// section header or out of range.
func (m *TaskFormModel) focusedExtrasItem() (extrasFocusKind, int) {
	if m.attachmentsExpanded && m.focusIndex >= 1 {
		attEnd := 1 + len(m.draft.attachments)
		if m.focusIndex < attEnd {
			return extrasFocusAttachmentItem, m.focusIndex - 1
		}
	}

	if m.commentsExpanded {
		cmStart := m.extrasCommentsHeaderIdx() + 1
		cmEnd := cmStart + len(m.draft.comments)
		if m.focusIndex >= cmStart && m.focusIndex < cmEnd {
			return extrasFocusCommentItem, m.focusIndex - cmStart
		}
	}

	return extrasFocusNone, -1
}

// clampExtrasFocusIndex ensures focusIndex is in range after section toggling.
func (m *TaskFormModel) clampExtrasFocusIndex() {
	count := m.fieldCountForExtras()
	if count == 0 {
		m.focusIndex = 0
		return
	}
	if m.focusIndex >= count {
		m.focusIndex = count - 1
	}
}

// fieldCountForChildren returns the dynamic field count for the Children tab.
func (m *TaskFormModel) fieldCountForChildren() int {
	count := 1 // checklist section header (always present)
	if m.checklistExpanded {
		count += len(m.draft.checklist)
	}
	count++ // subtasks section header
	if m.subtasksExpanded {
		count += len(m.draft.subtasks)
	}
	return count
}

// childrenSubtasksHeaderIdx returns the focusIndex of the Subtasks section header.
func (m *TaskFormModel) childrenSubtasksHeaderIdx() int {
	idx := 1
	if m.checklistExpanded {
		idx += len(m.draft.checklist)
	}
	return idx
}

// focusedChildItem resolves the current focusIndex on the Children tab into
// a (kind, localIndex) pair. Returns childFocusNone when the focus is on a
// section header or out of range.
func (m *TaskFormModel) focusedChildItem() (childFocusKind, int) {
	if m.checklistExpanded && m.focusIndex >= 1 {
		clEnd := 1 + len(m.draft.checklist)
		if m.focusIndex < clEnd {
			return childFocusChecklistItem, m.focusIndex - 1
		}
	}

	if m.subtasksExpanded {
		stStart := m.childrenSubtasksHeaderIdx() + 1
		stEnd := stStart + len(m.draft.subtasks)
		if m.focusIndex >= stStart && m.focusIndex < stEnd {
			return childFocusSubtaskItem, m.focusIndex - stStart
		}
	}

	return childFocusNone, -1
}

// clampChildrenFocusIndex ensures focusIndex is in range after section toggling.
func (m *TaskFormModel) clampChildrenFocusIndex() {
	count := m.fieldCountForChildren()
	if count == 0 {
		m.focusIndex = 0
		return
	}
	if m.focusIndex >= count {
		m.focusIndex = count - 1
	}
}

func (m *TaskFormModel) focusNext() {
	count := m.fieldCountForActiveTab()
	if count == 0 {
		return
	}
	m.focusIndex = (m.focusIndex + 1) % count
}

func (m *TaskFormModel) focusPrev() {
	count := m.fieldCountForActiveTab()
	if count == 0 {
		return
	}
	m.focusIndex = (m.focusIndex + count - 1) % count
}
