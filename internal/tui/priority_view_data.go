package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// loadActionableTasks categorizes and sorts tasks by urgency.
func (m *PriorityViewModel) loadActionableTasks() {
	m.overdue = nil
	m.today = nil
	m.thisWeek = nil
	m.later = nil
	m.backlog = nil
	m.waitingOn = nil

	for _, task := range m.project.Tasks {
		// Skip completed tasks
		if task.Status.IsComplete() {
			continue
		}

		// Skip snoozed tasks
		if task.IsSnoozed() {
			continue
		}

		// Check if blocked (status=blocked or has incomplete dependencies)
		isBlocked := task.Status == models.StatusBlocked || m.project.HasIncompleteDependencies(task)

		pt := &PriorityTask{
			Task:         task,
			UrgencyScore: m.computeUrgencyScore(task),
			ParentTitle:  m.getParentTitle(task),
			RelativeDue:  FormatRelativeDue(task.DueDate, false),
		}

		if isBlocked {
			m.waitingOn = append(m.waitingOn, pt)
			continue
		}

		// Categorize by due date
		group := m.categorizeByDueDate(task)
		switch group {
		case PriorityGroupOverdue:
			m.overdue = append(m.overdue, pt)
		case PriorityGroupToday:
			m.today = append(m.today, pt)
		case PriorityGroupThisWeek:
			m.thisWeek = append(m.thisWeek, pt)
		case PriorityGroupLater:
			m.later = append(m.later, pt)
		case PriorityGroupBacklog:
			m.backlog = append(m.backlog, pt)
		}
	}

	// Sort each group by urgency score (higher = more urgent)
	sortByUrgency := func(tasks []*PriorityTask) {
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].UrgencyScore > tasks[j].UrgencyScore
		})
	}
	sortByUrgency(m.overdue)
	sortByUrgency(m.today)
	sortByUrgency(m.thisWeek)
	sortByUrgency(m.later)
	sortByUrgency(m.backlog)
	sortByUrgency(m.waitingOn)

	// Apply sub-filters to each group
	m.overdue = m.applyFiltersToGroup(m.overdue)
	m.today = m.applyFiltersToGroup(m.today)
	m.thisWeek = m.applyFiltersToGroup(m.thisWeek)
	m.later = m.applyFiltersToGroup(m.later)
	m.backlog = m.applyFiltersToGroup(m.backlog)
	m.waitingOn = m.applyFiltersToGroup(m.waitingOn)

	// Compute score-ranked list if a scoring profile is assigned
	m.scoredTasks = nil
	if m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
			var all []*PriorityTask
			all = append(all, m.overdue...)
			all = append(all, m.today...)
			all = append(all, m.thisWeek...)
			all = append(all, m.later...)
			all = append(all, m.backlog...)
			sort.SliceStable(all, func(i, j int) bool {
				return scoring.ComputeScore(all[i].Task, profile) > scoring.ComputeScore(all[j].Task, profile)
			})
			m.scoredTasks = all
		}
	}

	// Recalculate navigation for the active tab
	m.recalculateNavigation()

	// Compute the smart suggestion
	m.computeSuggestion()
}

// activeTabHint returns a brief description of the active tab's purpose for the footer.
func (m *PriorityViewModel) activeTabHint() string {
	switch m.activeTab {
	case priorityTabUrgent:
		return " · Showing overdue and due today"
	case priorityTabUpcoming:
		return " · Showing this week and later"
	case priorityTabBacklog:
		return " · Showing tasks with no due date"
	case priorityTabBlocked:
		return " · Showing blocked and waiting tasks"
	case priorityTabByScore:
		return " · Ranked by composite score"
	default:
		return ""
	}
}

