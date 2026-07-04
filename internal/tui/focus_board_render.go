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

	// Next Action strip — Today tab only, normal and list modes only. The
	// strip is recomputed every render because the engine is deterministic.
	if m.tabs.Active == boardTabToday && m.focusMode != focusModeTunnel {
		rec := m.computeNextActionRecommendation()
		strip := renderNextActionStrip(NextActionStripOpts{
			Action:      rec,
			Width:       m.width,
			HasProjects: m.hasAnyProjectsForRecommendation(),
			OpenLoops:   m.openLoops,
		})
		if strip != "" {
			b.WriteString(strip)
			b.WriteString("\n\n")
		}
	}

	// Search input
	if m.searchActive {
		b.WriteString("  / ")
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
	}

	// Quick-add input
	if m.addMode {
		b.WriteString("  > ")
		b.WriteString(m.addInput.View())
		b.WriteString("\n")

		if m.isAutocompleteActive() {
			b.WriteString(renderFocusBoardIndentedWrappedLine("    ", "Tab: accept · ↑↓: navigate · Esc: dismiss", m.width, MutedStyle.Render))
		} else {
			b.WriteString(renderFocusBoardIndentedWrappedLine("    ", "%project · !priority · #tag · @date · /section · $status · -s source · -u url", m.width, MutedStyle.Render))
		}
		b.WriteString("\n")

		// Autocomplete dropdown
		if m.isAutocompleteActive() {
			dropdown := m.ac.RenderDropdown("    ")
			b.WriteString(dropdown)
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
				b.WriteString(renderFocusBoardIndentedWrappedLine("    ", strings.Join(previewParts, " · "), m.width, HelpStyle.Render))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	statusBlock := renderStatusMessages(m.statusMessage, m.err)

	// Empty state
	if len(m.filtered) == 0 {
		b.WriteString("\n")
		var emptyContent string
		if m.HasActiveFilters() {
			emptyContent = renderEmptyState("  "+EmptyStateFiltered, "  Press Fx to clear filters, or Esc to dismiss.")
		} else if m.tabs.Active == boardTabToday {
			emptyContent = m.renderTodayEmptyStateBody()
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
		b.WriteString(statusBlock)
		left := composeFullHeightFooterLeft("a add · tab switch · Esc close")
		right := MutedStyle.Render("0 items")
		b.WriteString(renderStatusBarFooter(left, right, m.width, m.AccentColor()))
		return tea.NewView(b.String())
	}

	metadataStrip := m.renderSelectedTaskMetadataStrip()
	footer := m.renderFocusBoardFooter(stats)
	contentHeight := (focusBoardCockpitLayout{
		topChrome:        b.String(),
		selectedMetadata: metadataStrip,
		statusMessages:   statusBlock,
		footer:           footer,
	}).listHeight(m.height)

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

	visibleLines := focusBoardVisibleListRows(lines, m.cursor, contentHeight, &m.viewScrollOffset)
	contentStr := strings.Join(visibleLines, "\n")
	if m.focusMode == focusModeList {
		contentStr = TabContentBgStyle.Width(m.width).Render(contentStr)
	}
	b.WriteString(contentStr)
	if metadataStrip != "" {
		b.WriteString("\n")
		b.WriteString(metadataStrip)
	}
	b.WriteString("\n\n")

	b.WriteString(statusBlock)

	// Footer stays last in the cockpit hierarchy. Transient input modes
	// (add/edit/multiselect) keep their narrow guidance footers because the
	// escape hatches are irrelevant while the user holds a half-finished edit.
	b.WriteString(footer)

	return tea.NewView(b.String())
}

func renderFocusBoardIndentedWrappedLine(indent, text string, width int, style func(...string) string) string {
	if width <= 0 {
		return style(indent + text)
	}
	bodyWidth := width - len(indent)
	if bodyWidth <= 0 {
		return strings.Join(wrapStyled(style(indent+text), width), "\n")
	}
	wrapped := wrapStyled(style(text), bodyWidth)
	for i := range wrapped {
		wrapped[i] = indent + wrapped[i]
	}
	return strings.Join(wrapped, "\n")
}
