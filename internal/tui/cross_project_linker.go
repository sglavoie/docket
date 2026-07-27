package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// CrossProjectLinkerStep represents the current step in the linking process.
type CrossProjectLinkerStep int

const (
	StepSelectProject CrossProjectLinkerStep = iota
	StepSelectTask
	StepSelectLinkType
)

// CrossProjectLinkerModel handles cross-project task linking.
type CrossProjectLinkerModel struct {
	BaseSearchableView
	sourceTask    *models.Task
	sourceProject *models.Project
	coordinator   LinkCoordinator

	step CrossProjectLinkerStep

	// Project selection
	allProjects      []*models.Project
	filteredProjects []*models.Project
	projectCursor    int

	// Task selection (after project is selected)
	targetProject *models.Project
	allTasks      []*models.Task
	filteredTasks []*models.Task
	taskCursor    int

	// Link type selection
	targetTask     *models.Task
	linkTypeCursor int
}

// NewCrossProjectLinkerModel creates a new cross-project linker; link creation
// commits through the link coordinator.
func NewCrossProjectLinkerModel(sourceTask *models.Task, sourceProject *models.Project, runtime *config.RuntimeState, coordinator LinkCoordinator) *CrossProjectLinkerModel {
	// Get all active projects except the source
	var projects []*models.Project
	for _, p := range runtime.ActiveProjects() {
		if p.ID != sourceProject.ID {
			projects = append(projects, p)
		}
	}

	m := &CrossProjectLinkerModel{
		sourceTask:       sourceTask,
		sourceProject:    sourceProject,
		coordinator:      coordinator,
		step:             StepSelectProject,
		allProjects:      projects,
		filteredProjects: projects,
	}
	m.SetSize(80, 24)
	m.InitSearchInput("Search...", 50, 30)
	return m
}

func (m *CrossProjectLinkerModel) GetHelpItems() []HelpItem {
	return helpItemsFromKeyMap(m.buildKeyMap())
}

// SetSize sets the view dimensions.
func (m *CrossProjectLinkerModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *CrossProjectLinkerModel) Init() tea.Cmd {
	return nil
}

func (m *CrossProjectLinkerModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// HasTextInput returns true when search mode is active.
func (m *CrossProjectLinkerModel) HasTextInput() bool {
	return m.IsSearchMode()
}

// ShouldCaptureEsc returns true when search is active so esc clears search first.
func (m *CrossProjectLinkerModel) ShouldCaptureEsc() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

func (m *CrossProjectLinkerModel) StatusBarLabel() string { return "Cross-Project Link" }
