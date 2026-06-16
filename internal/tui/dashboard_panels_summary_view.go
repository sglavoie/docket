package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// renderProjectSummarySection renders the project summary card and alerts card.
// When stack is true, the cards are stacked vertically at full width regardless
// of available horizontal room; otherwise they render side-by-side when wide.
func (m *DashboardModel) renderProjectSummarySection(width, paneHeight int, selected *models.Project, stack bool) string {
	cfg := DashboardCardConfig()

	numCards := 1
	hasSummary := selected != nil
	if hasSummary {
		numCards++
	}

	wide := !stack && m.width >= DashboardInfoSectionWideMinWidth && numCards > 1
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

	alertsContent := m.renderGlobalAlerts(alertsContentW, selected)
	cards = append(cards, dashboardPanelCard{title: "Alerts", content: alertsContent, width: alertsCardW})

	if stack && hasSummary && len(cards) == 2 {
		return renderStackedSummaryAndAlerts(width, paneHeight, cards[0], cards[1])
	}

	cfg.Height = paneHeight
	return renderDashboardCards(width, wide, cfg, cards)
}

func renderStackedSummaryAndAlerts(width, paneHeight int, summaryCard, alertsCard dashboardPanelCard) string {
	const gapHeight = 1

	minCardHeight := CardVerticalBorderOverhead + 1
	totalHeight := max(paneHeight, minCardHeight*2+gapHeight)

	maxAlertsHeight := max(totalHeight-gapHeight-minCardHeight, minCardHeight)
	alertsHeight := min(max(dashboardNaturalCardHeight(alertsCard.content), minCardHeight), maxAlertsHeight)
	summaryHeight := totalHeight - gapHeight - alertsHeight

	summaryCfg := DashboardCardConfig()
	summaryCfg.Height = summaryHeight
	alertsCfg := DashboardCardConfig()
	alertsCfg.Height = alertsHeight

	return renderCard(summaryCard.title, summaryCard.content, width, summaryCfg) +
		"\n" +
		renderCard(alertsCard.title, alertsCard.content, width, alertsCfg)
}

func dashboardNaturalCardHeight(content string) int {
	contentHeight := lipgloss.Height(content)
	if contentHeight < 1 {
		contentHeight = 1
	}
	return contentHeight + CardVerticalBorderOverhead
}

// renderProjectSummaryContent renders the section and status breakdown for a project.
func (m *DashboardModel) renderProjectSummaryContent(project *models.Project) string {
	var statusLine string
	if icon := models.ProjectStatusIconByKey(project.StatusIcon); icon != nil {
		if project.Description != "" {
			statusLine = fmt.Sprintf("%s %s — %s", icon.DisplayIcon(), icon.Label, project.Description)
		} else {
			statusLine = fmt.Sprintf("%s %s", icon.DisplayIcon(), icon.Label)
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
			lines = append(lines, NormalStyle.Render(fmt.Sprintf("  %-12s%d", s.label, count)))
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
		{models.StatusMaybe, "Maybe"},
		{models.StatusBlocked, "Blocked"},
	}

	for _, s := range statusOrder {
		if count := statusCounts[s.status]; count > 0 {
			icon := s.status.Icon()
			iconW := lipgloss.Width(icon)
			labelCol := max(14-iconW, 1)
			lines = append(lines, NormalStyle.Render(fmt.Sprintf("  %s %-*s%d", icon, labelCol, s.label, count)))
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
