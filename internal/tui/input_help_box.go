package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// appStatusBarHeight is the number of terminal rows the AppModel status bar
// occupies below a non-full-height view's content (separator + breadcrumb).
// Input views reserve this space so their content never overflows the terminal.
const appStatusBarHeight = 2

// clampHelpBoxToHeight renders a help box for bodyLines, budgeting the rows
// against viewHeight minus the height already consumed by preHelp (everything
// the view rendered above the box) and the app status bar. A non-positive
// viewHeight (an unsized view) renders the box unclamped. It returns "" when
// there is no room for the box, signaling the caller to omit it.
func clampHelpBoxToHeight(bodyLines []string, viewHeight int, preHelp string) string {
	if viewHeight <= 0 {
		return BoxStyle.Render(strings.Join(bodyLines, "\n"))
	}
	budget := viewHeight - lipgloss.Height(preHelp) - appStatusBarHeight
	return renderClampedHelpBox(bodyLines, budget)
}

// renderClampedHelpBox renders bodyLines inside BoxStyle, trimmed so the whole
// rendered box (rounded border + padding + body) fits within maxHeight terminal
// rows. When body rows must be dropped, the last visible row collapses into a
// muted "… N more" indicator so the panel degrades gracefully on short screens
// instead of overflowing. It returns "" when maxHeight leaves no room for even a
// minimal box, signaling the caller to omit the help entirely.
func renderClampedHelpBox(bodyLines []string, maxHeight int) string {
	chrome := BoxStyle.GetVerticalBorderSize() + BoxStyle.GetVerticalPadding()
	bodyBudget := maxHeight - chrome

	// Enough room for every row (or an unclamped/unsized budget): render as-is.
	if bodyBudget >= len(bodyLines) {
		return BoxStyle.Render(strings.Join(bodyLines, "\n"))
	}
	// Not even room for the box chrome plus one row: caller omits the box.
	if bodyBudget < 1 {
		return ""
	}

	// Keep as many leading rows as fit, reserving the final row for the
	// "… N more" indicator that accounts for every dropped row.
	kept := append([]string{}, bodyLines[:bodyBudget-1]...)
	hidden := len(bodyLines) - (bodyBudget - 1)
	kept = append(kept, MutedStyle.Render(fmt.Sprintf("… %d more", hidden)))
	return BoxStyle.Render(strings.Join(kept, "\n"))
}
