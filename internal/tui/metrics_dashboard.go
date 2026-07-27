package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

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
type CloseMetricsDashboardMsg struct{ PopToAnchorMsg }

// MetricsDashboardSelectMsg navigates to a task from the metrics dashboard.
type MetricsDashboardSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// MetricsDashboardModel is the TUI model for the metrics dashboard.
type MetricsDashboardModel struct {
	BaseView
	configManager *config.Manager
	dataDir       string

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

// NewMetricsDashboardModel creates a global metrics dashboard. It sources its
// active projects and week-start setting from the domain-backed runtime seam
// rather than the config.Config facade.
func NewMetricsDashboardModel(manager *config.Manager) *MetricsDashboardModel {
	m := &MetricsDashboardModel{
		configManager: manager,
		dataDir:       metricsDataDir(manager),
		isGlobal:      true,
		canToggle:     false,
		timeRange:     metrics.TimeRange30d,
	}
	m.computeMetrics()
	return m
}

// NewMetricsDashboardModelForProject creates a project-scoped metrics dashboard.
func NewMetricsDashboardModelForProject(manager *config.Manager, project *models.Project) *MetricsDashboardModel {
	m := &MetricsDashboardModel{
		configManager: manager,
		dataDir:       metricsDataDir(manager),
		project:       project,
		isGlobal:      false,
		canToggle:     true,
		timeRange:     metrics.TimeRange30d,
	}
	m.computeMetrics()
	return m
}

// metricsDataDir derives the workspace data directory from a manager, or ""
// when no manager is bound (isolated render tests).
func metricsDataDir(manager *config.Manager) string {
	if manager == nil {
		return ""
	}
	return config.WorkspaceRootFromConfigPath(manager.GetConfigPath())
}

func (m *MetricsDashboardModel) computeMetrics() {
	var projects []*models.Project
	var allArchived []metrics.TaskSummary

	deps := m.runtimeDeps()

	if m.isGlobal {
		active, err := m.activeProjects(deps)
		if err != nil {
			logging.Warn("failed to load active projects for metrics: %v", err)
		}
		for _, p := range active {
			if err := storage.LoadProject(p); err != nil {
				logging.Warn("failed to load project %s: %v", p.Name, err)
			}
			projects = append(projects, p)
			summaries, err := storage.LoadArchivedTaskSummaries(p)
			if err != nil {
				logging.Warn("failed to load archive summaries for %s: %v", p.Name, err)
				continue
			}
			allArchived = append(allArchived, archiveSummariesForMetrics(summaries)...)
		}
	} else if m.project != nil {
		if err := storage.LoadProject(m.project); err != nil {
			logging.Warn("failed to load project %s: %v", m.project.Name, err)
		}
		projects = []*models.Project{m.project}
		summaries, err := storage.LoadArchivedTaskSummaries(m.project)
		if err != nil {
			logging.Warn("failed to load archive summaries for %s: %v", m.project.Name, err)
		} else {
			allArchived = append(allArchived, archiveSummariesForMetrics(summaries)...)
		}
	}

	weekStart := metrics.WeekStartFromWeekEnd(m.weekEndDay(deps))
	m.metrics = metrics.ComputeAllAtWithWeekStartAndSummaries(projects, m.timeRange, time.Now(), weekStart, allArchived)
}

// runtimeDeps returns the domain-backed runtime dependencies, or nil when no
// manager is bound (isolated render tests).
func (m *MetricsDashboardModel) runtimeDeps() *config.RuntimeDependencies {
	if m.configManager == nil {
		return nil
	}
	return m.configManager.RuntimeDependencies()
}

// activeProjects returns the workspace's active projects via the catalog domain,
// mirroring the config facade's GetActiveProjects for the global metrics scope.
func (m *MetricsDashboardModel) activeProjects(deps *config.RuntimeDependencies) ([]*models.Project, error) {
	if deps == nil {
		return nil, nil
	}
	return deps.ActiveProjects(m.dataDir)
}

// weekEndDay resolves the configured week-end day via the settings domain,
// defaulting to "sun" when settings are unavailable.
func (m *MetricsDashboardModel) weekEndDay(deps *config.RuntimeDependencies) string {
	if deps == nil || deps.Settings == nil {
		return "sun"
	}
	settings, err := deps.Settings.LoadSettings()
	if err != nil {
		logging.Warn("failed to load settings for metrics week-start: %v", err)
		return "sun"
	}
	return settings.GetWeekEndDay()
}

func archiveSummariesForMetrics(summaries []storage.ArchiveTaskSummary) []metrics.TaskSummary {
	metricSummaries := make([]metrics.TaskSummary, 0, len(summaries))
	for _, summary := range summaries {
		metricSummaries = append(metricSummaries, metrics.TaskSummary{
			ID:             summary.ID,
			WorkflowState:  summary.WorkflowState,
			CompletedAt:    summary.CompletedAt,
			FirstStartedAt: summary.FirstStartedAt,
			EstimatedDays:  summary.EstimatedDays,
			Effort:         summary.Effort,
			StoryPoints:    summary.StoryPoints,
		})
	}
	return metricSummaries
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

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseMetricsDashboardMsg{} }

		case "tab", "shift+right":
			tab := int(m.activeTab)
			CycleTab(&tab, len(metricsTabNames), 1)
			m.activeTab = MetricsTab(tab)
			m.blockedCursor = 0
		case "shift+tab", "shift+left":
			tab := int(m.activeTab)
			CycleTab(&tab, len(metricsTabNames), -1)
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
			return m, helpCmd(ViewMetricsDashboard, "Metrics Dashboard", helpItemsFromKeyMap(m.buildKeyMap()))
		}
	}
	return m, nil
}

func (m *MetricsDashboardModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *MetricsDashboardModel) getTaskRef() (*models.Task, *models.Project) {
	if m.activeTab != MetricsTabBlocked || m.metrics == nil {
		return nil, nil
	}
	entries := m.metrics.Blocked.Entries
	if m.blockedCursor < 0 || m.blockedCursor >= len(entries) {
		return nil, nil
	}
	entry := entries[m.blockedCursor]
	return entry.Task, entry.Project
}

func (m *MetricsDashboardModel) StatusBarLabel() string { return "Metrics" }

// IsFullHeight returns true because the metrics dashboard renders its own footer.
func (m *MetricsDashboardModel) IsFullHeight() bool { return true }
