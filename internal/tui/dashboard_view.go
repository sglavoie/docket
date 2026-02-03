package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

func (m *DashboardModel) View() string {
	var b strings.Builder

	// Confirmation prompt
	if m.confirm.IsActive() {
		b.WriteString("\n")
		b.WriteString(m.confirm.Render())
		b.WriteString("\n\n")
	}

	filterBar, chromeHeight := m.renderFilterBar()
	b.WriteString(filterBar)

	visible := m.visibleProjects()

	// Determine if we use split-pane layout
	useSplitPane := m.width >= DashboardSplitPaneMinWidth

	if useSplitPane {
		b.WriteString(m.renderSplitPane(visible, chromeHeight))
	} else {
		b.WriteString(m.renderFullWidthDashboard(visible, chromeHeight))
	}

	return b.String()
}

// renderSplitPane renders the dashboard as a left (project list) + right (info panel) layout.
// When recent tasks are hidden, it switches to a stacked layout (projects on top, alerts below).
func (m *DashboardModel) renderSplitPane(visible []*models.Project, chromeHeight int) string {
	if !m.config.GetShowRecentTasks() {
		return m.renderSplitPaneStacked(visible, chromeHeight)
	}

	leftWidth := m.width * 55 / 100
	rightWidth := m.width - leftWidth - 1 // 1 char divider

	cardWidth := leftWidth - 1
	if cardWidth < MinCardWidth {
		cardWidth = MinCardWidth
	}

	// Build project card
	cardTitle := m.projectCardTitle(visible)
	cardContent := m.renderProjectList(visible, chromeHeight, cardWidth, 0)

	cfg := DashboardCardConfig()
	leftContent := renderCard(cardTitle, cardContent, cardWidth, cfg)

	// Get selected project for right panel
	var selected *models.Project
	if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
		selected = visible[m.cursor]
	}

	rightContent := m.renderRightPanel(rightWidth, m.height-chromeHeight, selected)

	// Render vertical divider
	dividerHeight := m.height - chromeHeight
	if dividerHeight < 1 {
		dividerHeight = 1
	}
	divider := renderDashboardVerticalDivider(dividerHeight)

	// Join horizontally
	leftStyled := lipgloss.NewStyle().Width(leftWidth).Render(leftContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, divider, rightContent)
}

// renderSplitPaneStacked renders projects on top and alerts below, using full width.
// Used in split-pane mode when recent tasks are hidden.
func (m *DashboardModel) renderSplitPaneStacked(visible []*models.Project, chromeHeight int) string {
	var b strings.Builder

	cardWidth := m.width - 2
	if cardWidth < MinCardWidth {
		cardWidth = MinCardWidth
	}

	// Reserve space for the alerts card below
	alertsHeight := AlertsCardHeight
	cfg := DashboardCardConfig()

	cardTitle := m.projectCardTitle(visible)
	cardContent := m.renderProjectList(visible, chromeHeight, cardWidth, alertsHeight)
	b.WriteString(renderCard(cardTitle, cardContent, cardWidth, cfg))
	b.WriteString("\n")

	// Alerts card
	alertsContent := m.renderGlobalAlerts(cardWidth - CardBorderOverhead)
	if alertsContent != "" {
		b.WriteString(renderCard("Alerts", alertsContent, cardWidth, cfg))
	}

	return b.String()
}

