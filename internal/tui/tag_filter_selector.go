package tui

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// TagFilterSelectorModel allows selecting multiple tags to filter by.
type TagFilterSelectorModel struct {
	*BasePicker[TagInfo]
	projectTree *ProjectTreeModel
}

// ShowTagFilterSelectorMsg is sent to open the tag filter selector.
type ShowTagFilterSelectorMsg struct {
	ProjectTree *ProjectTreeModel
}

// TagFilterSelectorDoneMsg is sent when the tag filter selector is done.
type TagFilterSelectorDoneMsg struct {
	SelectedTags []string
	Cancelled    bool
}

// NewTagFilterSelectorModel creates a new tag filter selector.
func NewTagFilterSelectorModel(pt *ProjectTreeModel) *TagFilterSelectorModel {
	tags := buildTagList(pt)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          "Filter by Tags",
		EmptyMessage:   "No tags in project",
		NoMatchMessage: "No matching tags",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	// Pre-select currently filtered tags
	for tag := range pt.filterTags {
		picker.PreSelectByID(tag)
	}

	return &TagFilterSelectorModel{
		BasePicker:  picker,
		projectTree: pt,
	}
}

// buildTagList scans all tasks and builds a sorted list of tags with counts.
func buildTagList(pt *ProjectTreeModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, task := range pt.project.Tasks {
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
func (m *TagFilterSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *TagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return TagFilterSelectorDoneMsg{Cancelled: true}
			}

		case "enter":
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return TagFilterSelectorDoneMsg{
					SelectedTags: selectedTags,
					Cancelled:    false,
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
func (m *TagFilterSelectorModel) View() string {
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
