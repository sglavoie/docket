package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

type focusBoardTodayStats struct {
	completedToday int
	plannedTotal   int
}

type focusBoardRowSegments struct {
	queueIndicator  string
	indent          string
	prefix          string
	statusIcon      string
	prioritySegment string
	projectBadge    string
	typeIcon        string
	title           string
	dueStr          string
}

func (m *FocusBoardModel) todayStats() focusBoardTodayStats {
	stats := focusBoardTodayStats{}
	for _, row := range m.todayRows {
		if row.TreePrefix != "" {
			continue
		}
		stats.plannedTotal++
		if row.Task.Status.IsComplete() {
			stats.completedToday++
		}
	}
	return stats
}

// renderRow renders a single board row line.
func (m *FocusBoardModel) renderRow(row boardRow, isSelected bool, maxTitleLen int, rowNum string, isFirstPlanItem bool, colLayout *ColumnLayout) string {
	segments := m.buildBoardRowSegments(row, isSelected, maxTitleLen, rowNum, isFirstPlanItem, colLayout)
	content := m.renderBoardRowContent(row, segments)
	line := m.renderBoardSelectableLine(row, content, isSelected, isFirstPlanItem)
	if hint := m.renderBoardEscalationHint(row.Task); hint != "" {
		return line + "\n" + hint
	}
	return line
}

func (m *FocusBoardModel) buildBoardRowSegments(row boardRow, isSelected bool, maxTitleLen int, rowNum string, isFirstPlanItem bool, colLayout *ColumnLayout) focusBoardRowSegments {
	task := row.Task
	segments := focusBoardRowSegments{
		queueIndicator:  m.renderBoardQueueIndicator(task.ID),
		indent:          MutedStyle.Render(row.TreePrefix),
		prefix:          renderBoardRowPrefix(rowNum, isFirstPlanItem),
		statusIcon:      renderBoardStatusIcon(task, colLayout),
		prioritySegment: renderBoardPrioritySegment(task, colLayout),
		projectBadge:    MutedStyle.Render(fmt.Sprintf("[%s]", truncateToVisualWidth(row.Project.Name, 10))),
		typeIcon:        renderBoardTypeIcon(task, colLayout),
		dueStr:          renderBoardDueString(task),
	}

	titleLen := maxTitleLen - len(row.TreePrefix)
	segments.title = m.renderBoardTitle(task, isSelected, titleLen)
	m.applyBoardHeroDemotion(task, row.Project, &segments)
	return segments
}

func (m *FocusBoardModel) renderBoardRowContent(row boardRow, segments focusBoardRowSegments) string {
	content := fmt.Sprintf("%s%s%s%s %s%s%s %s",
		segments.queueIndicator,
		segments.indent,
		segments.prefix,
		segments.statusIcon,
		segments.prioritySegment,
		segments.projectBadge,
		segments.typeIcon,
		segments.title,
	)

	trailing := m.renderBoardTrailing(row, segments.dueStr)
	if trailing != "" {
		content += " " + trailing
	}
	if m.tabs.Active == boardTabToday && m.carriedOverIDs[row.Task.ID] {
		content += " " + MutedStyle.Render("(carried)")
	}
	return content
}

func (m *FocusBoardModel) renderBoardSelectableLine(row boardRow, content string, isSelected bool, isFirstPlanItem bool) string {
	task := row.Task
	isClosed := task.Status.IsComplete()
	dimmed := task.Dimmed || (isClosed && m.tabs.Active == boardTabToday) || (m.focusMode == focusModeTunnel && !isSelected)
	isOverdue := task.IsOverdue()
	isBlocked := task.Status == models.StatusBlocked || row.Project.HasIncompleteDependencies(task)

	line := RenderSelectableRow(isSelected, task.Active, task.Pinned, m.cursorFlash, isOverdue, isBlocked, content, m.width-2, dimmed)
	if isSelected && m.mutationFlash {
		line = MutationFlashLine(line, m.width-2, m.mutationFlashBg)
	}
	if isFirstPlanItem && !isSelected && !dimmed && !isBlocked && !isOverdue {
		line = FirstPlanItemStyle.Width(m.width - 2).Render(strings.TrimRight(line, " "))
	}
	return line
}

func (m *FocusBoardModel) renderBoardQueueIndicator(taskID string) string {
	switch {
	case m.dailyPlanIDs[taskID]:
		return ActiveStyle.Render("w") + " "
	case m.queueIDs[taskID]:
		return ActiveStyle.Render("q") + " "
	default:
		return "  "
	}
}

func renderBoardRowPrefix(rowNum string, isFirstPlanItem bool) string {
	if rowNum == "" {
		return ""
	}
	if isFirstPlanItem {
		return ActiveStyle.Render(IconScrollRight) + fmt.Sprintf("%-2s ", rowNum)
	}
	return fmt.Sprintf("%3s ", rowNum)
}

