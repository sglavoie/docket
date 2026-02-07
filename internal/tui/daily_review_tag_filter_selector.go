package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// DailyReviewTagFilterSelectorModel allows selecting multiple tags to filter by in daily review.
type DailyReviewTagFilterSelectorModel struct {
	*BasePicker[TagInfo]
}

// NewDailyReviewTagFilterSelectorModel creates a new tag filter selector for daily review.
func NewDailyReviewTagFilterSelectorModel(dr *DailyReviewModel) *DailyReviewTagFilterSelectorModel {
	tags := buildDailyReviewTagList(dr)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          "Filter by Tags",
		EmptyMessage:   "No tags found",
		NoMatchMessage: "No matching tags",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for tag := range dr.filter.Tags {
		picker.PreSelectByID(tag)
	}

	return &DailyReviewTagFilterSelectorModel{
		BasePicker: picker,
	}
}

// buildDailyReviewTagList scans all tasks in all review sections and builds a sorted tag list.
func buildDailyReviewTagList(dr *DailyReviewModel) []TagInfo {
	tagCounts := make(map[string]int)
	sections := [][]DailyReviewTask{
		dr.overdue, dr.dueToday, dr.dueSoon, dr.dueLater,
		dr.future, dr.inProgress, dr.blocked, dr.old, dr.completed,
	}
	for _, section := range sections {
		for _, dt := range section {
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
func (m *DailyReviewTagFilterSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *DailyReviewTagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *DailyReviewTagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return DailyReviewTagFilterSelectorDoneMsg{Canceled: true} },
		func(selected []string) tea.Msg {
			return DailyReviewTagFilterSelectorDoneMsg{SelectedTags: selected, Canceled: false}
		},
	)
	return m, cmd
}

// View renders the tag filter selector.
func (m *DailyReviewTagFilterSelectorModel) View() string {
	return filterSelectorView(m.BasePicker, "tag")
}
