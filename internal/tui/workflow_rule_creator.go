package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
)

// TriggerFormState holds the form state for a single trigger.
type TriggerFormState struct {
	fieldCursor int
	opCursor    int
	valueInput  textinput.Model
	groupIndex  int // -1 = ungrouped, 0+ = group index
}

// TriggerGroupFormState holds form state for a trigger sub-group.
type TriggerGroupFormState struct {
	logic workflow.TriggerLogic // Logic within this sub-group
}

// ActionFormState holds the form state for a single action.
type ActionFormState struct {
	typeCursor int
	valueInput textinput.Model
	// Condition fields (optional per-action gate)
	hasCondition    bool
	condFieldCursor int
	condOpCursor    int
	condValueInput  textinput.Model
}

// Tab indices for the workflow rule creator
const (
	tabTriggers = iota
	tabActions
	tabScope
)

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
	nameInput     textinput.Model
	priorityInput textinput.Model

	// Triggers (at least one required)
	triggers      []TriggerFormState
	triggerLogic  workflow.TriggerLogic
	triggerGroups []TriggerGroupFormState // Sub-groups for nested trigger logic

	// Actions (at least one required)
	actions []ActionFormState

	// Scope inputs (comma-separated)
	scopeProjectsInput textinput.Model
	scopeTypesInput    textinput.Model
	scopeSectionsInput textinput.Model

	// Static lists for selectors
	triggerFields []workflow.TriggerField
	triggerOps    []workflow.TriggerOperation
	actionTypes   []workflow.ActionType

	// Tab and layout state
	activeTab    int  // 0=triggers, 1=actions
	previewShown bool // for narrow screen toggle (Ctrl+p)

	// Focus tracking (tab-scoped)
	// Triggers tab: 0=name, then 3 fields per trigger (field, op, value), then logic
	// Actions tab: 0=name, then 2 fields per action (type, value)
	focusIndex int

	// Scroll offset for editor pane when content exceeds height
	editorScrollOffset int

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

	// Discard confirmation state
	confirmDiscard bool
	confirmOverlay ConfirmDialog
}

// NewWorkflowRuleCreatorModel creates a new workflow rule creator.
func NewWorkflowRuleCreatorModel(cfg *config.Config, manager *config.Manager) *WorkflowRuleCreatorModel {
	factory := DefaultFormFieldFactory()

	nameInput := factory.NewTextInput("Rule name")
	nameInput.Focus()

	priorityInput := factory.NewTextInput("0-99, 0=default")
	priorityInput.CharLimit = 2

	// Initialize with one trigger
	triggerValue := factory.NewTextInput("Value")
	triggers := []TriggerFormState{
		{fieldCursor: 0, opCursor: 0, valueInput: triggerValue, groupIndex: -1},
	}

	// Initialize with one action
	actionValue := factory.NewTextInput("Value")
	condValue := factory.NewTextInput("Condition value")
	actions := []ActionFormState{
		{typeCursor: 0, valueInput: actionValue, condValueInput: condValue},
	}

	scopeProjectsInput := factory.NewTextInput("Projects (comma-separated)")
	scopeTypesInput := factory.NewTextInput("Types (comma-separated)")
	scopeSectionsInput := factory.NewTextInput("Sections (comma-separated)")

	return &WorkflowRuleCreatorModel{
		config:             cfg,
		configManager:      manager,
		nameInput:          nameInput,
		priorityInput:      priorityInput,
		triggers:           triggers,
		triggerLogic:       workflow.LogicAnd,
		actions:            actions,
		scopeProjectsInput: scopeProjectsInput,
		scopeTypesInput:    scopeTypesInput,
		scopeSectionsInput: scopeSectionsInput,
		triggerFields:      workflow.ValidTriggerFields(),
		triggerOps:         workflow.ValidTriggerOperations(),
		actionTypes:        workflow.ValidActionTypes(),
		activeTab:          tabTriggers,
		previewShown:       false,
		focusIndex:         0,
		editorScrollOffset: 0,
		width:              80,
		height:             24,
	}
}

// isWideScreen returns true if the screen is wide enough for split layout (≥81 chars)
func (m *WorkflowRuleCreatorModel) isWideScreen() bool {
	return m.width >= 81
}

