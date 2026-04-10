package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

func (m *FocusBoardModel) View() tea.View { //nolint:gocyclo
	var b strings.Builder

	// Title
	totalActions := 0
	totalTasks := 0
	for _, r := range m.rows {
		if r.isAction() {
			totalActions++
		} else {
			totalTasks++
		}
	}
	subtitle := fmt.Sprintf("%d actions, %d tasks", totalActions, totalTasks)
	b.WriteString(renderFullscreenTitle("Focus Board", subtitle, m.AccentColor()))
	b.WriteString("\n")

	// Tab bar
	b.WriteString(renderTabBarWithOptionalSubTabs(m.computedTabNames(), m.tabs.Active, nil, false, m.width, m.AccentColor()))
	b.WriteString("\n")

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
			b.WriteString(MutedStyle.Render("    %project · !priority · #tag · -s source · -u url"))
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

	// Calculate available height
	contentHeight := fullscreenContentHeight(m.height, 3) // tab bar(2) + status(1)
	if m.addMode {
		contentHeight -= addBlockHeight
	}
	contentHeight -= inputBlockHeight

	// Empty state
	if len(m.filtered) == 0 {
		b.WriteString("\n")
		if m.HasActiveFilters() {
			b.WriteString(MutedStyle.Render("  " + EmptyStateFiltered))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("  Press Fx to clear filters, or Esc to dismiss."))
		} else if m.tabs.Active == boardTabToday {
			b.WriteString(MutedStyle.Render("  No tasks planned for today"))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("  Press w on any task to plan it for today."))
		} else if m.tabs.Active == boardTabClosed {
			b.WriteString(MutedStyle.Render("  " + EmptyStateClosed))
		} else if len(m.rows) == 0 {
			b.WriteString(MutedStyle.Render("  " + EmptyStateNoTasks))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("  Press a to add an action, or mark tasks as active in a project."))
		} else {
			b.WriteString(MutedStyle.Render("  " + EmptyStateNoMatch))
		}
		b.WriteString("\n\n")
		b.WriteString(renderStatusMessages(m.statusMessage, m.err))
		left := HelpStyle.Render("a add · tab switch · q close · ? help")
		right := MutedStyle.Render("0 items")
		b.WriteString(renderStatusBarFooter(left, right, m.width, m.AccentColor()))
		return tea.NewView(b.String())
	}

	// Render rows
	var lines []string
	maxTitleLen := max(m.width-FocusBoardTitlePadding, FocusBoardMinTitleWidth)

	for i, row := range m.filtered {
		isSelected := i == m.cursor

		// Quick-edit mode for the selected row
		if m.editMode && i == m.editIdx {
			line := "  > " + m.editInput.View()
			lines = append(lines, line)
			continue
		}

		rowNum := ""
		if m.tabs.Active == boardTabToday {
			rowNum = fmt.Sprintf("%d.", i+1)
		}
		line := m.renderRow(row, isSelected, maxTitleLen, rowNum)
		lines = append(lines, line)
	}

	// Insert inline preview after cursor row
	if m.previewMode && m.cursor >= 0 && m.cursor < len(m.filtered) {
		row := m.getRowAtCursor()
		if row != nil {
			indent := "        " // align with board row content
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

	b.WriteString(strings.Join(applyScroll(lines, m.cursor, contentHeight), "\n"))
	b.WriteString("\n\n")

	b.WriteString(renderStatusMessages(m.statusMessage, m.err))

	// Footer
	var left string
	if m.addMode || m.editMode {
		left = HelpStyle.Render("Enter: save · Esc: cancel")
	} else if m.tabs.Active == boardTabClosed {
		left = HelpStyle.Render("d reopen · D delete · o open URL · ? help")
	} else {
		left = HelpStyle.Render("a add · x done · X dismiss · p priority · e edit · enter open/go")
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
	if m.tabs.Active == boardTabClosed {
		rightParts = append(rightParts, fmt.Sprintf("%d closed", len(m.closedRows)))
	} else if m.tabs.Active == boardTabToday {
		completedToday := 0
		for _, r := range m.todayRows {
			if r.Task.Status.IsComplete() {
				completedToday++
			}
		}
		rightParts = append(rightParts, fmt.Sprintf("%d/%d done", completedToday, len(m.todayRows)))
	} else {
		rightParts = append(rightParts, fmt.Sprintf("%d/%d items", len(m.filtered), len(m.rows)))
	}
	right := MutedStyle.Render(strings.Join(rightParts, " · "))
	b.WriteString(renderStatusBarFooter(left, right, m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

// renderRow renders a single board row line.
func (m *FocusBoardModel) renderRow(row boardRow, isSelected bool, maxTitleLen int, rowNum string) string {
	var b strings.Builder

	task := row.Task
	projectName := truncateToVisualWidth(row.Project.Name, 10)

	priorityBadge := boardPriorityBadge(task.Priority)
	isClosed := task.Status.IsComplete()

	// Queue / daily-plan indicator: always 2 chars wide (first column).
	queueIndicator := "  "
	if m.dailyPlanIDs[task.ID] {
		queueIndicator = ActiveStyle.Render("T") + " "
	} else if m.queueIDs[task.ID] {
		queueIndicator = ActiveStyle.Render("Q") + " "
	}

	// Row number prefix for Today tab, or plain indent.
	prefix := ""
	if rowNum != "" {
		prefix = fmt.Sprintf("%3s ", rowNum)
	}

	if row.isAction() {
		// Action item rendering
		var sourceBadge string
		if task.Source != "" {
			sourceBadge = DependencyStyle.Render(fmt.Sprintf("[%s]", truncateToVisualWidth(task.Source, 10)))
		} else {
			sourceBadge = DependencyStyle.Render("[action]")
		}
		projectBadge := ""
		if !row.Project.Internal {
			projectBadge = " " + MutedStyle.Render(fmt.Sprintf("[%s]", truncateToVisualWidth(projectName, 12)))
		}
		urlIndicator := ""
		if task.URL != "" {
			urlIndicator = " ↗"
		}

		statusMarker := ""
		if isClosed {
			if task.Status == models.StatusDone {
				statusMarker = "[x] "
			} else {
				statusMarker = "[-] "
			}
		}

		truncatedTitle := truncateToVisualWidth(task.Title, maxTitleLen)
		title := m.highlightTitle(truncatedTitle, task.Title)
		if isSelected && !task.Dimmed && !isClosed {
			title = SelectedTaskTitleStyle.Render(title)
		}
		content := fmt.Sprintf("%s%s%s %s%s%s %s%s", queueIndicator, prefix, priorityBadge, sourceBadge, projectBadge, urlIndicator, statusMarker, title)

		if len(task.Tags) > 0 {
			content += " " + renderInlineTags(task.Tags)
		}
		if m.tabs.Active == boardTabToday && m.carriedOverIDs[task.ID] {
			content += " " + MutedStyle.Render("(carried)")
		}

		if !task.Dimmed && isClosed {
			content = MutedStyle.Render(content)
		}

		line := RenderSelectableRow(isSelected, task.Active, task.Pinned, m.cursorFlash, content, m.width-2, task.Dimmed)
		b.WriteString(line)
	} else {
		// Regular task rendering
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
		}
		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
		content := fmt.Sprintf("%s%s%s %s %s%s %s", queueIndicator, prefix, statusIcon, priorityBadge, projectBadge, typeIcon, title)

		structureGroup := scoreBadge
		lifecycleGroup := dueStr
		contentGroup := renderTruncatedTags(task.Tags, 2, 8)
		trailing := JoinIndicatorGroups(structureGroup, lifecycleGroup, contentGroup)
		if trailing != "" {
			content += " " + trailing
		}
		if m.tabs.Active == boardTabToday && m.carriedOverIDs[task.ID] {
			content += " " + MutedStyle.Render("(carried)")
		}

		dimmed := task.Dimmed || (isClosed && m.tabs.Active == boardTabToday)
		line := RenderSelectableRow(isSelected, task.Active, task.Pinned, m.cursorFlash, content, m.width-2, dimmed)
		b.WriteString(line)
	}

	return b.String()
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

// computedTabNames returns the tab name slice with a dynamic "Today (done/total)" label.
func (m *FocusBoardModel) computedTabNames() []string {
	names := make([]string, len(focusBoardTabNames))
	copy(names, focusBoardTabNames)
	completedToday := 0
	for _, r := range m.todayRows {
		if r.Task.Status.IsComplete() {
			completedToday++
		}
	}
	names[boardTabToday] = fmt.Sprintf("Today (%d/%d)", completedToday, len(m.todayRows))
	return names
}

// boardPriorityBadge returns a colored priority label for board items (P1, P2, etc.).
func boardPriorityBadge(priority int) string {
	label := fmt.Sprintf("P%d", priority)
	switch priority {
	case 1:
		return OverdueStyle.Render(label)
	case 2:
		return DueSoonStyle.Render(label)
	default:
		return MutedStyle.Render(label)
	}
}
