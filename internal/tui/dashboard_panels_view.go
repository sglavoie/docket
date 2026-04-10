package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

type dashboardPanelCard struct {
	title   string
	content string
	width   int
}

// renderInfoSection renders the info cards below the project list.
// Wide (>= DashboardInfoSectionWideMinWidth): cards side-by-side.
// Narrow: cards stacked vertically.
func (m *DashboardModel) renderInfoSection(width int, selected *models.Project) string {
	cfg := DashboardCardConfig()
	cfg.Height = DashboardBottomPaneHeight

	hasRecent := selected != nil
	var scoringProfile *scoring.ScoringProfile
	hasTopScored := false
	if selected != nil {
		scoringProfile = m.config.GetProjectScoringProfile(selected.ID)
		if scoringProfile != nil {
			for _, task := range selected.Tasks {
				if task == nil {
					continue
				}
				if !task.Status.IsComplete() {
					hasTopScored = true
					break
				}
			}
		}
	}

	numCards := 1
	if hasRecent {
		numCards++
	}
	if hasTopScored {
		numCards++
	}

	wide := m.width >= DashboardInfoSectionWideMinWidth && numCards > 1
	otherCardW := width
	alertsCardW := width
	if wide {
		alertsReduction := width / 10
		otherCount := numCards - 1
		if otherCount > 0 {
			alertsCardW = width/numCards - alertsReduction
			otherCardW = (width - alertsCardW) / otherCount
		} else {
			alertsCardW = width
		}
		if alertsCardW < MinCardWidth {
			alertsCardW = MinCardWidth
		}
		if otherCardW < MinCardWidth {
			otherCardW = MinCardWidth
		}
	}
	otherContentW := max(otherCardW-CardBorderOverhead, 1)
	alertsContentW := max(alertsCardW-CardBorderOverhead, 1)

	var cards []dashboardPanelCard

	if hasRecent {
		recentContent := MutedStyle.Render(EmptyStateNoTasks)
		if len(selected.Tasks) > 0 {
			recentContent = m.renderRecentTasksContentWithWidth(selected, otherContentW)
		}
		recentTitle := fmt.Sprintf("Recent — %s", truncateToVisualWidth(selected.Name, otherCardW-16))
		cards = append(cards, dashboardPanelCard{title: recentTitle, content: recentContent, width: otherCardW})
	}

	if hasTopScored {
		topContent := m.renderTopScoredContent(selected, scoringProfile, otherContentW)
		if topContent != "" {
			cards = append(cards, dashboardPanelCard{title: "Top Scored", content: topContent, width: otherCardW})
		}
	}

	alertsContent := m.renderGlobalAlerts(alertsContentW)
	cards = append(cards, dashboardPanelCard{title: "Alerts", content: alertsContent, width: alertsCardW})

	return renderDashboardCards(width, wide, cfg, cards)
}

// renderProjectSummarySection renders the project summary card and alerts card.
func (m *DashboardModel) renderProjectSummarySection(width int, selected *models.Project) string {
	cfg := DashboardCardConfig()
	cfg.Height = DashboardBottomPaneHeight

	numCards := 1
	hasSummary := selected != nil
	if hasSummary {
		numCards++
	}

	wide := m.width >= DashboardInfoSectionWideMinWidth && numCards > 1
	summaryCardW := width
	alertsCardW := width
	if wide {
		alertsReduction := width / 10
		summaryCardW = width - (width/numCards - alertsReduction)
		alertsCardW = max(width-summaryCardW, MinCardWidth)
		if summaryCardW < MinCardWidth {
			summaryCardW = MinCardWidth
		}
	}
	alertsContentW := max(alertsCardW-CardBorderOverhead, 1)

	var cards []dashboardPanelCard

	if hasSummary {
		summaryContent := m.renderProjectSummaryContent(selected)
		summaryTitle := fmt.Sprintf("%s Summary", truncateToVisualWidth(selected.Name, summaryCardW-16))
		cards = append(cards, dashboardPanelCard{title: summaryTitle, content: summaryContent, width: summaryCardW})
	}

	alertsContent := m.renderGlobalAlerts(alertsContentW)
	cards = append(cards, dashboardPanelCard{title: "Alerts", content: alertsContent, width: alertsCardW})

	return renderDashboardCards(width, wide, cfg, cards)
}

