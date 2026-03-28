package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

func (m *DashboardModel) View() tea.View {
	var b strings.Builder

	filterBar, chromeHeight := m.renderFilterBar()
	b.WriteString(filterBar)

	visible := m.visibleProjects()
	b.WriteString(m.renderDashboard(visible, chromeHeight))

	return tea.NewView(b.String())
}

// renderDashboard renders the always-stacked dashboard: full-width projects on top,
// optional info section below.
func (m *DashboardModel) renderDashboard(visible []*models.Project, chromeHeight int) string {
	var b strings.Builder

	cardWidth := max(m.width-2, MinCardWidth)

	// Pre-render info section to measure its actual height
	var infoContent string
	bottomHeight := 0
	panelMode := m.config.GetInfoPanelMode()
	if panelMode != config.InfoPanelOff && m.SearchQuery() == "" {
		var selected *models.Project
		if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
			selected = visible[m.cursor]
		}
		switch panelMode {
		case config.InfoPanelInfoCards:
			infoContent = m.renderInfoSection(cardWidth, selected)
		case config.InfoPanelProjectSummary:
			infoContent = m.renderProjectSummarySection(cardWidth, selected)
		default:
		}
		if infoContent != "" {
			// +1 for the final line (Count gives separators), +1 for the gap newline
			bottomHeight = strings.Count(infoContent, "\n") + 2
		}
	}

	cardTitle := m.projectCardTitle(visible)
	cardContent := m.renderProjectList(visible, chromeHeight, cardWidth, bottomHeight)

	cfg := DashboardCardConfig()
	b.WriteString(renderCard(cardTitle, cardContent, cardWidth, cfg))

	if infoContent != "" {
		b.WriteString("\n")
		b.WriteString(infoContent)
	}

	return b.String()
}

// renderInfoSection renders the info cards below the project list.
// Wide (>= DashboardInfoSectionWideMinWidth): cards side-by-side.
// Narrow: cards stacked vertically.
func (m *DashboardModel) renderInfoSection(width int, selected *models.Project) string {
	cfg := DashboardCardConfig()

	// Determine which cards will be shown (before rendering content)
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

	numCards := 1 // Alerts is always present
	if hasRecent {
		numCards++
	}
	if hasTopScored {
		numCards++
	}

	// Determine per-card widths so content renders at the correct width.
	// JoinHorizontal adds no gaps, so total card widths must equal width exactly.
	// Alerts card is 10% narrower; other cards split the extra space.
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

	// Render card contents at the correct width
	type card struct {
		title   string
		content string
		width   int
	}
	var cards []card

	if hasRecent {
		var recentContent string
		if len(selected.Tasks) == 0 {
			recentContent = MutedStyle.Render(EmptyStateNoTasks)
		} else {
			recentContent = m.renderRecentTasksContentWithWidth(selected, otherContentW)
		}
		recentTitle := fmt.Sprintf("Recent — %s", truncateToVisualWidth(selected.Name, otherCardW-16))
		cards = append(cards, card{title: recentTitle, content: recentContent, width: otherCardW})
	}

	if hasTopScored {
		topContent := m.renderTopScoredContent(selected, scoringProfile, otherContentW)
		if topContent != "" {
			cards = append(cards, card{title: "Top Scored", content: topContent, width: otherCardW})
		}
	}

	alertsContent := m.renderGlobalAlerts(alertsContentW)
	cards = append(cards, card{title: "Alerts", content: alertsContent, width: alertsCardW})

	if len(cards) == 0 {
		return ""
	}

	if wide && len(cards) > 1 {
		// Last card absorbs integer division remainder so cards fill full width
		var rendered []string
		usedWidth := 0
		for i, c := range cards {
			w := c.width
			if i == len(cards)-1 {
				w = width - usedWidth
			}
			usedWidth += w
			rendered = append(rendered, renderCard(c.title, c.content, w, cfg))
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	}

	// Narrow layout: stacked vertically
	var parts []string
	for _, c := range cards {
		parts = append(parts, renderCard(c.title, c.content, width, cfg))
	}
	return strings.Join(parts, "\n")
}

// renderProjectSummarySection renders the project summary card and alerts card.
func (m *DashboardModel) renderProjectSummarySection(width int, selected *models.Project) string {
	cfg := DashboardCardConfig()

	numCards := 1 // Alerts always present
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

	type card struct {
		title   string
		content string
		width   int
	}
	var cards []card

	if hasSummary {
		summaryContent := m.renderProjectSummaryContent(selected)
		summaryTitle := fmt.Sprintf("%s Summary", truncateToVisualWidth(selected.Name, summaryCardW-16))
		cards = append(cards, card{title: summaryTitle, content: summaryContent, width: summaryCardW})
	}

	alertsContent := m.renderGlobalAlerts(alertsContentW)
	cards = append(cards, card{title: "Alerts", content: alertsContent, width: alertsCardW})

	if wide && len(cards) > 1 {
		var rendered []string
		usedWidth := 0
		for i, c := range cards {
			w := c.width
			if i == len(cards)-1 {
				w = width - usedWidth
			}
			usedWidth += w
			rendered = append(rendered, renderCard(c.title, c.content, w, cfg))
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
	// Build status line: icon + description
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

	// Show status line at the top if set
	if statusLine != "" {
		lines = append(lines, MutedStyle.Render(statusLine))
		lines = append(lines, "")
	}

	// Count by section and status, skipping completed/canceled tasks
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

	// GTD section order
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
		count := sectionCounts[s.key]
		if count == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %-12s%d", s.label, count))
	}

	if len(lines) > 0 {
		lines = append(lines, "")
	}

	// Status breakdown
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
		count := statusCounts[s.status]
		if count == 0 {
			continue
		}
		icon := s.status.Icon()
		lines = append(lines, fmt.Sprintf("  %s %-12s%d", icon, s.label, count))
	}

	if !latestUpdate.IsZero() {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("  Last activity: %s", storage.FormatRelativeTime(latestUpdate))))
	}

	if len(lines) == 0 {
		return MutedStyle.Render("No active tasks.")
	}
	return strings.Join(lines, "\n")
}

