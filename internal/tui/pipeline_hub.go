package tui

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ShowPipelineHubMsg opens the Pipeline Hub view.
type ShowPipelineHubMsg struct{}

// PipelineHubCaptureMsg is emitted when Enter is pressed on a capture entry
// in the pipeline hub. Quick Capture opens the shared Inbox-default flow;
// capture modes that require a destination still open the project picker.
type PipelineHubCaptureMsg struct{ Purpose PickerPurpose }

// pipelineEntry is a single row within a pipeline column.
type pipelineEntry struct {
	key        string   // display shortcut key
	label      string   // short description
	count      int      // 0 = no count displayed
	viewType   ViewType // which view to open (0 = no view)
	plannerTab int      // Planner tab to open when viewType == ViewFocusBoard

	// allTasksGroup selects the initial grouping when viewType == ViewAllTasks
	// and setAllTasksGroup is true (e.g. the ;D Due Dates tile).
	allTasksGroup    AllTasksGroupBy
	setAllTasksGroup bool
}

// pipelineColumn represents one stage in the Pipeline Hub.
type pipelineColumn struct {
	name    string
	accent  color.Color
	entries []pipelineEntry
}

// pipelineWorkColumnName identifies the Work column for the
// project-aware Enter handling that opens the current project rather than a
// global view. Keyed by name so column reordering does not silently break it.
const pipelineWorkColumnName = "Work"

// PipelineHubModel displays a six-stage workflow map (Capture, Triage, Plan,
// Work, Review, Support) with live counts and the current Next Action
// recommendation pinned above the grid.
type PipelineHubModel struct {
	BaseView

	columns        []pipelineColumn
	activeCol      int
	activeRow      int
	runtime        *config.RuntimeState
	counts         pipelineCounts
	currentProject *models.Project
	recommendation RecommendedAction
	openLoops      OpenLoops
}

// NewPipelineHubModel creates a Pipeline Hub model populated with live counts
// and the engine-derived Next Action recommendation.
func NewPipelineHubModel(runtime *config.RuntimeState, queueIDs, dailyPlanIDs map[string]bool, contextProject *models.Project) *PipelineHubModel {
	counts, projects := loadPipelineCounts(runtime, queueIDs, dailyPlanIDs)
	m := &PipelineHubModel{
		runtime:        runtime,
		counts:         counts,
		currentProject: contextProject,
	}
	m.recommendation = computePipelineRecommendation(runtime, projects)
	m.buildColumns()
	return m
}

// setOpenLoops satisfies the openLoopsAware interface. AppModel.View injects
// the latest cross-project counters before each render so the recommendation
// strip can surface inbox-only fallbacks and distinguish first-run state.
func (m *PipelineHubModel) setOpenLoops(o OpenLoops) {
	m.openLoops = o
}

// computePipelineRecommendation runs the shared deterministic recommendation
// engine against the already-loaded project slice plus the persisted daily
// plan. Returns the zero RecommendedAction when cfg is nil.
func computePipelineRecommendation(runtime *config.RuntimeState, projects []*models.Project) RecommendedAction {
	if runtime == nil {
		return RecommendedAction{}
	}
	return Recommend(RecommendationEngineInput{
		Projects:  projects,
		DailyPlan: runtime.Planning().GetDailyPlan(),
		Now:       time.Now(),
	})
}

