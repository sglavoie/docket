package tui

import (
	tea "charm.land/bubbletea/v2"
)

// ColumnInfo represents a single column in the column picker.
type ColumnInfo struct {
	ID           string // Column constant (e.g., ColPriority)
	Label        string // Human-readable label
	OutlinerOnly bool   // If true, excluded from Project Tree column picker
}

func (c ColumnInfo) FilterText() string  { return c.Label }
func (c ColumnInfo) DisplayText() string { return c.Label }
func (c ColumnInfo) CountValue() int     { return 0 }
func (c ColumnInfo) ItemID() string      { return c.ID }

// AllColumns defines the ordered list of configurable columns.
var AllColumns = []ColumnInfo{
	{ID: ColShortID, Label: "Short ID"},
	{ID: ColPriority, Label: "Priority"},
	{ID: ColDueDate, Label: "Due Date"},
	{ID: ColTags, Label: "Tags"},
	{ID: ColScore, Label: "Score"},
	{ID: ColCustomSection, Label: "Custom Section"},
	{ID: ColDependencies, Label: "Dependencies"},
	{ID: ColComments, Label: "Comments"},
	{ID: ColDescription, Label: "Description"},
	{ID: ColAttachments, Label: "Attachments"},
	{ID: ColChecklist, Label: "Checklist"},
	{ID: ColDecisions, Label: "Decisions"},
	{ID: ColProgress, Label: "Progress"},
	{ID: ColRecurrence, Label: "Recurrence"},
	{ID: ColSnooze, Label: "Snooze"},
}

// ShowColumnPickerMsg opens the column picker for a specific view.
type ShowColumnPickerMsg struct {
	Source       ViewType
	ColumnConfig *ColumnConfig
}

// ColumnPickerDoneMsg is returned when the column picker is closed.
type ColumnPickerDoneMsg struct {
	Source   ViewType
	Canceled bool
	Toggles  map[string]bool // Updated toggle map (nil means all visible)
}

// ColumnPickerModel is the picker for toggling column visibility.
type ColumnPickerModel struct {
	*BasePicker[ColumnInfo]
	source ViewType
}

// NewColumnPickerModel creates a column picker pre-selecting currently visible columns.
func NewColumnPickerModel(msg ShowColumnPickerMsg) *ColumnPickerModel {
	cols := AllColumns
	if msg.Source == ViewProjectTree {
		cols = make([]ColumnInfo, 0, len(AllColumns))
		for _, col := range AllColumns {
			if col.OutlinerOnly {
				continue
			}
			cols = append(cols, col)
		}
	}

	picker := NewBasePicker(cols, PickerConfig{
		Title:          "Display Columns",
		EmptyMessage:   "No columns available.",
		NoMatchMessage: "No matching columns.",
		MultiSelect:    true,
	})

	// Pre-select columns that are currently visible
	for _, col := range cols {
		if msg.ColumnConfig.IsVisible(col.ID) {
			picker.PreSelectByID(col.ID)
		}
	}

	return &ColumnPickerModel{
		BasePicker: picker,
		source:     msg.Source,
	}
}

func (m *ColumnPickerModel) Init() tea.Cmd { return nil }

func (m *ColumnPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	source := m.source
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return ColumnPickerDoneMsg{Source: source, Canceled: true} },
		func(selected []string) tea.Msg {
			toggles := buildColumnToggles(selected)
			return ColumnPickerDoneMsg{Source: source, Toggles: toggles}
		},
	)
	return m, cmd
}

func (m *ColumnPickerModel) View() tea.View {
	return tea.NewView(filterSelectorView(m.BasePicker, "column"))
}

func (m *ColumnPickerModel) StatusBarLabel() string { return "Display Columns" }

// buildColumnToggles converts a list of selected column IDs into a toggle map.
// Columns in the selected list are set to true, all others are set to false.
// Always returns an explicit entry for every column so the user's choice
// overrides any column hidden via the global defaults (Settings → Default
// Columns); a nil map would fall through to those defaults.
func buildColumnToggles(selected []string) map[string]bool {
	selectedSet := make(map[string]bool, len(selected))
	for _, id := range selected {
		selectedSet[id] = true
	}

	toggles := make(map[string]bool, len(AllColumns))
	for _, col := range AllColumns {
		toggles[col.ID] = selectedSet[col.ID]
	}
	return toggles
}
