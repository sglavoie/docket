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
	task *models.Task
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
	SelectedTags []string
	Canceled     bool
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
		EmptyMessage:   "No tags in project · a add tag.",
		NoMatchMessage: "No matching tags · a add tag.",
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
			if m.IsSearchMode() {
				handled, cmd := m.HandleKeyMsg(msg)
				if handled {
					return m, cmd
				}
			}
			return m, func() tea.Msg {
				return TagSelectorDoneMsg{
					Task:     m.task,
					Canceled: true,
				}
			}

		case "enter":
			if m.IsSearchMode() {
				handled, cmd := m.HandleKeyMsg(msg)
				if handled {
					return m, cmd
				}
			}
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return TagSelectorDoneMsg{
					Task:         m.task,
					SelectedTags: selectedTags,
					Canceled:     false,
				}
			}

		case "a":
			if m.IsSearchMode() {
				_, cmd := m.HandleKeyMsg(msg)
				return m, cmd
			}
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
