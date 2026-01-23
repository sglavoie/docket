package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
)

// TriggerFormState holds the form state for a single trigger.
type TriggerFormState struct {
	fieldCursor int
	opCursor    int
	valueInput  textinput.Model
}

// ActionFormState holds the form state for a single action.
type ActionFormState struct {
	typeCursor int
	valueInput textinput.Model
}

// WorkflowRuleCreatorModel handles creating a new workflow rule via TUI.
// The implementation is split across multiple files:
// - workflow_rule_creator.go: core model and constructors
// - workflow_rule_creator_update.go: Update() and input handlers
// - workflow_rule_creator_view.go: View() and rendering
// - workflow_rule_creator_validation.go: saveRule() validation
type WorkflowRuleCreatorModel struct {
	config        *config.Config
	configManager *config.Manager

	// Form fields
	nameInput textinput.Model

	// Triggers (at least one required)
	triggers     []TriggerFormState
	triggerLogic workflow.TriggerLogic

	// Actions (at least one required)
	actions []ActionFormState

	// Static lists for selectors
	triggerFields []workflow.TriggerField
	triggerOps    []workflow.TriggerOperation
	actionTypes   []workflow.ActionType

	// Focus tracking
	// 0 = name, then triggers (3 fields each), then logic toggle, then actions (2 fields each)
	focusIndex int

	// For editing existing rules
	editingRule *workflow.WorkflowRule

	// Overlay pickers
	showingFieldPicker  bool
	fieldPickerTarget   int
	fieldPicker         *TriggerFieldPickerModel
	showingActionPicker bool
	actionPickerTarget  int
	actionPicker        *ActionTypePickerModel

	width  int
	height int
	err    error
}

// NewWorkflowRuleCreatorModel creates a new workflow rule creator.
func NewWorkflowRuleCreatorModel(cfg *config.Config, manager *config.Manager) *WorkflowRuleCreatorModel {
	factory := DefaultFormFieldFactory()

	nameInput := factory.NewTextInput("Rule name")
	nameInput.Focus()

	// Initialize with one trigger
	triggerValue := factory.NewTextInput("Value")
	triggers := []TriggerFormState{
		{fieldCursor: 0, opCursor: 0, valueInput: triggerValue},
	}

	// Initialize with one action
	actionValue := factory.NewTextInput("Value")
	actions := []ActionFormState{
		{typeCursor: 0, valueInput: actionValue},
	}

	return &WorkflowRuleCreatorModel{
		config:        cfg,
		configManager: manager,
		nameInput:     nameInput,
		triggers:      triggers,
		triggerLogic:  workflow.LogicAnd,
		actions:       actions,
		triggerFields: workflow.ValidTriggerFields(),
		triggerOps:    workflow.ValidTriggerOperations(),
		actionTypes:   workflow.ValidActionTypes(),
		focusIndex:    0,
		width:         80,
		height:        24,
	}
}

// NewWorkflowRuleEditorModel creates a model for editing an existing rule.
func NewWorkflowRuleEditorModel(cfg *config.Config, manager *config.Manager, rule workflow.WorkflowRule) *WorkflowRuleCreatorModel {
	m := NewWorkflowRuleCreatorModel(cfg, manager)
	m.editingRule = &rule

	// Populate name
	m.nameInput.SetValue(rule.Name)

	// Populate triggers
	ruleTriggers := rule.GetTriggers()
	m.triggers = make([]TriggerFormState, len(ruleTriggers))
	for i, t := range ruleTriggers {
		factory := DefaultFormFieldFactory()
		valueInput := factory.NewTextInput("Value")
		valueInput.SetValue(t.Value)

		// Find cursor positions
		fieldCursor := 0
		for j, f := range m.triggerFields {
			if f == t.Field {
				fieldCursor = j
				break
			}
		}
		opCursor := 0
		for j, op := range m.triggerOps {
			if op == t.Operation {
				opCursor = j
				break
			}
		}

		m.triggers[i] = TriggerFormState{
			fieldCursor: fieldCursor,
			opCursor:    opCursor,
			valueInput:  valueInput,
		}
	}
	if len(m.triggers) == 0 {
		factory := DefaultFormFieldFactory()
		valueInput := factory.NewTextInput("Value")
		m.triggers = []TriggerFormState{{valueInput: valueInput}}
	}

	m.triggerLogic = rule.GetTriggerLogic()

	// Populate actions
	m.actions = make([]ActionFormState, len(rule.Actions))
	for i, a := range rule.Actions {
		factory := DefaultFormFieldFactory()
		valueInput := factory.NewTextInput("Value")
		valueInput.SetValue(a.Value)

		typeCursor := 0
		for j, at := range m.actionTypes {
			if at == a.ActionType {
				typeCursor = j
				break
			}
		}

		m.actions[i] = ActionFormState{
			typeCursor: typeCursor,
			valueInput: valueInput,
		}
	}
	if len(m.actions) == 0 {
		factory := DefaultFormFieldFactory()
		valueInput := factory.NewTextInput("Value")
		m.actions = []ActionFormState{{valueInput: valueInput}}
	}

	return m
}

// SetSize sets the view dimensions.
func (m *WorkflowRuleCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextInput(&m.nameInput, width)
	for i := range m.triggers {
		formFactory.ResizeTextInput(&m.triggers[i].valueInput, width)
	}
	for i := range m.actions {
		formFactory.ResizeTextInput(&m.actions[i].valueInput, width)
	}
}

// Init initializes the model.
func (m *WorkflowRuleCreatorModel) Init() tea.Cmd {
	return textinput.Blink
}

// focusFieldCount returns total number of focusable fields.
// Layout: name(1) + triggers(3 each) + logic(1 if >1 trigger) + actions(2 each)
func (m *WorkflowRuleCreatorModel) focusFieldCount() int {
	count := 1 // name
	count += len(m.triggers) * 3
	if len(m.triggers) > 1 {
		count++ // logic toggle
	}
	count += len(m.actions) * 2
	return count
}

// getFocusSection returns which section the current focus is in and the offset within that section.
// Returns: section ("name", "trigger", "logic", "action"), trigger/action index, field within section.
func (m *WorkflowRuleCreatorModel) getFocusSection() (section string, itemIdx int, fieldIdx int) {
	idx := m.focusIndex

	// Name field
	if idx == 0 {
		return "name", 0, 0
	}
	idx--

	// Trigger fields (3 per trigger: field, op, value)
	triggersTotal := len(m.triggers) * 3
	if idx < triggersTotal {
		itemIdx = idx / 3
		fieldIdx = idx % 3
		return "trigger", itemIdx, fieldIdx
	}
	idx -= triggersTotal

	// Logic toggle (only if >1 trigger)
	if len(m.triggers) > 1 {
		if idx == 0 {
			return "logic", 0, 0
		}
		idx--
	}

	// Action fields (2 per action: type, value)
	itemIdx = idx / 2
	fieldIdx = idx % 2
	return "action", itemIdx, fieldIdx
}

// WorkflowRuleCreatedMsg signals a new rule was created.
type WorkflowRuleCreatedMsg struct {
	Rule workflow.WorkflowRule
}

// WorkflowRuleEditedMsg signals an existing rule was edited.
type WorkflowRuleEditedMsg struct {
	Rule workflow.WorkflowRule
}

// ShowWorkflowRuleCreatorMsg triggers showing the rule creator.
type ShowWorkflowRuleCreatorMsg struct{}

// ShowWorkflowRuleEditorMsg triggers showing the rule editor.
type ShowWorkflowRuleEditorMsg struct {
	Rule workflow.WorkflowRule
}
