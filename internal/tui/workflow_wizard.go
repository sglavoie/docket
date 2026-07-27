package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
)

// wizardStep identifies the current step in the workflow wizard.
type wizardStep int

const (
	wizStepTriggerType  wizardStep = iota // 0: pick trigger field
	wizStepTriggerOp                      // 1: pick trigger operation
	wizStepTriggerValue                   // 2: enter trigger value
	wizStepActionType                     // 3: pick action type
	wizStepActionValue                    // 4: enter action value (may be skipped)
	wizStepNameReview                     // 5: enter rule name + review and save
)

const wizardTotalSteps = 6

// noValueActionTypes lists action types that do not require a value.
var noValueActionTypes = map[workflow.ActionType]bool{
	workflow.ActionClearDue:       true,
	workflow.ActionCompleteParent: true,
	workflow.ActionBlockParent:    true,
}

// WorkflowWizardModel is a linear wizard for creating a workflow rule.
// It guides the user through trigger → operation → value → action → value → name+review.
type WorkflowWizardModel struct {
	// runtime backs the automation-domain commit; the new rule is written through
	// the coordinator over it (RuntimeState.CommitAutomation).
	runtime     *config.RuntimeState
	coordinator WorkflowCoordinator

	step wizardStep

	// Trigger field selection (step 0)
	triggerFields []workflow.TriggerField
	triggerCursor int
	listScrollOff int // scroll offset for long lists

	// Trigger operation selection (step 1)
	triggerOps      []workflow.TriggerOperation
	triggerOpCursor int

	// Trigger value input (step 2)
	triggerValueInput textinput.Model

	// Action type selection (step 3)
	actionTypes      []workflow.ActionType
	actionTypeCursor int

	// Action value input (step 4, may be skipped)
	actionValueInput textinput.Model

	// Rule name input (step 5)
	nameInput textinput.Model

	width  int
	height int
	err    error

	// Confirm discard overlay
	confirmDiscard bool
}

// ShowWorkflowWizardMsg triggers opening the workflow rule wizard.
type ShowWorkflowWizardMsg struct{}

// WorkflowWizardSavedMsg is emitted when the wizard saves a new rule.
type WorkflowWizardSavedMsg struct {
	Rule workflow.WorkflowRule
}

// NewWorkflowWizardModel creates a new workflow rule wizard.
func NewWorkflowWizardModel(runtime *config.RuntimeState) *WorkflowWizardModel {
	factory := DefaultFormFieldFactory()

	triggerValueInput := factory.NewTextInput("Trigger value (e.g. done, 1, active)")
	actionValueInput := factory.NewTextInput("Action value (e.g. done, 1, #tag, +3d)")
	nameInput := factory.NewTextInput("Rule name")

	var coordinator WorkflowCoordinator
	if runtime != nil {
		coordinator = newWorkflowCoordinator(runtime)
	}
	m := &WorkflowWizardModel{
		runtime:           runtime,
		coordinator:       coordinator,
		triggerFields:     workflow.ValidTriggerFields(),
		triggerOps:        workflow.ValidTriggerOperations(),
		actionTypes:       workflow.ValidActionTypes(),
		triggerValueInput: triggerValueInput,
		actionValueInput:  actionValueInput,
		nameInput:         nameInput,
		width:             80,
		height:            24,
	}

	return m
}

// SetSize sets the view dimensions.
func (m *WorkflowWizardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextInput(&m.triggerValueInput, width)
	formFactory.ResizeTextInput(&m.actionValueInput, width)
	formFactory.ResizeTextInput(&m.nameInput, width)
}

// Init initializes the model.
func (m *WorkflowWizardModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *WorkflowWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
			if confirmed {
				return m, func() tea.Msg { return BackMsg{} }
			}
			m.confirmDiscard = false
			return m, nil
		}
		return m.handleStepKey(msg)
	}

	return m, nil
}

