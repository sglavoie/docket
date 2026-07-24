package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// isWorkCycleMode reports whether a mode belongs to the pinned five-item Work
// tab bar (Today, Next, Backlog, Active, All).
func isWorkCycleMode(id OutlinerModeID) bool {
	for _, cycle := range workModeCycleOrder {
		if cycle == id {
			return true
		}
	}
	return false
}

// renderWorkModeStrip draws the compact Work tab bar plus the trailing
// selected-mode label (when the active mode is outside the five-item cycle) and
// the fixed-order indicator. Counts reflect eligible items under the current
// scope, filters, and search.
func (m *OutlinerModel) renderWorkModeStrip(width int) string {
	active := m.activeOutlinerMode()

	parts := make([]string, 0, len(workModeCycleOrder))
	for _, id := range workModeCycleOrder {
		mode, _ := OutlinerModeByID(id)
		label := fmt.Sprintf("%s %d", mode.Label, m.workModeItemCount(id))
		if id == active {
			parts = append(parts, activeTabStyle(1, false).Render(label))
		} else {
			parts = append(parts, inactiveTabStyle(1, false).Render(label))
		}
	}
	strip := strings.Join(parts, MutedStyle.Render("·"))

	var tail strings.Builder
	if !isWorkCycleMode(active) {
		mode, _ := OutlinerModeByID(active)
		tail.WriteString("  ")
		tail.WriteString(GetViewModeStyleForSection(mode.Label).Render("◆ " + mode.Label))
	}
	if m.isPlanningReorderMode() {
		tail.WriteString("  ")
		tail.WriteString(MutedStyle.Render("⇅ J/K reorder"))
	} else if m.isComputedOrderMode() {
		tail.WriteString("  ")
		tail.WriteString(MutedStyle.Render("⇅ auto-order"))
	}

	line := strip + tail.String()
	// The strip is budgeted as exactly one row by contentHeight, so it must never
	// wrap. Truncate to the available width at narrow sizes.
	if width > 0 && lipgloss.Width(line) > width {
		line = truncateToVisualWidth(line, width)
	}
	return line
}

// renderTodayProgress renders a one-line Today completion summary and the
// next-action recommendation. Shown only while the Today mode is active.
func (m *OutlinerModel) renderTodayProgress(width int) string {
	if !m.workTodayMode {
		return ""
	}
	projects := m.workModeScopeProjects()
	planIDs := m.resolveDailyPlanIDs(projects)

	total, done := 0, 0
	var nextAction *models.Task
	for _, p := range projects {
		if p == nil {
			continue
		}
		for _, t := range p.Tasks {
			if !planIDs[taskIdentityFor(t, p)] {
				continue
			}
			total++
			if t.IsTerminalState() {
				done++
				continue
			}
			if nextAction == nil && !t.IsSnoozed() {
				nextAction = t
			}
		}
	}

	if total == 0 {
		return MutedStyle.Render("Today: no planned tasks — press q to shortlist Next, w to plan Today")
	}

	summary := fmt.Sprintf("Today: %d/%d done", done, total)
	line := GetViewModeStyleForSection("Today").Render(summary)
	if nextAction != nil {
		line += "  " + MutedStyle.Render("→ next: ") + nextAction.Title
	} else if done == total {
		line += "  " + MutedStyle.Render("→ all done 🎉")
	}
	if width > 0 && lipgloss.Width(line) > width {
		line = truncateToVisualWidth(line, width)
	}
	return line
}

// workModeChromeHeaderLines reports how many extra header rows the work-mode
// chrome occupies so contentHeight can reserve them.
func (m *OutlinerModel) workModeChromeHeaderLines() int {
	lines := 1 // the work-mode strip is always shown
	if m.workTodayMode {
		lines++ // Today progress line
	}
	return lines
}

// writeWorkModeChrome appends the work-mode strip (and Today progress) to a
// header builder. Shared by the populated and empty-state header paths so the
// chrome stays identical in both.
func (m *OutlinerModel) writeWorkModeChrome(b *strings.Builder, width int) {
	b.WriteString(m.renderWorkModeStrip(width))
	b.WriteString("\n")
	if progress := m.renderTodayProgress(width); progress != "" {
		b.WriteString(progress)
		b.WriteString("\n")
	}
}