func renderDashboardCards(width int, wide bool, cfg CardConfig, cards []dashboardPanelCard) string {
	if len(cards) == 0 {
		return ""
	}
	if wide && len(cards) > 1 {
		var rendered []string
		usedWidth := 0
		for i, c := range cards {
			cardWidth := c.width
			if i == len(cards)-1 {
				cardWidth = width - usedWidth
			}
			usedWidth += cardWidth
			rendered = append(rendered, renderCard(c.title, c.content, cardWidth, cfg))
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	}

	var parts []string
	for _, c := range cards {
		parts = append(parts, renderCard(c.title, c.content, width, cfg))
	}
	return strings.Join(parts, "\n")
}

// renderProjectSummaryContent renders the section and status breakdown for a project.
func (m *DashboardModel) renderProjectSummaryContent(project *models.Project) string {
	var statusLine string
	if icon := models.ProjectStatusIconByKey(project.StatusIcon); icon != nil {
		if project.Description != "" {
			statusLine = fmt.Sprintf("%s %s — %s", icon.Emoji, icon.Label, project.Description)
		} else {
			statusLine = fmt.Sprintf("%s %s", icon.Emoji, icon.Label)
		}
	} else if project.Description != "" {
		statusLine = project.Description
	}

	if len(project.Tasks) == 0 {
		if statusLine != "" {
			return MutedStyle.Render(statusLine) + "\n\n" + MutedStyle.Render(EmptyStateNoTasks)
		}
		return MutedStyle.Render(EmptyStateNoTasks)
	}

	var lines []string
	if statusLine != "" {
		lines = append(lines, MutedStyle.Render(statusLine), "")
	}

	sectionCounts := make(map[string]int)
	statusCounts := make(map[models.TaskStatus]int)
	var latestUpdate time.Time

	for _, task := range project.Tasks {
		if task == nil || task.Status.IsComplete() {
			continue
		}
		section := task.Section
		if section == "" {
			section = "Active"
		}
		sectionCounts[section]++
		if task.CustomSection != "" {
			sectionCounts[task.CustomSection]++
		}
		statusCounts[task.Status]++
		if task.UpdatedAt.After(latestUpdate) {
			latestUpdate = task.UpdatedAt
		}
	}

	sectionOrder := []struct {
		key   string
		label string
	}{
		{models.SectionInbox, "Inbox"},
		{models.SectionNext, "Next"},
		{"Active", "Active"},
		{models.SectionWaiting, "Waiting"},
		{models.SectionSomeday, "Someday"},
		{models.SectionStandby, "Standby"},
		{models.SectionReview, "Review"},
	}

	for _, s := range sectionOrder {
		if count := sectionCounts[s.key]; count > 0 {
			lines = append(lines, fmt.Sprintf("  %-12s%d", s.label, count))
		}
	}

	if len(lines) > 0 {
		lines = append(lines, "")
	}

	statusOrder := []struct {
		status models.TaskStatus
		label  string
	}{
		{models.StatusTodo, "Todo"},
		{models.StatusInProgress, "In Progress"},
		{models.StatusReview, "Review"},
		{models.StatusStandby, "Standby"},
		{models.StatusBlocked, "Blocked"},
	}

	for _, s := range statusOrder {
		if count := statusCounts[s.status]; count > 0 {
			lines = append(lines, fmt.Sprintf("  %s %-12s%d", s.status.Icon(), s.label, count))
		}
	}

	if !latestUpdate.IsZero() {
		lines = append(lines, "", MutedStyle.Render(fmt.Sprintf("  Last activity: %s", storage.FormatRelativeTime(latestUpdate))))
	}

	if len(lines) == 0 {
		return MutedStyle.Render("No active tasks.")
	}
	return strings.Join(lines, "\n")
}

// renderGlobalAlerts aggregates alert counts across all active projects.
func (m *DashboardModel) renderGlobalAlerts(_ int) string {
	var totalOverdue, totalDueToday, totalBlocked, totalBlockers, totalCompletedThisWeek int
	var inboxProjects []string

	now := time.Now()
	today := util.StartOfDay(now)
	sevenDaysAgo := now.AddDate(0, 0, -7)

	activeProjects := m.projects
	if m.showArchived {
		activeProjects = m.config.GetActiveProjects()
		for _, p := range activeProjects {
			if err := storage.LoadProject(p); err != nil {
				logging.Warn("failed to load project %s: %v", p.Name, err)
			}
		}
	}

	for _, p := range activeProjects {
		totalOverdue += p.CountOverdueTasks()
		totalBlocked += p.CountBlockedTasks()
		totalBlockers += storage.GetActiveBlockerCount(p)

		for _, task := range p.Tasks {
			if task == nil {
				continue
			}
			if task.DueDate != nil && !task.Status.IsComplete() && util.StartOfDay(*task.DueDate).Equal(today) {
				totalDueToday++
			}
			if task.CompletedAt != nil && task.CompletedAt.After(sevenDaysAgo) {
				totalCompletedThisWeek++
			}
		}

		inboxCount := 0
		for _, task := range p.Tasks {
			if task != nil && task.Section == models.SectionInbox && !task.Status.IsComplete() {
				inboxCount++
			}
		}
		if inboxCount >= 5 {
			inboxProjects = append(inboxProjects, fmt.Sprintf("%s(%d)", p.Name, inboxCount))
		}
	}

	var lines []string
	if totalOverdue > 0 {
		lines = append(lines, OverdueStyle.Render(fmt.Sprintf("🔥 %d overdue", totalOverdue)))
	}
	if totalDueToday > 0 {
		lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("⏰ %d due today", totalDueToday)))
	}
	if totalBlocked > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("◈ %d blocked", totalBlocked)))
	}
	if totalBlockers > 0 {
		lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("🚧 %d active blockers", totalBlockers)))
	}
	if len(inboxProjects) > 0 {
		lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("📥 Inbox backlog: %s", strings.Join(inboxProjects, ", "))))
	}
	if totalCompletedThisWeek > 0 {
		lines = append(lines, SuccessStyle.Render(fmt.Sprintf("✅ %d completed this week", totalCompletedThisWeek)))
	}

	globalNotes, _ := storage.ListNotesFrom(storage.GlobalNotesDir(m.config.DataDir))
	if len(globalNotes) > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("📝 %d global notes", len(globalNotes))))
	}

	if len(lines) == 0 {
		return MutedStyle.Render("No alerts")
	}
	return strings.Join(lines, "\n")
}