func (m *PipelineHubModel) buildColumns() {
	workEntries := []pipelineEntry{
		{key: ";w", label: "Today", count: m.counts.dailyPlanLen, viewType: ViewFocusBoard},
	}
	if m.currentProject != nil {
		// The current-project entry (Enter → SelectProjectMsg) is the sole route
		// to this project's Outliner; a separate "Outliner" tile would duplicate
		// it since the retired ProjectTree left the Outliner as the only
		// per-project surface.
		workEntries = append(workEntries,
			pipelineEntry{key: "Enter", label: m.currentProject.Name, viewType: ViewOutliner},
			pipelineEntry{key: "→", label: "Kanban", viewType: ViewKanban},
		)
	} else {
		workEntries = append(workEntries,
			pipelineEntry{key: "·", label: "Open a project from Dashboard"},
		)
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
			name:   "Triage",
			accent: AccentTriage,
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
				{key: ";q", label: "Next", count: m.counts.queueLen, viewType: ViewFocusBoard, plannerTab: boardTabNext},
				{key: ";D", label: "Due Dates", count: m.counts.overdueCount, viewType: ViewAllTasks, allTasksGroup: ATGroupByDueDate, setAllTasksGroup: true},
				{key: ";d", label: "Daily Review", viewType: ViewDailyReview},
				{key: ";R", label: "Roadmap", viewType: ViewRoadmap},
			},
		},
		{
			name:    pipelineWorkColumnName,
			accent:  AccentWork,
			entries: workEntries,
		},
		{
			name:   "Review",
			accent: AccentReview,
			entries: []pipelineEntry{
				{key: ";r", label: "Weekly", viewType: ViewWeeklyReview},
				{key: ";m", label: "Metrics", viewType: ViewMetricsDashboard},
				{key: ";l", label: "Activity", viewType: ViewRecentActivity},
			},
		},
		{
			name:   "Support",
			accent: AccentSupport,
			entries: []pipelineEntry{
				{key: ";B", label: "Blockers", viewType: ViewGlobalBlockers},
				{key: ";N", label: "Notes", viewType: ViewGlobalNotes},
				{key: ";W", label: "Rules", viewType: ViewWorkflowRules},
				{key: ";n", label: "Alerts", viewType: ViewNotificationCenter},
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
	entry := col.entries[m.activeRow]
	if col.name == pipelineWorkColumnName && m.currentProject != nil {
		if cmd := m.workProjectEntryCmd(entry); cmd != nil {
			return cmd
		}
	}
	if entry.viewType == ViewFocusBoard {
		return func() tea.Msg { return ShowFocusBoardMsg{Tab: entry.plannerTab, SetTab: true} }
	}
	if entry.viewType == ViewAllTasks && entry.setAllTasksGroup {
		group := entry.allTasksGroup
		return func() tea.Msg { return ShowAllTasksMsg{Group: group, SetGroup: true} }
	}
	return pipelineEntryCmd(entry.viewType)
}

// workProjectEntryCmd returns a tea.Cmd for the project-specific entries that
// live inside the Work column (Open project, Outliner, Kanban).
// Returns nil when the entry is the shared Today shortcut so the caller falls
// back to the global pipelineEntryCmd dispatcher.
func (m *PipelineHubModel) workProjectEntryCmd(entry pipelineEntry) tea.Cmd {
	p := m.currentProject
	switch entry.viewType {
	case ViewOutliner:
		if entry.key == "Enter" {
			return func() tea.Msg { return SelectProjectMsg{Project: p} }
		}
		return func() tea.Msg { return ShowOutlinerMsg{Project: p} }
	case ViewKanban:
		return func() tea.Msg { return ShowKanbanMsg{Project: p} }
	default:
		return nil
	}
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
	case ViewFocusBoard:
		return func() tea.Msg { return ShowFocusBoardMsg{} }
	case ViewDailyReview:
		return func() tea.Msg { return ShowDailyReviewMsg{} }
	case ViewWeeklyReview:
		return func() tea.Msg { return ShowWeeklyReviewMsg{} }
	case ViewMetricsDashboard:
		return func() tea.Msg { return ShowMetricsDashboardMsg{} }
	case ViewRecentActivity:
		return func() tea.Msg { return ShowRecentActivityMsg{} }
	case ViewRoadmap:
		return func() tea.Msg { return ShowRoadmapMsg{} }
	case ViewQuickCapture:
		return func() tea.Msg { return PipelineHubCaptureMsg{Purpose: PickerPurposeQuickCapture} }
	case ViewBulkCapture:
		return func() tea.Msg { return PipelineHubCaptureMsg{Purpose: PickerPurposeBulkCapture} }
	case ViewGlobalBlockers:
		return func() tea.Msg { return ShowGlobalBlockersMsg{} }
	case ViewGlobalNotes:
		return func() tea.Msg { return ShowGlobalNotesMsg{} }
	case ViewWorkflowRules:
		return func() tea.Msg { return ShowWorkflowRulesMsg{} }
	case ViewNotificationCenter:
		return func() tea.Msg { return ShowNotificationCenterMsg{} }
	default:
	}
	return nil
}

func (m *PipelineHubModel) StatusBarLabel() string { return "Pipeline" }

func (m *PipelineHubModel) IsFullHeight() bool { return true }

func (m *PipelineHubModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
