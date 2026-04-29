package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/metrics"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/planner"
	"github.com/sglavoie/tt/internal/storage"
)

// RoadmapModel is the TUI model for the roadmap / Gantt view.
type RoadmapModel struct {
	BaseView
	config *config.Config

	// Scope
	project   *models.Project // nil = global scope
	isGlobal  bool
	canToggle bool // true if opened from project tree (can switch scope)

	// Data (computed by planner package)
	rows         []TimelineRow
	axis         TimelineAxis
	scheduled    []planner.ScheduledTask
	criticalPath *planner.CriticalPathResult
	velocity     float64

	// UI state
	activeTab        RoadmapTab
	cursor           int
	scrollOff        int
	hScrollOff       int
	zoom             ZoomLevel
	collapsed        map[string]bool
	labelWidth       int  // left pane width (~30 chars)
	showCriticalPath bool // when true, dims non-critical rows
	hasDeps          bool // true if any task has DependsOn edges
}

// NewRoadmapModel creates a global roadmap view.
func NewRoadmapModel(cfg *config.Config) *RoadmapModel {
	m := &RoadmapModel{
		config:     cfg,
		isGlobal:   true,
		canToggle:  false,
		zoom:       ZoomMonth,
		collapsed:  make(map[string]bool),
		labelWidth: 30,
	}
	m.computeData()
	return m
}

// NewRoadmapModelForProject creates a project-scoped roadmap view.
func NewRoadmapModelForProject(cfg *config.Config, project *models.Project) *RoadmapModel {
	m := &RoadmapModel{
		config:     cfg,
		project:    project,
		isGlobal:   false,
		canToggle:  true,
		zoom:       ZoomMonth,
		collapsed:  make(map[string]bool),
		labelWidth: 30,
	}
	m.computeData()
	return m
}

func (m *RoadmapModel) computeData() {
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

	// Compute effort-weighted velocity from throughput data.
	agg := metrics.ComputeAllAt(projects, metrics.TimeRange30d, time.Now(), allArchived)
	var allTasks []*models.Task
	for _, p := range projects {
		allTasks = append(allTasks, p.Tasks...)
	}
	if agg != nil {
		m.velocity = planner.WeightedVelocity(agg.Throughput, allTasks)
	}

	// Compute critical path and forward schedule across all incomplete tasks.
	allIncomplete := planner.CollectAllIncompleteTasks(projects)
	m.hasDeps = false
	for _, t := range allIncomplete {
		if len(t.DependsOn) > 0 {
			m.hasDeps = true
			break
		}
	}
	m.criticalPath = planner.CriticalPath(allIncomplete)
	m.scheduled = planner.SerialSchedule(allIncomplete, time.Now(), m.velocity)

	// Build axis and rows.
	m.buildAxis()
	m.buildRows(projects)
}

func (m *RoadmapModel) buildAxis() {
	var unitDays float64
	var totalCols int
	switch m.zoom {
	case ZoomWeek:
		unitDays = 1
		totalCols = 60 // ~2 months
	case ZoomMonth:
		unitDays = 7
		totalCols = 26 // ~6 months
	default: // ZoomQuarter
		unitDays = 30
		totalCols = 13 // ~1 year
	}

	now := time.Now()
	start := now.AddDate(0, 0, -7) // one week before today
	end := start.Add(time.Duration(float64(totalCols)*unitDays*24) * time.Hour)

	labels := make([]string, totalCols)
	for i := range labels {
		t := start.Add(time.Duration(float64(i)*unitDays*24) * time.Hour)
		switch m.zoom {
		case ZoomWeek:
			labels[i] = t.Format("01/02")
		case ZoomMonth:
			labels[i] = t.Format("Jan02")
		default:
			labels[i] = t.Format("Jan 06")
		}
	}

	todayCol := int(now.Sub(start).Hours() / 24 / unitDays)

	m.axis = TimelineAxis{
		Start:     start,
		End:       end,
		UnitDays:  unitDays,
		TotalCols: totalCols,
		Labels:    labels,
		TodayCol:  todayCol,
	}
}