// NewWorkflowRuleEditorModel creates a model for editing an existing rule.
func NewWorkflowRuleEditorModel(cfg *config.Config, manager *config.Manager, rule workflow.WorkflowRule) *WorkflowRuleCreatorModel {
	m := NewWorkflowRuleCreatorModel(cfg, manager)
	m.editingRule = &rule

	// Populate name and priority
	m.nameInput.SetValue(rule.Name)
	if rule.Priority > 0 {
		m.priorityInput.SetValue(fmt.Sprintf("%d", rule.Priority))
	}

	// Populate triggers - use TriggerGroup if present, otherwise flat triggers
	if rule.TriggerGroup != nil {
		m.populateFromTriggerGroup(rule.TriggerGroup)
	} else {
		ruleTriggers := rule.GetTriggers()
		m.triggers = make([]TriggerFormState, len(ruleTriggers))
		for i, t := range ruleTriggers {
			factory := DefaultFormFieldFactory()
			valueInput := factory.NewTextInput("Value")
			valueInput.SetValue(t.Value)

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
				groupIndex:  -1,
			}
		}
	}
	if len(m.triggers) == 0 {
		factory := DefaultFormFieldFactory()
		valueInput := factory.NewTextInput("Value")
		m.triggers = []TriggerFormState{{valueInput: valueInput, groupIndex: -1}}
	}

	m.triggerLogic = rule.GetTriggerLogic()

	// Populate actions
	m.actions = make([]ActionFormState, len(rule.Actions))
	for i, a := range rule.Actions {
		factory := DefaultFormFieldFactory()
		valueInput := factory.NewTextInput("Value")
		valueInput.SetValue(a.Value)
		condValueInput := factory.NewTextInput("Condition value")

		typeCursor := 0
		for j, at := range m.actionTypes {
			if at == a.ActionType {
				typeCursor = j
				break
			}
		}

		state := ActionFormState{
			typeCursor:     typeCursor,
			valueInput:     valueInput,
			condValueInput: condValueInput,
		}

		// Populate condition if present
		if a.Condition != nil {
			state.hasCondition = true
			condValueInput.SetValue(a.Condition.Value)
			for j, f := range m.triggerFields {
				if f == a.Condition.Field {
					state.condFieldCursor = j
					break
				}
			}
			for j, op := range m.triggerOps {
				if op == a.Condition.Operation {
					state.condOpCursor = j
					break
				}
			}
		}

		m.actions[i] = state
	}
	if len(m.actions) == 0 {
		factory := DefaultFormFieldFactory()
		valueInput := factory.NewTextInput("Value")
		condValueInput := factory.NewTextInput("Condition value")
		m.actions = []ActionFormState{{valueInput: valueInput, condValueInput: condValueInput}}
	}

	// Populate scope
	if rule.Scope != nil {
		if len(rule.Scope.Projects) > 0 {
			m.scopeProjectsInput.SetValue(strings.Join(rule.Scope.Projects, ", "))
		}
		if len(rule.Scope.Types) > 0 {
			m.scopeTypesInput.SetValue(strings.Join(rule.Scope.Types, ", "))
		}
		if len(rule.Scope.Sections) > 0 {
			m.scopeSectionsInput.SetValue(strings.Join(rule.Scope.Sections, ", "))
		}
	}

	return m
}

// populateFromTriggerGroup populates form state from a nested TriggerGroup.
// The root group's logic becomes triggerLogic. Sub-groups become triggerGroups.
// Root-level triggers are ungrouped; sub-group triggers get their groupIndex set.
func (m *WorkflowRuleCreatorModel) populateFromTriggerGroup(tg *workflow.TriggerGroup) {
	m.triggerLogic = tg.Logic
	if m.triggerLogic == "" {
		m.triggerLogic = workflow.LogicAnd
	}
	m.triggers = nil
	m.triggerGroups = nil

	// Add root-level triggers (ungrouped)
	for _, t := range tg.Triggers {
		m.triggers = append(m.triggers, m.makeTriggerFormState(t, -1))
	}

	// Add sub-groups and their triggers
	for _, sub := range tg.Groups {
		groupIdx := len(m.triggerGroups)
		logic := sub.Logic
		if logic == "" {
			logic = workflow.LogicAnd
		}
		m.triggerGroups = append(m.triggerGroups, TriggerGroupFormState{logic: logic})
		for _, t := range sub.Triggers {
			m.triggers = append(m.triggers, m.makeTriggerFormState(t, groupIdx))
		}
		// Note: deeper nesting is flattened to one level in the TUI
		for _, deepSub := range sub.Groups {
			for _, t := range deepSub.Triggers {
				m.triggers = append(m.triggers, m.makeTriggerFormState(t, groupIdx))
			}
		}
	}
}

