package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	case ViewDashboard, ViewProjectTree, ViewOutliner, ViewFocusBoard:
		return m.alertCount() > 0
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

	// Need at least 2 chars gap between content and indicator
	minGap := 2
	if firstLineWidth+indicatorWidth+minGap > width {
		// Not enough room; skip the indicator
		return content
	}

	gap := width - firstLineWidth - indicatorWidth
	spacer := strings.Repeat(" ", gap)
	newFirst := firstLine + spacer + indicator

	if len(lines) > 1 {
		return newFirst + "\n" + lines[1]
	}
	return newFirst
}
