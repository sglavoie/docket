package tui

import "github.com/sglavoie/tt/internal/models"

// getAllTags returns all unique tags from the project's tasks.
func (m *ProjectTreeModel) getAllTags() []string {
	return CollectProjectTagNames(m.project.Tasks)
}

// calculateFilterCounts calculates task counts for each filter category.
// This enables the filter bar to show how many tasks match each filter.
func (m *ProjectTreeModel) calculateFilterCounts() {
	counts := make(map[string]int)
	for _, task := range m.project.Tasks {
		// Count completed tasks
		if task.IsComplete() {
			counts["completed"]++
		}
		// Count high priority tasks (priority 1-2)
		if task.Priority <= 2 {
			counts["highPriority"]++
		}
		// Count by type
		counts["type:"+string(task.Type)]++
		// Count by status
		counts["status:"+string(task.Status)]++
		// Count by due date category
		if task.IsOverdue() {
			counts["overdue"]++
		}
		if task.IsDueToday() {
			counts["today"]++
		}
		if task.IsDueThisWeek() {
			counts["thisWeek"]++
		}
		// Count by section (check both GTD and custom section fields)
		if task.Section != "" {
			counts["section:"+task.Section]++
		}
		if task.CustomSection != "" {
			counts["section:"+task.CustomSection]++
		}
		// Count by tag
		for _, tag := range task.Tags {
			counts["tag:"+tag]++
		}
		// Count actionable tasks (ready to work on)
		if !task.IsComplete() &&
			!m.project.HasIncompleteDependencies(task) &&
			task.Status != models.StatusBlocked &&
			task.Section != models.SectionInbox &&
			task.Section != models.SectionNext &&
			task.Section != models.SectionWaiting &&
			task.Section != models.SectionSomeday &&
			task.Section != models.SectionReview &&
			task.Section != models.SectionStandby {
			counts["actionable"]++
		}
	}

	// Only set counts if filterBar component is initialized
	if m.filterBar != nil {
		m.filterBar.SetCounts(counts)

		// Calculate children in other sections for each task with children
		childrenInOtherSections := make(map[string]map[string]int)
		for _, task := range m.project.Tasks {
			if len(task.Children) > 0 {
				childrenInOtherSections[task.ID] = m.countChildrenBySection(task)
			}
		}
		m.filterBar.SetChildrenInOtherSections(childrenInOtherSections)
	}
}

// countChildrenBySection recursively counts all descendants by their section.
// Returns a map of section name -> count. Empty section is normalized to "Active".
func (m *ProjectTreeModel) countChildrenBySection(task *models.Task) map[string]int {
	counts := make(map[string]int)
	for _, child := range task.Children {
		section := child.Section
		if section == "" {
			section = "Active"
		}
		counts[section]++
		// Recursively count descendants
		for s, c := range m.countChildrenBySection(child) {
			counts[s] += c
		}
	}
	return counts
}

// getHiddenSectionsForViewMode returns the sections that are hidden by the current view mode.
// Returns nil when all sections are visible (no filtering).
func (m *ProjectTreeModel) getHiddenSectionsForViewMode() []string {
	switch m.viewMode {
	case ViewModeActive:
		return []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionSomeday, models.SectionReview, models.SectionStandby, models.SectionCompleted}
	case ViewModeNext:
		return []string{models.SectionInbox, models.SectionWaiting, models.SectionSomeday, models.SectionReview, models.SectionStandby, models.SectionCompleted, "Active"}
	case ViewModeInbox:
		return []string{models.SectionNext, models.SectionWaiting, models.SectionSomeday, models.SectionReview, models.SectionStandby, models.SectionCompleted, "Active"}
	case ViewModeWaiting:
		return []string{models.SectionInbox, models.SectionNext, models.SectionSomeday, models.SectionReview, models.SectionStandby, models.SectionCompleted, "Active"}
	case ViewModeSomeday:
		return []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionReview, models.SectionStandby, models.SectionCompleted, "Active"}
	case ViewModeReview:
		return []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionSomeday, models.SectionStandby, models.SectionCompleted, "Active"}
	case ViewModeStandby:
		return []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionSomeday, models.SectionReview, models.SectionCompleted, "Active"}
	case ViewModeAll:
		return []string{models.SectionCompleted} // Hide Completed from All view (use Completed view instead)
	default:
	}
	return nil
}
