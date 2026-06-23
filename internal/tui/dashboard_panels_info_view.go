package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
)

// renderInfoSection dispatches between the compact (default) and full
// dashboard info card layouts based on m.dashboardCompactCards.
func (m *DashboardModel) renderInfoSection(width, paneHeight int, selected *models.Project) string {
	if m.dashboardCompactCards {
		return m.renderInfoSectionCompact(width, paneHeight, selected)
	}
	return m.renderInfoSectionFull(width, paneHeight, selected)
}

// renderInfoSectionFull renders all four info cards regardless of whether
// they carry actionable content. Used when the user has toggled `, c` to
// opt out of compact mode.
func (m *DashboardModel) renderInfoSectionFull(width, paneHeight int, selected *models.Project) string {
	cfg := DashboardCardConfig()
	cfg.Height = paneHeight

	hasRecent := selected != nil
	hasPipeline := true
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
	if hasPipeline {
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

	if hasPipeline {
		pipelineContent := m.renderPipelineCardContent(otherContentW)
		cards = append(cards, dashboardPanelCard{title: "Pipeline", content: pipelineContent, width: otherCardW})
	}

	alertsContent := m.renderGlobalAlerts(alertsContentW, selected)
	cards = append(cards, dashboardPanelCard{title: "Alerts", content: alertsContent, width: alertsCardW})

	return renderDashboardCards(width, wide, cfg, cards)
}

// infoCardKind enumerates the four info cards in canonical compact-mode order.
type infoCardKind int

const (
	infoCardAlerts infoCardKind = iota
	infoCardTopScored
	infoCardPipeline
	infoCardRecent
)

// infoCardSpec carries enough information to render either a full card or a
// one-line inert summary row for a single info card.
type infoCardSpec struct {
	kind       infoCardKind
	title      string
	actionable bool
	summary    string
}

// renderInfoSectionCompact renders actionable cards in the usual grid and
// collapses inert cards into one-line summary rows beneath. Card order
// within each partition: Alerts → Top Scored → Pipeline → Recent.
func (m *DashboardModel) renderInfoSectionCompact(width, paneHeight int, selected *models.Project) string {
	cfg := DashboardCardConfig()
	cfg.Height = paneHeight

	alertLines := m.computeGlobalAlertLines(selected)

	var scoringProfile *scoring.ScoringProfile
	var bestTask *models.Task
	var bestScore float64
	if selected != nil {
		scoringProfile = m.config.GetProjectScoringProfile(selected.ID)
		if scoringProfile != nil {
			for _, task := range selected.Tasks {
				if task == nil || task.Status.IsComplete() {
					continue
				}
				score := scoring.ComputeScore(task, scoringProfile)
				if bestTask == nil || score > bestScore {
					bestTask = task
					bestScore = score
				}
			}
		}
	}

	var recentTasks []*models.Task
	if selected != nil {
		recentTasks = storage.GetRecentTasks(selected.Tasks, 5)
	}

	specs := []infoCardSpec{
		{kind: infoCardAlerts, title: "Alerts", actionable: len(alertLines) > 0, summary: "clear"},
	}
	if selected != nil && scoringProfile != nil {
		specs = append(specs, infoCardSpec{
			kind:       infoCardTopScored,
			title:      "Top Scored",
			actionable: bestTask != nil,
			summary:    "no scoring candidates",
		})
	}
	specs = append(specs, infoCardSpec{
		kind:       infoCardPipeline,
		title:      "Pipeline",
		actionable: m.pipelineHasContent() || selected == nil,
		summary:    "idle",
	})
	if selected != nil {
		recentTitle := fmt.Sprintf("Recent — %s", selected.Name)
		specs = append(specs, infoCardSpec{
			kind:       infoCardRecent,
			title:      recentTitle,
			actionable: len(recentTasks) > 0,
			summary:    "no tasks",
		})
	} else {
		specs = append(specs, infoCardSpec{
			kind:       infoCardRecent,
			title:      "Recent",
			actionable: false,
			summary:    "no project selected",
		})
	}

	var actionable, inert []infoCardSpec
	for _, s := range specs {
		if s.actionable {
			actionable = append(actionable, s)
		} else {
			inert = append(inert, s)
		}
	}

	cards := m.buildActionableInfoCards(actionable, width, selected, scoringProfile, alertLines, recentTasks)
	wide := m.width >= DashboardInfoSectionWideMinWidth && len(cards) > 1

	var b strings.Builder
	if len(cards) > 0 {
		b.WriteString(renderDashboardCards(width, wide, cfg, cards))
	}
	for i, s := range inert {
		if b.Len() > 0 || i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(renderCompactInertRow(s.title, s.summary, width))
	}
	return b.String()
}

// pipelineHasContent returns true when any pipeline stage has a non-zero count.
func (m *DashboardModel) pipelineHasContent() bool {
	pc := m.pipelineCounts
	return pc.inboxCount > 0 || pc.queueLen > 0 || pc.dailyPlanLen > 0 ||
		pc.inProgressCount > 0 || pc.completedTodayCount > 0
}

// renderCompactInertRow renders a single inert-card summary row in muted style.
func renderCompactInertRow(title, summary string, width int) string {
	line := fmt.Sprintf("%s · %s", title, summary)
	return MutedStyle.Render(truncateToVisualWidth(line, max(width, 1)))
}

// buildActionableInfoCards converts actionable specs into rendered card payloads.
func (m *DashboardModel) buildActionableInfoCards(
	specs []infoCardSpec,
	width int,
	selected *models.Project,
	scoringProfile *scoring.ScoringProfile,
	alertLines []string,
	recentTasks []*models.Task,
) []dashboardPanelCard {
	if len(specs) == 0 {
		return nil
	}

	numCards := len(specs)
	wide := m.width >= DashboardInfoSectionWideMinWidth && numCards > 1

	hasAlerts := false
	for _, s := range specs {
		if s.kind == infoCardAlerts {
			hasAlerts = true
			break
		}
	}

	otherCardW := width
	alertsCardW := width
	if wide {
		if hasAlerts {
			alertsReduction := width / 10
			otherCount := numCards - 1
			if otherCount > 0 {
				alertsCardW = width/numCards - alertsReduction
				otherCardW = (width - alertsCardW) / otherCount
			} else {
				alertsCardW = width
			}
		} else {
			otherCardW = width / numCards
		}
		if alertsCardW < MinCardWidth {
			alertsCardW = MinCardWidth
		}
		if otherCardW < MinCardWidth {
			otherCardW = MinCardWidth
		}
	}

	cards := make([]dashboardPanelCard, 0, numCards)
	for _, s := range specs {
		w := otherCardW
		if s.kind == infoCardAlerts {
			w = alertsCardW
		}
		contentW := max(w-CardBorderOverhead, 1)

		var title, content string
		switch s.kind {
		case infoCardAlerts:
			title = "Alerts"
			if len(alertLines) > 0 {
				content = strings.Join(alertLines, "\n")
			} else {
				content = MutedStyle.Render("No alerts")
			}
		case infoCardTopScored:
			title = "Top Scored"
			content = m.renderTopScoredContent(selected, scoringProfile, contentW)
		case infoCardPipeline:
			title = "Pipeline"
			content = m.renderPipelineCardContent(contentW)
		case infoCardRecent:
			if selected != nil {
				title = fmt.Sprintf("Recent — %s", truncateToVisualWidth(selected.Name, w-16))
				if len(recentTasks) > 0 {
					content = m.renderRecentTasksContentWithWidth(selected, contentW)
				} else {
					content = MutedStyle.Render(EmptyStateNoTasks)
				}
			} else {
				title = "Recent"
				content = MutedStyle.Render("no project selected")
			}
		}
		cards = append(cards, dashboardPanelCard{title: title, content: content, width: w})
	}
	return cards
}

// bestScoredTask returns the highest-scored active task in a project, or nil if none exist.
// Skips completed tasks and tasks in Someday/Completed sections. When profile is nil, returns
// the first active task without scoring.
func bestScoredTask(project *models.Project, profile *scoring.ScoringProfile) *models.Task {
	var best *models.Task
	var bestScore float64
	for _, task := range project.Tasks {
		if task == nil || task.Status.IsComplete() {
			continue
		}
		if task.Section == models.SectionSomeday || task.Section == models.SectionCompleted {
			continue
		}
		if profile != nil {
			score := scoring.ComputeScore(task, profile)
			if best == nil || score > bestScore {
				best = task
				bestScore = score
			}
		} else if best == nil {
			best = task
		}
	}
	return best
}

// renderSuggestedTask formats a single suggested-task alert line.
func renderSuggestedTask(t *models.Task) string {
	typeIcon := GetTypeBadge(string(t.Type), t.Type.Icon())
	title := TaskTitleStyleFor(t, TaskTitleStyleOptions{}).
		Render(truncateToVisualWidth(t.Title, 40))
	prefix := DueSoonStyle.Render("▸ Suggested:")
	return fmt.Sprintf("%s %s %s", prefix, typeIcon, title)
}

// renderTopScoredContent returns the highest-scored incomplete task with a score bar.
func (m *DashboardModel) renderTopScoredContent(project *models.Project, profile *scoring.ScoringProfile, contentWidth int) string {
	bestTask := bestScoredTask(project, profile)
	if bestTask == nil {
		return ""
	}

	bar := RenderScoreBar(bestTask, profile)
	badge := RenderScoreBadge(bestTask, profile)
	titleWidth := max(contentWidth-15, 10)
	title := TaskTitleStyleFor(bestTask, TaskTitleStyleOptions{}).
		Render(truncateToVisualWidth(bestTask.Title, titleWidth))
	typeIcon := GetTypeBadge(string(bestTask.Type), bestTask.Type.Icon())
	statusIcon := GetStatusBadge(string(bestTask.Status), bestTask.Status.Icon())

	return fmt.Sprintf("%s %s %s %s %s", bar, badge, typeIcon, statusIcon, title)
}

func (m *DashboardModel) renderRecentTasksContentWithWidth(project *models.Project, contentWidth int) string {
	recentTasks := storage.GetRecentTasks(project.Tasks, 5)
	if len(recentTasks) == 0 {
		return MutedStyle.Render(EmptyStateNoTasks)
	}

	titleWidth := max(contentWidth-5, MinTitleTruncWidth)

	// Align badge columns across the 5 rows. Narrow cards skip alignment.
	var layout *ColumnLayout
	if contentWidth >= MinColumnAlignWidth {
		layout = ComputeBadgeWidths(recentTasks, BadgeWidthOpts{
			IncludeStatus: true,
			IncludeType:   true,
		})
	}

	var lines []string
	for _, task := range recentTasks {
		timeStr := storage.FormatRelativeTime(task.UpdatedAt)
		typeBadge := GetTypeBadge(string(task.Type), task.Type.Icon())
		statusBadge := GetStatusBadge(string(task.Status), task.Status.Icon())
		if layout != nil && layout.HasType {
			typeBadge = PadBadge(typeBadge, layout.TypeWidth)
		}
		if layout != nil && layout.HasStatus {
			statusBadge = PadBadge(statusBadge, layout.StatusWidth)
		}

		titleStyle := TaskTitleStyleFor(task, TaskTitleStyleOptions{})
		if lipgloss.Width(task.Title)+2+lipgloss.Width(timeStr) <= titleWidth {
			gap := max(titleWidth-lipgloss.Width(task.Title)-lipgloss.Width(timeStr), 2)
			line := fmt.Sprintf("%s %s %s%s%s",
				typeBadge,
				statusBadge,
				titleStyle.Render(task.Title),
				strings.Repeat(" ", gap),
				MutedStyle.Render(timeStr))
			lines = append(lines, line, "")
			continue
		}

		line1 := fmt.Sprintf("%s %s %s",
			typeBadge,
			statusBadge,
			titleStyle.Render(truncateAndPad(task.Title, titleWidth)))
		padding := max(contentWidth-lipgloss.Width(timeStr), 0)
		line2 := MutedStyle.Render(strings.Repeat(" ", padding) + timeStr)
		lines = append(lines, line1, line2)
	}

	return strings.Join(lines, "\n")
}

// renderPipelineCardContent renders a compact pipeline stage summary for the dashboard info card.
func (m *DashboardModel) renderPipelineCardContent(width int) string {
	pc := m.pipelineCounts
	stages := []struct {
		name  string
		count int
	}{
		{"Triage", pc.inboxCount},
		{"Plan", pc.queueLen},
		{"Today", pc.dailyPlanLen},
		{"Execute", pc.inProgressCount},
		{"Review", pc.completedTodayCount},
	}

	var parts []string
	for _, s := range stages {
		parts = append(parts, fmt.Sprintf("%s(%d)", s.name, s.count))
	}
	line := strings.Join(parts, " → ")
	hint := MutedStyle.Render(";; to open hub")
	return strings.Join([]string{
		truncateToVisualWidth(line, width),
		hint,
	}, "\n")
}
