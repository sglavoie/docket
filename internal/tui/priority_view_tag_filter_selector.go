package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// PriorityTagFilterSelectorModel allows selecting multiple tags to filter by in priority view.
type PriorityTagFilterSelectorModel struct {
	*BasePicker[TagInfo]
	priority *PriorityViewModel
}

// NewPriorityTagFilterSelectorModel creates a new tag filter selector for priority view.
func NewPriorityTagFilterSelectorModel(priority *PriorityViewModel) *PriorityTagFilterSelectorModel {
	tags := buildPriorityTagList(priority)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          "Filter by Tags",
		EmptyMessage:   "No tags in project",
		NoMatchMessage: "No matching tags",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for tag := range priority.filter.Tags {
		picker.PreSelectByID(tag)
	}

	return &PriorityTagFilterSelectorModel{
		BasePicker: picker,
		priority:   priority,
	}
}

// buildPriorityTagList scans all non-completed tasks and builds a sorted list of tags with counts.
func buildPriorityTagList(priority *PriorityViewModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, task := range priority.project.Tasks {
		if task.Status.IsComplete() {
			continue
		}
		for _, tag := range task.Tags {
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
func (m *PriorityTagFilterSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *PriorityTagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *PriorityTagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return PriorityTagFilterSelectorDoneMsg{Canceled: true} },
		func(selected []string) tea.Msg {
			return PriorityTagFilterSelectorDoneMsg{SelectedTags: selected, Canceled: false}
		},
	)
	return m, cmd
}

// View renders the tag filter selector.
func (m *PriorityTagFilterSelectorModel) View() string {
	return filterSelectorView(m.BasePicker, "tag")
}
