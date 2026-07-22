package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

type homePane uint8

const (
	homePaneProjects homePane = iota
	homePaneAttention
)

type homeAttentionCategory string

type parkedResumeEntry struct {
	task    *models.Task
	project *models.Project
}

func (m *DashboardModel) resumeCardLimit() int {
	if m.runtime == nil {
		return config.DefaultResumeCardLimit
	}
	return m.runtime.Settings().GetResumeCardLimit()
}

const (
	homeAttentionRecommendation homeAttentionCategory = "Recommendation"
	homeAttentionAlert          homeAttentionCategory = "Alert"
	homeAttentionParked         homeAttentionCategory = "Parked"
	homeAttentionPipeline       homeAttentionCategory = "Pipeline"
)

type homeAttentionAction uint8

const (
	homeAttentionNoAction homeAttentionAction = iota
	homeAttentionOpenTask
	homeAttentionOpenProject
	homeAttentionOpenGlobalInbox
	homeAttentionOpenProjectInbox
	homeAttentionOpenPlannerToday
	homeAttentionOpenPlannerNext
	homeAttentionOpenDueDates
	homeAttentionOpenGlobalBlockers
	homeAttentionOpenProjectBlockers
	homeAttentionOpenDailyReview
	homeAttentionOpenWeeklyReview
	homeAttentionOpenNotifications
)

// homeAttentionRow is deliberately action-first: inert summaries are not part
// of Home. Task-backed rows can also be peeked through Shift+Enter.
type homeAttentionRow struct {
	Category homeAttentionCategory
	Label    string
	Detail   string
	Task     *models.Task
	Project  *models.Project
	Action   homeAttentionAction
}

func (r homeAttentionRow) searchableText() string {
	parts := []string{string(r.Category), r.Label, r.Detail}
	if r.Project != nil {
		parts = append(parts, r.Project.Name)
	}
	if r.Task != nil {
		parts = append(parts, r.Task.Title)
	}
	return strings.Join(parts, " ")
}

// SetAttentionAlerts supplies the app-level, dismissal-filtered alert snapshot
// to Home. Home keeps the snapshot in memory only.
func (m *DashboardModel) SetAttentionAlerts(snapshot *alerts.AlertSnapshot) {
	m.attentionAlerts = snapshot
	m.rebuildHomeAttention()
}

// FocusAttention selects the Attention pane for palette aliases and alerts.
func (m *DashboardModel) FocusAttention() {
	m.homePane = homePaneAttention
	m.attentionCursor = ClampCursor(m.attentionCursor, len(m.visibleAttentionRows()))
}

func (m *DashboardModel) activeHomePane() homePane {
	if m == nil {
		return homePaneProjects
	}
	return m.homePane
}

func (m *DashboardModel) visibleAttentionRows() []homeAttentionRow {
	if m == nil || m.SearchQuery() == "" {
		return m.attentionRows
	}
	query := strings.TrimSpace(m.SearchQuery())
	rows := make([]homeAttentionRow, 0, len(m.attentionRows))
	for _, row := range m.attentionRows {
		if fuzzy.MatchTerms(row.searchableText(), query) {
			rows = append(rows, row)
		}
	}
	return rows
}

func (m *DashboardModel) selectedAttentionRow() *homeAttentionRow {
	rows := m.visibleAttentionRows()
	if !CursorInBounds(m.attentionCursor, len(rows)) {
		return nil
	}
	row := rows[m.attentionCursor]
	return &row
}

// getTaskRef lets the app-level Enter/Shift+Enter grammar open or peek the
// concrete task represented by an Attention row.
func (m *DashboardModel) getTaskRef() (*models.Task, *models.Project) {
	if m == nil || m.homePane != homePaneAttention {
		return nil, nil
	}
	row := m.selectedAttentionRow()
	if row == nil || row.Task == nil || row.Project == nil {
		return nil, nil
	}
	return row.Task, row.Project
}

func (m *DashboardModel) rebuildHomeAttention() {
	if m == nil || m.runtime == nil {
		return
	}

	dailyPlanIDs := make(map[TaskIdentity]bool)
	for _, item := range m.runtime.Planning().GetDailyPlan() {
		identity := TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}
		if identity.Valid() {
			dailyPlanIDs[identity] = true
		}
	}
	projects := m.attentionProjects
	if projects == nil && !m.showArchived {
		// Test/headless models may install their in-memory project slice without
		// going through loadProjects.
		projects = m.projects
	}
	counts := homePipelineCounts(projects, len(dailyPlanIDs))
	m.attentionCounts = counts

	rows := make([]homeAttentionRow, 0, 16)
	if row, ok := m.homeRecommendationRow(projects); ok {
		rows = append(rows, row)
	}
	rows = append(rows, m.homeAlertRows(projects)...)
	rows = append(rows, homeParkedRows(projects, m.resumeCardLimit())...)
	rows = append(rows, homePipelineRows(counts)...)

	m.attentionRows = rows
	m.attentionCursor = ClampCursor(m.attentionCursor, len(m.visibleAttentionRows()))
	m.attentionViewScrollOffset = 0
}

