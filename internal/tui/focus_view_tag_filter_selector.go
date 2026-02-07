package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// FocusTagFilterSelectorModel allows selecting multiple tags to filter by in focus view.
type FocusTagFilterSelectorModel struct {
	*BasePicker[TagInfo]
}

// NewFocusTagFilterSelectorModel creates a new tag filter selector for focus view.
func NewFocusTagFilterSelectorModel(focus *FocusViewModel) *FocusTagFilterSelectorModel {
	tags := buildFocusTagList(focus)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          "Filter by Tags",
		EmptyMessage:   "No tags found",
		NoMatchMessage: "No matching tags",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for tag := range focus.filter.Tags {
		picker.PreSelectByID(tag)
	}

	return &FocusTagFilterSelectorModel{
		BasePicker: picker,
	}
}

// buildFocusTagList scans all focus tasks and builds a sorted list of tags with counts.
func buildFocusTagList(focus *FocusViewModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, ft := range focus.tasks {
		for _, tag := range ft.Task.Tags {
			tagCounts[tag]++
		}
	}

	tags := make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		tags = append(tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}

// SetSize sets the view dimensions.
func (m *FocusTagFilterSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *FocusTagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *FocusTagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return FocusTagFilterSelectorDoneMsg{Canceled: true} },
		func(selected []string) tea.Msg {
			return FocusTagFilterSelectorDoneMsg{SelectedTags: selected, Canceled: false}
		},
	)
	return m, cmd
}

// View renders the tag filter selector.
func (m *FocusTagFilterSelectorModel) View() string {
	return filterSelectorView(m.BasePicker, "tag")
}