func (m *RoadmapModel) buildRows(projects []*models.Project) {
	m.rows = nil
	schedMap := make(map[string]planner.ScheduledTask, len(m.scheduled))
	for _, s := range m.scheduled {
		schedMap[s.Task.ID] = s
	}

	for _, p := range projects {
		for _, t := range p.Tasks {
			if t.IsComplete() {
				continue
			}
			m.rows = append(m.rows, m.buildRow(t, p, 0, schedMap))
			if !m.collapsed[t.ID] {
				m.appendChildRows(t, p, 1, schedMap)
			}
		}
	}
}

func (m *RoadmapModel) appendChildRows(parent *models.Task, p *models.Project, indent int, schedMap map[string]planner.ScheduledTask) {
	for _, child := range parent.Children {
		if child.IsComplete() {
			continue
		}
		m.rows = append(m.rows, m.buildRow(child, p, indent, schedMap))
		if !m.collapsed[child.ID] {
			m.appendChildRows(child, p, indent+1, schedMap)
		}
	}
}

func (m *RoadmapModel) buildRow(t *models.Task, p *models.Project, indent int, schedMap map[string]planner.ScheduledTask) TimelineRow {
	row := TimelineRow{
		Task:      t,
		Project:   p,
		Indent:    indent,
		Collapsed: m.collapsed[t.ID],
	}

	if m.criticalPath != nil {
		row.IsCritical = m.criticalPath.OnPath[t.ID]
		row.Slack = m.criticalPath.Slack[t.ID]
	}

	if s, ok := schedMap[t.ID]; ok {
		row.ScheduledStart = s.EarliestStart
		row.ScheduledEnd = s.EarliestEnd
		row.BarStart = m.dateToCol(s.EarliestStart)
		row.BarEnd = m.dateToCol(s.EarliestEnd)
	}

	// Progress: fraction of direct children complete.
	total := len(t.Children)
	if total > 0 {
		done := 0
		for _, c := range t.Children {
			if c.IsComplete() {
				done++
			}
		}
		row.Progress = float64(done) / float64(total)
	}

	// Forecast completion.
	if m.velocity > 0 && len(t.Children) > 0 {
		fr := planner.ForecastCompletion(t, m.velocity, time.Now())
		row.ForecastDate = fr.ForecastDate
		row.Confidence = fr.Confidence
	}

	// IsBlocked: has any dependency listed (proxy for unresolved blockers).
	row.IsBlocked = len(t.DependsOn) > 0

	return row
}

func (m *RoadmapModel) dateToCol(t time.Time) int {
	return int(t.Sub(m.axis.Start).Hours() / 24 / m.axis.UnitDays)
}

