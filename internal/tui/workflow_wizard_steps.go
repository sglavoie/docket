package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

// wizardListChrome is the number of terminal lines the wizard reserves around
// a list step (title/progress/heading/blanks/2 scroll hints/footer/breadcrumb).
const wizardListChrome = 14

// visibleListRows returns the number of list rows that fit in the current
// terminal height, mirroring the height-derived window used by the sibling
// field picker (workflow_field_picker.go). Falls back to a fixed window before
// SetSize has run.
func (m *WorkflowWizardModel) visibleListRows() int {
	if m.height <= 0 {
		return 16 // pre-SetSize default
	}
	if r := m.height - wizardListChrome; r > 3 {
		return r
	}
	return 3
}

// handleStepKey dispatches key presses to the current step handler.
func (m *WorkflowWizardModel) handleStepKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.step {
	case wizStepTriggerType:
		return m.handleListStepKey(msg, len(m.triggerFields), &m.triggerCursor)
	case wizStepTriggerOp:
		return m.handleListStepKey(msg, len(m.triggerOps), &m.triggerOpCursor)
	case wizStepTriggerValue:
		return m.handleInputStepKey(msg, &m.triggerValueInput)
	case wizStepActionType:
		return m.handleListStepKey(msg, len(m.actionTypes), &m.actionTypeCursor)
	case wizStepActionValue:
		return m.handleInputStepKey(msg, &m.actionValueInput)
	case wizStepNameReview:
		return m.handleNameReviewKey(msg)
	}
	return m, nil
}

// handleListStepKey handles j/k/enter/esc for list-selection steps.
func (m *WorkflowWizardModel) handleListStepKey(msg tea.KeyPressMsg, listLen int, cursor *int) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if *cursor < listLen-1 {
			*cursor++
			m.clampListScroll(cursor)
		}
	case "k", "up":
		if *cursor > 0 {
			*cursor--
			m.clampListScroll(cursor)
		}
	case "enter":
		m.goForward()
	case "esc":
		m.goBack()
	}
	return m, nil
}

// handleInputStepKey handles text input and enter/esc for value-entry steps.
func (m *WorkflowWizardModel) handleInputStepKey(msg tea.KeyPressMsg, input *textInputModel) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.goForward()
		return m, nil
	case "esc":
		m.goBack()
		return m, nil
	}
	var cmd tea.Cmd
	*input, cmd = input.Update(msg)
	return m, cmd
}

// handleNameReviewKey handles the final step: name input + save/back.
func (m *WorkflowWizardModel) handleNameReviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+s", "enter":
		m.err = nil
		cmd := m.saveRule()
		return m, cmd
	case "esc":
		m.goBack()
		return m, nil
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

// goForward advances to the next step, focusing the appropriate input.
func (m *WorkflowWizardModel) goForward() {
	m.err = nil
	m.listScrollOff = 0

	switch m.step {
	case wizStepTriggerType:
		m.step = wizStepTriggerOp

	case wizStepTriggerOp:
		m.step = wizStepTriggerValue
		m.triggerValueInput.Focus()

	case wizStepTriggerValue:
		m.triggerValueInput.Blur()
		m.step = wizStepActionType

	case wizStepActionType:
		if m.needsActionValue() {
			m.step = wizStepActionValue
			m.actionValueInput.Focus()
		} else {
			m.step = wizStepNameReview
			m.nameInput.Focus()
		}

	case wizStepActionValue:
		m.actionValueInput.Blur()
		m.step = wizStepNameReview
		m.nameInput.Focus()

	case wizStepNameReview:
		// Final step — nothing to advance to
	}
}

// goBack moves to the previous step, restoring the appropriate focus.
func (m *WorkflowWizardModel) goBack() {
	m.err = nil
	m.listScrollOff = 0

	switch m.step {
	case wizStepTriggerType:
		// Step 0: Esc → cancel entirely (handled by ShouldCaptureEsc returning false)
		// This should not be reached since ShouldCaptureEsc() returns false for step 0.

	case wizStepTriggerOp:
		m.step = wizStepTriggerType

	case wizStepTriggerValue:
		m.triggerValueInput.Blur()
		m.step = wizStepTriggerOp

	case wizStepActionType:
		m.step = wizStepTriggerValue
		m.triggerValueInput.Focus()

	case wizStepActionValue:
		m.actionValueInput.Blur()
		m.step = wizStepActionType

	case wizStepNameReview:
		m.nameInput.Blur()
		if m.needsActionValue() {
			m.step = wizStepActionValue
			m.actionValueInput.Focus()
		} else {
			m.step = wizStepActionType
		}
	}
}

