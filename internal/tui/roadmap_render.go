package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const (
	ganttSepRaw    = " │ "
	barCharDone    = "▓"
	barCharActive  = "█"
	barCharPlanned = "░"
	barCharBlocked = "─"
	todayMarkerStr = "▼"
	milestoneChar  = "◆"
)

// View implements tea.Model.
func (m *RoadmapModel) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Loading roadmap...")
	}

	scope := "All Projects"
	if !m.isGlobal && m.project != nil {
		scope = m.project.Name
	}
	zoomLabels := []string{"Week", "Month", "Quarter"}
	subtitle := zoomLabels[m.zoom] + " · " + scope
	if m.showCriticalPath {
		subtitle += " · critical path"
	}
	title := renderFullscreenTitle("Roadmap", subtitle, m.AccentColor())

	tabBar := RenderTabBar(roadmapTabNames, int(m.activeTab), m.width, 1, m.AccentColor())

	var helpParts []string
	switch m.activeTab {
	case RoadmapTabTimeline:
		helpParts = []string{"Tab tabs", "j/k cursor", "H/L scroll", "z zoom", "space expand", "↵ task"}
		if m.showCriticalPath {
			helpParts = append(helpParts, "c normal")
		} else {
			helpParts = append(helpParts, "c crit path")
		}
	default:
		helpParts = []string{"Tab tabs", "j/k cursor"}
	}
	if m.canToggle {
		helpParts = append(helpParts, "s scope")
	}
	helpParts = append(helpParts, "r refresh", "? help")
	helpText := HelpStyle.Render(strings.Join(helpParts, " · "))
	footer := renderStatusBarFooter("", helpText, m.width, m.AccentColor())

	contentHeight := max(fullscreenContentHeight(m.height, 2), 4)
	chartWidth := max(m.width-m.labelWidth-lipgloss.Width(ganttSepRaw), 10)

	var content string
	switch m.activeTab {
	case RoadmapTabTimeline:
		content = m.renderTimelineTab(contentHeight, chartWidth)
	case RoadmapTabCriticalPath:
		content = m.renderCriticalPathTab(contentHeight)
	case RoadmapTabForecasts:
		content = m.renderForecastsTab(contentHeight)
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, title, tabBar, content, footer))
}

// renderTimelineTab renders the Gantt chart split-pane layout.
func (m *RoadmapModel) renderTimelineTab(height, chartWidth int) string {
	totalCols := max(m.axis.TotalCols, 1)
	const minGanttColWidth = 6
	colWidth := max(minGanttColWidth, chartWidth/totalCols)
	visibleCols := chartWidth / max(colWidth, 1)

	axisHeader := m.renderAxisHeader(chartWidth, colWidth, visibleCols)
	divider := MutedStyle.Render(
		strings.Repeat("─", m.labelWidth) + "─┼─" + strings.Repeat("─", chartWidth))

	overhead := 2 // axis header + divider
	contentRows := max(height-overhead, 1)

	var lines []string
	lines = append(lines, axisHeader, divider)

	if len(m.rows) == 0 {
		lines = append(lines, renderEmptyState(
			"No tasks to display",
			"Add tasks with due dates or dependencies to see them here"))
		return strings.Join(lines, "\n")
	}

	// Derive scroll offset: keep cursor visible without mutating state.
	scrollOff := m.scrollOff
	if m.cursor < scrollOff {
		scrollOff = m.cursor
	} else if m.cursor >= scrollOff+contentRows {
		scrollOff = m.cursor - contentRows + 1
	}
	scrollOff = max(0, min(scrollOff, max(0, len(m.rows)-contentRows)))

	end := min(scrollOff+contentRows, len(m.rows))
	for i := scrollOff; i < end; i++ {
		lines = append(lines, m.renderGanttRow(m.rows[i], colWidth, visibleCols, i == m.cursor))
	}

	if len(m.rows) > contentRows {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("  row %d/%d", m.cursor+1, len(m.rows))))
	}

	return strings.Join(lines, "\n")
}

// renderAxisHeader renders the time axis label row above the Gantt bars.
func (m *RoadmapModel) renderAxisHeader(chartWidth, colWidth, visibleCols int) string {
	labelPad := strings.Repeat(" ", m.labelWidth)
	sep := MutedStyle.Render(ganttSepRaw)

	accentSt := lipgloss.NewStyle().Foreground(m.AccentColor()).Bold(true)

	// Show a label every N columns so labels don't overlap.
	minLabelWidth := 5
	sampleEvery := max(1, minLabelWidth/max(colWidth, 1))

	var chartBuf strings.Builder
	for visCol := 0; visCol < visibleCols; visCol++ {
		axisCol := m.hScrollOff + visCol
		if axisCol >= m.axis.TotalCols {
			chartBuf.WriteString(strings.Repeat(" ", colWidth))
			continue
		}
		if axisCol == m.axis.TodayCol {
			chartBuf.WriteString(accentSt.Render(truncateAndPad(todayMarkerStr, colWidth)))
		} else if axisCol%sampleEvery == 0 {
			chartBuf.WriteString(MutedStyle.Render(truncateAndPad(m.axis.Labels[axisCol], colWidth)))
		} else {
			chartBuf.WriteString(strings.Repeat(" ", colWidth))
		}
	}

	return labelPad + sep + chartBuf.String()
}

