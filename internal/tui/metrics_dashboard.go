package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/metrics"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// MetricsTab represents which metric tab is active.
type MetricsTab int

const (
	MetricsTabSummary MetricsTab = iota
	MetricsTabCompletion
	MetricsTabCycleTime
	MetricsTabThroughput
	MetricsTabBlocked
)

var metricsTabNames = []string{"Summary", "Completion", "Cycle Time", "Throughput", "Blocked"}

// ShowMetricsDashboardMsg opens the metrics dashboard view.
type ShowMetricsDashboardMsg struct {
	Project *models.Project // nil = global (all projects)
}

// CloseMetricsDashboardMsg closes the metrics dashboard view.
type CloseMetricsDashboardMsg struct{}

// MetricsDashboardSelectMsg navigates to a task from the metrics dashboard.
type MetricsDashboardSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// MetricsDashboardModel is the TUI model for the metrics dashboard.
type MetricsDashboardModel struct {
	BaseView
	config *config.Config

	// Scope
	project   *models.Project // nil = global scope
	isGlobal  bool
	canToggle bool // true if opened from project tree (can switch scope)

	// Data
	metrics   *metrics.AggregateMetrics
	timeRange metrics.TimeRange

	// UI state
	activeTab     MetricsTab
	blockedCursor int // cursor for scrollable blocked task list
}

// NewMetricsDashboardModel creates a global metrics dashboard.
func NewMetricsDashboardModel(cfg *config.Config) *MetricsDashboardModel {
	m := &MetricsDashboardModel{
		config:    cfg,
		isGlobal:  true,
		canToggle: false,
		timeRange: metrics.TimeRange30d,
	}
	m.computeMetrics()
	return m
}

// NewMetricsDashboardModelForProject creates a project-scoped metrics dashboard.
func NewMetricsDashboardModelForProject(cfg *config.Config, project *models.Project) *MetricsDashboardModel {
	m := &MetricsDashboardModel{
		config:    cfg,
		project:   project,
		isGlobal:  false,
		canToggle: true,
		timeRange: metrics.TimeRange30d,
	}
	m.computeMetrics()
	return m
}

func (m *MetricsDashboardModel) computeMetrics() {
	var projects []*models.Project
	var allArchived []*models.Task

	if m.isGlobal {
		for _, p := range m.config.GetActiveProjects() {
			if err := storage.LoadProject(p); err != nil {
				logging.Warn("failed to load project %s: %v", p.Name, err)
			}
			projects = append(projects, p)
			allArchived = append(allArchived, storage.LoadArchivedTasks(p)...)
		}
	} else if m.project != nil {
		if err := storage.LoadProject(m.project); err != nil {
			logging.Warn("failed to load project %s: %v", m.project.Name, err)
		}
		projects = []*models.Project{m.project}
		allArchived = append(allArchived, storage.LoadArchivedTasks(m.project)...)
	}

	m.metrics = metrics.ComputeAllAt(projects, m.timeRange, time.Now(), allArchived)
}

// Init implements tea.Model.
func (m *MetricsDashboardModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *MetricsDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return CloseMetricsDashboardMsg{} }

		case "[", "h":
			tab := int(m.activeTab)
			CycleTab(&tab, len(metricsTabNames), -1)
			m.activeTab = MetricsTab(tab)
			m.blockedCursor = 0
		case "]", "l":
			tab := int(m.activeTab)
			CycleTab(&tab, len(metricsTabNames), 1)
			m.activeTab = MetricsTab(tab)
			m.blockedCursor = 0

		case "t":
			m.timeRange = m.timeRange.Next()
			m.computeMetrics()
		case "T":
			m.timeRange = m.timeRange.Prev()
			m.computeMetrics()

		case "g":
			if m.canToggle {
				m.isGlobal = !m.isGlobal
				m.computeMetrics()
			}

		case "r":
			m.computeMetrics()

		case "j", "down":
			if m.activeTab == MetricsTabBlocked && m.metrics != nil {
				maxIdx := len(m.metrics.Blocked.Entries) - 1
				if m.blockedCursor < maxIdx {
					m.blockedCursor++
				}
			}
		case "k", "up":
			if m.activeTab == MetricsTabBlocked {
				if m.blockedCursor > 0 {
					m.blockedCursor--
				}
			}

		case "enter":
			if m.activeTab == MetricsTabBlocked && m.metrics != nil {
				entries := m.metrics.Blocked.Entries
				if m.blockedCursor < len(entries) {
					e := entries[m.blockedCursor]
					return m, func() tea.Msg {
						return MetricsDashboardSelectMsg{Task: e.Task, Project: e.Project}
					}
				}
			}

		case "?":
			return m, helpCmd(ViewMetricsDashboard, "Metrics Dashboard", GetKeybindingRegistry().GetForView(ViewMetricsDashboard))
		}
	}
	return m, nil
}

func (m *MetricsDashboardModel) StatusBarLabel() string { return "Metrics" }
