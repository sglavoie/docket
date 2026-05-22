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
	todayMarkerStr = IconTodayMarker
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
	for visCol := range visibleCols {
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
	labelCell := truncateAndPad(m.ganttLabelText(row), m.labelWidth)
	sep := MutedStyle.Render(ganttSepRaw)
	fullLine := labelCell + sep + m.renderGanttBars(row, colWidth, visibleCols, accentSt) + m.renderGanttAnnotation(row)
	if isCursor {
		return SelectedLine(fullLine, 0)
	}
	return fullLine
}

func (m *RoadmapModel) ganttLabelText(row TimelineRow) string {
	labelText := strings.Repeat("  ", row.Indent) + roadmapTypePrefix(row.Task)
	if row.Task != nil {
		labelText += row.Task.Title
		labelText += m.renderScopeDeltaBadge(row.Task)
		labelText += m.renderGanttCollapseIndicator(row)
	}
	return labelText
}

func roadmapTypePrefix(task *models.Task) string {
	if task == nil {
		return "· "
	}
	switch task.Type {
	case models.TaskTypeEpic:
		return "◈ "
	case models.TaskTypeStory:
		return "○ "
	case models.TaskTypeTask, models.TaskTypeBug, models.TaskTypeIdea:
		return "· "
	default:
		return "· "
	}
}

func (m *RoadmapModel) renderScopeDeltaBadge(task *models.Task) string {
	if task == nil || task.Type != models.TaskTypeEpic || task.EpicAttrs == nil {
		return ""
	}
	if task.EpicAttrs.ScopeBaselinedAt == nil {
		if childCount := len(task.Children); childCount > 0 {
			return MutedStyle.Render(fmt.Sprintf(" (%d)", childCount))
		}
		return ""
	}

	current := len(task.Children)
	initial := task.EpicAttrs.InitialChildCount
	delta := current - initial
	pct := 0
	if initial > 0 {
		pct = delta * 100 / initial
	}
	sign := ""
	if delta >= 0 {
		sign = "+"
	}
	return MutedStyle.Render(fmt.Sprintf(" (%d→%d %s%d%%)", initial, current, sign, pct))
}

func (m *RoadmapModel) renderGanttCollapseIndicator(row TimelineRow) string {
	if row.Task == nil || len(row.Task.Children) == 0 {
		return ""
	}
	if row.Collapsed {
		return MutedStyle.Render(" " + IconCollapsed)
	}
	return MutedStyle.Render(" " + IconExpanded)
}

func (m *RoadmapModel) renderGanttBars(row TimelineRow, colWidth, visibleCols int, accentSt lipgloss.Style) string {
	milestoneCol := m.ganttMilestoneColumn(row)
	var barBuf strings.Builder
	for visCol := range visibleCols {
		axisCol := m.hScrollOff + visCol
		barBuf.WriteString(m.renderGanttCell(row, axisCol, colWidth, milestoneCol, accentSt))
	}
	return barBuf.String()
}

func (m *RoadmapModel) ganttMilestoneColumn(row TimelineRow) int {
	if row.Task == nil || row.Task.Type != models.TaskTypeEpic || row.Task.EpicAttrs == nil || row.Task.EpicAttrs.Milestone == "" || row.Task.DueDate == nil {
		return -1
	}
	return m.dateToCol(*row.Task.DueDate)
}

func (m *RoadmapModel) renderGanttCell(row TimelineRow, axisCol, colWidth, milestoneCol int, accentSt lipgloss.Style) string {
	if axisCol == milestoneCol {
		return accentSt.Render(strings.Repeat(milestoneChar, colWidth))
	}
	inBar := row.BarStart < row.BarEnd && axisCol >= row.BarStart && axisCol < row.BarEnd
	cell := strings.Repeat(m.ganttBarChar(row, axisCol, inBar), colWidth)
	if !inBar {
		return cell
	}
	return m.styleGanttCell(row, cell, accentSt)
}

func (m *RoadmapModel) ganttBarChar(row TimelineRow, axisCol int, inBar bool) string {
	if !inBar {
		return " "
	}
	switch {
	case row.IsBlocked:
		return barCharBlocked
	case row.Task != nil && row.Task.IsComplete():
		return barCharDone
	case axisCol < m.axis.TodayCol:
		return barCharDone
	case axisCol == m.axis.TodayCol:
		return barCharActive
	default:
		return barCharPlanned
	}
}

func (m *RoadmapModel) styleGanttCell(row TimelineRow, cell string, accentSt lipgloss.Style) string {
	switch {
	case m.showCriticalPath && row.IsCritical:
		return accentSt.Render(cell)
	case m.showCriticalPath && !row.IsCritical:
		return MutedStyle.Render(cell)
	case row.IsBlocked:
		return OverdueStyle.Render(cell)
	default:
		return NormalStyle.Render(cell)
	}
}

func (m *RoadmapModel) renderGanttAnnotation(row TimelineRow) string {
	var annot strings.Builder
	if row.Task != nil && len(row.Task.Children) > 0 {
		done := 0
		for _, child := range row.Task.Children {
			if child.IsComplete() {
				done++
			}
		}
		annot.WriteString(MutedStyle.Render(fmt.Sprintf("  %d/%d", done, len(row.Task.Children))))
	}
	if !row.ForecastDate.IsZero() {
		annot.WriteString(MutedStyle.Render("  " + row.ForecastDate.Format("Jan 2")))
	}
	return annot.String()
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