// renderFullWidthDashboard renders the original full-width dashboard layout.
func (m *DashboardModel) renderFullWidthDashboard(visible []*models.Project, chromeHeight int) string {
	var b strings.Builder

	cardWidth := m.width - 2
	if cardWidth < MinCardWidth {
		cardWidth = MinCardWidth
	}

	// Calculate space for bottom section (Recent Tasks + Stats cards)
	bottomHeight := 0
	showRecentTasks := m.config.GetShowRecentTasks()
	if showRecentTasks && m.SearchQuery() == "" && m.config.CurrentProject != "" {
		current := m.config.GetCurrentProject()
		if current != nil && len(current.Tasks) > 0 {
			bottomHeight = DashboardBottomSectionHeight
		}
	}

	cardTitle := m.projectCardTitle(visible)
	cardContent := m.renderProjectList(visible, chromeHeight, cardWidth, bottomHeight)

	cfg := DashboardCardConfig()
	b.WriteString(renderCard(cardTitle, cardContent, cardWidth, cfg))
	b.WriteString("\n")

	// Current project details (only show when not searching and toggle is on)
	if showRecentTasks && m.SearchQuery() == "" && m.config.CurrentProject != "" {
		current := m.config.GetCurrentProject()
		if current != nil && len(current.Tasks) > 0 {
			b.WriteString("\n")

			recentContent := m.renderRecentTasksContentWithWidth(current, cardWidth-CardBorderOverhead)
			b.WriteString(renderCard("Recent", recentContent, cardWidth, cfg))
			b.WriteString("\n")

			statsContent := m.renderStatsContent(current)
			if statsContent != "" {
				b.WriteString(renderCard("Stats", statsContent, cardWidth, cfg))
			}
		}
	}

	return b.String()
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
func (m *DashboardModel) renderProjectList(visible []*models.Project, chromeHeight, cardWidth, bottomHeight int) string {
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
	for _, p := range visible {
		nameStr := p.Name
		if p.ID == m.config.CurrentProject {
			nameStr += " *"
		}
		if len(nameStr) > maxNameWidth {
			maxNameWidth = len(nameStr)
		}
		if showContext && p.Context != "" {
			ctxStr := fmt.Sprintf("[%s]", p.Context)
			if len(ctxStr) > maxContextWidth {
				maxContextWidth = len(ctxStr)
			}
		}
		statusW := m.projectStatusWidth(p, dueSoonDays)
		if statusW > maxStatusWidth {
			maxStatusWidth = statusW
		}
	}

	// Clamp column widths to fit available row width (wide mode only).
	rowWidth := cardWidth - 4
	if !narrowMode {
		fixedCols := 20
		if maxStatusWidth > 0 {
			fixedCols += 2 + maxStatusWidth
		}
		availForNameCtx := rowWidth - fixedCols
		if availForNameCtx < 10 {
			availForNameCtx = 10
		}
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

	// Calculate max visible projects
	cardOverhead := 2
	headerLines := 1
	maxVisible := m.height - chromeHeight - bottomHeight - cardOverhead - 1 - headerLines
	if narrowMode {
		maxVisible = (m.height - chromeHeight - bottomHeight - cardOverhead - 1 - headerLines) / 2
	}
	if maxVisible < DashboardMinVisibleProjects {
		maxVisible = 3
	}

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}
	if scrollOffset > len(visible)-maxVisible {
		scrollOffset = len(visible) - maxVisible
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Column header row
	if !narrowMode {
		prefix := strings.Repeat(" ", 5)
		namePad := strings.Repeat(" ", maxNameWidth)
		ctxPad := ""
		if showContext && maxContextWidth > 0 {
			ctxPad = strings.Repeat(" ", maxContextWidth+1)
		}
		header := fmt.Sprintf("%s%s %s %6s %6s", prefix, namePad, ctxPad, "Active", "Total")
		withStatus := header + "  Status"
		if len(withStatus) <= rowWidth {
			header = withStatus
		}
		cardContent.WriteString(MutedStyle.Render(header))
		cardContent.WriteString("\n")
	}

	// Scroll-up indicator inside card
	if scrollOffset > 0 {
		cardContent.WriteString(MutedStyle.Render(fmt.Sprintf("↑ %d more above", scrollOffset)))
		cardContent.WriteString("\n")
	}

	// Project list (windowed)
	end := scrollOffset + maxVisible
	if end > len(visible) {
		end = len(visible)
	}

	pinnedNum := 0
	for i := 0; i < scrollOffset; i++ {
		if visible[i].Pinned {
			pinnedNum++
		}
	}

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
				style = BoldStyle.Foreground(ColorSuccess)
			} else {
				style = BoldStyle.Foreground(ColorPrimary)
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

		currentMarker := ""
		if p.ID == m.config.CurrentProject {
			currentMarker = " " + MutedStyle.Foreground(ColorSecondary).Render("*")
		}

		// Build the name string, padded/truncated to max width
		nameRaw := p.Name
		if p.ID == m.config.CurrentProject {
			nameRaw += " *"
		}
		displayName := p.Name
		if len(nameRaw) > maxNameWidth && maxNameWidth > 2 {
			truncLen := maxNameWidth
			if p.ID == m.config.CurrentProject {
				truncLen -= 2
			}
			if truncLen < 1 {
				truncLen = 1
			}
			if truncLen < len(p.Name) {
				displayName = p.Name[:truncLen-1] + "…"
				nameRaw = displayName
				if p.ID == m.config.CurrentProject {
					nameRaw += " *"
				}
			}
		}
		namePadding := maxNameWidth - len(nameRaw)
		if namePadding < 0 {
			namePadding = 0
		}
		nameRendered := style.Render(displayName) + currentMarker + strings.Repeat(" ", namePadding)

		// Context badge, padded/truncated
		contextRendered := ""
		if showContext && maxContextWidth > 0 {
			if p.Context != "" {
				ctxStr := fmt.Sprintf("[%s]", p.Context)
				if len(ctxStr) > maxContextWidth && maxContextWidth > 2 {
					ctxStr = ctxStr[:maxContextWidth-1] + "…"
				} else if len(ctxStr) > maxContextWidth {
					ctxStr = ctxStr[:maxContextWidth]
				}
				ctxPadding := maxContextWidth - len(ctxStr)
				if ctxPadding < 0 {
					ctxPadding = 0
				}
				contextRendered = MutedStyle.Render(ctxStr) + strings.Repeat(" ", ctxPadding)
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

		if narrowMode {
			line1Content := fmt.Sprintf("%s%s %s", numberPrefix, nameRendered, contextRendered)
			line1 := RenderSelectableRow(selected, false, line1Content, cardWidth-4)
			cardContent.WriteString(line1)
			cardContent.WriteString("\n")

			statsLine := fmt.Sprintf("      %s", MutedStyle.Render(fmt.Sprintf("Active: %d  Total: %d", active, total)))
			if noteCount > 0 {
				statsLine += "  " + MutedStyle.Render(fmt.Sprintf("📝%d", noteCount))
			}
			if statusIndicator != "" {
				statsLine += "  " + statusIndicator
			}
			cardContent.WriteString(statsLine)
		} else {
			activeStr := MutedStyle.Render(fmt.Sprintf("%6d", active))
			totalStr := MutedStyle.Render(fmt.Sprintf("%6d", total))

			content := fmt.Sprintf("%s%s %s %s %s", numberPrefix, nameRendered, contextRendered, activeStr, totalStr)
			if noteCount > 0 {
				content += "  " + MutedStyle.Render(fmt.Sprintf("📝%d", noteCount))
			}
			if statusIndicator != "" {
				content += "  " + statusIndicator
			}
			line := RenderSelectableRow(selected, false, content, cardWidth-4)
			cardContent.WriteString(line)
		}

		if i < end-1 || (len(visible)-end) > 0 {
			cardContent.WriteString("\n")
		}
	}

	// Scroll-down indicator inside card
	if remaining := len(visible) - end; remaining > 0 {
		cardContent.WriteString(MutedStyle.Render(fmt.Sprintf("↓ %d more below", remaining)))
	}

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

// renderRightPanel renders the right-side info panel with recent activity and global alerts.
func (m *DashboardModel) renderRightPanel(width, height int, selected *models.Project) string {
	cfg := DashboardCardConfig()
	cardWidth := width
	if cardWidth < MinCardWidth {
		cardWidth = MinCardWidth
	}

	// Recent Activity card
	var recentContent string
	if selected == nil {
		recentContent = MutedStyle.Render("No project selected.")
	} else if len(selected.Tasks) == 0 {
		recentContent = MutedStyle.Render("No tasks yet.")
	} else {
		recentContent = m.renderRecentTasksContentWithWidth(selected, cardWidth-CardBorderOverhead)
	}

	var recentTitle string
	if selected != nil {
		recentTitle = fmt.Sprintf("Recent — %s", util.Truncate(selected.Name, cardWidth-16))
	} else {
		recentTitle = "Recent Activity"
	}

	var b strings.Builder
	b.WriteString(renderCard(recentTitle, recentContent, cardWidth, cfg))
	b.WriteString("\n\n")

	// Alerts card
	alertsContent := m.renderGlobalAlerts(cardWidth)
	b.WriteString(renderCard("Alerts", alertsContent, cardWidth, cfg))

	return b.String()
}

// renderGlobalAlerts aggregates alert counts across all active projects.
func (m *DashboardModel) renderGlobalAlerts(width int) string {
	var totalOverdue, totalDueToday, totalBlocked, totalBlockers, totalCompletedThisWeek int
	var inboxProjects []string

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	sevenDaysAgo := now.AddDate(0, 0, -7)

	var activeProjects []*models.Project
	if !m.showArchived {
		activeProjects = m.projects
	} else {
		activeProjects = m.config.GetActiveProjects()
		for _, p := range activeProjects {
			_ = storage.LoadProject(p)
		}
	}
	for _, p := range activeProjects {
		overdue := p.CountOverdueTasks()
		totalOverdue += overdue

		blocked := p.CountBlockedTasks()
		totalBlocked += blocked

		totalBlockers += storage.GetActiveBlockerCount(p)

		for _, task := range p.Tasks {
			if task.DueDate != nil && !task.Status.IsComplete() {
				dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
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
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorSuccess).Render(fmt.Sprintf("✅ %d completed this week", totalCompletedThisWeek)))
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

// renderDashboardVerticalDivider renders a vertical divider for the dashboard split pane.
func renderDashboardVerticalDivider(height int) string {
	if height <= 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < height; i++ {
		b.WriteString("│")
		if i < height-1 {
			b.WriteString("\n")
		}
	}
	return MutedStyle.Render(b.String())
}

func (m *DashboardModel) renderRecentTasksContentWithWidth(project *models.Project, contentWidth int) string {
	recentTasks := storage.GetRecentTasks(project.Tasks, 5)
	if len(recentTasks) == 0 {
		return MutedStyle.Render("No tasks yet.")
	}

	// Type badge + status badge + spaces = ~5 visible columns
	titleWidth := contentWidth - 5
	if titleWidth < MinTitleTruncWidth {
		titleWidth = MinTitleTruncWidth
	}

	var lines []string
	for _, task := range recentTasks {
		timeStr := storage.FormatRelativeTime(task.UpdatedAt)
		// Check if title + 2-space gap + time fits on one line
		fitsOnOneLine := len(task.Title)+2+len(timeStr) <= titleWidth

		if fitsOnOneLine {
			gap := titleWidth - len(task.Title) - len(timeStr)
			if gap < 2 {
				gap = 2
			}
			line1 := fmt.Sprintf("%s %s %s%s%s",
				GetTypeBadge(string(task.Type), task.Type.Icon()),
				GetStatusBadge(string(task.Status), task.Status.Icon()),
				task.Title,
				strings.Repeat(" ", gap),
				MutedStyle.Render(timeStr))
			lines = append(lines, line1, "")
		} else {
			line1 := fmt.Sprintf("%s %s %s",
				GetTypeBadge(string(task.Type), task.Type.Icon()),
				GetStatusBadge(string(task.Status), task.Status.Icon()),
				util.Truncate(task.Title, titleWidth))

			padding := contentWidth - len(timeStr)
			if padding < 0 {
				padding = 0
			}
			line2 := MutedStyle.Render(strings.Repeat(" ", padding) + timeStr)
			lines = append(lines, line1, line2)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *DashboardModel) renderStatsContent(project *models.Project) string {
	if len(project.Tasks) == 0 {
		return ""
	}

	typeCounts := make(map[models.TaskType]int)
	statusCounts := make(map[models.TaskStatus]int)

	for _, task := range project.Tasks {
		typeCounts[task.Type]++
		statusCounts[task.Status]++
	}

	var typeStats []string
	for _, tt := range models.AllTaskTypes() {
		if count := typeCounts[tt]; count > 0 {
			style := GetTypeStyle(string(tt))
			typeStats = append(typeStats, style.Render(fmt.Sprintf("%s:%d", tt.Icon(), count)))
		}
	}

	var statusStats []string
	for _, s := range models.AllTaskStatuses() {
		if count := statusCounts[s]; count > 0 {
			style := GetStatusStyle(string(s))
			statusStats = append(statusStats, style.Render(fmt.Sprintf("%s:%d", s.Icon(), count)))
		}
	}

	var lines []string
	lines = append(lines, MutedStyle.Render("Types: ")+strings.Join(typeStats, " "))
	lines = append(lines, MutedStyle.Render("Status: ")+strings.Join(statusStats, " "))

	overdueCount := 0
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, task := range project.Tasks {
		if task.DueDate != nil && !task.Status.IsComplete() {
			dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
			if dueDay.Before(today) {
				overdueCount++
			}
		}
	}
	if overdueCount > 0 {
		lines = append(lines, OverdueStyle.Render(fmt.Sprintf("Overdue: %d", overdueCount)))
	}

	return strings.Join(lines, "\n")
}
