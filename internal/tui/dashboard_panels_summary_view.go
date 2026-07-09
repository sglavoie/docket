package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// renderProjectSummarySection renders the project summary, resume, and alerts cards.
// When stack is true, the cards are stacked vertically at full width regardless
// of available horizontal room; otherwise they render side-by-side when wide.
func (m *DashboardModel) renderProjectSummarySection(width, paneHeight int, selected *models.Project, stack bool) string {
	cfg := DashboardCardConfig()

	numCards := 1
	hasSummary := selected != nil
	if hasSummary {
		numCards++
	}
	resumeEntries := m.collectParkedResumeTasks(m.resumeCardLimit())
	hasResume := len(resumeEntries) > 0
	if hasResume {
		numCards++
	}

	wide := !stack && m.width >= DashboardInfoSectionWideMinWidth && numCards > 1
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

	if hasSummary {
		summaryContent := m.renderProjectSummaryContent(selected)
		summaryTitle := fmt.Sprintf("%s Summary", truncateToVisualWidth(selected.Name, otherCardW-16))
		cards = append(cards, dashboardPanelCard{title: summaryTitle, content: summaryContent, width: otherCardW})
	}

	if hasResume {
		resumeContent := m.renderResumeCardContent(resumeEntries, otherContentW)
		cards = append(cards, dashboardPanelCard{title: "Resume", content: resumeContent, width: otherCardW})
	}

	alertsContent := m.renderGlobalAlerts(alertsContentW, selected)
	cards = append(cards, dashboardPanelCard{title: "Alerts", content: alertsContent, width: alertsCardW})

	if stack && len(cards) > 1 {
		return renderStackedProjectSummaryCards(width, paneHeight, cards)
	}

	cfg.Height = paneHeight
	return renderDashboardCards(width, wide, cfg, cards)
}

func renderStackedProjectSummaryCards(width, paneHeight int, cards []dashboardPanelCard) string {
	minCardHeight := CardVerticalBorderOverhead + 1
	totalHeight := max(paneHeight, minCardHeight*len(cards))

	heights := make([]int, len(cards))
	remaining := totalHeight
	for i := len(cards) - 1; i >= 1; i-- {
		maxHeight := max(remaining-minCardHeight*i, minCardHeight)
		cardHeight := max(dashboardNaturalCardHeight(cards[i].content), minCardHeight)
		heights[i] = min(cardHeight, maxHeight)
		remaining -= heights[i]
	}
	heights[0] = max(remaining, minCardHeight)

	parts := make([]string, 0, len(cards))
	for i, card := range cards {
		cardCfg := DashboardCardConfig()
		cardCfg.Height = heights[i]
		parts = append(parts, renderCard(card.title, card.content, width, cardCfg))
	}
	return strings.Join(parts, "\n")
}

func dashboardNaturalCardHeight(content string) int {
	contentHeight := max(lipgloss.Height(content), 1)
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
