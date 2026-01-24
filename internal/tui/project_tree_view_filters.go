package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// renderViewModeIndicator returns a styled indicator for the current view mode with counts.
func (m *ProjectTreeModel) renderViewModeIndicator() string {
	// Count by section: active, next, inbox, waiting, someday, completed
	// Skip notes - they don't count toward section totals
	var activeCount, nextCount, inboxCount, waitingCount, somedayCount, completedCount int
	for _, task := range m.project.Tasks {
		if task.IsNote() {
			continue
		}
		if task.IsComplete() {
			completedCount++
			continue
		}
		switch task.Section {
		case models.SectionNext:
			nextCount++
		case models.SectionInbox:
			inboxCount++
		case models.SectionWaiting:
			waitingCount++
		case models.SectionSomeday:
			somedayCount++
		default:
			// Empty section = Active
			activeCount++
		}
	}

	// Build indicator for each view with counts
	var parts []string

	// Active view
	activeLabel := fmt.Sprintf("Active[%d]", activeCount)
	if m.viewMode == ViewModeActive {
		parts = append(parts, ViewModeActiveStyle.Render(activeLabel))
	} else {
		parts = append(parts, MutedStyle.Render(activeLabel))
	}

	// Next view
	nextLabel := fmt.Sprintf("Next[%d]", nextCount)
	if m.viewMode == ViewModeNext {
		parts = append(parts, ViewModeNextStyle.Render(nextLabel))
	} else {
		parts = append(parts, MutedStyle.Render(nextLabel))
	}

	// Inbox view
	inboxLabel := fmt.Sprintf("Inbox[%d]", inboxCount)
	if m.viewMode == ViewModeInbox {
		parts = append(parts, ViewModeInboxStyle.Render(inboxLabel))
	} else {
		parts = append(parts, MutedStyle.Render(inboxLabel))
	}

	// Waiting view
	waitingLabel := fmt.Sprintf("Waiting[%d]", waitingCount)
	if m.viewMode == ViewModeWaiting {
		parts = append(parts, ViewModeWaitingStyle.Render(waitingLabel))
	} else {
		parts = append(parts, MutedStyle.Render(waitingLabel))
	}

	// Someday view
	somedayLabel := fmt.Sprintf("Someday[%d]", somedayCount)
	if m.viewMode == ViewModeSomeday {
		parts = append(parts, ViewModeSomedayStyle.Render(somedayLabel))
	} else {
		parts = append(parts, MutedStyle.Render(somedayLabel))
	}

	return strings.Join(parts, " ")
}

func (m *ProjectTreeModel) renderFilterBar() string {
	var result strings.Builder

	// If filterBar component is not initialized, return empty string
	if m.filterBar == nil {
		return ""
	}

	if !m.filterBar.IsEnabled() {
		// When filter bar is disabled, just show view mode indicator
		result.WriteString(m.renderViewModeIndicator())
		result.WriteString("\n")
		return result.String()
	}

	// Line 1: Filter badges (F1-F7)
	var badges []string

	// F1: Completed view (switch to Completed view)
	count := m.filterBar.GetCounts()["completed"]
	active := m.viewMode == ViewModeCompleted
	badges = append(badges, m.renderFilterBadge("F1", "Done", count, active))

	// F2: High Priority
	count = m.filterBar.GetCounts()["highPriority"]
	active = m.filter.Priority == 1
	badges = append(badges, m.renderFilterBadge("F2", "!Hi", count, active))

	// F3: Type (show current type if filtered)
	typeName := "Type"
	if m.filter.Type != "" {
		typeName = string(m.filter.Type)
		count = m.filterBar.GetCounts()["type:"+string(m.filter.Type)]
	} else {
		count = len(m.project.Tasks)
	}
	active = m.filter.Type != ""
	badges = append(badges, m.renderFilterBadge("F3", typeName, count, active))

	// F4: Status (show current status if filtered)
	statusName := "Status"
	if m.filter.Status != "" {
		statusName = string(m.filter.Status)
		count = m.filterBar.GetCounts()["status:"+string(m.filter.Status)]
	} else {
		count = len(m.project.Tasks)
	}
	active = m.filter.Status != ""
	badges = append(badges, m.renderFilterBadge("F4", statusName, count, active))

	// F5: Tags (show count of selected tags)
	tagCount := len(m.filter.Tags)
	if tagCount > 0 {
		badges = append(badges, m.renderFilterBadge("F5", fmt.Sprintf("#%dtag", tagCount), tagCount, true))
	} else {
		badges = append(badges, m.renderFilterBadge("F5", "Tags", 0, false))
	}

	// F6: Section (show count of selected sections)
	sectionCount := len(m.filter.Sections)
	if sectionCount > 0 {
		badges = append(badges, m.renderFilterBadge("F6", fmt.Sprintf("@%dsec", sectionCount), sectionCount, true))
	} else {
		badges = append(badges, m.renderFilterBadge("F6", "Sect", 0, false))
	}

	// F7: Due Date
	dueName := "Due"
	switch m.filter.DueDate {
	case DueDateOverdue:
		dueName = "Ovrd"
		count = m.filterBar.GetCounts()["overdue"]
	case DueDateToday:
		dueName = "Today"
		count = m.filterBar.GetCounts()["today"]
	case DueDateThisWeek:
		dueName = "Week"
		count = m.filterBar.GetCounts()["thisWeek"]
	default:
		count = 0
	}
	active = m.filter.DueDate != DueDateAll
	badges = append(badges, m.renderFilterBadge("F7", dueName, count, active))

	// F8: Actionable (tasks ready to work on)
	count = m.filterBar.GetCounts()["actionable"]
	active = m.filter.Actionable
	badges = append(badges, m.renderFilterBadge("F8", "Ready", count, active))

	result.WriteString(strings.Join(badges, " "))
	result.WriteString("\n")

	// Line 2: View mode indicator
	result.WriteString(m.renderViewModeIndicator())
	result.WriteString("\n")

	return result.String()
}

