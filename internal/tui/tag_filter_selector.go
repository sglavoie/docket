package tui

import (
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// ShowTagFilterMsg is the unified message to open the tag filter selector.
// Callers pre-build the tag list and pass currently selected tags.
type ShowTagFilterMsg struct {
	Tags     []TagInfo
	Selected map[string]bool
	Source   ViewType
	Title    string
}

// TagFilterDoneMsg is the unified response from the tag filter selector.
// Source identifies which view opened it so the handler can route the result.
type TagFilterDoneMsg struct {
	SelectedTags []string
	Canceled     bool
	Source       ViewType
}

// TagFilterSelectorModel is the unified tag filter selector.
type TagFilterSelectorModel struct {
	*BasePicker[TagInfo]
	source ViewType
}

// NewTagFilterSelectorModel creates a unified tag filter selector from a ShowTagFilterMsg.
func NewTagFilterSelectorModel(msg ShowTagFilterMsg) *TagFilterSelectorModel {
	title := msg.Title
	if title == "" {
		title = "Filter by Tags"
	}

	picker := NewBasePicker(msg.Tags, PickerConfig{
		Title:          title,
		EmptyMessage:   "No tags found.",
		NoMatchMessage: "No matching tags.",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for tag := range msg.Selected {
		picker.PreSelectByID(tag)
	}

	return &TagFilterSelectorModel{
		BasePicker: picker,
		source:     msg.Source,
	}
}

// BuildTagList scans tasks and builds a sorted list of tags with counts.
// If excludeCompleted is true, completed tasks are skipped.
func BuildTagList(tasks []*models.Task, excludeCompleted bool) []TagInfo {
	tagCounts := make(map[string]int)
	for _, task := range tasks {
		if excludeCompleted && task.Status.IsComplete() {
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
func (m *TagFilterSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *TagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	source := m.source
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return TagFilterDoneMsg{Canceled: true, Source: source} },
		func(selected []string) tea.Msg {
			return TagFilterDoneMsg{SelectedTags: selected, Canceled: false, Source: source}
		},
	)
	return m, cmd
}

// View renders the tag filter selector.
func (m *TagFilterSelectorModel) View() tea.View {
	return tea.NewView(filterSelectorView(m.BasePicker, "tag"))
}

func (m *TagFilterSelectorModel) StatusBarLabel() string { return "Filter by Tags" }
