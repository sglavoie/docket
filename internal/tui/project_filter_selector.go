package tui

import (
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// ProjectFilterInfo holds project name, ID, and task count for display.
type ProjectFilterInfo struct {
	Name      string
	ID        string
	TaskCount int
}

func (p ProjectFilterInfo) FilterText() string  { return p.Name }
func (p ProjectFilterInfo) DisplayText() string { return p.Name }
func (p ProjectFilterInfo) CountValue() int     { return p.TaskCount }
func (p ProjectFilterInfo) ItemID() string      { return p.ID }

// ShowProjectFilterSelectorMsg is sent to open the project filter selector.
type ShowProjectFilterSelectorMsg struct {
	Projects []*models.Project
	Selected map[string]bool
}

// ProjectFilterSelectorDoneMsg is sent when the project filter selector is done.
type ProjectFilterSelectorDoneMsg struct {
	SelectedProjectIDs []string
	Canceled           bool
}

// ProjectFilterSelectorModel allows selecting multiple projects to filter by.
type ProjectFilterSelectorModel struct {
	*BasePicker[ProjectFilterInfo]
}

// NewProjectFilterSelectorModel creates a new project filter selector.
func NewProjectFilterSelectorModel(projects []*models.Project, selected map[string]bool) *ProjectFilterSelectorModel {
	items := buildProjectFilterList(projects)

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Filter by Projects",
		EmptyMessage:   "No projects loaded.",
		NoMatchMessage: "No matching projects.",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for id := range selected {
		picker.PreSelectByID(id)
	}

	return &ProjectFilterSelectorModel{
		BasePicker: picker,
	}
}

// buildProjectFilterList builds a sorted list of projects with task counts.
func buildProjectFilterList(projects []*models.Project) []ProjectFilterInfo {
	items := make([]ProjectFilterInfo, 0, len(projects))
	for _, p := range projects {
		items = append(items, ProjectFilterInfo{
			Name:      p.Name,
			ID:        p.ID,
			TaskCount: len(p.Tasks),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

// Init initializes the model.
func (m *ProjectFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *ProjectFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return ProjectFilterSelectorDoneMsg{Canceled: true} },
		func(selected []string) tea.Msg {
			return ProjectFilterSelectorDoneMsg{SelectedProjectIDs: selected, Canceled: false}
		},
	)
	return m, cmd
}

// View renders the project filter selector.
func (m *ProjectFilterSelectorModel) View() tea.View {
	return tea.NewView(filterSelectorView(m.BasePicker, "project"))
}

func (m *ProjectFilterSelectorModel) StatusBarLabel() string { return "Filter by Projects" }