// HandleConfirmKey delegates to the confirm overlay for discard confirmation.
func (m *WorkflowWizardModel) HandleConfirmKey(key string) (confirmed, handled bool) {
	if !m.confirmDiscard {
		return false, false
	}
	switch key {
	case "y", "enter":
		return true, true
	case "n", "esc":
		m.confirmDiscard = false
		return false, true
	}
	return false, true
}

// ShouldCaptureEsc returns true when the wizard should handle Esc internally
// (going back one step) rather than letting the global handler pop the view.
func (m *WorkflowWizardModel) ShouldCaptureEsc() bool {
	return m.step > wizStepTriggerType || m.confirmDiscard
}

// HandledKeys returns the keys this view handles.
func (m *WorkflowWizardModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// HasTextInput returns true when a text input step is active.
func (m *WorkflowWizardModel) HasTextInput() bool {
	return m.step == wizStepTriggerValue || m.step == wizStepActionValue || m.step == wizStepNameReview
}

// IsFullHeight returns false (wizard is a normal-height form view).
func (m *WorkflowWizardModel) IsFullHeight() bool { return false }

// StatusBarLabel returns the label for the status bar.
func (m *WorkflowWizardModel) StatusBarLabel() string {
	return fmt.Sprintf("Workflow Wizard – Step %d/%d", int(m.step)+1, wizardTotalSteps)
}

// selectedTriggerField returns the currently selected trigger field.
func (m *WorkflowWizardModel) selectedTriggerField() workflow.TriggerField {
	if m.triggerCursor < len(m.triggerFields) {
		return m.triggerFields[m.triggerCursor]
	}
	return workflow.FieldState
}

// selectedTriggerOp returns the currently selected trigger operation.
func (m *WorkflowWizardModel) selectedTriggerOp() workflow.TriggerOperation {
	if m.triggerOpCursor < len(m.triggerOps) {
		return m.triggerOps[m.triggerOpCursor]
	}
	return workflow.OpEquals
}

// selectedActionType returns the currently selected action type.
func (m *WorkflowWizardModel) selectedActionType() workflow.ActionType {
	if m.actionTypeCursor < len(m.actionTypes) {
		return m.actionTypes[m.actionTypeCursor]
	}
	return workflow.ActionSetState
}

// needsActionValue returns true if the selected action type requires a value.
func (m *WorkflowWizardModel) needsActionValue() bool {
	return !noValueActionTypes[m.selectedActionType()]
}

// saveRule builds and saves the workflow rule.
func (m *WorkflowWizardModel) saveRule() tea.Cmd {
	name := strings.TrimSpace(m.nameInput.Value())
	if name == "" {
		m.err = fmt.Errorf("rule name is required")
		return nil
	}

	trigger := workflow.RuleTrigger{
		Field:     m.selectedTriggerField(),
		Operation: m.selectedTriggerOp(),
		Value:     strings.TrimSpace(m.triggerValueInput.Value()),
	}
	if err := trigger.Validate(); err != nil {
		m.err = fmt.Errorf("trigger: %w", err)
		return nil
	}

	actionValue := ""
	if m.needsActionValue() {
		actionValue = strings.TrimSpace(m.actionValueInput.Value())
	}
	action := workflow.RuleAction{
		ActionType: m.selectedActionType(),
		Value:      actionValue,
	}
	if err := action.Validate(); err != nil {
		m.err = fmt.Errorf("action: %w", err)
		return nil
	}

	rule := workflow.NewRule(name, trigger, []workflow.RuleAction{action})
	if err := rule.ValidateForAuthoring(); err != nil {
		m.err = err
		return nil
	}

	automation := cloneAutomationConfig(m.runtime.Automation())
	automation.AddWorkflowRule(*rule)
	savedRule := *rule

	return m.coordinator.SaveThen(automation, func() tea.Msg {
		return WorkflowWizardSavedMsg{Rule: savedRule}
	})
}
