package tui

import (
	"fmt"
	"strings"

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

func (m *WorkflowRuleCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle picker overlays first
	if m.showingFieldPicker && m.fieldPicker != nil {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.fieldPicker.SetSize(msg.Width, msg.Height)
			return m, nil
		case TriggerFieldPickerDoneMsg:
			m.showingFieldPicker = false
			if !msg.Cancelled && m.fieldPickerTarget < len(m.triggers) {
				// Find cursor position for selected field
				for i, f := range m.triggerFields {
					if f == msg.Field {
						m.triggers[m.fieldPickerTarget].fieldCursor = i
						break
					}
				}
			}
			m.fieldPicker = nil
			return m, nil
		default:
			_, cmd = m.fieldPicker.Update(msg)
			return m, cmd
		}
	}

	if m.showingActionPicker && m.actionPicker != nil {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.actionPicker.SetSize(msg.Width, msg.Height)
			return m, nil
		case ActionTypePickerDoneMsg:
			m.showingActionPicker = false
			if !msg.Cancelled && m.actionPickerTarget < len(m.actions) {
				// Find cursor position for selected action type
				for i, at := range m.actionTypes {
					if at == msg.ActionType {
						m.actions[m.actionPickerTarget].typeCursor = i
						break
					}
				}
			}
			m.actionPicker = nil
			return m, nil
		default:
			_, cmd = m.actionPicker.Update(msg)
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "ctrl+f":
			// Open field picker for current or first trigger
			section, itemIdx, _ := m.getFocusSection()
			targetIdx := 0
			if section == "trigger" {
				targetIdx = itemIdx
			}
			if targetIdx < len(m.triggers) {
				currentField := m.triggerFields[m.triggers[targetIdx].fieldCursor]
				m.fieldPicker = NewTriggerFieldPickerModel(targetIdx, currentField)
				m.fieldPicker.SetSize(m.width, m.height)
				m.fieldPickerTarget = targetIdx
				m.showingFieldPicker = true
			}
			return m, nil

		case "ctrl+t":
			// Open action type picker for current or first action
			section, itemIdx, _ := m.getFocusSection()
			targetIdx := 0
			if section == "action" {
				targetIdx = itemIdx
			}
			if targetIdx < len(m.actions) {
				currentType := m.actionTypes[m.actions[targetIdx].typeCursor]
				m.actionPicker = NewActionTypePickerModel(targetIdx, currentType)
				m.actionPicker.SetSize(m.width, m.height)
				m.actionPickerTarget = targetIdx
				m.showingActionPicker = true
			}
			return m, nil

		case "tab", "shift+tab":
			m.blurCurrentField()
			total := m.focusFieldCount()
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % total
			} else {
				m.focusIndex = (m.focusIndex + total - 1) % total
			}
			m.focusCurrentField()
			return m, nil

		case "enter":
			// Move to next field
			m.blurCurrentField()
			total := m.focusFieldCount()
			m.focusIndex = (m.focusIndex + 1) % total
			m.focusCurrentField()
			return m, nil

		case "ctrl+s":
			return m, m.saveRule()

		case "+", "=":
			return m, m.handleAdd()

		case "-", "_":
			return m, m.handleRemove()

		case " ":
			section, _, _ := m.getFocusSection()
			if section == "logic" {
				if m.triggerLogic == workflow.LogicAnd {
					m.triggerLogic = workflow.LogicOr
				} else {
					m.triggerLogic = workflow.LogicAnd
				}
				return m, nil
			}

		case "left":
			return m, m.handleLeft()

		case "right":
			return m, m.handleRight()

		case "h":
			// Only handle as navigation if not in a text input field
			if !m.isInTextInput() {
				return m, m.handleLeft()
			}

		case "l":
			// Only handle as navigation if not in a text input field
			if !m.isInTextInput() {
				return m, m.handleRight()
			}
		}
	}

	// Update focused text input
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput, cmd = m.nameInput.Update(msg)
	case "trigger":
		if fieldIdx == 2 && itemIdx < len(m.triggers) {
			m.triggers[itemIdx].valueInput, cmd = m.triggers[itemIdx].valueInput.Update(msg)
		}
	case "action":
		if fieldIdx == 1 && itemIdx < len(m.actions) {
			m.actions[itemIdx].valueInput, cmd = m.actions[itemIdx].valueInput.Update(msg)
		}
	}

	return m, cmd
}

// isInTextInput returns true if the current focus is on a text input field.
func (m *WorkflowRuleCreatorModel) isInTextInput() bool {
	section, _, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		return true
	case "trigger":
		return fieldIdx == 2 // value input
	case "action":
		return fieldIdx == 1 // value input
	}
	return false
}

