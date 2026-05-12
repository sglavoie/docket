package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

func (m *FocusBoardModel) View() tea.View { //nolint:gocyclo
	if m.focusMode == focusModeZen {
		return m.renderZenMode()
	}

	var b strings.Builder

	if m.focusMode == focusModeTunnel {
		// Tunnel mode: show current task as header, hide tab bar
		title := "Focus Mode"
		if row := m.getRowAtCursor(); row != nil && row.Task != nil {
			title = "Currently: " + row.Task.Title
		}
		b.WriteString(renderFullscreenTitle(title, "", m.AccentColor()))
		b.WriteString("\n")
	} else {
		// Normal mode: show title + tab bar
		subtitle := fmt.Sprintf("%d tasks", len(m.rows))
		b.WriteString(renderFullscreenTitle("Today", subtitle, m.AccentColor()))
		b.WriteString("\n")
		b.WriteString(renderTabBarWithOptionalSubTabs(m.computedTabNames(), m.tabs.Active, nil, false, m.width, m.AccentColor()))
		b.WriteString("\n")
	}

	// Search input
	inputBlockHeight := 0
	if m.searchActive {
		b.WriteString("  / ")
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
		inputBlockHeight++
	}

	// Quick-add input
	addBlockHeight := 0
	if m.addMode {
		b.WriteString("  > ")
		b.WriteString(m.addInput.View())
		b.WriteString("\n")
		addBlockHeight++

		if m.isAutocompleteActive() {
			b.WriteString(MutedStyle.Render("    Tab: accept · ↑↓: navigate · Esc: dismiss"))
		} else {
			b.WriteString(MutedStyle.Render("    %project · !priority · #tag · @date · /section · $status · -s source · -u url"))
		}
		b.WriteString("\n")
		addBlockHeight++

		// Autocomplete dropdown
		if m.isAutocompleteActive() {
			dropdown := m.ac.RenderDropdown("    ")
			b.WriteString(dropdown)
			addBlockHeight += strings.Count(dropdown, "\n")
		}

		// Live preview of parsed prefixes/flags
		if value := strings.TrimSpace(m.addInput.Value()); value != "" {
			result := parseBoardAddInput(value)
			hasExtras := result.source != "" || result.url != "" ||
				(result.priority >= 1 && result.priority <= 5) ||
				len(result.tags) > 0 || result.projectSlug != ""
			if hasExtras {
				var previewParts []string
				if result.title != "" {
					previewParts = append(previewParts, "Title: "+result.title)
				}
				if result.projectSlug != "" {
					projectName := result.projectSlug
					for _, p := range m.acProjectCache {
						if p.Slug == result.projectSlug {
							projectName = p.Name
							break
						}
					}
					previewParts = append(previewParts, "Project: "+projectName)
				}
				if result.priority >= 1 && result.priority <= 5 {
					previewParts = append(previewParts, fmt.Sprintf("P%d", result.priority))
				}
				if len(result.tags) > 0 {
					tagStrs := make([]string, len(result.tags))
					for i, t := range result.tags {
						tagStrs[i] = "#" + t
					}
					previewParts = append(previewParts, strings.Join(tagStrs, " "))
				}
				if result.source != "" {
					previewParts = append(previewParts, "Source: "+result.source)
				}
				if result.url != "" {
					previewParts = append(previewParts, "URL: "+result.url)
				}
				b.WriteString(HelpStyle.Render("    " + strings.Join(previewParts, " · ")))
				b.WriteString("\n")
				addBlockHeight++
			}
		}
		b.WriteString("\n")
		addBlockHeight++
	}

	// Calculate available height (tunnel mode has no tab bar: save 2 lines)
	tabOverhead := 3 // tab bar(2) + status(1)
	if m.focusMode == focusModeTunnel {
		tabOverhead = 1 // status(1) only
	}
	contentHeight := fullscreenContentHeight(m.height, tabOverhead)
	if m.addMode {
		contentHeight -= addBlockHeight
	}
	contentHeight -= inputBlockHeight

	// Empty state
	if len(m.filtered) == 0 {
		b.WriteString("\n")
		var emptyContent string
		if m.HasActiveFilters() {
			emptyContent = renderEmptyState("  "+EmptyStateFiltered, "  Press Fx to clear filters, or Esc to dismiss.")
		} else if m.tabs.Active == boardTabToday && m.completedTodayCount() > 0 {
			n := m.completedTodayCount()
			suffix := "task"
			if n != 1 {
				suffix = "tasks"
			}
			emptyContent = SuccessStyle.Render(fmt.Sprintf("  All done for today! %d %s completed.", n, suffix)) +
				"\n\n" + HelpStyle.Render("  Press w to plan more, or Esc to close.")
		} else if m.tabs.Active == boardTabToday {
			emptyContent = renderEmptyState("  No tasks planned for today", "  Press w on any task to plan it for today.")
		} else if m.tabs.Active == boardTabClosed {
			emptyContent = MutedStyle.Render("  " + EmptyStateClosed)
		} else if len(m.rows) == 0 {
			if m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix() {
				emptyContent = renderContextualEmptyState("  "+EmptyStateNoTasks,
					[]string{";i capture", ";q backlog", ": commands", "? help"})
			} else {
				emptyContent = renderEmptyState("  "+EmptyStateNoTasks, "  Press a to add a task, or mark tasks as active in a project.")
			}
		} else {
			emptyContent = MutedStyle.Render("  " + EmptyStateNoMatch)
		}
		b.WriteString(emptyContent)
		b.WriteString("\n\n")
		b.WriteString(renderStatusMessages(m.statusMessage, m.err))
		left := HelpStyle.Render("a add · tab switch · Esc close · ? help")
		right := MutedStyle.Render("0 items")
		b.WriteString(renderStatusBarFooter(left, right, m.width, m.AccentColor()))
		return tea.NewView(b.String())
	}

	// Render rows
	var lines []string
	maxTitleLen := max(m.width-FocusBoardTitlePadding, FocusBoardMinTitleWidth)

	plannedIdx := 0
	for i, row := range m.filtered {
		isSelected := i == m.cursor

		// Group header rows are non-selectable dividers
		if row.GroupHeader != "" {
			line := renderSectionHeader(SectionHeaderOpts{
				Name:        row.GroupHeader,
				AccentColor: ActiveStyle,
				Width:       m.width,
			})
			lines = append(lines, line)
			continue
		}

		// Quick-edit mode for the selected row
		if m.editMode && i == m.editIdx {
			line := "  > " + m.editInput.View()
			lines = append(lines, line)
			continue
		}

		rowNum := ""
		isFirstPlanItem := false
		if m.tabs.Active == boardTabToday && row.TreePrefix == "" {
			plannedIdx++
			rowNum = fmt.Sprintf("%d.", plannedIdx)
			isFirstPlanItem = plannedIdx == 1
		}
		line := m.renderRow(row, isSelected, maxTitleLen, rowNum, isFirstPlanItem)
		lines = append(lines, line)
	}

	// Insert inline preview after cursor row
	if m.previewMode && m.cursor >= 0 && m.cursor < len(m.filtered) {
		row := m.getRowAtCursor()
		if row != nil {
			indent := "        " // align with board row content
			if row.TreePrefix != "" {
				indent += strings.Repeat("    ", strings.Count(row.TreePrefix, connectorPipe)+strings.Count(row.TreePrefix, connectorSpace)+1)
			}
			pLines := inlinePreviewLines(row.Task, indent, m.width)
			if len(pLines) > 0 {
				insertAt := m.cursor + 1
				if insertAt <= len(lines) {
					tail := append([]string{}, lines[insertAt:]...)
					lines = append(lines[:insertAt], pLines...)
					lines = append(lines, tail...)
					contentHeight -= len(pLines)
					if contentHeight < 1 {
						contentHeight = 1
					}
				}
			}
		}
	}

	contentStr := strings.Join(applyScroll(lines, m.cursor, contentHeight), "\n")
	if m.focusMode == focusModeList {
		contentStr = TabContentBgStyle.Width(m.width).Render(contentStr)
	}
	b.WriteString(contentStr)
	b.WriteString("\n\n")

	b.WriteString(renderStatusMessages(m.statusMessage, m.err))

	// Footer
	var left string
	if m.addMode || m.editMode {
		left = HelpStyle.Render("Enter: save · Esc: cancel")
	} else if m.tabs.Active == boardTabClosed {
		left = HelpStyle.Render("d reopen · D delete · o open URL · ? help")
	} else {
		left = HelpStyle.Render("a add · x done · X dismiss · n/N parents · e edit · enter open/go")
	}
	var rightParts []string
	if indicator := m.renderActiveFilterIndicator(); indicator != "" {
		rightParts = append(rightParts, indicator)
	}
	if m.sort.Field != SortByDefault {
		direction := "↑"
		if !m.sort.Ascending {
			direction = "↓"
		}
		rightParts = append(rightParts, fmt.Sprintf("Sort: %s %s", m.sort.Field.ShortString(), direction))
	}
	if m.tabs.Active == boardTabAll && m.groupBy != boardGroupByNone {
		groupLabel := map[boardGroupBy]string{
			boardGroupBySource:   "source",
			boardGroupByPriority: "priority",
			boardGroupByScore:    "score",
		}[m.groupBy]
		rightParts = append(rightParts, "grouped by "+groupLabel)
	}
	switch m.tabs.Active {
	case boardTabClosed:
		rightParts = append(rightParts, fmt.Sprintf("%d closed", len(m.closedRows)))
	case boardTabToday:
		completedToday := 0
		plannedTotal := 0
		for _, r := range m.todayRows {
			if r.TreePrefix != "" {
				continue
			}
			plannedTotal++
			if r.Task.Status.IsComplete() {
				completedToday++
			}
		}
		rightParts = append(rightParts, fmt.Sprintf("%d/%d done", completedToday, plannedTotal))
		if est := sumEstimatesForRows(m.filtered); est > 0 {
			rightParts = append(rightParts, formatEstimateDays(est)+" est")
		}
	case boardTabAll:
		taskCount := 0
		for _, r := range m.filtered {
			if r.GroupHeader == "" {
				taskCount++
			}
		}
		rightParts = append(rightParts, fmt.Sprintf("%d/%d items", taskCount, len(m.allRows)))
		if est := sumEstimatesForRows(m.filtered); est > 0 {
			rightParts = append(rightParts, formatEstimateDays(est)+" est")
		}
	default:
		rightParts = append(rightParts, fmt.Sprintf("%d/%d items", len(m.filtered), len(m.rows)))
		if est := sumEstimatesForRows(m.filtered); est > 0 {
			rightParts = append(rightParts, formatEstimateDays(est)+" est")
		}
	}
	right := MutedStyle.Render(strings.Join(rightParts, " · "))
	b.WriteString(renderStatusBarFooter(left, right, m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

// renderRow renders a single board row line.
func (m *FocusBoardModel) renderRow(row boardRow, isSelected bool, maxTitleLen int, rowNum string, isFirstPlanItem bool) string {
	var b strings.Builder

	task := row.Task
	projectName := truncateToVisualWidth(row.Project.Name, 10)

	priorityBadge := renderPriorityBadge(task.Priority)
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

	scoreBadge := ""
	if profile := m.config.GetProjectScoringProfile(row.Project.ID); profile != nil {
		scoreBadge = RenderScoreBadge(task, profile)
	}

	truncatedTitle := truncateToVisualWidth(task.Title, maxTitleLen)
	title := m.highlightTitle(truncatedTitle, task.Title)
	if isSelected && !task.Dimmed {
		title = SelectedTaskTitleStyle.Render(title)
	} else {
		title = GetTitlePriorityStyle(task.Priority).Render(title)
	}
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	content := fmt.Sprintf("%s%s%s%s %s %s%s %s", queueIndicator, indent, prefix, statusIcon, priorityBadge, projectBadge, typeIcon, title)

	estimateStr := ""
	if task.EstimatedDays > 0 {
		estimateStr = MutedStyle.Render("(" + formatEstimateDays(task.EstimatedDays) + ")")
	}
	staleBadge := ""
	if threshold := m.config.GetStaleTaskDays(); threshold > 0 {
		effectiveUpdate := task.UpdatedAt
		if effectiveUpdate.IsZero() {
			effectiveUpdate = task.CreatedAt
		}
		if !effectiveUpdate.IsZero() {
			days := int(time.Since(effectiveUpdate).Hours() / 24)
			if days >= threshold {
				staleBadge = MutedStyle.Render("stale")
			}
		}
	}
	structureGroup := scoreBadge
	lifecycleGroup := dueStr
	contentGroup := JoinGroupParts(renderTruncatedTags(task.Tags, 2, 8), staleBadge)
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
	for _, r := range rows {
		if r.Task != nil {
			total += r.Task.EstimatedDays
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
	for _, p := range positions {
		if p < len(titleRunes) {
			validPositions = append(validPositions, p)
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
			check := "☐"
			if item.Done {
				check = "☑"
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
	for _, r := range m.filtered {
		if r.Task != nil && !r.Task.Status.IsComplete() {
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
	completedToday := 0
	plannedTotal := 0
	for _, r := range m.todayRows {
		if r.TreePrefix != "" {
			continue
		}
		plannedTotal++
		if r.Task.Status.IsComplete() {
			completedToday++
		}
	}
	if plannedTotal > 0 {
		pct := float64(completedToday) / float64(plannedTotal) * 100
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
		names[boardTabToday] = fmt.Sprintf("Today (%d/%d) %s", completedToday, plannedTotal, bar.String())
	} else {
		names[boardTabToday] = fmt.Sprintf("Today (%d/%d)", completedToday, plannedTotal)
	}
	return names
}
