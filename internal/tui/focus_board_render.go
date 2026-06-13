package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *FocusBoardModel) View() tea.View { //nolint:gocyclo
	if m.focusMode == focusModeZen {
		return m.renderZenMode()
	}

	var b strings.Builder

	stats := m.todayStats()
	completedToday := stats.completedToday
	plannedTotal := stats.plannedTotal

	if m.focusMode == focusModeTunnel {
		// Tunnel mode: show current task as header, hide tab bar
		titleContent := fullscreenTitleStyle(m.AccentColor()).Render("Focus Mode")
		if row := m.getRowAtCursor(); row != nil && row.Task != nil {
			titleContent = fullscreenTitleStyle(m.AccentColor()).Render("Currently: ") +
				TaskTitleStyleFor(row.Task, TaskTitleStyleOptions{}).Render(row.Task.Title)
		}
		b.WriteString(renderFullscreenTitleContentWithCounter(titleContent, "", FormatOpenLoops(m.openLoops), m.width))
		b.WriteString("\n")
	} else {
		// Normal mode: show title + tab bar
		title := "Today"
		titleColor := m.AccentColor()
		var subtitle string
		if m.tabs.Active == boardTabToday {
			switch {
			case plannedTotal == 0 && m.openLoops.Overdue > 0:
				subtitle = fmt.Sprintf("0 planned · ⚠ %d overdue", m.openLoops.Overdue)
				titleColor = ColorWarning
			case plannedTotal == 0:
				subtitle = "0 planned"
			case completedToday == plannedTotal:
				title = "Today ✓"
				subtitle = "All done!"
				titleColor = ColorSuccess
			default:
				subtitle = fmt.Sprintf("%d tasks · %d/%d done", len(m.rows), completedToday, plannedTotal)
			}
		} else {
			subtitle = fmt.Sprintf("%d tasks", len(m.rows))
		}
		if name := PerspectiveLensName(m.activePerspective); name != "" {
			subtitle += " · " + name
		}
		b.WriteString(renderFullscreenTitleWithCounter(title, subtitle, FormatOpenLoops(m.openLoops), m.width, titleColor))
		b.WriteString("\n")
		b.WriteString(renderTabBarWithOptionalSubTabs(m.computedTabNames(), m.tabs.Active, nil, false, m.width, m.AccentColor()))
		b.WriteString("\n")
		if name := PerspectiveLensName(m.activePerspective); name != "" {
			b.WriteString(MutedStyle.Render("  Perspective: "+name) + "\n")
		}
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

	// Compute badge widths once per render pass for column alignment.
	var colLayout *ColumnLayout
	if m.width >= MinColumnAlignWidth {
		tasks := make([]*models.Task, 0, len(m.filtered))
		for _, r := range m.filtered {
			if r.Task != nil {
				tasks = append(tasks, r.Task)
			}
		}
		colLayout = ComputeBadgeWidths(tasks, BadgeWidthOpts{
			HideLowPriority: true,
			IncludeStatus:   true,
			IncludeType:     true,
			IncludeStage:    true,
		})
	}

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
		line := m.renderRow(row, isSelected, maxTitleLen, rowNum, isFirstPlanItem, colLayout)
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
		left = HelpStyle.Render("enter save · Esc cancel")
	} else if m.multiSelectMode {
		left = HelpStyle.Render("x done · space toggle · A deselect")
	} else if m.tabs.Active == boardTabClosed {
		left = HelpStyle.Render("d reopen · enter open · D delete")
	} else if m.tabs.Active == boardTabToday && completedToday == plannedTotal && plannedTotal > 0 {
		left = HelpStyle.Render("w plan more · a add · Esc close")
	} else if m.tabs.Active == boardTabToday {
		left = HelpStyle.Render("x done · a add · enter open")
	} else {
		left = HelpStyle.Render("w plan today · a add · enter open")
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
	switch m.tabs.Active {
	case boardTabClosed:
		rightParts = append(rightParts, fmt.Sprintf("%d closed", len(m.closedRows)))
	case boardTabToday:
		rightParts = append(rightParts, fmt.Sprintf("%d/%d done", completedToday, plannedTotal))
		if m.childDepthLimit >= 0 {
			rightParts = append(rightParts, fmt.Sprintf("fold:%d", m.childDepthLimit))
		}
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
