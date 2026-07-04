package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

// View renders the workflow wizard interface.
func (m *WorkflowWizardModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m *WorkflowWizardModel) render() string {
	var out strings.Builder

	// Header: title + step progress
	stepLabel := fmt.Sprintf("Step %d of %d", int(m.step)+1, wizardTotalSteps)
	out.WriteString(renderFullscreenTitle("Workflow Wizard", stepLabel))
	out.WriteString("\n\n")

	// Progress summary (what's been picked so far)
	out.WriteString(m.renderProgress())
	out.WriteString("\n")

	// Current step content
	out.WriteString(m.renderCurrentStep())
	out.WriteString("\n")

	// Error message if any
	if m.err != nil {
		out.WriteString(ValidationErrorStyle.Render(fmt.Sprintf("Error: %s", m.err.Error())))
		out.WriteString("\n")
	}

	// Footer help text
	helpText := m.stepHelpText()
	out.WriteString(renderStatusBarFooter(helpText, "", m.width))

	return out.String()
}

// renderProgress shows the selections made so far (fields chosen in previous steps).
func (m *WorkflowWizardModel) renderProgress() string {
	if m.step == wizStepTriggerType {
		return MutedStyle.Render("Choose a trigger field to get started.") + "\n"
	}

	var parts []string

	triggerField := string(m.selectedTriggerField())
	parts = append(parts, fmt.Sprintf("When: %s", lipgloss.NewStyle().Foreground(ColorPrimary).Render(triggerField)))

	if m.step > wizStepTriggerOp {
		op := string(m.selectedTriggerOp())
		parts = append(parts, fmt.Sprintf("Op: %s", lipgloss.NewStyle().Foreground(ColorPrimary).Render(op)))
	}

	if m.step > wizStepTriggerValue {
		val := m.triggerValueInput.Value()
		if val == "" {
			val = "(empty)"
		}
		parts = append(parts, fmt.Sprintf("Value: %s", lipgloss.NewStyle().Foreground(ColorPrimary).Render(val)))
	}

	if m.step > wizStepActionType {
		action := string(m.selectedActionType())
		parts = append(parts, fmt.Sprintf("Then: %s", lipgloss.NewStyle().Foreground(ColorSecondary).Render(action)))
	}

	if m.step > wizStepActionValue {
		val := m.actionValueInput.Value()
		if val == "" {
			val = "(none)"
		}
		parts = append(parts, fmt.Sprintf("Action value: %s", lipgloss.NewStyle().Foreground(ColorSecondary).Render(val)))
	}

	return strings.Join(parts, "  ·  ") + "\n"
}

// renderCurrentStep renders the content for the active step.
func (m *WorkflowWizardModel) renderCurrentStep() string {
	switch m.step {
	case wizStepTriggerType:
		return m.renderListStep(
			"Select trigger field:",
			m.triggerFieldItems(),
			m.triggerCursor,
		)
	case wizStepTriggerOp:
		return m.renderListStep(
			"Select trigger operation:",
			m.triggerOpItems(),
			m.triggerOpCursor,
		)
	case wizStepTriggerValue:
		return m.renderInputStep(
			"Enter trigger value:",
			m.triggerValueInputHint(),
			m.triggerValueInput.View(),
		)
	case wizStepActionType:
		return m.renderListStep(
			"Select action type:",
			m.actionTypeItems(),
			m.actionTypeCursor,
		)
	case wizStepActionValue:
		return m.renderInputStep(
			"Enter action value:",
			m.actionValueInputHint(),
			m.actionValueInput.View(),
		)
	case wizStepNameReview:
		return m.renderNameReviewStep()
	}
	return ""
}

// listItem represents a single row in a list step.
type listItem struct {
	label string
	hint  string
}

// triggerFieldItems builds display rows for the trigger field list.
func (m *WorkflowWizardModel) triggerFieldItems() []listItem {
	items := make([]listItem, len(m.triggerFields))
	for i, f := range m.triggerFields {
		items[i] = listItem{
			label: triggerFieldLabel(f),
			hint:  triggerFieldHint(f),
		}
	}
	return items
}