// renderTopScoredContent returns the highest-scored incomplete task with a score bar.
func (m *DashboardModel) renderTopScoredContent(project *models.Project, profile *scoring.ScoringProfile, contentWidth int) string {
	var bestTask *models.Task
	var bestScore float64
	for _, task := range project.Tasks {
		if task == nil || task.Status.IsComplete() {
			continue
		}
		score := scoring.ComputeScore(task, profile)
		if bestTask == nil || score > bestScore {
			bestTask = task
			bestScore = score
		}
	}
	if bestTask == nil {
		return ""
	}

	bar := RenderScoreBar(bestTask, profile)
	badge := RenderScoreBadge(bestTask, profile)
	titleWidth := max(contentWidth-15, 10)
	title := truncateToVisualWidth(bestTask.Title, titleWidth)
	typeIcon := GetTypeBadge(string(bestTask.Type), bestTask.Type.Icon())
	stageBadge := GetStageBadge(bestTask.Stage)
	stageSegment := ""
	if stageBadge != "" {
		stageSegment = stageBadge + " "
	}
	statusIcon := GetStatusBadge(string(bestTask.Status), bestTask.Status.Icon())

	return fmt.Sprintf("%s %s %s %s%s %s", bar, badge, typeIcon, stageSegment, statusIcon, title)
}

func (m *DashboardModel) renderRecentTasksContentWithWidth(project *models.Project, contentWidth int) string {
	recentTasks := storage.GetRecentTasks(project.Tasks, 5)
	if len(recentTasks) == 0 {
		return MutedStyle.Render(EmptyStateNoTasks)
	}

	titleWidth := max(contentWidth-5, MinTitleTruncWidth)

	var lines []string
	for _, task := range recentTasks {
		timeStr := storage.FormatRelativeTime(task.UpdatedAt)
		stageBadge := GetStageBadge(task.Stage)
		stageSegment := ""
		if stageBadge != "" {
			stageSegment = stageBadge + " "
		}

		if lipgloss.Width(task.Title)+2+lipgloss.Width(timeStr) <= titleWidth {
			gap := max(titleWidth-lipgloss.Width(task.Title)-lipgloss.Width(timeStr), 2)
			line := fmt.Sprintf("%s %s%s %s%s%s",
				GetTypeBadge(string(task.Type), task.Type.Icon()),
				stageSegment,
				GetStatusBadge(string(task.Status), task.Status.Icon()),
				task.Title,
				strings.Repeat(" ", gap),
				MutedStyle.Render(timeStr))
			lines = append(lines, line, "")
			continue
		}

		line1 := fmt.Sprintf("%s %s%s %s",
			GetTypeBadge(string(task.Type), task.Type.Icon()),
			stageSegment,
			GetStatusBadge(string(task.Status), task.Status.Icon()),
			truncateAndPad(task.Title, titleWidth))
		padding := max(contentWidth-lipgloss.Width(timeStr), 0)
		line2 := MutedStyle.Render(strings.Repeat(" ", padding) + timeStr)
		lines = append(lines, line1, line2)
	}

	return strings.Join(lines, "\n")
}
