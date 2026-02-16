package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// OutlinerModel is the outliner view for navigating tasks in a hierarchical,
// zoomable tree. It implements the View interface as a standalone struct
// (not extending ProjectTreeModel) to allow independent evolution of
// navigation semantics, sort modes, and zoom-stack behavior.
type OutlinerModel struct {
	BaseView

	project       *models.Project
	config        *config.Config
	configManager *config.Manager

	zoomDepth   int
	multiSelect bool

	cursor int           // Index into rows (flat visible node list)
	rows   []outlinerRow // Flattened rows, rebuilt on each render
}

// NewOutlinerModel creates a new OutlinerModel for the given project.
func NewOutlinerModel(project *models.Project, cfg *config.Config, cfgManager *config.Manager) *OutlinerModel {
	return &OutlinerModel{
		project:       project,
		config:        cfg,
		configManager: cfgManager,
	}
}

// rebuildRows flattens the current task tree into the rows slice and clamps
// the cursor to stay within bounds.
func (m *OutlinerModel) rebuildRows() {
	m.rows = m.rows[:0]
	if m.project != nil {
		roots := m.project.GetRootTasks()
		flattenTaskTree(roots, nil, &m.rows)
	}
	m.clampCursor()
}

// clampCursor ensures the cursor stays within [0, len(rows)-1].
func (m *OutlinerModel) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if len(m.rows) > 0 && m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

// contentHeight returns the number of task rows visible on screen,
// accounting for the title header (project name + blank line = 2 lines).
func (m *OutlinerModel) contentHeight() int {
	h := m.height - 2 // title + blank line
	if h < 1 {
		return 1
	}
	return h
}

// Init implements tea.Model.
func (m *OutlinerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *OutlinerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

// SetSize implements the View interface.
func (m *OutlinerModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
}