// triggerOpItems builds display rows for the trigger operation list.
func (m *WorkflowWizardModel) triggerOpItems() []listItem {
	descs := map[workflow.TriggerOperation]string{
		workflow.OpEquals:      "equals – exact match",
		workflow.OpNotEquals:   "not_equals – does not match",
		workflow.OpContains:    "contains – substring/tag match",
		workflow.OpGreaterThan: "greater_than – numeric comparison",
		workflow.OpLessThan:    "less_than – numeric comparison",
	}
	items := make([]listItem, len(m.triggerOps))
	for i, op := range m.triggerOps {
		items[i] = listItem{label: descs[op]}
	}
	return items
}

// actionTypeItems builds display rows for the action type list.
func (m *WorkflowWizardModel) actionTypeItems() []listItem {
	items := make([]listItem, len(m.actionTypes))
	for i, a := range m.actionTypes {
		items[i] = listItem{
			label: actionTypeLabel(a),
			hint:  actionTypeHint(a),
		}
	}
	return items
}

// renderListStep renders a scrollable item list for the current step.
func (m *WorkflowWizardModel) renderListStep(heading string, items []listItem, cursor int) string {
	var sb strings.Builder

	sb.WriteString(SubtitleStyle.Render(heading))
	sb.WriteString("\n\n")

	// Determine visible window
	start := m.listScrollOff
	end := min(start+m.visibleListRows(), len(items))

	for i := start; i < end; i++ {
		item := items[i]
		selected := i == cursor

		var line string
		if item.hint != "" {
			line = fmt.Sprintf("  %s %s", item.label, MutedStyle.Render(item.hint))
		} else {
			line = fmt.Sprintf("  %s", item.label)
		}

		// Pad line to width
		lineWidth := lipgloss.Width(line)
		availWidth := m.width - 4
		if lineWidth < availWidth {
			line += strings.Repeat(" ", availWidth-lineWidth)
		}

		if selected {
			line = SelectedStyle.Render(line)
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Scroll indicators
	if m.listScrollOff > 0 {
		sb.WriteString(MutedStyle.Render(fmt.Sprintf("  ↑ %d more above", m.listScrollOff)))
		sb.WriteString("\n")
	}
	remaining := len(items) - end
	if remaining > 0 {
		sb.WriteString(MutedStyle.Render(fmt.Sprintf("  ↓ %d more below", remaining)))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderInputStep renders a text input for a value step.
func (m *WorkflowWizardModel) renderInputStep(heading, hint, inputView string) string {
	var sb strings.Builder
	sb.WriteString(SubtitleStyle.Render(heading))
	sb.WriteString("\n\n")
	if hint != "" {
		sb.WriteString(MutedStyle.Render(hint))
		sb.WriteString("\n")
	}
	sb.WriteString(inputView)
	sb.WriteString("\n")
	return sb.String()
}

// renderNameReviewStep renders the final step: rule name input + plain-language summary.
func (m *WorkflowWizardModel) renderNameReviewStep() string {
	var sb strings.Builder

	// Rule preview in plain language
	triggerStr := fmt.Sprintf("%s %s %s",
		m.selectedTriggerField(),
		m.selectedTriggerOp(),
		m.triggerValueInput.Value(),
	)

	actionValue := m.actionValueInput.Value()
	var actionStr string
	if actionValue != "" {
		actionStr = fmt.Sprintf("%s → %s", m.selectedActionType(), actionValue)
	} else {
		actionStr = string(m.selectedActionType())
	}

	sb.WriteString(SubtitleStyle.Render("Rule preview:"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "  When: %s\n", lipgloss.NewStyle().Foreground(ColorPrimary).Render(triggerStr))
	fmt.Fprintf(&sb, "  Then: %s\n", lipgloss.NewStyle().Foreground(ColorSecondary).Render(actionStr))
	sb.WriteString("\n")
	sb.WriteString(SubtitleStyle.Render("Rule name:"))
	sb.WriteString("\n\n")
	sb.WriteString(m.nameInput.View())
	sb.WriteString("\n\n")
	sb.WriteString(MutedStyle.Render("Press Enter or Ctrl+S to save · Esc to go back"))
	sb.WriteString("\n")

	return sb.String()
}

// triggerValueInputHint returns a contextual hint for the trigger value input.
func (m *WorkflowWizardModel) triggerValueInputHint() string {
	field := m.selectedTriggerField()
	switch field {
	case workflow.FieldStatus:
		return "e.g. todo, in_progress, maybe, done, blocked, canceled"
	case workflow.FieldPriority, workflow.FieldLevel, workflow.FieldDueDays,
		workflow.FieldCreatedDays, workflow.FieldUpdatedDays, workflow.FieldBlockerCount,
		workflow.FieldDependencyCount, workflow.FieldChildrenProgress,
		workflow.FieldActiveTaskCount, workflow.FieldCompletionRatio, workflow.FieldDaysSinceUpdate:
		return "numeric value (e.g. 1, 3, 50)"
	case workflow.FieldTags, workflow.FieldParentTags:
		return "tag name (e.g. urgent, feature)"
	case workflow.FieldType, workflow.FieldParentType:
		return "e.g. task, bug, story, epic, idea"
	case workflow.FieldCompleted, workflow.FieldReopened, workflow.FieldHasDueDate,
		workflow.FieldHasTags, workflow.FieldDescriptionEmpty, workflow.FieldHasChildren,
		workflow.FieldAllChildrenDone, workflow.FieldIsBlocked, workflow.FieldHasInProgressTasks:
		return "true or false"
	case workflow.FieldStatusChanged, workflow.FieldSectionChanged,
		workflow.FieldPriorityChanged, workflow.FieldTypeChanged:
		return "transition pattern: old->new, *->done, blocked->*"
	case workflow.FieldTagAdded, workflow.FieldTagRemoved:
		return "tag name (e.g. urgent)"
	case workflow.FieldDueDateChanged:
		return "added, removed, changed, or *"
	case workflow.FieldRecurrenceCompleted, workflow.FieldStreakBroken:
		return "true"
	default:
		return ""
	}
}

// actionValueInputHint returns a contextual hint for the action value input.
func (m *WorkflowWizardModel) actionValueInputHint() string {
	action := m.selectedActionType()
	switch action {
	case workflow.ActionSetStatus:
		return "e.g. todo, in_progress, maybe, done, blocked, canceled"
	case workflow.ActionSetPriority:
		return "numeric: 1 (highest) to 99, 0 = unset"
	case workflow.ActionSetSection:
		return "section name (e.g. Active, Next, Inbox) or leave empty for Active"
	case workflow.ActionAddTag, workflow.ActionRemoveTag, workflow.ActionCascadeTag, workflow.ActionInheritTag:
		return "tag name (e.g. urgent, feature)"
	case workflow.ActionSetDueRelative, workflow.ActionSetSnooze:
		return "relative duration (e.g. +3d, +1w, +2m)"
	case workflow.ActionSetActive, workflow.ActionSetPinned:
		return "true or false"
	case workflow.ActionAddComment, workflow.ActionNotifyParent, workflow.ActionCreateFollowup:
		return "text content"
	case workflow.ActionSetType:
		return "e.g. task, bug, story, epic, idea"
	case workflow.ActionMoveToProject:
		return "project name"
	case workflow.ActionSetRecurrence:
		return "e.g. daily, weekly, 2d, 3w"
	case workflow.ActionSetProjectStatus:
		return "project status icon key"
	default:
		return ""
	}
}

// stepHelpText returns contextual help text for the current step.
func (m *WorkflowWizardModel) stepHelpText() string {
	switch m.step {
	case wizStepTriggerType, wizStepTriggerOp, wizStepActionType:
		return "j/k: navigate · Enter: select · Esc: back/cancel"
	case wizStepTriggerValue, wizStepActionValue:
		return "Enter: confirm · Esc: back"
	case wizStepNameReview:
		return "Enter/Ctrl+S: save rule · Esc: back"
	}
	return ""
}
