package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// alertIndicatorStyle is a bright style for the alert count indicator.
var alertIndicatorStyle = lipgloss.NewStyle().
	Foreground(ColorAccent). // Bright accent (follows palette)
	Bold(true)

// renderAlertIndicator returns a styled alert indicator string.
// Returns empty string when count is 0.
func renderAlertIndicator(count int) string {
	if count == 0 {
		return ""
	}
	return alertIndicatorStyle.Render(fmt.Sprintf("📫 %d", count))
}

// shouldShowAlertIndicator returns true for views that display the alert indicator.
func (m *AppModel) shouldShowAlertIndicator() bool {
	switch m.currentView() {
	case ViewDashboard,
		ViewCrossProjectOutliner,
		ViewDailyReview, ViewWeeklyReview,
		ViewMetricsDashboard,
		ViewRecentActivity,
		ViewRoadmap,
		ViewBlockers, ViewGlobalBlockers,
		ViewProjectRecentActivity,
		ViewGlobalNotes:
		return m.alertCount() > 0
	default:
	}
	return false
}

// overlayAlertIndicator places the alert indicator right-aligned on the first
// line of the given content string. Used by AppModel.View() for main views.
func overlayAlertIndicator(content string, indicator string, width int) string {
	if indicator == "" || width <= 0 {
		return content
	}

	lines := strings.SplitN(content, "\n", 2)
	if len(lines) == 0 {
		return content
	}

	firstLine := lines[0]
	firstLineWidth := lipgloss.Width(firstLine)
	indicatorWidth := lipgloss.Width(indicator)

	var newFirst string
	switch {
	case firstLineWidth+indicatorWidth+2 <= width:
		// Inline overlay: pad first line and append indicator (needs >=2 cell gap).
		gap := width - firstLineWidth - indicatorWidth
		newFirst = firstLine + strings.Repeat(" ", gap) + indicator
	case isBoxTopBorder(firstLine) && firstLineWidth >= indicatorWidth+8:
		// Splice indicator into the card top border near the right corner:
		// "╭── Title ────────╮" becomes "╭── Title ──── 📫 N ─╮".
		// Reserve " " + indicator + " " + "─╮" on the right (indicatorWidth+4 cells).
		keep := firstLineWidth - indicatorWidth - 4
		left := ansi.Cut(firstLine, 0, keep)
		right := ansi.Cut(firstLine, firstLineWidth-2, firstLineWidth)
		newFirst = left + " " + indicator + " " + right
	default:
		// Not enough room; skip the indicator.
		return content
	}

	if len(lines) > 1 {
		return newFirst + "\n" + lines[1]
	}
	return newFirst
}

// isBoxTopBorder reports whether line is a lipgloss rounded-top border,
// i.e. it starts with ╭ and ends with ╮ (ignoring ANSI styling).
func isBoxTopBorder(line string) bool {
	stripped := ansi.Strip(line)
	runes := []rune(stripped)
	if len(runes) < 2 {
		return false
	}
	return runes[0] == '╭' && runes[len(runes)-1] == '╮'
}
