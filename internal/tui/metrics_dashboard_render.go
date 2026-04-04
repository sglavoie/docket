package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
	"github.com/guptarohit/asciigraph"
)

// View implements tea.Model.
func (m *MetricsDashboardModel) View() tea.View {
	if m.metrics == nil {
		return tea.NewView("Computing metrics...")
	}

	// Title
	scope := "All Projects"
	if !m.isGlobal && m.project != nil {
		scope = m.project.Name
	}
	subtitle := m.timeRange.Label() + " · " + scope
	title := renderFullscreenTitle("Metrics Dashboard", subtitle, m.AccentColor())

	// Tab bar
	tabBar := RenderTabBar(metricsTabNames, int(m.activeTab), m.width, 1, m.AccentColor())

	// Footer
	helpParts := []string{"[/] tabs", "t/T range", "r refresh"}
	if m.canToggle {
		helpParts = append(helpParts[:2], append([]string{"g scope"}, helpParts[2:]...)...)
	}
	if m.activeTab == MetricsTabBlocked && m.metrics != nil && len(m.metrics.Blocked.Entries) > 0 {
		helpParts = append(helpParts, "↵ go to task")
	}
	helpText := HelpStyle.Render(strings.Join(helpParts, " · "))
	footer := renderStatusBarFooter("", helpText, m.width, m.AccentColor())

	contentHeight := max(
		// tab bar(2)
		fullscreenContentHeight(m.height, 2), 4)

	var content string
	switch m.activeTab {
	case MetricsTabSummary:
		content = m.renderSummaryTab(contentHeight)
	case MetricsTabCompletion:
		content = m.renderCompletionTab(contentHeight)
	case MetricsTabCycleTime:
		content = m.renderCycleTimeTab(contentHeight)
	case MetricsTabThroughput:
		content = m.renderThroughputTab(contentHeight)
	case MetricsTabBlocked:
		content = m.renderBlockedTab(contentHeight)
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, title, tabBar, content, footer))
}

// renderSummaryTab renders a 2x2 card grid with mini charts.
func (m *MetricsDashboardModel) renderSummaryTab(height int) string {
	narrow := m.width < 100
	contentWidth := m.width - 2

	if narrow {
		// Single column layout
		cardWidth := contentWidth
		cardHeight := max((height-2)/4, 4)

		cards := []string{
			m.renderCompletionCard(cardWidth, cardHeight),
			m.renderCycleTimeCard(cardWidth, cardHeight),
			m.renderThroughputCard(cardWidth, cardHeight),
			m.renderBlockedCard(cardWidth, cardHeight),
		}
		return lipgloss.JoinVertical(lipgloss.Left, cards...)
	}

	// 2x2 grid layout
	cardWidth := contentWidth / 2
	cardHeight := max((height-1)/2, 4)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderCompletionCard(cardWidth, cardHeight),
		m.renderCycleTimeCard(cardWidth, cardHeight),
	)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderThroughputCard(cardWidth, cardHeight),
		m.renderBlockedCard(cardWidth, cardHeight),
	)
	return lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow)
}

func (m *MetricsDashboardModel) renderCompletionCard(width, height int) string {
	c := m.metrics.Completion
	rateStr := fmt.Sprintf("%.0f%%", c.Rate*100)
	stat := fmt.Sprintf("%s (%d/%d)", rateStr, c.Completed, c.Total)
	chart := miniChart(c.Trend, max(width-12, 1), max(height-4, 1))
	content := stat + "\n" + chart
	cfg := DashboardCardConfig()
	cfg.Height = height
	return renderCard("Completion Rate", content, width, cfg)
}

func (m *MetricsDashboardModel) renderCycleTimeCard(width, height int) string {
	c := m.metrics.CycleTime
	line1 := fmt.Sprintf("Avg: %s  Med: %s", formatDuration(c.Average), formatDuration(c.Median))
	line2 := fmt.Sprintf("P90: %s  (%d tasks)", formatDuration(c.P90), c.Count)
	chart := miniChart(c.Trend, max(width-12, 1), max(height-5, 1))
	content := line1 + "\n" + line2 + "\n" + chart
	cfg := DashboardCardConfig()
	cfg.Height = height
	return renderCard("Cycle Time", content, width, cfg)
}

func (m *MetricsDashboardModel) renderThroughputCard(width, height int) string {
	c := m.metrics.Throughput
	stat := fmt.Sprintf("%d tasks · %.1f/day", c.Total, c.AvgPerDay)
	chart := miniChart(c.PerBucket, max(width-12, 1), max(height-4, 1))
	content := stat + "\n" + chart
	cfg := DashboardCardConfig()
	cfg.Height = height
	return renderCard("Throughput", content, width, cfg)
}

func (m *MetricsDashboardModel) renderBlockedCard(width, height int) string {
	b := m.metrics.Blocked
	line1 := fmt.Sprintf("%d tasks blocked", b.TaskCount)
	var line2 string
	if b.TaskCount > 0 {
		longest := formatDuration(b.Entries[0].TotalBlocked)
		line2 = fmt.Sprintf("Avg: %s  Longest: %s", formatDuration(b.AverageBlocked), longest)
	} else {
		line2 = "No blocked tasks"
	}
	content := line1 + "\n" + line2
	cfg := DashboardCardConfig()
	cfg.Height = height
	return renderCard("Blocked", content, width, cfg)
}

