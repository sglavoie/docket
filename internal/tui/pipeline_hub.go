package tui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// ShowPipelineHubMsg opens the Pipeline Hub view.
type ShowPipelineHubMsg struct{}

// pipelineEntry is a single row within a pipeline column.
type pipelineEntry struct {
	key      string   // display shortcut key
	label    string   // short description
	count    int      // 0 = no count displayed
	viewType ViewType // which view to open (0 = no view)
}

// pipelineColumn represents one stage in the Pipeline Hub.
type pipelineColumn struct {
	name    string
	accent  color.Color
	entries []pipelineEntry
}

// PipelineHubModel displays a 5-column overview of all pipeline stages with live counts.
type PipelineHubModel struct {
	BaseView

	columns   []pipelineColumn
	activeCol int
	activeRow int
	config    *config.Config
	manager   *config.Manager
	counts    pipelineCounts
}

// NewPipelineHubModel creates a Pipeline Hub model populated with live counts.
func NewPipelineHubModel(cfg *config.Config, manager *config.Manager, queueIDs, dailyPlanIDs map[string]bool) *PipelineHubModel {
	counts := loadPipelineCounts(cfg, queueIDs, dailyPlanIDs)
	m := &PipelineHubModel{
		config:  cfg,
		manager: manager,
		counts:  counts,
	}
	m.buildColumns()
	return m
}

func (m *PipelineHubModel) buildColumns() {
	currentProject := m.config.GetCurrentProject()
	executeLabel := "Open project"
	if currentProject != nil {
		executeLabel = currentProject.Name
	}

	m.columns = []pipelineColumn{
		{
			name:   "Capture",
			accent: AccentCapture,
			entries: []pipelineEntry{
				{key: ";i", label: "Quick", viewType: ViewQuickCapture},
				{key: ";I", label: "Bulk", viewType: ViewBulkCapture},
			},
		},
		{
			name:   "Organize",
			accent: AccentOrganize,
			entries: []pipelineEntry{
				{key: "ctrl+i", label: "Inbox", count: m.counts.inboxCount, viewType: ViewGlobalInboxProcessor},
				{key: ";a", label: "All Tasks", viewType: ViewAllTasks},
				{key: ";o", label: "Outliner", viewType: ViewCrossProjectOutliner},
				{key: ";k", label: "Kanban", viewType: ViewCrossProjectKanban},
			},
		},
		{
			name:   "Plan",
			accent: AccentPlan,
			entries: []pipelineEntry{
				{key: ";q", label: "Backlog", count: m.counts.queueLen, viewType: ViewFocusView},
				{key: ";f", label: "Today", count: m.counts.dailyPlanLen, viewType: ViewFocusBoard},
				{key: ";D", label: "Due Dates", count: m.counts.overdueCount, viewType: ViewDueDateOverview},
				{key: ";d", label: "Daily Review", viewType: ViewDailyReview},
			},
		},
		{
			name:   "Execute",
			accent: AccentExecute,
			entries: []pipelineEntry{
				{key: "Enter", label: executeLabel},
				{key: "→", label: "Project Tree"},
				{key: "→", label: "Outliner"},
				{key: "→", label: "Kanban"},
			},
		},
		{
			name:   "Review",
			accent: AccentReview,
			entries: []pipelineEntry{
				{key: ";w", label: "Weekly", viewType: ViewWeeklyReview},
				{key: ";m", label: "Metrics", viewType: ViewMetricsDashboard},
				{key: ";l", label: "Activity", viewType: ViewRecentActivity},
			},
		},
	}
}

func (m *PipelineHubModel) Init() tea.Cmd { return nil }

func (m *PipelineHubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "h", "left":
			m.activeCol = (m.activeCol - 1 + len(m.columns)) % len(m.columns)
			m.clampRow()
		case "l", "right":
			m.activeCol = (m.activeCol + 1) % len(m.columns)
			m.clampRow()
		case "j", "down":
			col := m.columns[m.activeCol]
			if m.activeRow < len(col.entries)-1 {
				m.activeRow++
			}
		case "k", "up":
			if m.activeRow > 0 {
				m.activeRow--
			}
		case "enter":
			return m, m.openSelectedEntry()
		}
	}
	return m, nil
}

func (m *PipelineHubModel) clampRow() {
	if m.activeCol >= len(m.columns) {
		return
	}
	col := m.columns[m.activeCol]
	if m.activeRow >= len(col.entries) {
		m.activeRow = max(0, len(col.entries)-1)
	}
}

func (m *PipelineHubModel) openSelectedEntry() tea.Cmd {
	if m.activeCol >= len(m.columns) {
		return nil
	}
	col := m.columns[m.activeCol]
	if m.activeRow >= len(col.entries) {
		return nil
	}
	return pipelineEntryCmd(col.entries[m.activeRow].viewType)
}

// pipelineEntryCmd returns a tea.Cmd that emits the message to open the given view.
func pipelineEntryCmd(vt ViewType) tea.Cmd {
	switch vt {
	case ViewGlobalInboxProcessor:
		return func() tea.Msg { return ShowGlobalInboxProcessorMsg{} }
	case ViewAllTasks:
		return func() tea.Msg { return ShowAllTasksMsg{} }
	case ViewCrossProjectOutliner:
		return func() tea.Msg { return ShowGlobalOutlinerMsg{} }
	case ViewCrossProjectKanban:
		return func() tea.Msg { return ShowCrossProjectKanbanMsg{} }
	case ViewFocusView:
		return func() tea.Msg { return ShowFocusViewMsg{} }
	case ViewFocusBoard:
		return func() tea.Msg { return ShowFocusBoardMsg{} }
	case ViewDueDateOverview:
		return func() tea.Msg { return ShowDueDateOverviewMsg{} }
	case ViewDailyReview:
		return func() tea.Msg { return ShowDailyReviewMsg{} }
	case ViewWeeklyReview:
		return func() tea.Msg { return ShowWeeklyReviewMsg{} }
	case ViewMetricsDashboard:
		return func() tea.Msg { return ShowMetricsDashboardMsg{} }
	case ViewRecentActivity:
		return func() tea.Msg { return ShowRecentActivityMsg{} }
	}
	return nil
}

func (m *PipelineHubModel) StatusBarLabel() string { return "Pipeline" }

func (m *PipelineHubModel) IsFullHeight() bool { return true }

func (m *PipelineHubModel) HandledKeys() []string {
	return []string{"h", "l", "j", "k", "enter"}
}
