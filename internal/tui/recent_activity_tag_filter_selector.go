package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// RecentActivityTagFilterSelectorModel allows selecting multiple tags to filter by in recent activity.
type RecentActivityTagFilterSelectorModel struct {
	*BasePicker[TagInfo]
}

// NewRecentActivityTagFilterSelectorModel creates a new tag filter selector for recent activity.
func NewRecentActivityTagFilterSelectorModel(ra *RecentActivityModel) *RecentActivityTagFilterSelectorModel {
	tags := buildRecentActivityTagList(ra)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          "Filter by Tags",
		EmptyMessage:   "No tags found",
		NoMatchMessage: "No matching tags",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for tag := range ra.filter.Tags {
		picker.PreSelectByID(tag)
	}

	return &RecentActivityTagFilterSelectorModel{
		BasePicker: picker,
	}
}

// buildRecentActivityTagList scans all tasks and builds a sorted tag list.
func buildRecentActivityTagList(ra *RecentActivityModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, dt := range ra.tasks {
		for _, tag := range dt.Task.Tags {
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
func (m *RecentActivityTagFilterSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *RecentActivityTagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *RecentActivityTagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return RecentActivityTagFilterSelectorDoneMsg{Canceled: true} },
		func(selected []string) tea.Msg {
			return RecentActivityTagFilterSelectorDoneMsg{SelectedTags: selected, Canceled: false}
		},
	)
	return m, cmd
}

// View renders the tag filter selector.
func (m *RecentActivityTagFilterSelectorModel) View() string {
	return filterSelectorView(m.BasePicker, "tag")
}
