package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// DueDateTagFilterSelectorModel allows selecting multiple tags to filter by in due date overview.
type DueDateTagFilterSelectorModel struct {
	*BasePicker[TagInfo]
}

// NewDueDateTagFilterSelectorModel creates a new tag filter selector for due date overview.
func NewDueDateTagFilterSelectorModel(ddo *DueDateOverviewModel) *DueDateTagFilterSelectorModel {
	tags := buildDueDateTagList(ddo)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          "Filter by Tags",
		EmptyMessage:   "No tags found",
		NoMatchMessage: "No matching tags",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for tag := range ddo.filter.Tags {
		picker.PreSelectByID(tag)
	}

	return &DueDateTagFilterSelectorModel{
		BasePicker: picker,
	}
}

// buildDueDateTagList scans all tasks in the due date groups and builds a sorted tag list.
func buildDueDateTagList(ddo *DueDateOverviewModel) []TagInfo {
	tagCounts := make(map[string]int)
	groups := [][]DueDateTask{ddo.overdue, ddo.today, ddo.thisWeek, ddo.later}
	for _, group := range groups {
		for _, dt := range group {
			for _, tag := range dt.Task.Tags {
				tagCounts[tag]++
			}
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
func (m *DueDateTagFilterSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *DueDateTagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *DueDateTagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return DueDateTagFilterSelectorDoneMsg{Canceled: true} },
		func(selected []string) tea.Msg {
			return DueDateTagFilterSelectorDoneMsg{SelectedTags: selected, Canceled: false}
		},
	)
	return m, cmd
}

// View renders the tag filter selector.
func (m *DueDateTagFilterSelectorModel) View() string {
	return filterSelectorView(m.BasePicker, "tag")
}
