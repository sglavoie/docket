package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// formOverlayKind tracks which inline overlay is active in TaskFormModel.
type formOverlayKind int

const (
	formOverlayNone          formOverlayKind = iota
	formOverlayType                          // formTypePicker
	formOverlayPriority                      // formPriorityPicker
	formOverlayFlags                         // formFlagsPicker
	formOverlayDue                           // formDueDatePicker
	formOverlayAttr                          // formAttrPicker (generic type-attr picker)
	formOverlayChecklistEdit                 // inline textinput for editing a checklist item
	formOverlaySubtaskEdit                   // inline overlay for editing a subtask draft
)

// ── Overlay rendering dispatch ────────────────────────────────────────────────

// renderFormOverlay renders the active inline overlay.
// Returns empty string if no overlay is active.
func (m *TaskFormModel) renderFormOverlay() string {
	switch m.activeOverlay {
	case formOverlayNone:
		return ""
	case formOverlayType:
		if m.typePicker != nil {
			return m.typePicker.render(m.width, m.draft.taskType)
		}
	case formOverlayPriority:
		if m.priorityPicker != nil {
			return m.priorityPicker.render(m.width, m.draft.priority)
		}
	case formOverlayFlags:
		if m.flagsPicker != nil {
			return m.flagsPicker.render(m.width)
		}
	case formOverlayDue:
		if m.duePicker != nil {
			return m.duePicker.render(m.width, m.draft.dueDate)
		}
	case formOverlayAttr:
		if m.attrPicker != nil {
			return m.attrPicker.render(m.width)
		}
	case formOverlayChecklistEdit:
		return m.renderChecklistEditOverlay()
	case formOverlaySubtaskEdit:
		return m.renderSubtaskEditOverlay()
	}
	return ""
}

// handleFormOverlayKey routes key events to the active inline overlay.
// Returns consumed=true when the key was handled by the overlay.
func (m *TaskFormModel) handleFormOverlayKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	var (
		cmd     tea.Cmd
		dismiss bool
		result  tea.Msg
	)
	switch m.activeOverlay {
	case formOverlayType:
		if m.typePicker != nil {
			cmd, dismiss, result = m.typePicker.handleKey(msg)
		}
	case formOverlayPriority:
		if m.priorityPicker != nil {
			cmd, dismiss, result = m.priorityPicker.handleKey(msg)
		}
	case formOverlayFlags:
		if m.flagsPicker != nil {
			cmd, dismiss, result = m.flagsPicker.handleKey(msg)
		}
	case formOverlayDue:
		if m.duePicker != nil {
			cmd, dismiss, result = m.duePicker.handleKey(msg)
		}
	case formOverlayAttr:
		if m.attrPicker != nil {
			cmd, dismiss, result = m.attrPicker.handleKey(msg)
		}
	case formOverlayChecklistEdit:
		cmd, dismiss, result = m.handleChecklistEditKey(msg)
	case formOverlaySubtaskEdit:
		cmd, dismiss, result = m.handleSubtaskEditKey(msg)
	default:
		return nil, false
	}

	if dismiss {
		// When canceling a brand-new item, remove it from the draft.
		if result == nil && m.childrenAddingNew {
			switch m.activeOverlay { //nolint:exhaustive // only checklist/subtask overlays have items to remove
			case formOverlayChecklistEdit:
				if m.childrenEditIdx >= 0 && m.childrenEditIdx < len(m.draft.checklist) {
					m.draft.checklist = append(
						m.draft.checklist[:m.childrenEditIdx],
						m.draft.checklist[m.childrenEditIdx+1:]...,
					)
				}
			case formOverlaySubtaskEdit:
				if m.childrenEditIdx >= 0 && m.childrenEditIdx < len(m.draft.subtasks) {
					m.draft.subtasks = append(
						m.draft.subtasks[:m.childrenEditIdx],
						m.draft.subtasks[m.childrenEditIdx+1:]...,
					)
				}
			}
		}
		m.childrenAddingNew = false
		m.closeInlineOverlay()
	}
	if result != nil {
		m.applyFormOverlayResult(result)
		if msg, ok := result.(TaskFormSubtaskEditedMsg); ok && (msg.OpenDetail || msg.OpenContent) {
			m.subtaskOpenIdx = msg.Idx
			m.subtaskOpenDetail = msg.OpenDetail
			m.subtaskOpenContent = msg.OpenContent
			return tea.Batch(cmd, m.handleSave()), true
		}
	}
	return cmd, true
}

// closeInlineOverlay resets the inline overlay state.
func (m *TaskFormModel) closeInlineOverlay() {
	m.activeOverlay = formOverlayNone
	m.typePicker = nil
	m.priorityPicker = nil
	m.flagsPicker = nil
	m.duePicker = nil
	m.attrPicker = nil
}