func (m *WorkflowRuleCreatorModel) blurCurrentField() {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Blur()
	case "trigger":
		if fieldIdx == 2 && itemIdx < len(m.triggers) {
			m.triggers[itemIdx].valueInput.Blur()
		}
	case "action":
		if fieldIdx == 1 && itemIdx < len(m.actions) {
			m.actions[itemIdx].valueInput.Blur()
		}
	}
}

func (m *WorkflowRuleCreatorModel) focusCurrentField() {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Focus()
	case "trigger":
		if fieldIdx == 2 && itemIdx < len(m.triggers) {
			m.triggers[itemIdx].valueInput.Focus()
		}
	case "action":
		if fieldIdx == 1 && itemIdx < len(m.actions) {
			m.actions[itemIdx].valueInput.Focus()
		}
	}
}

func (m *WorkflowRuleCreatorModel) handleLeft() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "trigger":
		if itemIdx < len(m.triggers) {
			switch fieldIdx {
			case 0: // field selector
				if m.triggers[itemIdx].fieldCursor > 0 {
					m.triggers[itemIdx].fieldCursor--
				}
			case 1: // op selector
				if m.triggers[itemIdx].opCursor > 0 {
					m.triggers[itemIdx].opCursor--
				}
			}
		}
	case "logic":
		if m.triggerLogic == workflow.LogicOr {
			m.triggerLogic = workflow.LogicAnd
		}
	case "action":
		if itemIdx < len(m.actions) && fieldIdx == 0 {
			if m.actions[itemIdx].typeCursor > 0 {
				m.actions[itemIdx].typeCursor--
			}
		}
	}
	return nil
}

func (m *WorkflowRuleCreatorModel) handleRight() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "trigger":
		if itemIdx < len(m.triggers) {
			switch fieldIdx {
			case 0: // field selector
				if m.triggers[itemIdx].fieldCursor < len(m.triggerFields)-1 {
					m.triggers[itemIdx].fieldCursor++
				}
			case 1: // op selector
				if m.triggers[itemIdx].opCursor < len(m.triggerOps)-1 {
					m.triggers[itemIdx].opCursor++
				}
			}
		}
	case "logic":
		if m.triggerLogic == workflow.LogicAnd {
			m.triggerLogic = workflow.LogicOr
		}
	case "action":
		if itemIdx < len(m.actions) && fieldIdx == 0 {
			if m.actions[itemIdx].typeCursor < len(m.actionTypes)-1 {
				m.actions[itemIdx].typeCursor++
			}
		}
	}
	return nil
}

func (m *WorkflowRuleCreatorModel) handleAdd() tea.Cmd {
	section, _, _ := m.getFocusSection()
	factory := DefaultFormFieldFactory()

	switch section {
	case "trigger", "name":
		// Add a new trigger
		valueInput := factory.NewTextInput("Value")
		m.triggers = append(m.triggers, TriggerFormState{valueInput: valueInput})
	case "action", "logic":
		// Add a new action
		valueInput := factory.NewTextInput("Value")
		m.actions = append(m.actions, ActionFormState{valueInput: valueInput})
	}
	return nil
}

func (m *WorkflowRuleCreatorModel) handleRemove() tea.Cmd {
	section, itemIdx, _ := m.getFocusSection()

	switch section {
	case "trigger":
		if len(m.triggers) > 1 && itemIdx < len(m.triggers) {
			m.triggers = append(m.triggers[:itemIdx], m.triggers[itemIdx+1:]...)
			// Adjust focus if needed
			total := m.focusFieldCount()
			if m.focusIndex >= total {
				m.focusIndex = total - 1
			}
		}
	case "action":
		if len(m.actions) > 1 && itemIdx < len(m.actions) {
			m.actions = append(m.actions[:itemIdx], m.actions[itemIdx+1:]...)
			// Adjust focus if needed
			total := m.focusFieldCount()
			if m.focusIndex >= total {
				m.focusIndex = total - 1
			}
		}
	}
	return nil
}

