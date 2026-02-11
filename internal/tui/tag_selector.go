package tui

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// TagSelectorModel allows multi-selecting tags and adding new ones.
type TagSelectorModel struct {
	*BasePicker[TagInfo]
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
	Tasks   []*models.Task
	Project *models.Project
}

// BulkTagSelectorDoneMsg is sent when the bulk tag selector is done.
type BulkTagSelectorDoneMsg struct {
	Tasks        []*models.Task
	Project      *models.Project
	SelectedTags []string
	Canceled     bool
}

// BulkTagSelectorModel allows multi-selecting tags for multiple tasks.
type BulkTagSelectorModel struct {
	*BasePicker[TagInfo]
	TagAdder
	tasks        []*models.Task
	project      *models.Project
	originalTags map[string][]string // taskID -> original tags
}

// NewBulkTagSelectorModel creates a new bulk tag selector.
func NewBulkTagSelectorModel(tasks []*models.Task, project *models.Project) *BulkTagSelectorModel {
	tags := BuildTagList(project.Tasks, false)
	commonTags := findCommonTags(tasks)

	title := fmt.Sprintf("Tags for %d tasks", len(tasks))
	picker := NewBasePicker(tags, PickerConfig{
		Title:          title,
		EmptyMessage:   "No tags in project (press 'a' to add)",
		NoMatchMessage: "No matching tags (press 'a' to add)",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for _, tag := range commonTags {
		picker.PreSelectByID(tag)
	}
	if len(commonTags) > 0 {
		picker.SetCursorToID(commonTags[0])
	}

	originalTags := make(map[string][]string)
	for _, task := range tasks {
		originalTags[task.ID] = append([]string{}, task.Tags...)
	}

	return &BulkTagSelectorModel{
		BasePicker:   picker,
		TagAdder:     NewTagAdder(),
		tasks:        tasks,
		project:      project,
		originalTags: originalTags,
	}
}

// findCommonTags returns tags that are present on all tasks.
func findCommonTags(tasks []*models.Task) []string {
	if len(tasks) == 0 {
		return nil
	}

	tagCount := make(map[string]int)
	for _, task := range tasks {
		for _, tag := range task.Tags {
			tagCount[tag]++
		}
	}

	var common []string
	for tag, count := range tagCount {
		if count == len(tasks) {
			common = append(common, tag)
		}
	}
	sort.Strings(common)
	return common
}

// SetSize sets the view dimensions.
func (m *BulkTagSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
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

	case tea.KeyMsg:
		if m.AddTagMode {
			handled, cmd := m.HandleAddTagKey(msg, m.BasePicker)
			if handled {
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return BulkTagSelectorDoneMsg{
					Tasks:    m.tasks,
					Project:  m.project,
					Canceled: true,
				}
			}

		case "enter":
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return BulkTagSelectorDoneMsg{
					Tasks:        m.tasks,
					Project:      m.project,
					SelectedTags: selectedTags,
					Canceled:     false,
				}
			}

		case "a":
			return m, m.EnterAddMode()

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
func (m *BulkTagSelectorModel) View() string {
	return RenderTagSelectorView(m.BasePicker, &m.TagAdder, m.Config.Title, m.Width(), m.Height())
}

// NewTagSelectorModel creates a new tag selector.
func NewTagSelectorModel(task *models.Task, project *models.Project, currentTags []string) *TagSelectorModel {
	tags := BuildTagList(project.Tasks, false)

	title := "Tags for New Task"
	if task != nil {
		title = fmt.Sprintf("Tags for #%d: %s", task.ShortID, task.Title)
	}

	picker := NewBasePicker(tags, PickerConfig{
		Title:          title,
		EmptyMessage:   "No tags in project (press 'a' to add)",
		NoMatchMessage: "No matching tags (press 'a' to add)",
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
		BasePicker:   picker,
		TagAdder:     NewTagAdder(),
		task:         task,
		project:      project,
		originalTags: append([]string{}, currentTags...),
	}
}

// SetSize sets the view dimensions.
func (m *TagSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
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

	case tea.KeyMsg:
		if m.AddTagMode {
			handled, cmd := m.HandleAddTagKey(msg, m.BasePicker)
			if handled {
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "esc":
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
func (m *TagSelectorModel) View() string {
	return RenderTagSelectorView(m.BasePicker, &m.TagAdder, m.Config.Title, m.Width(), m.Height())
}