func homePipelineCounts(projects []*models.Project, dailyPlanLen int) attentionCounts {
	counts := attentionCounts{dailyPlanLen: dailyPlanLen}
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || !task.IsRoot() {
				continue
			}
			counts.inboxCount += countInbox(task)
			counts.queueLen += countNext(task)
			counts.overdueCount += countOverdue(task)
			counts.inProgressCount += countInProgress(task)
			counts.completedTodayCount += countCompletedToday(task, startOfDay)
		}
	}
	return counts
}

func (m *DashboardModel) homeRecommendationRow(projects []*models.Project) (homeAttentionRow, bool) {
	recommendation := Recommend(RecommendationEngineInput{
		Projects:  projects,
		DailyPlan: m.runtime.Planning().GetDailyPlan(),
	})
	row := homeAttentionRow{Category: homeAttentionRecommendation}
	switch recommendation.Kind {
	case RecommendationKindWorkOnTask:
		if recommendation.Task == nil || recommendation.Project == nil {
			return homeAttentionRow{}, false
		}
		row.Label = "Work on " + recommendation.Task.Title
		row.Detail = recommendationDetail(recommendation)
		row.Task, row.Project = recommendation.Task, recommendation.Project
		row.Action = homeAttentionOpenTask
	case RecommendationKindReviewBlocked:
		if recommendation.Task == nil || recommendation.Project == nil {
			return homeAttentionRow{}, false
		}
		row.Label = "Review blocked · " + recommendation.Task.Title
		row.Detail = recommendation.Project.Name
		row.Task, row.Project = recommendation.Task, recommendation.Project
		row.Action = homeAttentionOpenTask
	case RecommendationKindTriageInbox:
		row.Label = "Triage the inbox"
		row.Detail = "Open the app-wide inbox"
		row.Action = homeAttentionOpenGlobalInbox
	case RecommendationKindPlanToday:
		row.Label = "Plan today"
		row.Detail = "Open Planner · Today"
		row.Action = homeAttentionOpenPlannerToday
	case RecommendationKindNone, RecommendationKindAllDone:
		return homeAttentionRow{}, false
	default:
		return homeAttentionRow{}, false
	}
	return row, true
}

func recommendationDetail(recommendation RecommendedAction) string {
	parts := make([]string, 0, len(recommendation.Reasons)+1)
	if recommendation.Project != nil {
		parts = append(parts, recommendation.Project.Name)
	}
	for _, reason := range recommendation.Reasons {
		parts = append(parts, reason.Label())
	}
	return strings.Join(parts, " · ")
}

func (m *DashboardModel) homeAlertRows(projects []*models.Project) []homeAttentionRow {
	if m.attentionAlerts == nil {
		return nil
	}
	projectsByID := make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		if project != nil {
			projectsByID[project.ID] = project
		}
	}

	var rows []homeAttentionRow
	for _, group := range m.attentionAlerts.Groups {
		for _, alert := range group.Alerts {
			row := homeAttentionRow{
				Category: homeAttentionAlert,
				Label:    alert.Title,
				Detail:   alert.Details,
				Action:   homeAttentionOpenNotifications,
			}
			project := projectsByID[alert.ProjectID]
			row.Project = project
			if project != nil && alert.TaskID != "" {
				if task := project.GetTaskByID(alert.TaskID); task != nil {
					row.Task = task
					row.Action = homeAttentionOpenTask
				}
			}
			if row.Task == nil {
				row.Action = homeAlertAction(alert.Category, project)
			}
			if strings.TrimSpace(row.Label) == "" {
				row.Label = group.Label
			}
			if strings.TrimSpace(row.Label) == "" || row.Action == homeAttentionNoAction {
				continue
			}
			rows = append(rows, row)
		}
	}
	return rows
}

func homeAlertAction(category alerts.AlertCategory, project *models.Project) homeAttentionAction {
	if project != nil {
		switch category {
		case alerts.CategoryInbox:
			return homeAttentionOpenProjectInbox
		case alerts.CategoryBlocked:
			return homeAttentionOpenProjectBlockers
		default:
			return homeAttentionOpenProject
		}
	}
	switch category {
	case alerts.CategoryDailyReview:
		return homeAttentionOpenDailyReview
	case alerts.CategoryWeeklyReview:
		return homeAttentionOpenWeeklyReview
	case alerts.CategoryInbox:
		return homeAttentionOpenGlobalInbox
	case alerts.CategoryBlocked:
		return homeAttentionOpenGlobalBlockers
	default:
		return homeAttentionOpenNotifications
	}
}