func (m *WorkflowRuleCreatorModel) saveRule() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("rule name is required")}
		}

		// Build triggers
		triggers := make([]workflow.RuleTrigger, len(m.triggers))
		for i, t := range m.triggers {
			triggers[i] = workflow.RuleTrigger{
				Field:     m.triggerFields[t.fieldCursor],
				Operation: m.triggerOps[t.opCursor],
				Value:     strings.TrimSpace(t.valueInput.Value()),
			}
			if err := triggers[i].Validate(); err != nil {
				return ErrorMsg{Err: fmt.Errorf("trigger %d: %w", i+1, err)}
			}
		}

		// Build actions
		actions := make([]workflow.RuleAction, len(m.actions))
		for i, a := range m.actions {
			actions[i] = workflow.RuleAction{
				ActionType: m.actionTypes[a.typeCursor],
				Value:      strings.TrimSpace(a.valueInput.Value()),
			}
			if err := actions[i].Validate(); err != nil {
				return ErrorMsg{Err: fmt.Errorf("action %d: %w", i+1, err)}
			}
		}

		var rule *workflow.WorkflowRule
		if len(triggers) == 1 {
			rule = workflow.NewRule(name, triggers[0], actions)
		} else {
			rule = workflow.NewCompoundRule(name, triggers, m.triggerLogic, actions)
		}

		// Validate the complete rule
		if err := rule.Validate(); err != nil {
			return ErrorMsg{Err: err}
		}

		// If editing, preserve the original ID and update
		if m.editingRule != nil {
			rule.ID = m.editingRule.ID
			rule.Enabled = m.editingRule.Enabled
			rule.TriggerCount = m.editingRule.TriggerCount
			rule.Source = m.editingRule.Source
			m.config.UpdateWorkflowRule(*rule)
		} else {
			m.config.AddWorkflowRule(*rule)
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		if m.editingRule != nil {
			return WorkflowRuleEditedMsg{Rule: *rule}
		}
		return WorkflowRuleCreatedMsg{Rule: *rule}
	}
}

func (m *WorkflowRuleCreatorModel) View() string {
	// Render picker overlays if active
	if m.showingFieldPicker && m.fieldPicker != nil {
		return m.fieldPicker.View()
	}
	if m.showingActionPicker && m.actionPicker != nil {
		return m.actionPicker.View()
	}

	var b strings.Builder
	section, itemIdx, fieldIdx := m.getFocusSection()

	// Name input
	var nameLabel string
	if section == "name" {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	} else {
		nameLabel = FieldLabelStyle.Render("Name: ")
	}
	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Triggers section
	b.WriteString(BoldStyle.Render("Triggers"))
	if len(m.triggers) > 1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d)", len(m.triggers))))
	}
	b.WriteString("\n")

	for i, t := range m.triggers {
		isFocusedTrigger := section == "trigger" && itemIdx == i

		// Trigger number
		if len(m.triggers) > 1 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d. ", i+1)))
		} else {
			b.WriteString("  ")
		}

		// Field selector
		fieldLabel := "Field: "
		if isFocusedTrigger && fieldIdx == 0 {
			fieldLabel = FieldLabelFocusedStyle.Render("Field: ")
		}
		b.WriteString(fieldLabel)
		b.WriteString(m.renderSelector(m.triggerFields, t.fieldCursor, isFocusedTrigger && fieldIdx == 0, 3))
		b.WriteString("\n")

		// Operation selector
		b.WriteString("     ")
		opLabel := "Op: "
		if isFocusedTrigger && fieldIdx == 1 {
			opLabel = FieldLabelFocusedStyle.Render("Op: ")
		}
		b.WriteString(opLabel)
		b.WriteString(m.renderOpsSelector(t.opCursor, isFocusedTrigger && fieldIdx == 1))
		b.WriteString("\n")

		// Value input
		b.WriteString("     ")
		valueLabel := "Value: "
		if isFocusedTrigger && fieldIdx == 2 {
			valueLabel = FieldLabelFocusedStyle.Render("Value: ")
		}
		b.WriteString(valueLabel)
		b.WriteString(t.valueInput.View())
		b.WriteString("\n")

		if i < len(m.triggers)-1 {
			b.WriteString("\n")
		}
	}

	// Logic toggle (only if >1 trigger)
	if len(m.triggers) > 1 {
		b.WriteString("\n")
		logicLabel := "Logic: "
		if section == "logic" {
			logicLabel = FieldLabelFocusedStyle.Render("Logic: ")
		}
		b.WriteString("  ")
		b.WriteString(logicLabel)
		if m.triggerLogic == workflow.LogicAnd {
			if section == "logic" {
				b.WriteString(SelectorOptionSelectedStyle.Render("[AND]"))
			} else {
				b.WriteString(BoldStyle.Render("[AND]"))
			}
			b.WriteString(SelectorOptionStyle.Render(" OR "))
		} else {
			b.WriteString(SelectorOptionStyle.Render(" AND "))
			if section == "logic" {
				b.WriteString(SelectorOptionSelectedStyle.Render("[OR]"))
			} else {
				b.WriteString(BoldStyle.Render("[OR]"))
			}
		}
		b.WriteString("\n")
	}

	// Actions section
	b.WriteString("\n")
	b.WriteString(BoldStyle.Render("Actions"))
	if len(m.actions) > 1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d)", len(m.actions))))
	}
	b.WriteString("\n")

	for i, a := range m.actions {
		isFocusedAction := section == "action" && itemIdx == i

		// Action number
		if len(m.actions) > 1 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d. ", i+1)))
		} else {
			b.WriteString("  ")
		}

		// Type selector
		typeLabel := "Type: "
		if isFocusedAction && fieldIdx == 0 {
			typeLabel = FieldLabelFocusedStyle.Render("Type: ")
		}
		b.WriteString(typeLabel)
		b.WriteString(m.renderActionSelector(a.typeCursor, isFocusedAction && fieldIdx == 0))
		b.WriteString("\n")

		// Value input (skip for actions that don't need values)
		actionType := m.actionTypes[a.typeCursor]
		if !m.actionNeedsNoValue(actionType) {
			b.WriteString("     ")
			valueLabel := "Value: "
			if isFocusedAction && fieldIdx == 1 {
				valueLabel = FieldLabelFocusedStyle.Render("Value: ")
			}
			b.WriteString(valueLabel)
			b.WriteString(a.valueInput.View())
			b.WriteString("\n")
		}

		if i < len(m.actions)-1 {
			b.WriteString("\n")
		}
	}

	// Error display
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	// Help footer
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("Ctrl+f: fields | Ctrl+t: types | Tab: nav | +/-: add/remove | Ctrl+S: save | Esc: cancel"))

	// Wrap in overlay card
	cardWidth := m.width - 4
	if cardWidth > 85 {
		cardWidth = 85
	}

	title := "Create Workflow Rule"
	if m.editingRule != nil {
		title = "Edit Workflow Rule"
	}

	cfg := OverlayCardConfig{
		Title:        title,
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, cfg)
}