// renderGanttRow renders one timeline row: label pane + separator + bar area.
func (m *RoadmapModel) renderGanttRow(row TimelineRow, colWidth, visibleCols int, isCursor bool) string {
	accentSt := lipgloss.NewStyle().Foreground(m.AccentColor()).Bold(true)

	// --- Label ---
	indent := strings.Repeat("  ", row.Indent)
	typePrefix := "· "
	if row.Task != nil {
		switch row.Task.Type {
		case models.TaskTypeEpic:
			typePrefix = "◈ "
		case models.TaskTypeStory:
			typePrefix = "○ "
		}
	}

	labelText := indent + typePrefix
	if row.Task != nil {
		labelText += row.Task.Title
	}

	// Scope delta badge for epics.
	if row.Task != nil && row.Task.Type == models.TaskTypeEpic && row.Task.EpicAttrs != nil {
		if row.Task.EpicAttrs.ScopeBaselinedAt != nil {
			current := len(row.Task.Children)
			initial := row.Task.EpicAttrs.InitialChildCount
			delta := current - initial
			pct := 0
			if initial > 0 {
				pct = delta * 100 / initial
			}
			sign := ""
			if delta >= 0 {
				sign = "+"
			}
			labelText += MutedStyle.Render(fmt.Sprintf(" (%d→%d %s%d%%)", initial, current, sign, pct))
		} else if nc := len(row.Task.Children); nc > 0 {
			labelText += MutedStyle.Render(fmt.Sprintf(" (%d)", nc))
		}
	}

	// Collapse indicator.
	if row.Task != nil && len(row.Task.Children) > 0 {
		if row.Collapsed {
			labelText += MutedStyle.Render(" ▶")
		} else {
			labelText += MutedStyle.Render(" ▼")
		}
	}

	labelCell := truncateAndPad(labelText, m.labelWidth)
	sep := MutedStyle.Render(ganttSepRaw)

	// Milestone column: epic with Milestone set and DueDate.
	milestoneCol := -1
	if row.Task != nil && row.Task.Type == models.TaskTypeEpic &&
		row.Task.EpicAttrs != nil && row.Task.EpicAttrs.Milestone != "" &&
		row.Task.DueDate != nil {
		milestoneCol = m.dateToCol(*row.Task.DueDate)
	}

	// --- Bar area ---
	var barBuf strings.Builder
	for visCol := 0; visCol < visibleCols; visCol++ {
		axisCol := m.hScrollOff + visCol

		if axisCol == milestoneCol {
			barBuf.WriteString(accentSt.Render(strings.Repeat(milestoneChar, colWidth)))
			continue
		}

		inBar := row.BarStart < row.BarEnd && axisCol >= row.BarStart && axisCol < row.BarEnd
		charStr := " "
		if inBar {
			if row.IsBlocked {
				charStr = barCharBlocked
			} else if row.Task != nil && row.Task.IsComplete() {
				charStr = barCharDone
			} else if axisCol < m.axis.TodayCol {
				charStr = barCharDone
			} else if axisCol == m.axis.TodayCol {
				charStr = barCharActive
			} else {
				charStr = barCharPlanned
			}
		}

		cell := strings.Repeat(charStr, colWidth)
		if inBar {
			switch {
			case m.showCriticalPath && row.IsCritical:
				cell = accentSt.Render(cell)
			case m.showCriticalPath && !row.IsCritical:
				cell = MutedStyle.Render(cell)
			case row.IsBlocked:
				cell = OverdueStyle.Render(cell)
			default:
				cell = NormalStyle.Render(cell)
			}
		}
		barBuf.WriteString(cell)
	}

	// Progress and forecast annotation.
	var annot strings.Builder
	if row.Task != nil && len(row.Task.Children) > 0 {
		done := 0
		for _, c := range row.Task.Children {
			if c.IsComplete() {
				done++
			}
		}
		annot.WriteString(MutedStyle.Render(fmt.Sprintf("  %d/%d", done, len(row.Task.Children))))
	}
	if !row.ForecastDate.IsZero() {
		annot.WriteString(MutedStyle.Render("  " + row.ForecastDate.Format("Jan 2")))
	}

	fullLine := labelCell + sep + barBuf.String() + annot.String()
	if isCursor {
		return SelectedLine(fullLine, 0)
	}
	return fullLine
}

