package tui

import (
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TagInfo holds a task tag name and its usage count.
type TagInfo struct {
	Name      string
	TaskCount int
}

// ShowTagFilterMsg is the unified message to open the tag filter selector.
// Callers pre-build the tag list and pass currently selected tags.
type ShowTagFilterMsg struct {
	Tags     []TagInfo
	Selected map[string]bool
	Source   PresentationID
	Title    string
}

// TagFilterDoneMsg is the unified response from the tag filter selector.
// Source identifies which view opened it so the handler can route the result.
type TagFilterDoneMsg struct {
	SelectedTags []string
	Canceled     bool
	Source       PresentationID
}

// TagFilterSelectorModel is the unified tag filter selector.
type TagFilterSelectorModel struct {
	*ListSurfaceAdapter[TagInfo]
	source PresentationID
}

// NewTagFilterSelectorModel creates a unified tag filter selector from a ShowTagFilterMsg.
func NewTagFilterSelectorModel(msg ShowTagFilterMsg) *TagFilterSelectorModel {
	title := msg.Title
	if title == "" {
		title = "Filter by Tags"
	}

	picker := NewListSurfaceAdapter(msg.Tags, PickerConfig{
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
		ListSurfaceAdapter: picker,
		source:             msg.Source,
	}
}

// BuildTagList scans tasks and builds a sorted list of task-used tags with
// counts. If excludeCompleted is true, completed tasks are skipped.
func BuildTagList(tasks []*models.Task, excludeCompleted bool) []TagInfo {
	tagCounts := make(map[string]int)
	addTaskTagCounts(tagCounts, tasks, excludeCompleted)

	return tagInfosFromCounts(tagCounts)
}

// BuildProjectTagList builds a sorted task-used tag list for a project.
func BuildProjectTagList(project *models.Project, excludeCompleted bool) []TagInfo {
	if project == nil {
		return []TagInfo{}
	}
	return BuildCatalogTagList([]*models.Project{project}, project.Tasks, excludeCompleted)
}

// BuildProjectsTagList builds a sorted task-used tag list across projects.
func BuildProjectsTagList(projects []*models.Project, excludeCompleted bool) []TagInfo {
	var tasks []*models.Task
	for _, project := range uniqueProjects(projects) {
		tasks = append(tasks, project.Tasks...)
	}
	return BuildCatalogTagList(projects, tasks, excludeCompleted)
}

// BuildCatalogTagList is a compatibility wrapper over task-derived tag counts.
func BuildCatalogTagList(projects []*models.Project, tasks []*models.Task, excludeCompleted bool) []TagInfo {
	tagCounts := make(map[string]int)
	addTaskTagCounts(tagCounts, tasks, excludeCompleted)

	return tagInfosFromCounts(tagCounts)
}

func addTaskTagCounts(tagCounts map[string]int, tasks []*models.Task, excludeCompleted bool) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if excludeCompleted && task.IsComplete() {
			continue
		}
		for _, tag := range models.DedupeTags(task.Tags) {
			tagCounts[tag]++
		}
	}
}

func tagInfosFromCounts(tagCounts map[string]int) []TagInfo {
	tags := make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		tags = append(tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}

func uniqueProjects(projects []*models.Project) []*models.Project {
	seenIDs := make(map[string]bool, len(projects))
	seenPtrs := make(map[*models.Project]bool, len(projects))
	result := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		if project.ID != "" {
			if seenIDs[project.ID] {
				continue
			}
			seenIDs[project.ID] = true
		} else {
			if seenPtrs[project] {
				continue
			}
			seenPtrs[project] = true
		}
		result = append(result, project)
	}
	return result
}

// SetSize sets the view dimensions.
func (m *TagFilterSelectorModel) SetSize(width, height int) {
	m.ListSurfaceAdapter.SetSize(width, height)
}

// Init initializes the model.
func (m *TagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	source := m.source
	cmd := filterSelectorUpdate(m.ListSurfaceAdapter, msg,
		func() tea.Msg { return TagFilterDoneMsg{Canceled: true, Source: source} },
		func(selected []string) tea.Msg {
			return TagFilterDoneMsg{SelectedTags: selected, Canceled: false, Source: source}
		},
	)
	return m, cmd
}

// View renders the tag filter selector.
func (m *TagFilterSelectorModel) View() tea.View {
	return tea.NewView(filterSelectorView(m.ListSurfaceAdapter, "tag"))
}

func (m *TagFilterSelectorModel) StatusBarLabel() string { return "Filter by Tags" }