// projectCardTitle returns the title for the project list card.
func (m *DashboardModel) projectCardTitle(visible []*models.Project) string {
	if m.showArchived {
		return fmt.Sprintf("Archived (%d)", len(visible))
	}
	return fmt.Sprintf("Projects (%d)", len(visible))
}

// renderProjectList renders the project list content for a card of the given width.
// bottomHeight is reserved space below the card (only used in full-width mode).
func (m *DashboardModel) renderProjectList(visible []*models.Project, chromeHeight, cardWidth, bottomHeight int) string { //nolint:gocyclo
	var cardContent strings.Builder

	if len(m.projects) == 0 {
		if m.showArchived {
			cardContent.WriteString(MutedStyle.Render("No archived projects."))
		} else {
			cardContent.WriteString(MutedStyle.Render("No projects yet. Press n to create one."))
		}
		return cardContent.String()
	}
	if len(visible) == 0 {
		cardContent.WriteString(MutedStyle.Render("No projects match your filters."))
		return cardContent.String()
	}

	narrowMode := cardWidth < DashboardNarrowModeWidth
	dueSoonDays := m.config.GetReviewDueSoonDays()

	// First pass: compute max widths for alignment
	showContext := m.filterContext == ""
	maxNameWidth := 0
	maxContextWidth := 0
	maxStatusWidth := 0
	// Check if any visible project has a status icon (reserves a fixed 3-col icon column)
	hasAnyIcon := false
	for _, p := range visible {
		if p.StatusIconEmoji() != "" {
			hasAnyIcon = true
			break
		}
	}
	iconColWidth := 0 // visual columns reserved for icon column
	if hasAnyIcon {
		iconColWidth = 3 // 2-col emoji + 1 space
	}
	for _, p := range visible {
		nameStr := p.Name
		if width := lipgloss.Width(nameStr); width > maxNameWidth {
			maxNameWidth = width
		}
		if showContext && p.Context != "" {
			ctxStr := fmt.Sprintf("[%s]", p.Context)
			if width := lipgloss.Width(ctxStr); width > maxContextWidth {
				maxContextWidth = width
			}
		}
		statusW := m.projectStatusWidth(p, dueSoonDays)
		if statusW > maxStatusWidth {
			maxStatusWidth = statusW
		}
	}

	// Calculate max visible projects (needed early to know if scrollbar appears).
	cardOverhead := CardVerticalBorderOverhead
	headerLines := 1
	maxVisible := m.height - chromeHeight - bottomHeight - cardOverhead - 1 - headerLines
	if narrowMode {
		availRows := m.height - chromeHeight - bottomHeight - cardOverhead - 1 - headerLines
		maxVisible = (availRows + 1) / 2 // ceiling division: partial last row still fits a project
	}
	if maxVisible < DashboardMinVisibleProjects {
		maxVisible = DashboardMinVisibleProjects
	}

	// Clamp column widths to fit available row width (wide mode only).
	// Reserve 1 column for the scrollbar when scrolling is needed.
	rowWidth := cardWidth - 4
	if len(visible) > maxVisible {
		rowWidth--
	}
	if !narrowMode {
		fixedCols := 20 + iconColWidth
		if maxStatusWidth > 0 {
			fixedCols += 2 + maxStatusWidth
		}
		availForNameCtx := max(rowWidth-fixedCols, 10)
		if showContext && maxContextWidth > 0 {
			needed := maxNameWidth + maxContextWidth
			if needed > availForNameCtx {
				maxContextWidth = availForNameCtx - maxNameWidth
				if maxContextWidth < 0 {
					maxContextWidth = 0
					maxNameWidth = availForNameCtx
				}
			}
		} else {
			if maxNameWidth > availForNameCtx {
				maxNameWidth = availForNameCtx
			}
		}
	}

	scrollOffset := max(min(calcScrollOffset(m.cursor, maxVisible), len(visible)-maxVisible), 0)

	// Column header row
	if !narrowMode {
		prefix := strings.Repeat(" ", 5+iconColWidth)
		namePad := strings.Repeat(" ", maxNameWidth)
		ctxPad := ""
		if showContext && maxContextWidth > 0 {
			ctxPad = strings.Repeat(" ", maxContextWidth+1)
		}
		header := fmt.Sprintf("%s%s %s %6s %6s", prefix, namePad, ctxPad, "Active", "Total")
		withStatus := header + "  Status"
		if lipgloss.Width(withStatus) <= rowWidth {
			header = withStatus
		}
		cardContent.WriteString(MutedStyle.Render(header))
		cardContent.WriteString("\n")
	}

	// Project list (windowed)
	end := min(scrollOffset+maxVisible, len(visible))

	pinnedNum := 0
	for i := range scrollOffset {
		if visible[i].Pinned {
			pinnedNum++
		}
	}

	var rowBuilder strings.Builder
	for i := scrollOffset; i < end; i++ {
		p := visible[i]
		selected := i == m.cursor
		style := NormalStyle
		numberPrefix := "  "
		if p.Pinned {
			pinnedNum++
			if pinnedNum <= 9 {
				numberPrefix = PinnedNumberStyle.Render(fmt.Sprintf("%d", pinnedNum)) + " "
			}
		}
		if selected {
			if p.Pinned {
				style = SuccessBoldStyle
			} else {
				style = SelectedItemTextStyle
			}
		} else if p.Pinned {
			style = PinnedProjectStyle
		}

		total := p.CountTasks() - p.CountCompletedTasks()
		active := p.CountActiveSectionTasks()

		overdue := p.CountOverdueTasks()
		dueSoon := p.CountDueSoonTasks(dueSoonDays)
		blocked := p.CountBlockedTasks()
		blockerCount := storage.GetActiveBlockerCount(p)

		// Build icon column (fixed visual width, separate from name)
		iconRendered := ""
		if hasAnyIcon {
			if emoji := p.StatusIconEmoji(); emoji != "" {
				iconRendered = emoji + " "
			} else {
				iconRendered = strings.Repeat(" ", iconColWidth)
			}
		}

		// Build the name string, padded/truncated to max width
		displayName := truncateAndPad(p.Name, maxNameWidth)
		var nameStr string
		if m.SearchQuery() != "" {
			nameStr = HighlightSubstring(displayName, m.SearchQuery(), style)
		} else {
			nameStr = style.Render(displayName)
		}
		nameRendered := iconRendered + nameStr

		// Context badge, padded/truncated
		contextRendered := ""
		if showContext && maxContextWidth > 0 {
			if p.Context != "" {
				ctxStr := truncateAndPad(fmt.Sprintf("[%s]", p.Context), maxContextWidth)
				if m.SearchQuery() != "" {
					contextRendered = HighlightSubstring(ctxStr, m.SearchQuery(), MutedStyle)
				} else {
					contextRendered = MutedStyle.Render(ctxStr)
				}
			} else {
				contextRendered = strings.Repeat(" ", maxContextWidth)
			}
		}

		// Per-project notes count
		projectNotes, _ := storage.ListNotes(p)
		noteCount := len(projectNotes)

		// Build status indicator
		completedCount := p.CountCompletedTasks()
		var statusIndicator string
		if overdue > 0 || dueSoon > 0 || blocked > 0 || blockerCount > 0 || completedCount > 0 {
			var parts []string
			if overdue > 0 {
				parts = append(parts, OverdueStyle.Render(fmt.Sprintf("🔥%d", overdue)))
			}
			if dueSoon > 0 {
				parts = append(parts, DueSoonStyle.Render(fmt.Sprintf("⏰%d", dueSoon)))
			}
			if blocked > 0 {
				parts = append(parts, MutedStyle.Render(fmt.Sprintf("◈%d", blocked)))
			}
			if blockerCount > 0 {
				parts = append(parts, DueSoonStyle.Render(fmt.Sprintf("🚧%d", blockerCount)))
			}
			if completedCount > 0 {
				parts = append(parts, MutedStyle.Render(fmt.Sprintf("✅%d", completedCount)))
			}
			statusIndicator = strings.Join(parts, " ")
		}

		// Score coverage badge (% of non-completed tasks with manual scores)
		scoreCoverageBadge := ""
		if profile := m.config.GetProjectScoringProfile(p.ID); profile != nil {
			scoredCount := 0
			totalScorable := 0
			for _, task := range p.Tasks {
				if task == nil {
					continue
				}
				if !task.Status.IsComplete() {
					totalScorable++
					if HasManualScores(task, profile) {
						scoredCount++
					}
				}
			}
			if totalScorable > 0 {
				pct := scoredCount * 100 / totalScorable
				scoreCoverageBadge = MutedStyle.Render(fmt.Sprintf("S:%d%%", pct))
			}
		}

		// Maximum visual width for content (rowWidth minus cursor prefix from RenderSelectableRow)
		maxContentWidth := rowWidth - 3

		if narrowMode {
			line1Content := fmt.Sprintf("%s%s %s", numberPrefix, nameRendered, contextRendered)
			line1 := RenderSelectableRow(selected, false, false, m.cursorFlash, line1Content, rowWidth)
			rowBuilder.WriteString(line1)
			rowBuilder.WriteString("\n")

			statsLine := fmt.Sprintf("      %s", MutedStyle.Render(fmt.Sprintf("Active: %d  Total: %d", active, total)))
			appendIfFits := func(badge string) {
				candidate := statsLine + "  " + badge
				if lipgloss.Width(candidate) <= rowWidth {
					statsLine = candidate
				}
			}
			if noteCount > 0 {
				appendIfFits(MutedStyle.Render(fmt.Sprintf("📝%d", noteCount)))
			}
			if scoreCoverageBadge != "" {
				appendIfFits(scoreCoverageBadge)
			}
			if statusIndicator != "" {
				appendIfFits(statusIndicator)
			}
			rowBuilder.WriteString(statsLine)
		} else {
			activeStr := MutedStyle.Render(fmt.Sprintf("%6d", active))
			totalStr := MutedStyle.Render(fmt.Sprintf("%6d", total))

			content := fmt.Sprintf("%s%s %s %s %s", numberPrefix, nameRendered, contextRendered, activeStr, totalStr)
			appendIfFits := func(badge string) {
				candidate := content + "  " + badge
				if lipgloss.Width(candidate) <= maxContentWidth {
					content = candidate
				}
			}
			if noteCount > 0 {
				appendIfFits(MutedStyle.Render(fmt.Sprintf("📝%d", noteCount)))
			}
			if scoreCoverageBadge != "" {
				appendIfFits(scoreCoverageBadge)
			}
			if statusIndicator != "" {
				appendIfFits(statusIndicator)
			}
			line := RenderSelectableRow(selected, false, false, m.cursorFlash, content, rowWidth)
			rowBuilder.WriteString(line)
		}

		if i < end-1 {
			rowBuilder.WriteString("\n")
		}
	}

	scrollbar := RenderScrollbar(len(visible), maxVisible, scrollOffset)
	cardContent.WriteString(AttachScrollbar(rowBuilder.String(), scrollbar))

	return cardContent.String()
}

