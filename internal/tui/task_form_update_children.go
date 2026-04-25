package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleChildrenTabKey handles keys specific to the Children tab.
// It returns handled=true when the key was consumed.
func (m *TaskFormModel) handleChildrenTabKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "c":
		m.checklistExpanded = !m.checklistExpanded
		m.clampChildrenFocusIndex()
	case "S":
		if m.mode == FormModeCreate && formTypeCanHaveSubtasks(m.draft.taskType) {
			m.subtasksExpanded = !m.subtasksExpanded
			m.clampChildrenFocusIndex()
		}
	case "ctrl+a":
		return m, m.handleChildrenCtrlA(), true
	case "ctrl+t":
		return m, m.handleChildrenCtrlT(), true
	case "ctrl+d":
		m.handleChildrenDelete()
	case "ctrl+j":
		m.handleChildrenReorder(1)
	case "ctrl+k":
		m.handleChildrenReorder(-1)
	case "ctrl+o":
		m.handleSubtaskToggleOptional()
	case "space":
		m.handleChecklistToggleDone()
	default:
		return m, nil, false
	}
	return m, nil, true
}

// handleChildrenEnter handles Enter on the Children tab:
// - on a section header: toggles expand/collapse
// - on a checklist item: opens checklist edit overlay
// - on a subtask item: opens subtask edit overlay
func (m *TaskFormModel) handleChildrenEnter() tea.Cmd {
	// Section headers: toggle expand/collapse.
	if m.focusIndex == 0 {
		m.checklistExpanded = !m.checklistExpanded
		m.clampChildrenFocusIndex()
		return nil
	}
	if stHeaderIdx := m.childrenSubtasksHeaderIdx(); m.focusIndex == stHeaderIdx {
		m.subtasksExpanded = !m.subtasksExpanded
		m.clampChildrenFocusIndex()
		return nil
	}

	kind, idx := m.focusedChildItem()
	switch kind {
	case childFocusNone:
		return nil
	case childFocusChecklistItem:
		m.childrenEditIdx = idx
		m.childrenAddingNew = false
		m.initChecklistItemInput(m.draft.checklist[idx].Text)
		m.checklistItemInput.Focus()
		m.activeOverlay = formOverlayChecklistEdit
	case childFocusSubtaskItem:
		m.childrenEditIdx = idx
		m.childrenAddingNew = false
		m.initSubtaskItemInput(m.draft.subtasks[idx].title)
		m.subtaskItemInput.Focus()
		m.subtaskTypeCursor = formSubtaskTypeCursorFor(m.draft.subtasks[idx].taskType)
		m.activeOverlay = formOverlaySubtaskEdit
	}
	return nil
}

// handleChildrenCtrlA adds a new checklist item and opens the edit overlay for it.
func (m *TaskFormModel) handleChildrenCtrlA() tea.Cmd {
	m.draft.checklist = append(m.draft.checklist, models.ChecklistItem{})
	m.checklistExpanded = true
	m.childrenEditIdx = len(m.draft.checklist) - 1
	m.childrenAddingNew = true
	m.initChecklistItemInput("")
	m.checklistItemInput.Focus()
	m.activeOverlay = formOverlayChecklistEdit
	m.hasChanges = true
	m.err = nil
	return nil
}

// handleChildrenCtrlT adds a new subtask draft and opens the edit overlay for it.
func (m *TaskFormModel) handleChildrenCtrlT() tea.Cmd {
	if m.mode != FormModeCreate || !formTypeCanHaveSubtasks(m.draft.taskType) {
		return nil
	}
	m.draft.subtasks = append(m.draft.subtasks, formSubtaskDraft{taskType: formSubtaskTypes[0]})
	m.subtasksExpanded = true
	m.childrenEditIdx = len(m.draft.subtasks) - 1
	m.childrenAddingNew = true
	m.initSubtaskItemInput("")
	m.subtaskItemInput.Focus()
	m.subtaskTypeCursor = 0
	m.activeOverlay = formOverlaySubtaskEdit
	m.hasChanges = true
	m.err = nil
	return nil
}

// handleChildrenDelete deletes the currently focused checklist item or subtask.
func (m *TaskFormModel) handleChildrenDelete() {
	kind, idx := m.focusedChildItem()
	switch kind {
	case childFocusChecklistItem:
		m.draft.checklist = append(m.draft.checklist[:idx], m.draft.checklist[idx+1:]...)
	case childFocusSubtaskItem:
		m.draft.subtasks = append(m.draft.subtasks[:idx], m.draft.subtasks[idx+1:]...)
	default:
		return
	}
	m.hasChanges = true
	m.err = nil
	m.clampChildrenFocusIndex()
}

// handleChildrenReorder moves the focused checklist item or subtask by delta
// positions (+1 = down, -1 = up).
func (m *TaskFormModel) handleChildrenReorder(delta int) {
	kind, idx := m.focusedChildItem()
	switch kind {
	case childFocusChecklistItem:
		neighbor := idx + delta
		if neighbor < 0 || neighbor >= len(m.draft.checklist) {
			return
		}
		m.draft.checklist[idx], m.draft.checklist[neighbor] = m.draft.checklist[neighbor], m.draft.checklist[idx]
	case childFocusSubtaskItem:
		neighbor := idx + delta
		if neighbor < 0 || neighbor >= len(m.draft.subtasks) {
			return
		}
		m.draft.subtasks[idx], m.draft.subtasks[neighbor] = m.draft.subtasks[neighbor], m.draft.subtasks[idx]
	default:
		return
	}
	m.focusIndex += delta
	m.hasChanges = true
	m.err = nil
}

// handleSubtaskToggleOptional toggles the optional flag on the focused subtask.
func (m *TaskFormModel) handleSubtaskToggleOptional() {
	if kind, idx := m.focusedChildItem(); kind == childFocusSubtaskItem {
		m.draft.subtasks[idx].optional = !m.draft.subtasks[idx].optional
		m.hasChanges = true
		m.err = nil
	}
}

// handleChecklistToggleDone toggles the Done state of the focused checklist item.
func (m *TaskFormModel) handleChecklistToggleDone() {
	if kind, idx := m.focusedChildItem(); kind == childFocusChecklistItem {
		m.draft.checklist[idx].Done = !m.draft.checklist[idx].Done
		m.hasChanges = true
		m.err = nil
	}
}