func renderBoardStatusIcon(task *models.Task, colLayout *ColumnLayout) string {
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	if colLayout != nil && colLayout.HasStatus {
		return PadBadge(statusIcon, colLayout.StatusWidth)
	}
	return statusIcon
}

func renderBoardTypeIcon(task *models.Task, colLayout *ColumnLayout) string {
	typeIcon := GetTypeStyle(string(task.Type)).Render(task.Type.Icon())
	if colLayout != nil && colLayout.HasType {
		typeIcon = PadBadge(typeIcon, colLayout.TypeWidth)
	}
	return " " + typeIcon
}

func renderBoardPrioritySegment(task *models.Task, colLayout *ColumnLayout) string {
	priorityBadge := renderPriorityBadge(task.Priority)
	if task.Priority > 2 {
		priorityBadge = ""
	}
	if colLayout != nil && colLayout.HasPriority {
		return PadBadge(priorityBadge, colLayout.PriorityWidth) + " "
	}
	if priorityBadge != "" {
		return priorityBadge + " "
	}
	return ""
}

func renderBoardDueString(task *models.Task) string {
	if task.DueDate == nil {
		return ""
	}
	return FormatRelativeDue(task.DueDate, true)
}

func (m *FocusBoardModel) renderBoardTitle(task *models.Task, isSelected bool, maxTitleLen int) string {
	truncatedTitle := truncateToVisualWidth(task.Title, maxTitleLen)
	title := m.highlightTitle(truncatedTitle, task.Title)
	if isSelected && !task.Dimmed {
		return SelectedTaskTitleStyle.Render(title)
	}
	return GetTitlePriorityStyle(task.Priority).Render(title)
}

func (m *FocusBoardModel) applyBoardHeroDemotion(task *models.Task, project *models.Project, segments *focusBoardRowSegments) {
	hero := pickHeroAttribute(task, project)
	if hero == HeroNone || task.Dimmed {
		return
	}
	if !statusIsHero(hero) {
		segments.statusIcon = demoteBadge(segments.statusIcon)
	}
	segments.typeIcon = " " + demoteBadge(strings.TrimPrefix(segments.typeIcon, " "))
	if !priorityIsHero(hero) && segments.prioritySegment != "" {
		if trimmed := strings.TrimRight(segments.prioritySegment, " "); trimmed != "" {
			segments.prioritySegment = demoteBadge(trimmed) + " "
		}
	}
	if segments.dueStr != "" && !dueIsHero(hero) {
		segments.dueStr = demoteBadge(segments.dueStr)
	}
}

func (m *FocusBoardModel) renderBoardTrailing(row boardRow, dueStr string) string {
	task := row.Task
	estimateStr := ""
	if task.EstimatedDays > 0 {
		estimateStr = MutedStyle.Render("(" + formatEstimateDays(task.EstimatedDays) + ")")
	}
	descBadge := ""
	if m.config.IsDescriptionColumnVisible() {
		descBadge = renderDescriptionIndicator(task)
	}
	contentGroup := JoinGroupParts(descBadge, renderTruncatedTags(task.Tags, 2, 8), renderStaleIndicator(m.config, task))
	return JoinIndicatorGroups(
		renderChecklistBar(task, m.width),
		dueStr,
		contentGroup,
		estimateStr,
	)
}

func (m *FocusBoardModel) renderBoardEscalationHint(task *models.Task) string {
	if m.tabs.Active != boardTabToday || task.Priority < 3 || task.DueDate == nil {
		return ""
	}
	escalationDays := m.config.GetPriorityEscalationDays()
	if escalationDays <= 0 || !task.DueDate.Before(time.Now().AddDate(0, 0, escalationDays+1)) {
		return ""
	}
	return "      " + InfoStatusStyle.Render("↑ Due soon — consider P2")
}

// formatEstimateDays formats an estimated-days value as a human-readable duration.
// Values under 1 day are shown in hours (8h/day); 1+ days shown as days.
func formatEstimateDays(days float64) string {
	if days < 1 {
		hours := days * 8
		if hours == float64(int(hours)) {
			return fmt.Sprintf("~%dh", int(hours))
		}
		return fmt.Sprintf("~%.1fh", hours)
	}
	if days == float64(int(days)) {
		return fmt.Sprintf("~%dd", int(days))
	}
	return fmt.Sprintf("~%.1fd", days)
}

// sumEstimatesForRows returns the total estimated days across all board rows.
func sumEstimatesForRows(rows []boardRow) float64 {
	var total float64
	for _, row := range rows {
		if row.Task != nil {
			total += row.Task.EstimatedDays
		}
	}
	return total
}

