package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// dashboardProjectIconGutterWidth is the number of spaces reserved after a
// project status icon so unicode glyphs like project.wip (⋙) do not read too
// tightly against the project title.
const dashboardProjectIconGutterWidth = 2

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
	showContext := m.filterContext == ""

	maxNameWidth, maxContextWidth, maxStatusWidth, iconColWidth := m.projectListColumnWidths(visible, showContext, dueSoonDays)
	maxVisible := m.projectListMaxVisible(chromeHeight, bottomHeight, narrowMode)
	rowWidth := cardWidth - 4
	if len(visible) > maxVisible {
		rowWidth--
	}

	if !narrowMode {
		maxNameWidth, maxContextWidth = clampProjectListColumnWidths(rowWidth, iconColWidth, maxNameWidth, maxContextWidth, maxStatusWidth, showContext)
	}

	startIdx, endIdx := persistentIndexWindow(m.cursor, len(visible), maxVisible, &m.viewScrollOffset)
	if !narrowMode {
		cardContent.WriteString(m.renderProjectListHeader(rowWidth, iconColWidth, maxNameWidth, maxContextWidth, showContext))
		cardContent.WriteString("\n")
	}

	pinnedNum := countPinnedProjects(visible[:startIdx])

	var rowBuilder strings.Builder
	for i := startIdx; i < endIdx; i++ {
		rowBuilder.WriteString(m.renderProjectRow(visible[i], i == m.cursor, narrowMode, rowWidth, maxNameWidth, maxContextWidth, showContext, iconColWidth, dueSoonDays, &pinnedNum))
		if i < endIdx-1 {
			rowBuilder.WriteString("\n")
		}
	}

	scrollbar := RenderScrollbar(len(visible), maxVisible, startIdx)
	cardContent.WriteString(attachScrollbarAtWidth(rowBuilder.String(), scrollbar, rowWidth))

	return cardContent.String()
}

func (m *DashboardModel) projectListColumnWidths(visible []*models.Project, showContext bool, dueSoonDays int) (int, int, int, int) {
	maxNameWidth := 0
	maxContextWidth := 0
	maxStatusWidth := 0
	iconColWidth := 0
	for _, p := range visible {
		if icon := p.StatusIconEmoji(); icon != "" {
			iconColWidth = max(iconColWidth, lipgloss.Width(icon)+dashboardProjectIconGutterWidth)
		}
		if width := lipgloss.Width(p.Name); width > maxNameWidth {
			maxNameWidth = width
		}
		if showContext && p.Context != "" {
			if width := lipgloss.Width(fmt.Sprintf("[%s]", p.Context)); width > maxContextWidth {
				maxContextWidth = width
			}
		}
		if statusWidth := m.projectStatusWidth(p, dueSoonDays); statusWidth > maxStatusWidth {
			maxStatusWidth = statusWidth
		}
	}

	return maxNameWidth, maxContextWidth, maxStatusWidth, iconColWidth
}

func (m *DashboardModel) projectListMaxVisible(chromeHeight, bottomHeight int, narrowMode bool) int {
	cardOverhead := CardVerticalBorderOverhead
	headerLines := 1
	maxVisible := m.height - chromeHeight - bottomHeight - cardOverhead - 1 - headerLines
	if narrowMode {
		availRows := m.height - chromeHeight - bottomHeight - cardOverhead - 1 - headerLines
		maxVisible = (availRows + 1) / 2
	}
	if maxVisible < DashboardMinVisibleProjects {
		return DashboardMinVisibleProjects
	}
	return maxVisible
}

func clampProjectListColumnWidths(rowWidth, iconColWidth, maxNameWidth, maxContextWidth, maxStatusWidth int, showContext bool) (int, int) {
	fixedCols := 29 + iconColWidth // +9 for progress bar column (2-space gap + 7-char bar)
	if maxStatusWidth > 0 {
		fixedCols += 2 + maxStatusWidth
	}
	availForNameCtx := max(rowWidth-fixedCols, 10)
	if showContext && maxContextWidth > 0 {
		if maxNameWidth+maxContextWidth > availForNameCtx {
			maxContextWidth = availForNameCtx - maxNameWidth
			if maxContextWidth < 0 {
				maxContextWidth = 0
				maxNameWidth = availForNameCtx
			}
		}
		return maxNameWidth, maxContextWidth
	}
	if maxNameWidth > availForNameCtx {
		maxNameWidth = availForNameCtx
	}
	return maxNameWidth, maxContextWidth
}