// Full-width tab renderers

func (m *MetricsDashboardModel) renderCompletionTab(height int) string {
	c := m.metrics.Completion
	chartWidth := m.width - 8
	chartHeight := max(height-6, 3)

	rateStr := fmt.Sprintf("%.0f%%", c.Rate*100)
	header := fmt.Sprintf("Completion Rate: %s (%d resolved, %d done)",
		BoldStyle.Render(rateStr), c.Total, c.Completed)

	chart := renderChart(c.Trend, chartWidth, chartHeight, "Completion rate over time")

	return lipgloss.JoinVertical(lipgloss.Left, header, "", chart)
}

func (m *MetricsDashboardModel) renderCycleTimeTab(height int) string {
	c := m.metrics.CycleTime
	chartWidth := m.width - 8
	chartHeight := max(height-6, 3)

	header := fmt.Sprintf("Average: %s  Median: %s  P90: %s  (%d tasks)",
		BoldStyle.Render(formatDuration(c.Average)),
		formatDuration(c.Median),
		formatDuration(c.P90),
		c.Count)

	chart := renderChart(c.Trend, chartWidth, chartHeight, "Avg cycle time (hours) per period")

	return lipgloss.JoinVertical(lipgloss.Left, header, "", chart)
}

func (m *MetricsDashboardModel) renderThroughputTab(height int) string {
	c := m.metrics.Throughput
	chartWidth := m.width - 8
	chartHeight := max(height-6, 3)

	header := fmt.Sprintf("Total: %s  Average: %s/day",
		BoldStyle.Render(fmt.Sprintf("%d", c.Total)),
		fmt.Sprintf("%.1f", c.AvgPerDay))

	chart := renderChart(c.PerBucket, chartWidth, chartHeight, "Tasks completed per period")

	return lipgloss.JoinVertical(lipgloss.Left, header, "", chart)
}

func (m *MetricsDashboardModel) renderBlockedTab(height int) string {
	b := m.metrics.Blocked

	// Header stats
	var header string
	if b.TaskCount > 0 {
		header = fmt.Sprintf("%s tasks with blocked time  Avg: %s  Total: %s",
			BoldStyle.Render(fmt.Sprintf("%d", b.TaskCount)),
			formatDuration(b.AverageBlocked),
			formatDuration(b.TotalBlocked))
	} else {
		header = "No tasks with blocked time recorded."
	}

	if len(b.Entries) == 0 {
		return header
	}

	// Scrollable task list
	listHeight := max(height-4, 3)

	var lines []string
	headerLine := MutedStyle.Render(fmt.Sprintf("%-5s %-40s %12s %8s", "#", "Task", "Blocked", "Periods"))
	lines = append(lines, headerLine)

	// Scroll window
	start := m.blockedCursor
	if start > 0 && start+listHeight-1 >= len(b.Entries) {
		start = max(len(b.Entries)-listHeight+1, 0)
	}
	end := min(start+listHeight-1, len(b.Entries))

	for i := start; i < end; i++ {
		entry := b.Entries[i]
		title := truncateToVisualWidth(entry.Task.Title, 38)

		prefix := "  "
		style := NormalStyle
		if i == m.blockedCursor {
			prefix = "> "
			style = TitleStyle
		}

		statusIcon := entry.Task.Status.Icon()
		line := fmt.Sprintf("%s%-3d %s %-38s %10s %8d",
			prefix,
			entry.Task.ShortID,
			statusIcon,
			title,
			formatDuration(entry.TotalBlocked),
			entry.BlockedPeriods)
		lines = append(lines, style.Render(line))
	}

	// Scroll indicator
	if len(b.Entries) > listHeight-1 {
		indicator := MutedStyle.Render(fmt.Sprintf("  %d/%d tasks (j/k to scroll)", m.blockedCursor+1, len(b.Entries)))
		lines = append(lines, indicator)
	}

	list := strings.Join(lines, "\n")
	return lipgloss.JoinVertical(lipgloss.Left, header, "", list)
}

// Chart rendering helpers

func miniChart(data []float64, width, height int) string {
	if len(data) == 0 || width < 5 || height < 1 {
		return MutedStyle.Render("No data")
	}

	// Ensure we have at least 2 data points for a chart
	if len(data) == 1 {
		data = append(data, data[0])
	}

	if height < 1 {
		height = 1
	}
	if width < 10 {
		width = 10
	}

	return asciigraph.Plot(data,
		asciigraph.Width(width),
		asciigraph.Height(height),
	)
}

func renderChart(data []float64, width, height int, caption string) string {
	if len(data) == 0 || width < 10 {
		return MutedStyle.Render("No data available")
	}

	if len(data) == 1 {
		data = append(data, data[0])
	}

	if height < 2 {
		height = 2
	}

	return asciigraph.Plot(data,
		asciigraph.Width(width),
		asciigraph.Height(height),
		asciigraph.Caption(caption),
	)
}

// formatDuration formats a duration as a human-friendly string.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	hours := d.Hours()
	if hours < 24 {
		return fmt.Sprintf("%.1fh", hours)
	}
	days := hours / 24
	if days < 7 {
		return fmt.Sprintf("%.1fd", days)
	}
	weeks := days / 7
	return fmt.Sprintf("%.1fw", weeks)
}