// makeTriggerFormState creates a TriggerFormState from a RuleTrigger.
func (m *WorkflowRuleCreatorModel) makeTriggerFormState(t workflow.RuleTrigger, groupIndex int) TriggerFormState {
	factory := DefaultFormFieldFactory()
	valueInput := factory.NewTextInput("Value")
	valueInput.SetValue(t.Value)

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

	return TriggerFormState{
		fieldCursor: fieldCursor,
		opCursor:    opCursor,
		valueInput:  valueInput,
		groupIndex:  groupIndex,
	}
}

// SetSize sets the view dimensions.
func (m *WorkflowRuleCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextInput(&m.nameInput, width)
	formFactory.ResizeTextInput(&m.priorityInput, width)
	for i := range m.triggers {
		formFactory.ResizeTextInput(&m.triggers[i].valueInput, width)
	}
	for i := range m.actions {
		formFactory.ResizeTextInput(&m.actions[i].valueInput, width)
		formFactory.ResizeTextInput(&m.actions[i].condValueInput, width)
	}
	formFactory.ResizeTextInput(&m.scopeProjectsInput, width)
	formFactory.ResizeTextInput(&m.scopeTypesInput, width)
	formFactory.ResizeTextInput(&m.scopeSectionsInput, width)
}

// Init initializes the model.
func (m *WorkflowRuleCreatorModel) Init() tea.Cmd {
	return textinput.Blink
}

// focusFieldCount returns total number of focusable fields for the current tab.
// Triggers tab: name(1) + priority(1) + triggers(3 each) + logic(1 if >1 trigger)
// Actions tab: name(1) + priority(1) + actions(2 each)
// Scope tab: name(1) + priority(1) + 3 text inputs (projects, types, sections)
func (m *WorkflowRuleCreatorModel) focusFieldCount() int {
	count := 2 // name + priority are always first
	switch m.activeTab {
	case tabTriggers:
		count += len(m.triggers) * 3
		if len(m.triggers) > 1 {
			count++ // logic toggle
		}
	case tabActions:
		for _, a := range m.actions {
			count += 2 // type + value
			if a.hasCondition {
				count += 3 // condField + condOp + condValue
			}
		}
	case tabScope:
		count += 3 // projects, types, sections
	}
	return count
}

// getFocusSection returns which section the current focus is in and the offset within that section.
// Returns: section ("name", "priority", "trigger", "logic", "action", "scope"), item index, field within item.
// Focus is scoped to the current tab.
func (m *WorkflowRuleCreatorModel) getFocusSection() (section string, itemIdx int, fieldIdx int) {
	idx := m.focusIndex

	// Name field (always index 0 on all tabs)
	if idx == 0 {
		return "name", 0, 0
	}
	idx--

	// Priority field (always index 1 on all tabs)
	if idx == 0 {
		return "priority", 0, 0
	}
	idx--

	switch m.activeTab {
	case tabTriggers:
		// Trigger fields (3 per trigger: field, op, value)
		triggersTotal := len(m.triggers) * 3
		if idx < triggersTotal {
			itemIdx = idx / 3
			fieldIdx = idx % 3
			return "trigger", itemIdx, fieldIdx
		}
		idx -= triggersTotal

		// Logic toggle (only if >1 trigger)
		if len(m.triggers) > 1 && idx == 0 {
			return "logic", 0, 0
		}
	case tabActions:
		// Action fields: 2 per action (type, value) + 3 if hasCondition (condField, condOp, condValue)
		for i, a := range m.actions {
			fieldsForAction := 2
			if a.hasCondition {
				fieldsForAction = 5
			}
			if idx < fieldsForAction {
				return "action", i, idx
			}
			idx -= fieldsForAction
		}
	case tabScope:
		// Scope fields: 0=projects, 1=types, 2=sections
		if idx < 3 {
			return "scope", 0, idx
		}
	}

	return "name", 0, 0 // fallback
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

// ShouldCaptureEsc returns true to let the creator handle esc internally.
func (m *WorkflowRuleCreatorModel) ShouldCaptureEsc() bool {
	return true
}

// GetConfirmDialog returns the confirm dialog for overlay rendering.
func (m *WorkflowRuleCreatorModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}

func (m *WorkflowRuleCreatorModel) HasTextInput() bool     { return true }
func (m *WorkflowRuleCreatorModel) IsFullHeight() bool     { return false }
func (m *WorkflowRuleCreatorModel) StatusBarLabel() string { return "Workflow: New Rule" }