// clampListScroll updates the scroll offset so the cursor is always visible.
func (m *WorkflowWizardModel) clampListScroll(cursor *int) {
	if *cursor < m.listScrollOff {
		m.listScrollOff = *cursor
	}
	if *cursor >= m.listScrollOff+m.visibleListRows() {
		m.listScrollOff = *cursor - m.visibleListRows() + 1
	}
}

// triggerFieldLabel returns a short human-readable name for a trigger field.
func triggerFieldLabel(f workflow.TriggerField) string {
	labels := map[workflow.TriggerField]string{
		workflow.FieldStatus:              "status",
		workflow.FieldPriority:            "priority",
		workflow.FieldSection:             "section",
		workflow.FieldTags:                "tags",
		workflow.FieldType:                "type",
		workflow.FieldDueDays:             "due_days",
		workflow.FieldCreatedDays:         "created_days",
		workflow.FieldUpdatedDays:         "updated_days",
		workflow.FieldStatusChanged:       "status_changed",
		workflow.FieldSectionChanged:      "section_changed",
		workflow.FieldPriorityChanged:     "priority_changed",
		workflow.FieldTagAdded:            "tag_added",
		workflow.FieldTagRemoved:          "tag_removed",
		workflow.FieldCompleted:           "completed",
		workflow.FieldReopened:            "reopened",
		workflow.FieldDueDateChanged:      "due_date_changed",
		workflow.FieldTypeChanged:         "type_changed",
		workflow.FieldRecurrenceCompleted: "recurrence_completed",
		workflow.FieldStreakBroken:        "streak_broken",
		workflow.FieldHasDueDate:          "has_due_date",
		workflow.FieldHasTags:             "has_tags",
		workflow.FieldDescriptionEmpty:    "description_empty",
		workflow.FieldParentStatus:        "parent_status",
		workflow.FieldParentType:          "parent_type",
		workflow.FieldParentTags:          "parent_tags",
		workflow.FieldLevel:               "level",
		workflow.FieldHasChildren:         "has_children",
		workflow.FieldChildrenProgress:    "children_progress",
		workflow.FieldAllChildrenDone:     "all_children_done",
		workflow.FieldIsBlocked:           "is_blocked",
		workflow.FieldBlockerCount:        "blocker_count",
		workflow.FieldDependencyCount:     "dependency_count",
		workflow.FieldHasInProgressTasks:  "has_in_progress_tasks",
		workflow.FieldActiveTaskCount:     "active_task_count",
		workflow.FieldAllTasksStatus:      "all_tasks_status",
		workflow.FieldMajorityTaskType:    "majority_task_type",
		workflow.FieldMajorityTaskStatus:  "majority_task_status",
		workflow.FieldCompletionRatio:     "completion_ratio",
		workflow.FieldDaysSinceUpdate:     "days_since_update",
		workflow.FieldProjectStatus:       "project_status",
		workflow.FieldHasTasksWithStatus:  "has_tasks_with_status",
	}
	if label, ok := labels[f]; ok {
		return label
	}
	return string(f)
}

// triggerFieldHint returns a short hint string for a trigger field.
func triggerFieldHint(f workflow.TriggerField) string {
	switch {
	case workflow.IsProjectTrigger(f):
		return "[project]"
	case workflow.IsEventTrigger(f):
		return "[event]"
	case workflow.IsHierarchicalTrigger(f):
		return "[hierarchy]"
	default:
		return ""
	}
}

// actionTypeLabel returns a short human-readable name for an action type.
func actionTypeLabel(a workflow.ActionType) string {
	return string(a)
}

// actionTypeHint returns a short hint for the action type.
func actionTypeHint(a workflow.ActionType) string {
	if workflow.IsHierarchicalAction(a) {
		return "[hierarchy]"
	}
	if noValueActionTypes[a] {
		return "[no value]"
	}
	return ""
}

// textInputModel is an alias to make the Update call typecheck correctly.
type textInputModel = textinput.Model
