package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"
)

func (m *WeeklyReviewModel) View() tea.View {
	totalCount := m.getTotalItemCount()
	projectCount, taskCount := m.countByType()

	var b strings.Builder

	// Title
	var title, subtitle string
	if m.reviewCompleted {
		title = "Weekly Review"
		subtitle = fmt.Sprintf("completed %s at %s", m.completedAt.Format("Mon"), m.completedAt.Format("15:04"))
	} else if totalCount > 0 {
		title = fmt.Sprintf("Weekly Review (%d/8)", m.reviewedCount())
		subtitle = fmt.Sprintf("%d projects, %d tasks", projectCount, taskCount)
	} else {
		title = "Weekly Review"
		subtitle = "all clear"
	}
	b.WriteString(renderFullscreenTitle(title, subtitle, m.AccentColor()))
	b.WriteString("\n")

	if totalCount == 0 {
		b.WriteString("\n")
		b.WriteString(renderEmptyState("All systems healthy — your projects are on track.", "Press Esc to close."))
		return tea.NewView(b.String())
	}

	// Progress bar (Goal-Gradient: show progress toward review completion)
	if !m.reviewCompleted {
		b.WriteString(m.renderWeeklyProgressBar())
		b.WriteString("\n")
	}

	// Tab bar
	chrome, chromeLines := renderFullscreenChrome(m.renderTabBar(), "", true)
	b.WriteString(chrome)

	if m.showingSummary {
		b.WriteString(m.renderCompletionSummary())
		return tea.NewView(b.String())
	}

	contentHeight := fullscreenContentHeight(m.height, chromeLines)

	if m.totalItems == 0 {
		emptyMsg := "No tasks in this tab. Use shift+←/→ to switch tabs."
		if m.tabs.Active == 2 {
			emptyMsg = "No tasks due in the upcoming week. Use this tab to review and confirm tasks scheduled for the coming week."
		}
		b.WriteString(lipgloss.NewStyle().Width(m.width).Render(MutedStyle.Render(emptyMsg)))
		b.WriteString("\n\n")
		footer := HelpStyle.Render("shift+←/→: tabs")
		b.WriteString(footer)
		return tea.NewView(b.String())
	}

	lines, cursorLineIndex := m.renderAllSections()

	// Account for preview lines in scroll calculation
	if m.previewMode {
		if dt := m.getTaskAtCursor(); dt != nil {
			previewCount := inlinePreviewLineCount(dt.Task)
			contentHeight -= previewCount
			if contentHeight < 1 {
				contentHeight = 1
			}
		}
	}

	contentStr := strings.Join(applyPersistentScroll(lines, cursorLineIndex, contentHeight, &m.scrollOffset), "\n")
	b.WriteString(TabContentBgStyle.Width(m.width).Render(contentStr))

	// Footer — progressively shorten when terminal is narrow
	reviewed := m.reviewedCount()
	fullRight := fmt.Sprintf("%d/8 reviewed | %d items", reviewed, totalCount)
	shortRight := fmt.Sprintf("%d/8 reviewed", reviewed)
	reviewHint := "R: reviewed"
	if m.reviewCompleted {
		reviewHint = "R: unmark"
	}
	fullLeft := "shift+←/→ tabs · n/N section · " + reviewHint + " · ,s/d/b/x status"
	shortLeft := "n/N section · " + reviewHint + " · ? help"

	rightText := fullRight
	leftText := fullLeft
	if lipgloss.Width(HelpStyle.Render(fullLeft))+lipgloss.Width(MutedStyle.Render(fullRight))+2 > m.width {
		leftText = shortLeft
	}
	if lipgloss.Width(HelpStyle.Render(leftText))+lipgloss.Width(MutedStyle.Render(fullRight))+2 > m.width {
		rightText = shortRight
	}
	left := HelpStyle.Render(leftText)
	if m.multiSelectMode {
		left = HelpStyle.Render("MULTI-SELECT: space toggle · a all · A deselect · ,/! status/priority · v exit")
	}
	right := MutedStyle.Render(rightText)
	b.WriteString(renderFullscreenFooterBlock(m.statusMessage, m.err, left, right, m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

func (m *WeeklyReviewModel) renderWeeklyProgressBar() string {
	const total = 8
	reviewed := m.reviewedCount()
	percent := float64(reviewed) / float64(total) * 100

	barWidth := min(m.width-4, 32)
	if barWidth < 4 {
		return ""
	}

	var bar string
	if reviewed == total {
		bar = RenderProgressBar(percent, barWidth)
	} else {
		filled := int(float64(barWidth) * percent / 100)
		var sb strings.Builder
		accentStyle := lipgloss.NewStyle().Foreground(m.AccentColor())
		for range filled {
			sb.WriteString(accentStyle.Render("█"))
		}
		for range barWidth - filled {
			sb.WriteString(ProgressEmptyStyle.Render("░"))
		}
		bar = sb.String()
	}

	var label string
	switch {
	case reviewed == total:
		label = "All sections reviewed!"
	case reviewed >= total-1:
		label = fmt.Sprintf("Almost there — %d/%d reviewed", reviewed, total)
	default:
		label = fmt.Sprintf("%d/%d reviewed", reviewed, total)
	}

	return bar + " " + MutedStyle.Render(label)
}