// renderSelector renders a scrollable selector with context around the current selection.
func (m *WorkflowRuleCreatorModel) renderSelector(fields []workflow.TriggerField, cursor int, focused bool, visible int) string {
	var b strings.Builder

	// Calculate visible range
	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(fields) {
		end = len(fields)
		start = end - visible
		if start < 0 {
			start = 0
		}
	}

	// Show left arrow if there are items before
	if start > 0 {
		b.WriteString(MutedStyle.Render("< "))
	} else {
		b.WriteString("  ")
	}

	for i := start; i < end; i++ {
		display := string(fields[i])
		if len(display) > 18 {
			display = display[:15] + "..."
		}

		if i == cursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	// Show right arrow if there are items after
	if end < len(fields) {
		b.WriteString(MutedStyle.Render(" >"))
	}

	return b.String()
}

func (m *WorkflowRuleCreatorModel) renderOpsSelector(cursor int, focused bool) string {
	var b strings.Builder

	opSymbols := map[workflow.TriggerOperation]string{
		workflow.OpEquals:      "=",
		workflow.OpNotEquals:   "!=",
		workflow.OpContains:    "has",
		workflow.OpGreaterThan: ">",
		workflow.OpLessThan:    "<",
	}

	for i, op := range m.triggerOps {
		display := opSymbols[op]
		if display == "" {
			display = string(op)
		}

		if i == cursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	return b.String()
}

func (m *WorkflowRuleCreatorModel) renderActionSelector(cursor int, focused bool) string {
	var b strings.Builder

	// Show limited context around cursor
	visible := 3
	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(m.actionTypes) {
		end = len(m.actionTypes)
		start = end - visible
		if start < 0 {
			start = 0
		}
	}

	if start > 0 {
		b.WriteString(MutedStyle.Render("< "))
	} else {
		b.WriteString("  ")
	}

	for i := start; i < end; i++ {
		display := string(m.actionTypes[i])
		if len(display) > 16 {
			display = display[:13] + "..."
		}

		if i == cursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	if end < len(m.actionTypes) {
		b.WriteString(MutedStyle.Render(" >"))
	}

	return b.String()
}

func (m *WorkflowRuleCreatorModel) actionNeedsNoValue(actionType workflow.ActionType) bool {
	switch actionType {
	case workflow.ActionClearDue, workflow.ActionCompleteParent, workflow.ActionBlockParent:
		return true
	}
	return false
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
