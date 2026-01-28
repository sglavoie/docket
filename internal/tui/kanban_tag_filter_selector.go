package tui

import (
	"fmt"
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

	// Pre-select currently filtered tags
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
		// Only count non-completed tasks (consistent with kanban view)
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return KanbanTagFilterSelectorDoneMsg{Canceled: true}
			}

		case "enter":
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return KanbanTagFilterSelectorDoneMsg{
					SelectedTags: selectedTags,
					Canceled:     false,
				}
			}

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// View renders the tag filter selector.
func (m *KanbanTagFilterSelectorModel) View() string {
	// Selection count header
	selectedCount := len(m.Selected)
	var headerContent string
	if selectedCount == 0 {
		headerContent = MutedStyle.Render("No tags selected (showing all)")
	} else {
		headerContent = MutedStyle.Render(fmt.Sprintf("%d tag%s selected", selectedCount, pluralize(selectedCount, "", "s")))
	}

	return m.RenderInBox(m.Config.Title, headerContent, 0)
}
