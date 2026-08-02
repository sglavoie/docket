package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TagSelectorModel allows multi-selecting tags and adding new ones.
type TagSelectorModel struct {
	*ListSurfaceAdapter[TagInfo]
	TagAdder
	task         *models.Task
	project      *models.Project
	originalTags []string
}

// ShowTagSelectorMsg is sent to open the tag selector.
type ShowTagSelectorMsg struct {
	Task        *models.Task
	Project     *models.Project
	CurrentTags []string
}

// TagSelectorDoneMsg is sent when the tag selector is done.
type TagSelectorDoneMsg struct {
	Task         *models.Task
	Project      *models.Project
	SelectedTags []string
	Canceled     bool
}

// ShowBulkTagSelectorMsg is sent to open the tag selector for multiple tasks.
type ShowBulkTagSelectorMsg struct {
	TaskRefs []TaskProjectRef
}

// BulkTagSelectorDoneMsg is sent when the bulk tag selector is done.
type BulkTagSelectorDoneMsg struct {
	TaskRefs     []TaskProjectRef
	SelectedTags []string
	Canceled     bool
}

// BulkTagSelectorModel allows multi-selecting tags for multiple tasks.
type BulkTagSelectorModel struct {
	*ListSurfaceAdapter[TagInfo]
	TagAdder
	taskRefs     []TaskProjectRef
	originalTags map[TaskIdentity][]string
}

// NewBulkTagSelectorModel creates a new bulk tag selector.
func NewBulkTagSelectorModel(taskRefs []TaskProjectRef) *BulkTagSelectorModel {
	taskRefs = validTaskProjectRefs(taskRefs)
	tags := BuildProjectsTagList(affectedProjectsFromTaskRefs(taskRefs), false)
	commonTags := findCommonTagsFromRefs(taskRefs)

	title := fmt.Sprintf("Tags for %d tasks", len(taskRefs))
	picker := NewListSurfaceAdapter(tags, PickerConfig{
		Title:          title,
		EmptyMessage:   "No tags in project (press 'a' to add).",
		NoMatchMessage: "No matching tags (press 'a' to add).",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for _, tag := range commonTags {
		picker.PreSelectByID(tag)
	}
	if len(commonTags) > 0 {
		picker.SetCursorToID(commonTags[0])
	}

	originalTags := make(map[TaskIdentity][]string)
	for _, ref := range taskRefs {
		originalTags[ref.Identity()] = append([]string{}, ref.Task.Tags...)
	}

	return &BulkTagSelectorModel{
		ListSurfaceAdapter: picker,
		TagAdder:           NewTagAdder(),
		taskRefs:           taskRefs,
		originalTags:       originalTags,
	}
}

// findCommonTagsFromRefs returns tags that are present on all task refs.
func findCommonTagsFromRefs(taskRefs []TaskProjectRef) []string {
	if len(taskRefs) == 0 {
		return nil
	}

	tagCount := make(map[string]int)
	for _, ref := range taskRefs {
		if ref.Task == nil {
			continue
		}
		for _, tag := range models.DedupeTags(ref.Task.Tags) {
			tagCount[tag]++
		}
	}

	var common []string
	for tag, count := range tagCount {
		if count == len(taskRefs) {
			common = append(common, tag)
		}
	}
	sort.Strings(common)
	return common
}

// SetSize sets the view dimensions.
func (m *BulkTagSelectorModel) SetSize(width, height int) {
	m.ListSurfaceAdapter.SetSize(width, height)
}

// Init initializes the model.
func (m *BulkTagSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *BulkTagSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if m.AddTagMode {
			handled, cmd := m.HandleAddTagKey(msg, m.ListSurfaceAdapter)
			if handled {
				return m, cmd
			}
		}

		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BulkTagSelectorDoneMsg{
					TaskRefs: m.taskRefs,
					Canceled: true,
				}
			}

		case "enter":
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return BulkTagSelectorDoneMsg{
					TaskRefs:     m.taskRefs,
					SelectedTags: selectedTags,
					Canceled:     false,
				}
			}

		case "a":
			return m, m.EnterAddMode()

		case "space":
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
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

// View renders the bulk tag selector.
func (m *BulkTagSelectorModel) View() tea.View {
	return tea.NewView(RenderTagSelectorView(m.ListSurfaceAdapter, &m.TagAdder, m.Config.Title, m.Width(), m.Height()))
}

// NewTagSelectorModel creates a new tag selector.
func NewTagSelectorModel(task *models.Task, project *models.Project, currentTags []string) *TagSelectorModel {
	tags := BuildProjectTagList(project, false)

	title := "Tags for New Task"
	if task != nil {
		title = fmt.Sprintf("Tags for #%d: %s", task.ShortID, task.Title)
	}

	picker := NewListSurfaceAdapter(tags, PickerConfig{
		Title:          title,
		EmptyMessage:   "No tags in project (press 'a' to add).",
		NoMatchMessage: "No matching tags (press 'a' to add).",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for _, tag := range currentTags {
		picker.PreSelectByID(tag)
	}
	if len(currentTags) > 0 {
		picker.SetCursorToID(currentTags[0])
	}

	return &TagSelectorModel{
		ListSurfaceAdapter: picker,
		TagAdder:           NewTagAdder(),
		task:               task,
		project:            project,
		originalTags:       append([]string{}, currentTags...),
	}
}

// SetSize sets the view dimensions.
func (m *TagSelectorModel) SetSize(width, height int) {
	m.ListSurfaceAdapter.SetSize(width, height)
}

// Init initializes the model.
func (m *TagSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TagSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if m.AddTagMode {
			handled, cmd := m.HandleAddTagKey(msg, m.ListSurfaceAdapter)
			if handled {
				return m, cmd
			}
		}

		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return TagSelectorDoneMsg{
					Task:     m.task,
					Project:  m.project,
					Canceled: true,
				}
			}

		case "enter":
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return TagSelectorDoneMsg{
					Task:         m.task,
					Project:      m.project,
					SelectedTags: selectedTags,
					Canceled:     false,
				}
			}

		case "a":
			return m, m.EnterAddMode()

		case "space":
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
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

// View renders the tag selector.
func (m *TagSelectorModel) View() tea.View {
	return tea.NewView(RenderTagSelectorView(m.ListSurfaceAdapter, &m.TagAdder, m.Config.Title, m.Width(), m.Height()))
}

func (m *TagSelectorModel) StatusBarLabel() string { return "Select Tags" }
