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
	var b strings.Builder

	task := row.Task
	projectName := truncateToVisualWidth(row.Project.Name, 10)

	priorityBadge := renderPriorityBadge(task.Priority)
	if task.Priority > 2 {
		priorityBadge = ""
	}
	isClosed := task.Status.IsComplete()

	// Tree prefix for auto-expanded children
	indent := MutedStyle.Render(row.TreePrefix)
	maxTitleLen -= len(row.TreePrefix)

	// Queue / daily-plan indicator: always 2 chars wide (first column).
	queueIndicator := "  "
	if m.dailyPlanIDs[task.ID] {
		queueIndicator = ActiveStyle.Render("w") + " "
	} else if m.queueIDs[task.ID] {
		queueIndicator = ActiveStyle.Render("q") + " "
	}

	// Row number prefix for Today tab, or plain indent.
	prefix := ""
	if rowNum != "" {
		if isFirstPlanItem {
			// ▶ in accent color + rowNum left-padded to match normal 4-char width
			prefix = ActiveStyle.Render(IconScrollRight) + fmt.Sprintf("%-2s ", rowNum)
		} else {
			prefix = fmt.Sprintf("%3s ", rowNum)
		}
	}

	projectBadge := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))
	typeIcon := " " + GetTypeStyle(string(task.Type)).Render(task.Type.Icon())

	dueStr := ""
	if task.DueDate != nil {
		dueStr = FormatRelativeDue(task.DueDate, true)
	}

	truncatedTitle := truncateToVisualWidth(task.Title, maxTitleLen)
	title := m.highlightTitle(truncatedTitle, task.Title)
	if isSelected && !task.Dimmed {
		title = SelectedTaskTitleStyle.Render(title)
	} else {
		title = GetTitlePriorityStyle(task.Priority).Render(title)
	}
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	if colLayout != nil && colLayout.HasStatus {
		statusIcon = PadBadge(statusIcon, colLayout.StatusWidth)
	}
	if colLayout != nil && colLayout.HasType {
		typeIcon = " " + PadBadge(GetTypeStyle(string(task.Type)).Render(task.Type.Icon()), colLayout.TypeWidth)
	}
	var prioritySegment string
	if colLayout != nil && colLayout.HasPriority {
		prioritySegment = PadBadge(priorityBadge, colLayout.PriorityWidth) + " "
	} else if priorityBadge != "" {
		prioritySegment = priorityBadge + " "
	}

	// Von Restorff: demote non-hero badges so the most urgent attribute stands out.
	hero := pickHeroAttribute(task, row.Project)
	if hero != HeroNone && !task.Dimmed {
		if !statusIsHero(hero) {
			statusIcon = demoteBadge(statusIcon)
		}
		typeIcon = " " + demoteBadge(strings.TrimPrefix(typeIcon, " "))
		if !priorityIsHero(hero) && prioritySegment != "" {
			if trimmed := strings.TrimRight(prioritySegment, " "); trimmed != "" {
				prioritySegment = demoteBadge(trimmed) + " "
			}
		}
		if dueStr != "" && !dueIsHero(hero) {
			dueStr = demoteBadge(dueStr)
		}
	}

	content := fmt.Sprintf("%s%s%s%s %s%s%s %s", queueIndicator, indent, prefix, statusIcon, prioritySegment, projectBadge, typeIcon, title)

	estimateStr := ""
	if task.EstimatedDays > 0 {
		estimateStr = MutedStyle.Render("(" + formatEstimateDays(task.EstimatedDays) + ")")
	}
	staleBadge := renderStaleIndicator(m.config, task)
	checklistInd := renderChecklistBar(task, m.width)
	structureGroup := checklistInd
	lifecycleGroup := dueStr
	descBadge := ""
	if m.config.IsDescriptionColumnVisible() {
		descBadge = renderDescriptionIndicator(task)
	}
	contentGroup := JoinGroupParts(descBadge, renderTruncatedTags(task.Tags, 2, 8), staleBadge)
	trailing := JoinIndicatorGroups(structureGroup, lifecycleGroup, contentGroup, estimateStr)
	if trailing != "" {
		content += " " + trailing
	}
	if m.tabs.Active == boardTabToday && m.carriedOverIDs[task.ID] {
		content += " " + MutedStyle.Render("(carried)")
	}

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
	b.WriteString(line)

	// Escalation hint: P3+ tasks due within the escalation window on the Today tab
	if m.tabs.Active == boardTabToday && task.Priority >= 3 && task.DueDate != nil {
		if escalationDays := m.config.GetPriorityEscalationDays(); escalationDays > 0 {
			if task.DueDate.Before(time.Now().AddDate(0, 0, escalationDays+1)) {
				b.WriteString("\n")
				b.WriteString("      " + InfoStatusStyle.Render("↑ Due soon — consider P2"))
			}
		}
	}

	return b.String()
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
		for _, line := range strings.Split(strings.TrimSpace(task.Description), "\n") {
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

	cardWidth := min(m.width-8, 80)
	if cardWidth < 20 {
		cardWidth = 20
	}
	cardCfg := CardConfig{
		Style:       SidebarCardStyle,
		TitleStyle:  TitleStyle,
		AccentColor: m.AccentColor(),
	}
	card := renderCard(task.Title, strings.TrimRight(body.String(), "\n"), cardWidth, cardCfg)

	// Center vertically
	cardLines := strings.Count(card, "\n") + 1
	availHeight := fullscreenContentHeight(m.height, 2) // 2: header + footer hint
	topPad := (availHeight - cardLines) / 2
	if topPad < 1 {
		topPad = 1
	}

	// Center horizontally
	leftPad := (m.width - cardWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	leftStr := strings.Repeat(" ", leftPad)

	for i := 0; i < topPad; i++ {
		b.WriteString("\n")
	}
	for _, line := range strings.Split(card, "\n") {
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