// highlightTitle returns the truncated title with search match positions highlighted.
// If there's no active search query, returns the plain title.
func (m *FocusBoardModel) highlightTitle(truncatedTitle, fullTitle string) string {
	if m.searchQuery == "" {
		return truncatedTitle
	}
	query := strings.TrimSpace(m.searchQuery)
	_, positions := fuzzy.ScoreTermsWithPositions(fullTitle, query)
	if len(positions) == 0 {
		return truncatedTitle
	}
	titleRunes := []rune(truncatedTitle)
	var validPositions []int
	for _, pos := range positions {
		if pos < len(titleRunes) {
			validPositions = append(validPositions, pos)
		}
	}
	return HighlightText(truncatedTitle, validPositions, NormalStyle)
}

// renderZenMode renders the deepest focus mode: a single centered card with title,
// description, and checklist. The status bar is suppressed by app_view.go (IsZenMode check).
func (m *FocusBoardModel) renderZenMode() tea.View {
	var b strings.Builder

	// Header (no tab bar)
	headerTitle := "Zen Focus"
	if row := m.getRowAtCursor(); row != nil && row.Task != nil {
		headerTitle = "Currently: " + row.Task.Title
	}
	b.WriteString(renderFullscreenTitle(headerTitle, "", m.AccentColor()))
	b.WriteString("\n")

	row := m.getRowAtCursor()
	if row == nil || row.Task == nil {
		b.WriteString(MutedStyle.Render("  No task selected"))
		return tea.NewView(b.String())
	}

	task := row.Task

	// Build card body
	var body strings.Builder
	body.WriteString(renderPriorityBadge(task.Priority))
	body.WriteString("  ")
	body.WriteString(GetStatusBadge(string(task.Status), task.Status.Icon()))
	body.WriteString("  ")
	body.WriteString(GetTypeStyle(string(task.Type)).Render(task.Type.Icon()))
	body.WriteString("\n")

	if task.Description != "" {
		body.WriteString("\n")
		for line := range strings.SplitSeq(strings.TrimSpace(task.Description), "\n") {
			body.WriteString(MutedStyle.Render(strings.TrimSpace(line)))
			body.WriteString("\n")
		}
	}

	if len(task.Checklist) > 0 {
		body.WriteString("\n")
		for _, item := range task.Checklist {
			check := icons.Get("checkbox.unchecked")
			if item.Done {
				check = icons.Get("checkbox.checked")
			}
			body.WriteString(MutedStyle.Render(fmt.Sprintf("  %s %s", check, item.Text)))
			body.WriteString("\n")
		}
	}

	cardWidth := max(min(m.width-8, 80), 20)
	cardCfg := CardConfig{
		Style:       SidebarCardStyle,
		TitleStyle:  TitleStyle,
		AccentColor: m.AccentColor(),
	}
	card := renderCard(task.Title, strings.TrimRight(body.String(), "\n"), cardWidth, cardCfg)

	// Center vertically
	cardLines := strings.Count(card, "\n") + 1
	availHeight := fullscreenContentHeight(m.height, 2) // 2: header + footer hint
	topPad := max((availHeight-cardLines)/2, 1)

	// Center horizontally
	leftPad := max((m.width-cardWidth)/2, 0)
	leftStr := strings.Repeat(" ", leftPad)

	for i := 0; i < topPad; i++ {
		b.WriteString("\n")
	}
	for line := range strings.SplitSeq(card, "\n") {
		b.WriteString(leftStr + line + "\n")
	}

	// Footer hint
	remaining := 0
	for _, row := range m.filtered {
		if row.Task != nil && !row.Task.Status.IsComplete() {
			remaining++
		}
	}
	hint := HelpStyle.Render(fmt.Sprintf("  x done · X dismiss · E edit · enter open · j/k nav · Esc exit  (%d remaining)", remaining))
	b.WriteString("\n")
	b.WriteString(hint)

	return tea.NewView(b.String())
}

// computedTabNames returns the tab name slice with a dynamic "Today (done/total)" label.
func (m *FocusBoardModel) computedTabNames() []string {
	names := make([]string, len(focusBoardTabNames))
	copy(names, focusBoardTabNames)

	stats := m.todayStats()
	if stats.plannedTotal > 0 {
		pct := float64(stats.completedToday) / float64(stats.plannedTotal) * 100
		fullStyle := lipgloss.NewStyle().Foreground(ColorAccent)
		if pct >= 80 {
			fullStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
		}
		filled := int(float64(8) * pct / 100)
		var bar strings.Builder
		for range filled {
			bar.WriteString(fullStyle.Render("█"))
		}
		for range 8 - filled {
			bar.WriteString(ProgressEmptyStyle.Render("░"))
		}
		names[boardTabToday] = fmt.Sprintf("Today (%d/%d) %s", stats.completedToday, stats.plannedTotal, bar.String())
	} else {
		names[boardTabToday] = fmt.Sprintf("Today (%d/%d)", stats.completedToday, stats.plannedTotal)
	}
	return names
}