func (m *DashboardModel) renderProjectListHeader(rowWidth, iconColWidth, maxNameWidth, maxContextWidth int, showContext bool) string {
	prefix := strings.Repeat(" ", 5+iconColWidth)
	namePad := strings.Repeat(" ", maxNameWidth)
	ctxPad := ""
	if showContext && maxContextWidth > 0 {
		ctxPad = strings.Repeat(" ", maxContextWidth+1)
	}
	header := fmt.Sprintf("%s%s %s %6s %6s  %s", prefix, namePad, ctxPad, "Active", "Total", "       ")
	withStatus := header + "  Status"
	if lipgloss.Width(withStatus) <= rowWidth {
		header = withStatus
	}
	return MutedStyle.Render(header)
}

func countPinnedProjects(projects []*models.Project) int {
	pinnedNum := 0
	for _, project := range projects {
		if project.Pinned {
			pinnedNum++
		}
	}
	return pinnedNum
}

func (m *DashboardModel) renderProjectRow(p *models.Project, selected, narrowMode bool, rowWidth, maxNameWidth, maxContextWidth int, showContext bool, iconColWidth, dueSoonDays int, pinnedNum *int) string {
	style := NormalStyle
	numberPrefix := "  "
	if p.Pinned {
		*pinnedNum = *pinnedNum + 1
		if *pinnedNum <= 9 {
			numberPrefix = PinnedNumberStyle.Render(fmt.Sprintf("%d", *pinnedNum)) + " "
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

	totalAll := p.CountTasks()
	completedCount := p.CountCompletedTasks()
	total := totalAll - completedCount
	active := p.CountActiveSectionTasks()
	statusIndicator := m.projectRowStatusIndicator(p, dueSoonDays)
	scoreCoverageBadge := m.projectRowScoreCoverageBadge(p)
	noteCount := m.projectRowNoteCount(p)

	// Mini progress bar: [█████] = 7 chars
	var bar string
	if totalAll == 0 {
		bar = MutedStyle.Render("[  —  ]")
	} else {
		percent := float64(completedCount) / float64(totalAll) * 100
		bar = "[" + RenderProgressBar(percent, 5) + "]"
	}

	nameRendered := m.renderProjectRowName(p, style, maxNameWidth, iconColWidth)
	contextRendered := m.renderProjectRowContext(p, maxContextWidth, showContext)
	maxContentWidth := rowWidth - 3

	if narrowMode {
		line1Content := fmt.Sprintf("%s%s %s", numberPrefix, nameRendered, contextRendered)
		line1 := RenderSelectableRow(selected, false, false, m.cursorFlash, false, false, line1Content, rowWidth)

		statsLine := fmt.Sprintf("      %s  %s", MutedStyle.Render(fmt.Sprintf("Active: %d  Total: %d", active, total)), bar)
		statsLine = appendProjectRowBadge(statsLine, rowWidth, noteCount, scoreCoverageBadge, statusIndicator)
		return line1 + "\n" + statsLine
	}

	activeStr := MutedStyle.Render(fmt.Sprintf("%6d", active))
	totalStr := MutedStyle.Render(fmt.Sprintf("%6d", total))
	content := fmt.Sprintf("%s%s %s %s %s  %s", numberPrefix, nameRendered, contextRendered, activeStr, totalStr, bar)
	content = appendProjectRowBadge(content, maxContentWidth, noteCount, scoreCoverageBadge, statusIndicator)
	return RenderSelectableRow(selected, false, false, m.cursorFlash, false, false, content, rowWidth)
}

func (m *DashboardModel) renderProjectRowName(p *models.Project, style lipgloss.Style, maxNameWidth, iconColWidth int) string {
	iconRendered := ""
	if iconColWidth > 0 {
		if emoji := p.StatusIconEmoji(); emoji != "" {
			iconRendered = PadBadge(emoji+strings.Repeat(" ", dashboardProjectIconGutterWidth), iconColWidth)
		} else {
			iconRendered = strings.Repeat(" ", iconColWidth)
		}
	}

	displayName := truncateAndPad(p.Name, maxNameWidth)
	if m.SearchQuery() != "" {
		return iconRendered + HighlightSubstring(displayName, m.SearchQuery(), style)
	}
	return iconRendered + style.Render(displayName)
}

func (m *DashboardModel) renderProjectRowContext(p *models.Project, maxContextWidth int, showContext bool) string {
	if !showContext || maxContextWidth <= 0 {
		return ""
	}
	if p.Context == "" {
		return strings.Repeat(" ", maxContextWidth)
	}

	ctxStr := truncateAndPad(fmt.Sprintf("[%s]", p.Context), maxContextWidth)
	if m.SearchQuery() != "" {
		return HighlightSubstring(ctxStr, m.SearchQuery(), MutedStyle)
	}
	return MutedStyle.Render(ctxStr)
}

func (m *DashboardModel) projectRowNoteCount(p *models.Project) int {
	projectNotes, _ := storage.ListNotes(p)
	return len(projectNotes)
}

func (m *DashboardModel) projectRowStatusIndicator(p *models.Project, dueSoonDays int) string {
	inbox := countProjectInboxTasks(p)
	overdue := p.CountOverdueTasks()
	dueSoon := p.CountDueSoonTasks(dueSoonDays)
	blocked := p.CountBlockedTasks()
	blockerCount := storage.GetActiveBlockerCount(p)
	completedCount := p.CountCompletedTasks()

	if inbox == 0 && overdue == 0 && dueSoon == 0 && blocked == 0 && blockerCount == 0 && completedCount == 0 {
		return ""
	}

	var parts []string
	if inbox > 0 {
		parts = append(parts, DueSoonStyle.Render(fmt.Sprintf("%s%d", icons.Get("inbox"), inbox)))
	}
	if overdue > 0 {
		parts = append(parts, OverdueStyle.Render(fmt.Sprintf("%s%d", icons.Get("overdue"), overdue)))
	}
	if dueSoon > 0 {
		parts = append(parts, DueSoonStyle.Render(fmt.Sprintf("%s%d", icons.Get("due_today"), dueSoon)))
	}
	if blocked > 0 {
		parts = append(parts, MutedStyle.Render(fmt.Sprintf("%s%d", icons.Get("blocked_dot"), blocked)))
	}
	if blockerCount > 0 {
		parts = append(parts, DueSoonStyle.Render(fmt.Sprintf("%s%d", icons.Get("blockers"), blockerCount)))
	}
	if completedCount > 0 {
		parts = append(parts, MutedStyle.Render(fmt.Sprintf("%s%d", IconChecklist(), completedCount)))
	}
	return strings.Join(parts, " ")
}

// countProjectInboxTasks counts processable root inbox tasks for a project.
// Matches the predicate used by the inbox processor and dashboard alerts.
func countProjectInboxTasks(p *models.Project) int {
	count := 0
	for _, task := range p.Tasks {
		if isProcessableInboxTask(task) {
			count++
		}
	}
	return count
}

func (m *DashboardModel) projectRowScoreCoverageBadge(p *models.Project) string {
	profile := m.config.GetProjectScoringProfile(p.ID)
	if profile == nil {
		return ""
	}

	scoredCount := 0
	totalScorable := 0
	for _, task := range p.Tasks {
		if task == nil || task.IsComplete() {
			continue
		}
		totalScorable++
		if HasManualScores(task, profile) {
			scoredCount++
		}
	}
	if totalScorable == 0 {
		return ""
	}

	pct := scoredCount * 100 / totalScorable
	return MutedStyle.Render(fmt.Sprintf("S:%d%%", pct))
}

func appendProjectRowBadge(content string, maxWidth, noteCount int, scoreCoverageBadge, statusIndicator string) string {
	appendIfFits := func(current, badge string) string {
		candidate := current + "  " + badge
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
		return current
	}

	if noteCount > 0 {
		content = appendIfFits(content, MutedStyle.Render(fmt.Sprintf("%s%d", icons.Get("notes"), noteCount)))
	}
	if scoreCoverageBadge != "" {
		content = appendIfFits(content, scoreCoverageBadge)
	}
	if statusIndicator != "" {
		content = appendIfFits(content, statusIndicator)
	}
	return content
}

// projectStatusWidth returns the terminal column width of the status indicator
// for a project (emojis are 2 columns wide, digits are 1, spaces are 1).
func (m *DashboardModel) projectStatusWidth(p *models.Project, dueSoonDays int) int {
	inbox := countProjectInboxTasks(p)
	overdue := p.CountOverdueTasks()
	dueSoon := p.CountDueSoonTasks(dueSoonDays)
	blocked := p.CountBlockedTasks()
	blockerCount := storage.GetActiveBlockerCount(p)
	completedCount := p.CountCompletedTasks()

	if inbox == 0 && overdue == 0 && dueSoon == 0 && blocked == 0 && blockerCount == 0 && completedCount == 0 {
		return 0
	}

	width := 0
	addPart := func(emojiWidth, count int) {
		if count <= 0 {
			return
		}
		if width > 0 {
			width++
		}
		width += emojiWidth + len(fmt.Sprintf("%d", count))
	}

	addPart(2, inbox)
	addPart(2, overdue)
	addPart(2, dueSoon)
	addPart(2, blocked)
	addPart(2, blockerCount)
	addPart(2, completedCount)
	return width
}