// renderFilterBadge renders a single filter badge with key, name, count, and active state.
func (m *ProjectTreeModel) renderFilterBadge(key, name string, count int, active bool) string {
	var countStr string
	if count > 0 {
		countStr = fmt.Sprintf("[%d]", count)
	}

	label := fmt.Sprintf("%s:%s%s", key, name, countStr)

	if active {
		return FilterBadgeStyle.Render(label)
	}
	return FilterBadgeInactiveStyle.Render(label)
}

// renderChildrenInOtherSectionsBadge returns a badge showing how many children
// are in sections hidden by the current view mode.
func (m *ProjectTreeModel) renderChildrenInOtherSectionsBadge(task *models.Task) string {
	hiddenSections := m.getHiddenSectionsForViewMode()
	if len(hiddenSections) == 0 {
		return ""
	}

	counts, ok := m.filterBar.GetChildrenInOtherSections()[task.ID]
	if !ok || len(counts) == 0 {
		return ""
	}

	var parts []string
	for _, section := range hiddenSections {
		if count, exists := counts[section]; exists && count > 0 {
			parts = append(parts, fmt.Sprintf("+%d %s", count, section))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return ChildrenBadgeStyle.Render("[" + strings.Join(parts, ", ") + "]")
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
// Returns empty string if no prefix is pending.
func (m *ProjectTreeModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"z": FoldPrefixHints,
		"g": ProjectTreeNavigationPrefixHints,
		"!": PriorityPrefixHints,
		"d": DueDatePrefixHints,
	}, m.width)
}

// getViewModeTitle returns the card title based on current view mode.
// If a single section is filtered, appends the section name.
func (m *ProjectTreeModel) getViewModeTitle() string {
	var base string
	switch m.viewMode {
	case ViewModeActive:
		base = "Active"
	case ViewModeNext:
		base = "Next"
	case ViewModeInbox:
		base = "Inbox"
	case ViewModeWaiting:
		base = "Waiting"
	case ViewModeSomeday:
		base = "Someday"
	case ViewModeCompleted:
		base = "Completed"
	default:
		base = "Tasks"
	}

	// Append section name if exactly one section is filtered
	if len(m.filter.Sections) == 1 {
		for name := range m.filter.Sections {
			base = base + " · @" + name
			break
		}
	}

	return base
}

// getSingleFilteredSectionColor returns the section color if exactly one section
// is being filtered, otherwise returns empty string.
func (m *ProjectTreeModel) getSingleFilteredSectionColor() string {
	if len(m.filter.Sections) != 1 {
		return ""
	}
	// Get the single filtered section name
	var sectionName string
	for name := range m.filter.Sections {
		sectionName = name
		break
	}
	return m.project.GetSectionColor(sectionName)
}
