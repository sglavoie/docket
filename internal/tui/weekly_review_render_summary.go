package tui

import (
	"fmt"
	"strings"
)

// countByType returns separate counts of project items and task items.
func (m *WeeklyReviewModel) countByType() (int, int) {
	projectCount := len(m.staleProjects) + len(m.inboxBacklog) + len(m.blockedWork) + len(m.overcommitted)
	taskCount := len(m.staleTasks) + len(m.priorityMismatch) + len(m.upcomingWeek) + len(m.recurringOverview)
	return projectCount, taskCount
}

// renderTabBar renders the tab bar.
func (m *WeeklyReviewModel) renderTabBar() string {
	tabCounts := []int{
		len(m.staleProjects) + len(m.inboxBacklog) + len(m.blockedWork) + len(m.overcommitted),
		len(m.staleTasks) + len(m.priorityMismatch),
		len(m.upcomingWeek) + len(m.recurringOverview),
	}
	labels := make([]string, len(m.tabs.Names))
	for i, name := range m.tabs.Names {
		labels[i] = fmt.Sprintf("%s (%d)", name, tabCounts[i])
	}
	emptyTabs := make([]bool, len(tabCounts))
	for i, c := range tabCounts {
		emptyTabs[i] = c == 0
	}
	return renderWrappingTabBarWithEmpty(labels, m.tabs.Active, emptyTabs, m.width, m.AccentColor())
}

// renderCompletionSummary renders the completion summary overlay.
func (m *WeeklyReviewModel) renderCompletionSummary() string {
	var b strings.Builder

	header := SuccessStyle.Bold(true).Render("Review Complete")
	b.WriteString(header)
	b.WriteString("\n\n")

	// Mutation summary
	totalMutations := 0
	for _, count := range m.mutationCounts {
		totalMutations += count
	}
	if totalMutations > 0 {
		fmt.Fprintf(&b, "Changes made during this review: %d\n", totalMutations)
		categoryLabels := []struct {
			key   string
			label string
		}{
			{"status", "Status changes"},
			{"priority", "Priority changes"},
			{"due_date", "Due date changes"},
			{"move", "Section moves"},
			{"delete", "Deletions"},
			{"tag", "Tag changes"},
			{"snooze", "Snoozes"},
		}
		for _, cl := range categoryLabels {
			if count, ok := m.mutationCounts[cl.key]; ok && count > 0 {
				fmt.Fprintf(&b, "  %s: %d\n", cl.label, count)
			}
		}
	} else {
		b.WriteString("No changes made during this review.\n")
	}
	b.WriteString("\n")

	// Remaining items
	hasRemaining := false
	allSections := []WeeklyReviewSection{
		WeeklySectionStaleProjects, WeeklySectionInboxBacklog,
		WeeklySectionBlockedWork, WeeklySectionOvercommitted,
		WeeklySectionStaleTasks, WeeklySectionPriorityMismatch,
		WeeklySectionUpcomingWeek, WeeklySectionRecurringOverview,
	}
	for _, s := range allSections {
		count := m.getSectionItemCount(s)
		if count > 0 {
			if !hasRemaining {
				b.WriteString("Remaining items:\n")
				hasRemaining = true
			}
			fmt.Fprintf(&b, "  %s: %d\n", getWeeklySectionName(s), count)
		}
	}
	if !hasRemaining {
		b.WriteString(SuccessStyle.Render("All sections cleared!"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(HelpStyle.Render("Press Esc to close, any other key to return to review"))

	return b.String()
}