// filteredGroups returns the task groups visible for the current tab.
// Returns overdue, today, thisWeek, later, backlog, waitingOn slices filtered by active tab.
func (m *PriorityViewModel) filteredGroups() (overdue, today, thisWeek, later, backlog, waitingOn []*PriorityTask) {
	switch m.activeTab {
	case priorityTabUrgent:
		return m.overdue, m.today, nil, nil, nil, nil
	case priorityTabUpcoming:
		return nil, nil, m.thisWeek, m.later, nil, nil
	case priorityTabBacklog:
		return nil, nil, nil, nil, m.backlog, nil
	case priorityTabBlocked:
		return nil, nil, nil, nil, nil, m.waitingOn
	case priorityTabByScore:
		return nil, nil, nil, nil, nil, nil
	default: // priorityTabAll
		return m.overdue, m.today, m.thisWeek, m.later, m.backlog, m.waitingOn
	}
}

// computeSubTabs builds dynamic sub-tab names based on active tab and current groups.
func (m *PriorityViewModel) computeSubTabs() {
	if m.project == nil || m.activeTab == priorityTabAll || m.activeTab == priorityTabByScore {
		m.subTabNames = nil
		if m.subTab > len(m.subTabNames) {
			m.subTab = 0
		}
		return
	}

	// Collect tasks visible for the current tab
	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroups()
	var tasks []*PriorityTask
	tasks = append(tasks, overdue...)
	tasks = append(tasks, today...)
	tasks = append(tasks, thisWeek...)
	tasks = append(tasks, later...)
	tasks = append(tasks, backlog...)
	tasks = append(tasks, waitingOn...)

	// Extract unique custom sections from these tasks
	seen := map[string]bool{}
	var names []string
	for _, pt := range tasks {
		section := pt.Task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if !seen[section] {
			seen[section] = true
			names = append(names, section)
		}
	}
	sort.Strings(names)
	m.subTabNames = names

	if m.subTab > len(m.subTabNames) {
		m.subTab = 0
	}
}

// activeSubTabFilter returns the selected sub-tab value, or "" if no sub-tab filtering.
func (m *PriorityViewModel) activeSubTabFilter() string {
	if len(m.subTabNames) <= 1 || m.subTab <= 0 || m.subTab > len(m.subTabNames) {
		return ""
	}
	return m.subTabNames[m.subTab-1]
}

// filterBySubTab narrows a task slice by the active sub-tab custom section filter.
func (m *PriorityViewModel) filterBySubTab(tasks []*PriorityTask) []*PriorityTask {
	filter := m.activeSubTabFilter()
	if filter == "" {
		return tasks
	}
	var result []*PriorityTask
	for _, pt := range tasks {
		section := pt.Task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if section == filter {
			result = append(result, pt)
		}
	}
	return result
}

// filteredGroupsWithSubTab returns filtered groups further narrowed by the active sub-tab.
func (m *PriorityViewModel) filteredGroupsWithSubTab() (overdue, today, thisWeek, later, backlog, waitingOn []*PriorityTask) {
	o, td, tw, l, bl, wo := m.filteredGroups()
	return m.filterBySubTab(o), m.filterBySubTab(td), m.filterBySubTab(tw), m.filterBySubTab(l), m.filterBySubTab(bl), m.filterBySubTab(wo)
}

// recalculateNavigation updates totalTasks and groupStarts based on the active tab.
func (m *PriorityViewModel) recalculateNavigation() {
	m.computeSubTabs()

	// By Score tab uses scoredTasks directly
	if m.activeTab == priorityTabByScore {
		m.subTabNames = nil
		m.totalTasks = len(m.scoredTasks)
		m.groupStarts = []int{0}
		return
	}

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	// For the Blocked tab, waiting tasks are navigable (not collapsible)
	if m.activeTab == priorityTabBlocked {
		m.totalTasks = len(waitingOn)
		m.groupStarts = []int{0}
	} else {
		m.totalTasks = len(overdue) + len(today) + len(thisWeek) + len(later) + len(backlog)
		m.groupStarts = []int{
			0,
			len(overdue),
			len(overdue) + len(today),
			len(overdue) + len(today) + len(thisWeek),
			len(overdue) + len(today) + len(thisWeek) + len(later),
		}
	}
}
