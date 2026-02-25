package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
)

// Smart list field definitions for the criteria builder.

// SLField represents a filterable field in the smart list builder.
type SLField struct {
	Name       string
	Operations []string
	ValueType  string   // "enum", "text", "numeric", "bool"
	EnumValues []string // for enum type
}

var smartListFields = []SLField{
	{Name: "type", Operations: []string{"equals", "not_equals"}, ValueType: "enum", EnumValues: []string{"epic", "story", "task", "bug", "idea", "action"}},
	{Name: "status", Operations: []string{"equals", "not_equals"}, ValueType: "enum", EnumValues: []string{"todo", "in_progress", "review", "standby", "blocked", "done", "canceled"}},
	{Name: "priority", Operations: []string{"equals", "less_than", "greater_than"}, ValueType: "enum", EnumValues: []string{"1", "2", "3", "4", "5"}},
	{Name: "tags", Operations: []string{"contains"}, ValueType: "text"},
	{Name: "due_date", Operations: []string{"equals"}, ValueType: "enum", EnumValues: []string{"overdue", "today", "this_week", "none"}},
	{Name: "section", Operations: []string{"equals", "not_equals"}, ValueType: "text"},
	{Name: "score_min", Operations: []string{"greater_than"}, ValueType: "numeric"},
	{Name: "actionable", Operations: []string{"equals"}, ValueType: "enum", EnumValues: []string{"true", "false"}},
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

// Tab indices for the smart list builder.
const (
	slTabCriteria = iota
	slTabOptions
)

// Sort field options for the smart list builder.
var slSortFieldNames = []string{"Default", "Priority", "Due Date", "Created", "Status", "Score"}

// SmartListBuilderModel handles creating or editing a smart list via TUI.
type SmartListBuilderModel struct {
	config        *config.Config
	configManager *config.Manager

	// Form fields
	nameInput textinput.Model

	// Criteria (at least one required)
	criteria []CriterionFormState
	logic    string // "and" or "or"

	// Options
	sortField        int
	sortAscending    bool
	includeCompleted bool

	// Tab and layout state
	activeTab int // 0=criteria, 1=options

	// Focus tracking (tab-scoped)
	// Criteria tab: 0=name, then 3 fields per criterion (field, op, value), then logic
	// Options tab: 0=sort field, 1=sort direction, 2=include completed
	focusIndex int

	// Scroll offset for editor pane when content exceeds height
	editorScrollOffset int

	// For editing existing smart lists
	editingID string // non-empty when editing

	width  int
	height int
	err    error
}

// NewSmartListBuilderModel creates a new smart list builder (create mode).
func NewSmartListBuilderModel(cfg *config.Config, manager *config.Manager) *SmartListBuilderModel {
	factory := DefaultFormFieldFactory()

	nameInput := factory.NewTextInput("Smart list name")
	nameInput.Focus()

	valueInput := factory.NewTextInput("Value")
	criteria := []CriterionFormState{
		{fieldCursor: 0, opCursor: 0, valueInput: valueInput},
	}

	return &SmartListBuilderModel{
		config:        cfg,
		configManager: manager,
		nameInput:     nameInput,
		criteria:      criteria,
		logic:         "and",
		activeTab:     slTabCriteria,
		focusIndex:    0,
		width:         80,
		height:        24,
	}
}

// NewSmartListEditorModel creates a builder for editing an existing smart list.
func NewSmartListEditorModel(cfg *config.Config, manager *config.Manager, sl *config.SmartList) *SmartListBuilderModel {
	m := NewSmartListBuilderModel(cfg, manager)
	m.editingID = sl.ID

	m.nameInput.SetValue(sl.Name)
	m.logic = sl.EffectiveLogic()
	m.sortField = sl.SortField
	m.sortAscending = sl.SortAscending
	m.includeCompleted = sl.IncludeCompleted

	// Populate criteria
	if len(sl.Criteria) > 0 {
		m.criteria = make([]CriterionFormState, len(sl.Criteria))
		for i, c := range sl.Criteria {
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
func (m *SmartListBuilderModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextInput(&m.nameInput, width)
	for i := range m.criteria {
		formFactory.ResizeTextInput(&m.criteria[i].valueInput, width)
	}
}

// Init initializes the model.
func (m *SmartListBuilderModel) Init() tea.Cmd {
	return textinput.Blink
}

// focusFieldCount returns total number of focusable fields for the current tab.
func (m *SmartListBuilderModel) focusFieldCount() int {
	switch m.activeTab {
	case slTabCriteria:
		count := 1 // name
		count += len(m.criteria) * 3
		if len(m.criteria) > 1 {
			count++ // logic toggle
		}
		return count
	case slTabOptions:
		return 3 // sort field, sort direction, include completed
	}
	return 1
}

// getFocusSection returns which section the current focus is in and the offset within that section.
func (m *SmartListBuilderModel) getFocusSection() (section string, itemIdx int, fieldIdx int) {
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
			return "sort_field", 0, 0
		case 1:
			return "sort_dir", 0, 0
		case 2:
			return "include_completed", 0, 0
		}
	}

	return "name", 0, 0
}

// currentField returns the SLField for a criterion at the given index.
func (m *SmartListBuilderModel) currentField(criterionIdx int) SLField {
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
func (m *SmartListBuilderModel) isEnumValue(criterionIdx int) bool {
	f := m.currentField(criterionIdx)
	return f.ValueType == "enum"
}

// SmartListSavedMsg signals a smart list was created or updated.
type SmartListSavedMsg struct {
	SmartList config.SmartList
	IsNew     bool
}

// SmartListDeletedMsg signals a smart list was deleted.
type SmartListDeletedMsg struct {
	ID string
}

// ShowSmartListBuilderMsg triggers showing the smart list builder (create mode).
type ShowSmartListBuilderMsg struct{}

// ShowSmartListEditorMsg triggers showing the smart list builder (edit mode).
type ShowSmartListEditorMsg struct {
	SmartList *config.SmartList
}

// ShouldCaptureEsc returns true to let the builder handle esc internally.
func (m *SmartListBuilderModel) ShouldCaptureEsc() bool {
	return true
}

func (m *SmartListBuilderModel) HasTextInput() bool { return true }
func (m *SmartListBuilderModel) IsFullHeight() bool { return false }

// addCriterion appends a new empty criterion.
func (m *SmartListBuilderModel) addCriterion() {
	factory := DefaultFormFieldFactory()
	valueInput := factory.NewTextInput("Value")
	m.criteria = append(m.criteria, CriterionFormState{valueInput: valueInput})
}

// removeCriterion removes the criterion at the given index (minimum 1 kept).
func (m *SmartListBuilderModel) removeCriterion(idx int) {
	if len(m.criteria) <= 1 || idx < 0 || idx >= len(m.criteria) {
		return
	}
	m.criteria = append(m.criteria[:idx], m.criteria[idx+1:]...)
	total := m.focusFieldCount()
	if m.focusIndex >= total {
		m.focusIndex = total - 1
	}
}

// buildSmartList constructs a SmartList from the current form state.
func (m *SmartListBuilderModel) buildSmartList() config.SmartList {
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
	}

	id := m.editingID
	if id == "" {
		id = uuid.New().String()
	}

	return config.SmartList{
		ID:               id,
		Name:             m.nameInput.Value(),
		Criteria:         criteria,
		Logic:            m.logic,
		SortField:        m.sortField,
		SortAscending:    m.sortAscending,
		IncludeCompleted: m.includeCompleted,
	}
}
