package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// Smart list field definitions for the criteria builder.

// SLField represents a filterable field in the filter builder.
type SLField struct {
	Name       string
	Operations []string
	ValueType  string   // "enum", "text", "numeric", "states"
	EnumValues []string // for enum type
}

var smartListFields = []SLField{
	{Name: "type", Operations: []string{"equals", "not_equals"}, ValueType: "enum", EnumValues: []string{"epic", "story", "task", "bug", "idea"}},
	{Name: "state", Operations: []string{"in", "not_in"}, ValueType: "states"},
	{Name: "priority", Operations: []string{"equals", "less_than", "greater_than"}, ValueType: "enum", EnumValues: []string{"1", "2", "3", "4", "5"}},
	{Name: "tags", Operations: []string{"contains"}, ValueType: "text"},
	{Name: "due_date", Operations: []string{"equals"}, ValueType: "enum", EnumValues: []string{"overdue", "today", "this_week", "none"}},
	{Name: "score_min", Operations: []string{"greater_than"}, ValueType: "numeric"},
	{Name: "actionable", Operations: []string{"equals"}, ValueType: "enum", EnumValues: []string{"true", "false"}},
	{Name: "parked", Operations: []string{"equals"}, ValueType: "enum", EnumValues: []string{"true", "false"}},
	{Name: "created_days", Operations: []string{"less_than", "greater_than"}, ValueType: "numeric"},
	{Name: "updated_days", Operations: []string{"less_than", "greater_than"}, ValueType: "numeric"},
	{Name: "completed_days", Operations: []string{"less_than", "greater_than"}, ValueType: "numeric"},
}

// CriterionFormState holds the form state for a single criterion.
type CriterionFormState struct {
	fieldCursor int // index into smartListFields
	opCursor    int // index into the current field's Operations
	valueCursor int // index into EnumValues (for enum fields)
	valueInput  textinput.Model
}

// Tab indices for the filter builder.
const (
	slTabCriteria = iota
	slTabOptions
)

// FilterBuilderModel handles creating or editing a smart list via TUI.
type FilterBuilderModel struct {
	runtime *config.RuntimeState

	// Form fields
	nameInput textinput.Model

	// Criteria (at least one required)
	criteria []CriterionFormState
	logic    string // "and" or "or"

	// Options
	includeCompleted bool

	// Tab and layout state
	activeTab int // 0=criteria, 1=options

	// Focus tracking (tab-scoped)
	// Criteria tab: 0=name, then 3 fields per criterion (field, op, value), then logic
	// Options tab: include completed
	focusIndex int

	// Scroll offset for editor pane when content exceeds height
	editorScrollOffset int

	// For editing existing smart lists
	editingID string // non-empty when editing

	width  int
	height int
	err    error
}

// NewFilterBuilderModel creates a new filter builder (create mode). The runtime
// is the shared planning-domain snapshot the builder commits saved filters
// through; it may be nil in isolated render/build tests that never persist.
func NewFilterBuilderModel(runtime *config.RuntimeState) *FilterBuilderModel {
	factory := DefaultFormFieldFactory()

	nameInput := factory.NewTextInput("Smart list name")
	nameInput.Focus()

	valueInput := factory.NewTextInput("Value")
	criteria := []CriterionFormState{
		{fieldCursor: 0, opCursor: 0, valueInput: valueInput},
	}

	return &FilterBuilderModel{
		runtime:    runtime,
		nameInput:  nameInput,
		criteria:   criteria,
		logic:      "and",
		activeTab:  slTabCriteria,
		focusIndex: 0,
		width:      80,
		height:     24,
	}
}

// NewFilterEditorModel creates a builder for editing an existing saved filter.
func NewFilterEditorModel(runtime *config.RuntimeState, sf *config.SavedFilter) *FilterBuilderModel {
	m := NewFilterBuilderModel(runtime)
	m.editingID = sf.ID

	m.nameInput.SetValue(sf.Name)
	m.logic = sf.EffectiveLogic()
	m.includeCompleted = sf.IncludeCompleted

	// Populate criteria
	if len(sf.Criteria) > 0 {
		m.criteria = make([]CriterionFormState, len(sf.Criteria))
		for i, c := range sf.Criteria {
			factory := DefaultFormFieldFactory()
			valueInput := factory.NewTextInput("Value")

			fieldCursor := 0
			for j, f := range smartListFields {
				if f.Name == c.Field {
					fieldCursor = j
					break
				}
			}

			opCursor := 0
			field := smartListFields[fieldCursor]
			for j, op := range field.Operations {
				if op == c.Operation {
					opCursor = j
					break
				}
			}

			valueCursor := 0
			if field.ValueType == "enum" {
				for j, v := range field.EnumValues {
					if v == c.Value {
						valueCursor = j
						break
					}
				}
			} else if field.ValueType == "states" {
				values := c.States.Values()
				states := make([]string, len(values))
				for j, state := range values {
					states[j] = string(state)
				}
				valueInput.SetValue(strings.Join(states, ","))
			} else {
				valueInput.SetValue(c.Value)
			}

			m.criteria[i] = CriterionFormState{
				fieldCursor: fieldCursor,
				opCursor:    opCursor,
				valueCursor: valueCursor,
				valueInput:  valueInput,
			}
		}
	}

	return m
}

