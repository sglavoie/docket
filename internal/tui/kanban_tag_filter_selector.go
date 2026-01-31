package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// KanbanTagFilterSelectorModel allows selecting multiple tags to filter by in kanban.
type KanbanTagFilterSelectorModel struct {
	*BasePicker[TagInfo]
	kanban *KanbanViewModel
}

// NewKanbanTagFilterSelectorModel creates a new tag filter selector for kanban.
func NewKanbanTagFilterSelectorModel(kanban *KanbanViewModel) *KanbanTagFilterSelectorModel {
	tags := buildKanbanTagList(kanban)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          "Filter by Tags",
		EmptyMessage:   "No tags in project",
		NoMatchMessage: "No matching tags",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for tag := range kanban.filter.Tags {
		picker.PreSelectByID(tag)
	}

	return &KanbanTagFilterSelectorModel{
		BasePicker: picker,
		kanban:     kanban,
	}
}

// buildKanbanTagList scans all non-completed tasks and builds a sorted list of tags with counts.
func buildKanbanTagList(kanban *KanbanViewModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, task := range kanban.Project.Tasks {
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
func (m *KanbanTagFilterSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *KanbanTagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *KanbanTagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return KanbanTagFilterSelectorDoneMsg{Canceled: true} },
		func(selected []string) tea.Msg {
			return KanbanTagFilterSelectorDoneMsg{SelectedTags: selected, Canceled: false}
		},
	)
	return m, cmd
}

// View renders the tag filter selector.
func (m *KanbanTagFilterSelectorModel) View() string {
	return filterSelectorView(m.BasePicker, "tag")
}