// Init implements tea.Model.
func (m *RoadmapModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *RoadmapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseRoadmapMsg{} }

		case "h", "[":
			tab := int(m.activeTab)
			CycleTab(&tab, len(roadmapTabNames), -1)
			m.activeTab = RoadmapTab(tab)
			m.cursor = 0

		case "l", "]":
			tab := int(m.activeTab)
			CycleTab(&tab, len(roadmapTabNames), 1)
			m.activeTab = RoadmapTab(tab)
			m.cursor = 0

		case "H":
			if m.hScrollOff > 0 {
				m.hScrollOff--
			}

		case "L":
			chartWidth := max(m.width-m.labelWidth-lipgloss.Width(ganttSepRaw), 10)
			totalCols := max(m.axis.TotalCols, 1)
			colWidth := max(6, chartWidth/totalCols)
			visibleCols := chartWidth / colWidth
			maxOff := max(0, totalCols-visibleCols)
			if m.hScrollOff < maxOff {
				m.hScrollOff++
			}

		case "j", "down":
			if count := m.activeRowCount(); m.cursor < count-1 {
				m.cursor++
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "g":
			m.cursor = 0

		case "G":
			if count := m.activeRowCount(); count > 0 {
				m.cursor = count - 1
			}

		case "ctrl+d":
			half := m.visibleRows() / 2
			m.cursor = min(m.cursor+half, m.activeRowCount()-1)

		case "ctrl+u":
			half := m.visibleRows() / 2
			m.cursor = max(m.cursor-half, 0)

		case "z":
			m.zoom = ZoomLevel((int(m.zoom) + 1) % 3)
			m.computeData()

		case "space":
			if m.activeTab == RoadmapTabTimeline && m.cursor < len(m.rows) {
				row := m.rows[m.cursor]
				if row.Task != nil {
					m.collapsed[row.Task.ID] = !m.collapsed[row.Task.ID]
					m.computeData()
				}
			}

		case "enter":
			if task, project := m.selectedRow(); task != nil {
				return m, func() tea.Msg {
					return RoadmapSelectMsg{Task: task, Project: project}
				}
			}

		case "o":
			if task, project := m.selectedRow(); task != nil {
				return m, func() tea.Msg {
					return ShowTaskDetailMsg{Task: task, Project: project}
				}
			}

		case "s":
			if m.canToggle {
				m.isGlobal = !m.isGlobal
				m.computeData()
			}

		case "t":
			m.zoom = ZoomLevel((int(m.zoom) + 1) % 3)
			m.computeData()

		case "T":
			m.zoom = ZoomLevel((int(m.zoom) + 2) % 3)
			m.computeData()

		case "c":
			if !m.hasDeps {
				break
			}
			m.showCriticalPath = !m.showCriticalPath

		case "r":
			m.computeData()

		case "?":
			return m, helpCmd(ViewRoadmap, "Roadmap", GetKeybindingRegistry().GetForView(ViewRoadmap))
		}
	}

	return m, nil
}

// visibleRows returns the number of content rows visible in the current tab.
func (m *RoadmapModel) visibleRows() int {
	contentHeight := max(fullscreenContentHeight(m.height, 2), 4)
	overhead := 2 // axis header + divider (timeline); similar for other tabs
	return max(contentHeight-overhead, 1)
}

// activeRowCount returns the number of items for the current tab.
func (m *RoadmapModel) activeRowCount() int {
	switch m.activeTab {
	case RoadmapTabCriticalPath:
		if m.criticalPath != nil {
			return len(m.criticalPath.Path)
		}
		return 0
	case RoadmapTabForecasts:
		return len(m.forecastRows())
	default:
		return len(m.rows)
	}
}

// forecastRows returns the subset of rows that have children (epics/stories).
func (m *RoadmapModel) forecastRows() []TimelineRow {
	var rows []TimelineRow
	for _, row := range m.rows {
		if row.Task != nil && len(row.Task.Children) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

// selectedRow returns the task and project for the current cursor in the active tab.
func (m *RoadmapModel) selectedRow() (*models.Task, *models.Project) {
	switch m.activeTab {
	case RoadmapTabCriticalPath:
		if m.criticalPath != nil && m.cursor < len(m.criticalPath.Path) {
			task := m.criticalPath.Path[m.cursor]
			for _, r := range m.rows {
				if r.Task != nil && r.Task.ID == task.ID {
					return task, r.Project
				}
			}
			return task, nil
		}
	case RoadmapTabForecasts:
		fRows := m.forecastRows()
		if m.cursor < len(fRows) {
			return fRows[m.cursor].Task, fRows[m.cursor].Project
		}
	default:
		if m.cursor < len(m.rows) {
			return m.rows[m.cursor].Task, m.rows[m.cursor].Project
		}
	}
	return nil, nil
}

// HandledKeys implements KeySyncView.
func (m *RoadmapModel) HandledKeys() []string {
	return []string{
		"h", "[", "l", "]",
		"H", "L",
		"j", "down", "k", "up",
		"g", "G", "ctrl+d", "ctrl+u",
		"z", "space", "enter", "o",
		"s", "t", "T", "c", "r",
		"?",
	}
}

func (m *RoadmapModel) StatusBarLabel() string { return "Roadmap" }

// IsFullHeight returns true because the roadmap renders its own footer.
func (m *RoadmapModel) IsFullHeight() bool { return true }