// applyTypeChange applies a task type change with all required side effects:
// clears incompatible parent, clamps focusIndex, marks dirty.
// No-op if newType == current type.
func (m *TaskFormModel) applyTypeChange(newType models.TaskType) {
	if newType == m.draft.taskType {
		return
	}
	m.draft.taskType = newType
	if m.draft.parentID != nil {
		if p := formFindTask(m.project.Tasks, *m.draft.parentID); p == nil || !p.CanAddChild(newType) {
			m.draft.parentID = nil
		}
	}
	// Epics cannot have parents — clear parentID unconditionally.
	if newType == models.TaskTypeEpic {
		m.draft.parentID = nil
	}
	maxIdx := formBasicsFieldCount + formAttrFieldCount(newType) - 1
	if m.focusIndex > maxIdx {
		m.focusIndex = maxIdx
	}
	if m.isFieldDisabled(m.focusIndex) {
		m.focusIndex++
	}
	m.hasChanges = true
	m.err = nil
}

// applyFormOverlayResult merges an overlay result into the draft and marks dirty.
func (m *TaskFormModel) applyFormOverlayResult(msg tea.Msg) {
	switch msg := msg.(type) {
	case TaskFormTypePickedMsg:
		m.applyTypeChange(msg.Type)
	case TaskFormPriorityPickedMsg:
		m.draft.priority = msg.Priority
		m.hasChanges = true
		m.err = nil
	case TaskFormFlagsPickedMsg:
		m.draft.pinned = msg.Pinned
		m.draft.active = msg.Active
		m.draft.dimmed = msg.Dimmed
		m.hasChanges = true
		m.err = nil
	case TaskFormDueDatePickedMsg:
		m.draft.dueDate = msg.Date
		m.draft.hasTime = false // Preset picker always sets date-only.
		if msg.Date == nil {
			m.draft.dueInput = ""
		} else {
			m.draft.dueInput = msg.Date.Format("2006-01-02")
		}
		// Sync the textinput so it shows the updated value if still on the Due row.
		m.dueTextInput.SetValue(m.draft.dueInput)
		m.hasChanges = true
		m.err = nil

	case TaskFormAttrPickedMsg:
		m.applyAttrPickedField(msg.Field, msg.Value)
		m.hasChanges = true
		m.err = nil

	case TaskFormChecklistItemEditedMsg:
		if msg.Idx >= 0 && msg.Idx < len(m.draft.checklist) {
			if strings.TrimSpace(msg.Text) == "" {
				// Remove the item when confirmed with blank text.
				m.draft.checklist = append(
					m.draft.checklist[:msg.Idx],
					m.draft.checklist[msg.Idx+1:]...,
				)
			} else {
				m.draft.checklist[msg.Idx].Text = msg.Text
			}
			m.hasChanges = true
			m.err = nil
		}
		m.childrenAddingNew = false

	case TaskFormSubtaskEditedMsg:
		if msg.Idx >= 0 && msg.Idx < len(m.draft.subtasks) {
			if strings.TrimSpace(msg.Title) == "" {
				// Remove the subtask when confirmed with blank title
				// (consistent with checklist item handling above).
				m.draft.subtasks = append(
					m.draft.subtasks[:msg.Idx],
					m.draft.subtasks[msg.Idx+1:]...,
				)
			} else {
				m.draft.subtasks[msg.Idx].title = msg.Title
				m.draft.subtasks[msg.Idx].taskType = msg.TaskType
			}
			m.hasChanges = true
			m.err = nil
		}
		m.childrenAddingNew = false
	}
}

// applyAttrPickedField merges a single attr field result into the draft.
func (m *TaskFormModel) applyAttrPickedField(field, value string) {
	switch field {
	case "bugReproducible":
		m.draft.bugAttrs.Reproducible = models.BugReproducible(value)
	case "bugSeverity":
		m.draft.bugAttrs.Severity = models.BugSeverity(value)
	case "ideaCategory":
		m.draft.ideaAttrs.Category = models.IdeaCategory(value)
	case "ideaEffort":
		m.draft.ideaAttrs.Effort = models.EffortEstimate(value)
	case "ideaValue":
		m.draft.ideaAttrs.Value = models.IdeaValue(value)
	case "ideaValidated":
		m.draft.ideaAttrs.Validated = value == "true"
	case "taskEffort":
		m.draft.taskAttrs.Effort = models.EffortEstimate(value)
	case "taskWorkType":
		m.draft.taskAttrs.WorkType = models.WorkType(value)
	case "storyPoints":
		if value == "" {
			m.draft.storyAttrs.StoryPoints = 0
		} else {
			v, _ := strconv.Atoi(value)
			m.draft.storyAttrs.StoryPoints = v
		}
	case "epicScope":
		m.draft.epicAttrs.Scope = models.EpicScope(value)
	}
}

// formFindTask returns the task with the given ID from a flat list, or nil.
func formFindTask(tasks []*models.Task, id string) *models.Task {
	for _, t := range tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}