// renderCriticalPathTab renders the critical path task list with slack.
func (m *RoadmapModel) renderCriticalPathTab(height int) string {
	if m.criticalPath == nil || len(m.criticalPath.Path) == 0 {
		return renderEmptyState(
			"No critical path computed",
			"Critical path requires tasks with dependencies and velocity data")
	}

	accentSt := lipgloss.NewStyle().Foreground(m.AccentColor()).Bold(true)

	// Detect if no tasks have dependency edges.
	hasDeps := false
	for _, t := range m.criticalPath.Path {
		if len(t.DependsOn) > 0 {
			hasDeps = true
			break
		}
	}

	header := MutedStyle.Render(fmt.Sprintf(
		"Critical path  length: %.1f days  tasks: %d",
		m.criticalPath.Length, len(m.criticalPath.Path)))
	if !hasDeps {
		header += "\n" + MutedStyle.Render("No dependency edges found — slack values assume all tasks are independent.")
	}
	colHeader := MutedStyle.Render(fmt.Sprintf("%-4s %-42s %8s %12s", "#", "Task", "Slack", "Status"))

	overhead := 3 // header + colHeader + blank line
	contentRows := max(height-overhead, 1)

	scrollOff := m.scrollOff
	if m.cursor < scrollOff {
		scrollOff = m.cursor
	} else if m.cursor >= scrollOff+contentRows {
		scrollOff = m.cursor - contentRows + 1
	}
	scrollOff = max(0, min(scrollOff, max(0, len(m.criticalPath.Path)-contentRows)))

	var lines []string
	lines = append(lines, header, "", colHeader)

	end := min(scrollOff+contentRows, len(m.criticalPath.Path))
	for i := scrollOff; i < end; i++ {
		task := m.criticalPath.Path[i]
		slack := m.criticalPath.Slack[task.ID]
		title := truncateToVisualWidth(task.Title, 40)
		slackStr := fmt.Sprintf("%8s", fmt.Sprintf("%.1fd", slack))
		if slack == 0 {
			slackStr = accentSt.Render(slackStr)
		}
		statusStr := string(task.Status)
		line := fmt.Sprintf("%-4d %-42s %s %12s", i+1, title, slackStr, statusStr)
		if i == m.cursor {
			lines = append(lines, SelectedLine(line, 0))
		} else {
			lines = append(lines, line)
		}
	}

	if len(m.criticalPath.Path) > contentRows {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("  %d/%d  (j/k)", m.cursor+1, len(m.criticalPath.Path))))
	}

	return strings.Join(lines, "\n")
}

// renderForecastsTab renders the per-epic/story forecast table.
func (m *RoadmapModel) renderForecastsTab(height int) string {
	forecastRows := m.forecastRows()

	if len(forecastRows) == 0 {
		return renderEmptyState(
			"No forecast data",
			"Forecasts require epics or stories with child tasks and completed velocity data")
	}

	velStr := "no velocity data"
	if m.velocity > 0 {
		velStr = fmt.Sprintf("%.2f tasks/day", m.velocity)
	}
	header := MutedStyle.Render("Velocity: " + velStr)
	colHeader := MutedStyle.Render(fmt.Sprintf(
		"%-35s %5s %10s %12s %10s", "Task", "Left", "Days", "Forecast", "Confidence"))

	overhead := 3
	contentRows := max(height-overhead, 1)

	scrollOff := m.scrollOff
	if m.cursor < scrollOff {
		scrollOff = m.cursor
	} else if m.cursor >= scrollOff+contentRows {
		scrollOff = m.cursor - contentRows + 1
	}
	scrollOff = max(0, min(scrollOff, max(0, len(forecastRows)-contentRows)))

	var lines []string
	lines = append(lines, header, "", colHeader)

	now := time.Now()
	end := min(scrollOff+contentRows, len(forecastRows))
	for i := scrollOff; i < end; i++ {
		row := forecastRows[i]
		remaining := 0
		for _, c := range row.Task.Children {
			if !c.IsComplete() {
				remaining++
			}
		}
		title := truncateToVisualWidth(row.Task.Title, 33)
		daysStr := "—"
		forecastStr := "—"
		if !row.ForecastDate.IsZero() {
			days := row.ForecastDate.Sub(now).Hours() / 24
			daysStr = fmt.Sprintf("%.1f", days)
			forecastStr = row.ForecastDate.Format("Jan 2 06")
		}
		confStr := row.Confidence.String()

		// Scope delta annotation.
		scopeStr := ""
		if row.Task.Type == models.TaskTypeEpic && row.Task.EpicAttrs != nil &&
			row.Task.EpicAttrs.ScopeBaselinedAt != nil {
			delta := len(row.Task.Children) - row.Task.EpicAttrs.InitialChildCount
			sign := ""
			if delta >= 0 {
				sign = "+"
			}
			scopeStr = MutedStyle.Render(fmt.Sprintf(" scope:%s%d", sign, delta))
		}

		line := fmt.Sprintf("%-35s %5d %10s %12s %10s", title, remaining, daysStr, forecastStr, confStr)
		line += scopeStr
		if i == m.cursor {
			lines = append(lines, SelectedLine(line, 0))
		} else {
			lines = append(lines, line)
		}
	}

	if len(forecastRows) > contentRows {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("  %d/%d  (j/k)", m.cursor+1, len(forecastRows))))
	}

	return strings.Join(lines, "\n")
}