func homeParkedRows(projects []*models.Project, limit int) []homeAttentionRow {
	if limit <= 0 {
		return nil
	}
	var entries []parkedResumeEntry
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.IsComplete() || !task.IsParked() {
				continue
			}
			entries = append(entries, parkedResumeEntry{task: task, project: project})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i].task, entries[j].task
		if left.ParkedAt != nil && right.ParkedAt != nil && !left.ParkedAt.Equal(*right.ParkedAt) {
			return left.ParkedAt.After(*right.ParkedAt)
		}
		if left.ParkedAt != nil && right.ParkedAt == nil {
			return true
		}
		if left.ParkedAt == nil && right.ParkedAt != nil {
			return false
		}
		return left.UpdatedAt.After(right.UpdatedAt)
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	rows := make([]homeAttentionRow, 0, len(entries))
	for _, entry := range entries {
		detail := entry.project.Name
		if note := strings.TrimSpace(entry.task.ParkedNote); note != "" {
			detail += " · " + note
		}
		rows = append(rows, homeAttentionRow{
			Category: homeAttentionParked,
			Label:    "Resume " + entry.task.Title,
			Detail:   detail,
			Task:     entry.task,
			Project:  entry.project,
			Action:   homeAttentionOpenTask,
		})
	}
	return rows
}

func homePipelineRows(counts attentionCounts) []homeAttentionRow {
	var rows []homeAttentionRow
	if counts.inboxCount > 0 {
		rows = append(rows, homeAttentionRow{
			Category: homeAttentionPipeline,
			Label:    fmt.Sprintf("Triage %d inbox item(s)", counts.inboxCount),
			Detail:   "Pipeline intake is waiting",
			Action:   homeAttentionOpenGlobalInbox,
		})
	}
	if counts.overdueCount > 0 {
		rows = append(rows, homeAttentionRow{
			Category: homeAttentionPipeline,
			Label:    fmt.Sprintf("Resolve %d overdue task(s)", counts.overdueCount),
			Detail:   "Open Task Workspace by due date",
			Action:   homeAttentionOpenDueDates,
		})
	}
	if counts.queueLen > 0 && counts.dailyPlanLen == 0 {
		rows = append(rows, homeAttentionRow{
			Category: homeAttentionPipeline,
			Label:    fmt.Sprintf("Plan from %d Next item(s)", counts.queueLen),
			Detail:   "Open Planner · Next",
			Action:   homeAttentionOpenPlannerNext,
		})
	}
	return rows
}

func (m *DashboardModel) activateAttentionRow() tea.Cmd {
	row := m.selectedAttentionRow()
	if row == nil {
		return nil
	}
	return homeAttentionActionCmd(*row)
}

func homeAttentionActionCmd(row homeAttentionRow) tea.Cmd {
	return func() tea.Msg {
		switch row.Action {
		case homeAttentionOpenTask:
			return ShowTaskDetailMsg{Task: row.Task, Project: row.Project}
		case homeAttentionOpenProject:
			return SelectProjectMsg{Project: row.Project}
		case homeAttentionOpenGlobalInbox:
			return ShowGlobalInboxProcessorMsg{}
		case homeAttentionOpenProjectInbox:
			return ShowInboxProcessorMsg{Project: row.Project}
		case homeAttentionOpenPlannerToday:
			return ShowFocusBoardMsg{Tab: boardTabToday, SetTab: true}
		case homeAttentionOpenPlannerNext:
			return ShowFocusBoardMsg{Tab: boardTabNext, SetTab: true}
		case homeAttentionOpenDueDates:
			return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{
				Focus: OutlinerAll, SetFocus: true, Sort: SortByDueDate, SetSort: true,
				Lens: OutlinerLensDueDate, SetLens: true,
			}}
		case homeAttentionOpenGlobalBlockers:
			return ShowGlobalBlockersMsg{}
		case homeAttentionOpenProjectBlockers:
			return ShowBlockersMsg{Project: row.Project}
		case homeAttentionOpenDailyReview:
			return ShowDailyReviewMsg{}
		case homeAttentionOpenWeeklyReview:
			return ShowWeeklyReviewMsg{}
		case homeAttentionOpenNotifications:
			return ShowNotificationCenterMsg{}
		default:
			return nil
		}
	}
}