// SetSize sets the view dimensions.
func (m *FilterBuilderModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextInput(&m.nameInput, width)
	for i := range m.criteria {
		formFactory.ResizeTextInput(&m.criteria[i].valueInput, width)
	}
}

// Init initializes the model.
func (m *FilterBuilderModel) Init() tea.Cmd {
	return textinput.Blink
}

// focusFieldCount returns total number of focusable fields for the current tab.
func (m *FilterBuilderModel) focusFieldCount() int {
	switch m.activeTab {
	case slTabCriteria:
		count := 1 // name
		count += len(m.criteria) * 3
		if len(m.criteria) > 1 {
			count++ // logic toggle
		}
		return count
	case slTabOptions:
		return 1
	}
	return 1
}

// getFocusSection returns which section the current focus is in and the offset within that section.
func (m *FilterBuilderModel) getFocusSection() (section string, itemIdx int, fieldIdx int) {
	idx := m.focusIndex

	switch m.activeTab {
	case slTabCriteria:
		if idx == 0 {
			return "name", 0, 0
		}
		idx--

		criteriaTotal := len(m.criteria) * 3
		if idx < criteriaTotal {
			itemIdx = idx / 3
			fieldIdx = idx % 3
			return "criterion", itemIdx, fieldIdx
		}
		idx -= criteriaTotal

		if len(m.criteria) > 1 && idx == 0 {
			return "logic", 0, 0
		}

	case slTabOptions:
		switch idx {
		case 0:
			return "include_completed", 0, 0
		}
	}

	return "name", 0, 0
}

// currentField returns the SLField for a criterion at the given index.
func (m *FilterBuilderModel) currentField(criterionIdx int) SLField {
	if criterionIdx < 0 || criterionIdx >= len(m.criteria) {
		return smartListFields[0]
	}
	fc := m.criteria[criterionIdx].fieldCursor
	if fc < 0 || fc >= len(smartListFields) {
		return smartListFields[0]
	}
	return smartListFields[fc]
}

// isEnumValue returns true if the current criterion's field uses enum values.
func (m *FilterBuilderModel) isEnumValue(criterionIdx int) bool {
	f := m.currentField(criterionIdx)
	return f.ValueType == "enum"
}

// FilterBuilderSavedMsg signals a saved filter was created or updated via the builder.
type FilterBuilderSavedMsg struct{}

// ShowFilterBuilderMsg triggers showing the filter builder (create mode).
type ShowFilterBuilderMsg struct{}

// ShowFilterEditorMsg triggers showing the filter builder (edit mode).
type ShowFilterEditorMsg struct {
	Filter *config.SavedFilter
}

// ShouldCaptureEsc returns true to let the builder handle esc internally.
func (m *FilterBuilderModel) ShouldCaptureEsc() bool {
	return true
}

func (m *FilterBuilderModel) HasTextInput() bool { return true }
func (m *FilterBuilderModel) IsFullHeight() bool { return false }

// addCriterion appends a new empty criterion.
func (m *FilterBuilderModel) addCriterion() {
	factory := DefaultFormFieldFactory()
	valueInput := factory.NewTextInput("Value")
	m.criteria = append(m.criteria, CriterionFormState{valueInput: valueInput})
}

// removeCriterion removes the criterion at the given index (minimum 1 kept).
func (m *FilterBuilderModel) removeCriterion(idx int) {
	if len(m.criteria) <= 1 || idx < 0 || idx >= len(m.criteria) {
		return
	}
	m.criteria = append(m.criteria[:idx], m.criteria[idx+1:]...)
	total := m.focusFieldCount()
	if m.focusIndex >= total {
		m.focusIndex = total - 1
	}
}

// buildSavedFilter constructs a SavedFilter from the current form state.
func (m *FilterBuilderModel) buildSavedFilter() config.SavedFilter {
	criteria := make([]config.SmartListCriterion, len(m.criteria))
	for i, c := range m.criteria {
		field := smartListFields[c.fieldCursor]
		op := field.Operations[c.opCursor]

		var value string
		if field.ValueType == "enum" {
			if c.valueCursor >= 0 && c.valueCursor < len(field.EnumValues) {
				value = field.EnumValues[c.valueCursor]
			}
		} else {
			value = c.valueInput.Value()
		}

		criteria[i] = config.SmartListCriterion{
			Field:     field.Name,
			Operation: op,
			Value:     value,
		}
		if field.ValueType == "states" {
			states, _ := parseWorkflowStateSetInput(c.valueInput.Value())
			criteria[i].Value = ""
			criteria[i].States = states
		}
	}

	id := m.editingID
	if id == "" {
		id = uuid.New().String()
	}

	return config.SavedFilter{
		ID:               id,
		Name:             m.nameInput.Value(),
		Mode:             "criteria",
		Criteria:         criteria,
		Logic:            m.logic,
		IncludeCompleted: m.includeCompleted,
	}
}

func parseWorkflowStateSetInput(value string) (models.WorkflowStateSet, error) {
	parts := strings.Split(value, ",")
	states := make([]models.WorkflowState, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		states = append(states, models.WorkflowState(strings.ToLower(part)))
	}
	if len(states) == 0 {
		return models.WorkflowStateSet{}, fmt.Errorf("state criterion requires at least one canonical state")
	}
	set, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		return models.WorkflowStateSet{}, err
	}
	return set, nil
}

func (m *FilterBuilderModel) StatusBarLabel() string { return "Smart List" }
