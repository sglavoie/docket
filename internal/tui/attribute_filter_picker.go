package tui

import (
	tea "charm.land/bubbletea/v2"
)

// ShowAttrFilterPickerMsg opens the attribute filter picker overlay.
type ShowAttrFilterPickerMsg struct {
	Source PresentationID
}

// AttrFilterPickerDoneMsg is returned after the user picks an attribute filter.
// Prefix is the filter prefix (e.g. "Fas", "Far") or "Fa" for direct actions.
// Key is the leaf key (e.g. "c" for Critical, "V" for validated toggle, "0" to clear).
type AttrFilterPickerDoneMsg struct {
	Prefix   string
	Key      string
	Canceled bool
	Source   PresentationID
}

// attrFilterItem represents a single attribute filter option in the picker.
type attrFilterItem struct {
	display string
	group   string
	prefix  string
	key     string
}

func (a attrFilterItem) FilterText() string  { return a.group + " " + a.display }
func (a attrFilterItem) DisplayText() string { return a.display }
func (a attrFilterItem) CountValue() int     { return 0 }
func (a attrFilterItem) ItemID() string      { return a.prefix + ":" + a.key }

// allAttrFilterItems is the complete list of attribute filter options shown in the picker.
var allAttrFilterItems = []attrFilterItem{
	// Bug severity
	{display: "Severity: Critical", group: "Bug Severity", prefix: "Fas", key: "c"},
	{display: "Severity: Major", group: "Bug Severity", prefix: "Fas", key: "j"},
	{display: "Severity: Minor", group: "Bug Severity", prefix: "Fas", key: "m"},
	{display: "Severity: Cosmetic", group: "Bug Severity", prefix: "Fas", key: "o"},
	// Bug reproducible
	{display: "Reproducible: Yes", group: "Bug Reproducible", prefix: "Far", key: "y"},
	{display: "Reproducible: No", group: "Bug Reproducible", prefix: "Far", key: "n"},
	{display: "Reproducible: Sometimes", group: "Bug Reproducible", prefix: "Far", key: "s"},
	// Effort
	{display: "Effort: Small", group: "Effort", prefix: "Fae", key: "s"},
	{display: "Effort: Medium", group: "Effort", prefix: "Fae", key: "m"},
	{display: "Effort: Large", group: "Effort", prefix: "Fae", key: "l"},
	{display: "Effort: Unknown", group: "Effort", prefix: "Fae", key: "u"},
	// Idea category
	{display: "Category: Feature", group: "Idea Category", prefix: "Fac", key: "f"},
	{display: "Category: Improvement", group: "Idea Category", prefix: "Fac", key: "i"},
	{display: "Category: Refactor", group: "Idea Category", prefix: "Fac", key: "r"},
	{display: "Category: Research", group: "Idea Category", prefix: "Fac", key: "s"},
	// Idea value
	{display: "Value: Low", group: "Idea Value", prefix: "Fav", key: "l"},
	{display: "Value: Medium", group: "Idea Value", prefix: "Fav", key: "m"},
	{display: "Value: High", group: "Idea Value", prefix: "Fav", key: "h"},
	// Task worktype
	{display: "Worktype: Development", group: "Task Worktype", prefix: "Faw", key: "d"},
	{display: "Worktype: Documentation", group: "Task Worktype", prefix: "Faw", key: "o"},
	{display: "Worktype: Testing", group: "Task Worktype", prefix: "Faw", key: "t"},
	{display: "Worktype: DevOps", group: "Task Worktype", prefix: "Faw", key: "v"},
	{display: "Worktype: Design", group: "Task Worktype", prefix: "Faw", key: "e"},
	// Epic scope
	{display: "Scope: Defined", group: "Epic Scope", prefix: "Fao", key: "d"},
	{display: "Scope: Evolving", group: "Epic Scope", prefix: "Fao", key: "e"},
	{display: "Scope: Undefined", group: "Epic Scope", prefix: "Fao", key: "u"},
	// Story points
	{display: "Points: 1", group: "Story Points", prefix: "Fap", key: "1"},
	{display: "Points: 2", group: "Story Points", prefix: "Fap", key: "2"},
	{display: "Points: 3", group: "Story Points", prefix: "Fap", key: "3"},
	{display: "Points: 5", group: "Story Points", prefix: "Fap", key: "5"},
	{display: "Points: 8", group: "Story Points", prefix: "Fap", key: "8"},
	// Direct actions
	{display: "Toggle: Validated Ideas", group: "Toggle", prefix: "Fa", key: "V"},
	{display: "Clear: All attribute filters", group: "Clear", prefix: "Fa", key: "0"},
}

// AttrFilterPickerModel is a searchable picker for attribute filter options.
type AttrFilterPickerModel struct {
	*BasePicker[attrFilterItem]
	source PresentationID
}

// NewAttrFilterPickerModel creates the picker for the given source view.
func NewAttrFilterPickerModel(msg ShowAttrFilterPickerMsg) *AttrFilterPickerModel {
	items := make([]attrFilterItem, len(allAttrFilterItems))
	copy(items, allAttrFilterItems)
	picker := NewBasePicker(items, PickerConfig{
		Title:          "Filter by Attribute",
		EmptyMessage:   "No attribute filters available.",
		NoMatchMessage: "No matching filters.",
		MultiSelect:    false,
		ShowCounts:     false,
	})
	return &AttrFilterPickerModel{
		BasePicker: picker,
		source:     msg.Source,
	}
}

func (m *AttrFilterPickerModel) Init() tea.Cmd { return nil }

func (m *AttrFilterPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	source := m.source
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return AttrFilterPickerDoneMsg{Canceled: true, Source: source}
			}
		case "enter":
			if m.IsSearchMode() {
				_, cmd := m.HandleKeyMsg(msg)
				return m, cmd
			}
			item := m.SelectedItem()
			var zero attrFilterItem
			if item == zero {
				return m, func() tea.Msg {
					return AttrFilterPickerDoneMsg{Canceled: true, Source: source}
				}
			}
			return m, func() tea.Msg {
				return AttrFilterPickerDoneMsg{
					Prefix:   item.prefix,
					Key:      item.key,
					Canceled: false,
					Source:   source,
				}
			}
		case "x":
			if !m.IsSearchMode() {
				return m, func() tea.Msg {
					return AttrFilterPickerDoneMsg{Prefix: "Fa", Key: "0", Source: source}
				}
			}
			_, cmd := m.HandleKeyMsg(msg)
			return m, cmd
		default:
			_, cmd := m.HandleKeyMsg(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *AttrFilterPickerModel) View() tea.View {
	return tea.NewView(m.RenderInBox(m.Config.Title, MutedStyle.Render("/ search · Enter select · x clear · Esc cancel"), 0))
}