// projectStatusWidth returns the terminal column width of the status indicator
// for a project (emojis are 2 columns wide, digits are 1, spaces are 1).
func (m *DashboardModel) projectStatusWidth(p *models.Project, dueSoonDays int) int {
	overdue := p.CountOverdueTasks()
	dueSoon := p.CountDueSoonTasks(dueSoonDays)
	blocked := p.CountBlockedTasks()
	blockerCount := storage.GetActiveBlockerCount(p)
	completedCount := p.CountCompletedTasks()

	if overdue == 0 && dueSoon == 0 && blocked == 0 && blockerCount == 0 && completedCount == 0 {
		return 0
	}

	w := 0
	addPart := func(emojiWidth, count int) {
		if count > 0 {
			if w > 0 {
				w++ // space separator
			}
			w += emojiWidth + len(fmt.Sprintf("%d", count))
		}
	}
	addPart(2, overdue)        // 🔥
	addPart(2, dueSoon)        // ⏰
	addPart(1, blocked)        // ◈ is 1 col wide
	addPart(2, blockerCount)   // 🚧
	addPart(2, completedCount) // ✅
	return w
}

// renderGlobalAlerts aggregates alert counts across all active projects.
func (m *DashboardModel) renderGlobalAlerts(_ int) string {
	var totalOverdue, totalDueToday, totalBlocked, totalBlockers, totalCompletedThisWeek int
	var inboxProjects []string

	now := time.Now()
	today := util.StartOfDay(now)
	sevenDaysAgo := now.AddDate(0, 0, -7)

	var activeProjects []*models.Project
	if !m.showArchived {
		activeProjects = m.projects
	} else {
		activeProjects = m.config.GetActiveProjects()
		for _, p := range activeProjects {
			if err := storage.LoadProject(p); err != nil {
				logging.Warn("failed to load project %s: %v", p.Name, err)
			}
		}
	}
	for _, p := range activeProjects {
		overdue := p.CountOverdueTasks()
		totalOverdue += overdue

		blocked := p.CountBlockedTasks()
		totalBlocked += blocked

		totalBlockers += storage.GetActiveBlockerCount(p)

		for _, task := range p.Tasks {
			if task == nil {
				continue
			}
			if task.DueDate != nil && !task.Status.IsComplete() {
				dueDay := util.StartOfDay(*task.DueDate)
				if dueDay.Equal(today) {
					totalDueToday++
				}
			}
			if task.CompletedAt != nil && task.CompletedAt.After(sevenDaysAgo) {
				totalCompletedThisWeek++
			}
		}

		inboxCount := 0
		for _, task := range p.Tasks {
			if task == nil {
				continue
			}
			if task.Section == models.SectionInbox && !task.Status.IsComplete() {
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

	// Global notes count
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

	// Type badge + status badge + spaces = ~5 visible columns
	titleWidth := max(contentWidth-5, MinTitleTruncWidth)

	var lines []string
	for _, task := range recentTasks {
		timeStr := storage.FormatRelativeTime(task.UpdatedAt)
		stageBadge := GetStageBadge(task.Stage)
		stageSegment := ""
		if stageBadge != "" {
			stageSegment = stageBadge + " "
		}
		// Check if title + 2-space gap + time fits on one line
		fitsOnOneLine := lipgloss.Width(task.Title)+2+lipgloss.Width(timeStr) <= titleWidth

		if fitsOnOneLine {
			gap := max(titleWidth-lipgloss.Width(task.Title)-lipgloss.Width(timeStr), 2)
			line1 := fmt.Sprintf("%s %s%s %s%s%s",
				GetTypeBadge(string(task.Type), task.Type.Icon()),
				stageSegment,
				GetStatusBadge(string(task.Status), task.Status.Icon()),
				task.Title,
				strings.Repeat(" ", gap),
				MutedStyle.Render(timeStr))
			lines = append(lines, line1, "")
		} else {
			line1 := fmt.Sprintf("%s %s%s %s",
				GetTypeBadge(string(task.Type), task.Type.Icon()),
				stageSegment,
				GetStatusBadge(string(task.Status), task.Status.Icon()),
				truncateAndPad(task.Title, titleWidth))

			padding := max(contentWidth-lipgloss.Width(timeStr), 0)
			line2 := MutedStyle.Render(strings.Repeat(" ", padding) + timeStr)
			lines = append(lines, line1, line2)
		}
	}
	return strings.Join(lines, "\n")
}
