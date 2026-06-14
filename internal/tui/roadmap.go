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
	var allArchived []metrics.TaskSummary

	if m.isGlobal {
		for _, p := range m.config.GetActiveProjects() {
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

	// Compute effort-weighted velocity from throughput data. Use the user's
	// configured week-start so weekly buckets match dashboard preferences.
	weekStart := metrics.WeekStartFromWeekEnd(m.config.GetWeekEndDay())
	agg := metrics.ComputeAllAtWithWeekStartAndSummaries(projects, metrics.TimeRange30d, time.Now(), weekStart, allArchived)
	var allTasks []*models.Task
	for _, p := range projects {
		allTasks = append(allTasks, p.Tasks...)
	}
	if agg != nil {
		m.velocity = planner.WeightedVelocityWithSummaries(agg.Throughput, metrics.TaskSummariesFromTasks(allTasks), allArchived)
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
	m.clampRoadmapCursor()
	m.resetRoadmapScrollOffset()
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

// SetSize updates the viewport dimensions and resets the stored vertical
// window because the visible row budget may have changed.
func (m *RoadmapModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.resetRoadmapScrollOffset()
}

// Update implements tea.Model.
func (m *RoadmapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if model, cmd, handled := m.handleRoadmapNavigationKey(msg.String()); handled {
			return model, cmd
		}
		if model, cmd, handled := m.handleRoadmapActionKey(msg.String()); handled {
			return model, cmd
		}
	}

	return m, nil
}

func (m *RoadmapModel) handleRoadmapNavigationKey(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "esc":
		return m, func() tea.Msg { return CloseRoadmapMsg{} }, true
	case "tab", "shift+right":
		m.cycleRoadmapTab(1)
	case "shift+tab", "shift+left":
		m.cycleRoadmapTab(-1)
	case "H":
		m.scrollRoadmapHoriz(-1)
	case "L":
		m.scrollRoadmapHoriz(1)
	case "j", "down":
		m.moveRoadmapCursor(1)
	case "k", "up":
		m.moveRoadmapCursor(-1)
	case "g":
		m.cursor = 0
		m.resetRoadmapScrollOffset()
	case "G":
		m.jumpRoadmapCursorToBottom()
	case "ctrl+d":
		m.pageRoadmapCursor(1)
	case "ctrl+u":
		m.pageRoadmapCursor(-1)
	default:
		return m, nil, false
	}
	return m, nil, true
}

func (m *RoadmapModel) handleRoadmapActionKey(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "space":
		m.toggleTimelineRowCollapsed()
	case "enter":
		return m.selectedRoadmapMsg(func(task *models.Task, project *models.Project) tea.Msg {
			return RoadmapSelectMsg{Task: task, Project: project}
		})
	case "o":
		return m.selectedRoadmapMsg(func(task *models.Task, project *models.Project) tea.Msg {
			return ShowTaskDetailMsg{Task: task, Project: project}
		})
	case "t":
		m.cycleRoadmapZoom(1)
	case "T":
		m.cycleRoadmapZoom(2)
	case "c":
		if m.hasDeps {
			m.showCriticalPath = !m.showCriticalPath
		}
	case "r":
		m.computeData()
	case "?":
		return m, helpCmd(ViewRoadmap, "Roadmap", GetKeybindingRegistry().GetForView(ViewRoadmap)), true
	default:
		return m, nil, false
	}
	return m, nil, true
}

func (m *RoadmapModel) cycleRoadmapTab(step int) {
	tab := int(m.activeTab)
	CycleTab(&tab, len(roadmapTabNames), step)
	m.activeTab = RoadmapTab(tab)
	m.cursor = 0
	m.resetRoadmapScrollOffset()
}

func (m *RoadmapModel) scrollRoadmapHoriz(delta int) {
	if delta < 0 {
		if m.hScrollOff > 0 {
			m.hScrollOff--
		}
		return
	}
	if m.hScrollOff < m.maxRoadmapHScrollOffset() {
		m.hScrollOff++
	}
}

func (m *RoadmapModel) maxRoadmapHScrollOffset() int {
	chartWidth := max(m.width-m.labelWidth-lipgloss.Width(ganttSepRaw), 10)
	totalCols := max(m.axis.TotalCols, 1)
	colWidth := max(6, chartWidth/totalCols)
	visibleCols := chartWidth / colWidth
	return max(0, totalCols-visibleCols)
}

func (m *RoadmapModel) moveRoadmapCursor(delta int) {
	count := m.activeRowCount()
	if count == 0 {
		m.cursor = 0
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), count-1)
}

func (m *RoadmapModel) clampRoadmapCursor() {
	m.cursor = ClampCursor(m.cursor, m.activeRowCount())
}

func (m *RoadmapModel) resetRoadmapScrollOffset() {
	m.scrollOff = 0
}

func (m *RoadmapModel) jumpRoadmapCursorToBottom() {
	if count := m.activeRowCount(); count > 0 {
		m.cursor = count - 1
	}
}

func (m *RoadmapModel) pageRoadmapCursor(direction int) {
	half := m.visibleRows() / 2
	m.moveRoadmapCursor(direction * half)
}

func (m *RoadmapModel) toggleTimelineRowCollapsed() {
	if m.activeTab != RoadmapTabTimeline || m.cursor < 0 || m.cursor >= len(m.rows) {
		return
	}
	row := m.rows[m.cursor]
	if row.Task == nil {
		return
	}
	m.collapsed[row.Task.ID] = !m.collapsed[row.Task.ID]
	m.computeData()
}

func (m *RoadmapModel) selectedRoadmapMsg(msg func(task *models.Task, project *models.Project) tea.Msg) (tea.Model, tea.Cmd, bool) {
	task, project := m.selectedRow()
	if task == nil {
		return m, nil, true
	}
	return m, func() tea.Msg { return msg(task, project) }, true
}

func (m *RoadmapModel) cycleRoadmapZoom(step int) {
	m.zoom = ZoomLevel((int(m.zoom) + step) % 3)
	m.computeData()
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
		if m.criticalPath != nil && m.cursor >= 0 && m.cursor < len(m.criticalPath.Path) {
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
		if m.cursor >= 0 && m.cursor < len(fRows) {
			return fRows[m.cursor].Task, fRows[m.cursor].Project
		}
	default:
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			return m.rows[m.cursor].Task, m.rows[m.cursor].Project
		}
	}
	return nil, nil
}

// HandledKeys implements KeySyncView.
func (m *RoadmapModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *RoadmapModel) StatusBarLabel() string { return "Roadmap" }

// IsFullHeight returns true because the roadmap renders its own footer.
func (m *RoadmapModel) IsFullHeight() bool { return true }
