package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

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
	if m.mode == FormModeCreate {
		return "New Task"
	}
	return "Edit Task"
}

// SupportsHelpShortcut is false while the form is a scaffold.
func (m *TaskFormModel) SupportsHelpShortcut() bool { return false }

// HandledKeys returns all keys TaskFormModel's Update method can process.
// Must stay in sync with keybindings_task_form.go and the switch cases in Update().
func (m *TaskFormModel) HandledKeys() []string {
	return []string{
		"shift+left", "shift+right",
		"tab", "shift+tab",
		"enter",
		"ctrl+s",
		"ctrl+v",
		"ctrl+e",
		"c",
		"S",
		"ctrl+a",
		"ctrl+t",
		"ctrl+d",
		"ctrl+j",
		"ctrl+k",
		"ctrl+o",
		"space",
		"1", "2", "3", "4", "5",
		"j", "k", "up", "down",
		"h", "l", "left", "right",
	}
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
// Basics tab includes dynamic type-attr rows. Children tab is computed dynamically.
func (m *TaskFormModel) fieldCountForActiveTab() int {
	if m.activeTab == formTabBasics {
		return formBasicsFieldCount + formAttrFieldCount(m.draft.taskType)
	}
	if m.activeTab == formTabChildren {
		return m.fieldCountForChildren()
	}
	return formStaticFieldCount(m.activeTab)
}

// fieldCountForChildren returns the dynamic field count for the Children tab.
func (m *TaskFormModel) fieldCountForChildren() int {
	count := 1 // checklist section header (always present)
	if m.checklistExpanded {
		count += len(m.draft.checklist)
	}
	if m.mode == FormModeCreate && formTypeCanHaveSubtasks(m.draft.taskType) {
		count++ // subtasks section header
		if m.subtasksExpanded {
			count += len(m.draft.subtasks)
		}
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

	if m.mode == FormModeCreate && formTypeCanHaveSubtasks(m.draft.taskType) && m.subtasksExpanded {
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
	if m.isFieldDisabled(m.focusIndex) {
		m.focusIndex = (m.focusIndex + 1) % count
	}
}

func (m *TaskFormModel) focusPrev() {
	count := m.fieldCountForActiveTab()
	if count == 0 {
		return
	}
	m.focusIndex = (m.focusIndex + count - 1) % count
	if m.isFieldDisabled(m.focusIndex) {
		m.focusIndex = (m.focusIndex + count - 1) % count
	}
}

// isFieldDisabled returns true when the field at the given index should be
// skipped during navigation. Currently only the Parent row (Basics field 4)
// is disabled — Epics are always top-level and cannot have a parent.
func (m *TaskFormModel) isFieldDisabled(idx int) bool {
	return m.activeTab == formTabBasics && idx == 4 && m.draft.taskType == models.TaskTypeEpic
}

// attrRowFocused returns true when the active tab is Basics and focus is on
// the attr row at the given local index (0-based within the type-attr section).
func (m *TaskFormModel) attrRowFocused(localIdx int) bool {
	return m.activeTab == formTabBasics && m.focusIndex == formBasicsFieldCount+localIdx
}

// formTypeCanHaveSubtasks reports whether a task type can have subtask children.
func formTypeCanHaveSubtasks(t models.TaskType) bool {
	return t == models.TaskTypeEpic || t == models.TaskTypeStory
}

// formSubtaskTypeCursorFor returns the cursor index in formSubtaskTypes for the given type.
func formSubtaskTypeCursorFor(t models.TaskType) int {
	for i, st := range formSubtaskTypes {
		if st == t {
			return i
		}
	}
	return 0
}
