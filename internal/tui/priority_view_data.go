package tui

import (
	"sort"
	"strings"

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
		default:
		}
	}

	// Sort each group by urgency score (higher = more urgent). Use a stable
	// sort because UrgencyScore is a coarse integer and ties are common; an
	// unstable sort would flip order between reloads.
	sortByUrgency := func(tasks []*PriorityTask) {
		sort.SliceStable(tasks, func(i, j int) bool {
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
	switch m.tabs.Active {
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
	switch m.tabs.Active {
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
	if m.project == nil || m.tabs.Active == priorityTabAll || m.tabs.Active == priorityTabByScore {
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

	m.subTabNames = nil

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

// filterBySubTab narrows a task slice by the active sub-tab filter.
func (m *PriorityViewModel) filterBySubTab(tasks []*PriorityTask) []*PriorityTask {
	return tasks
}

// applySearchFilter returns tasks whose title or description contains the search query (case-insensitive).
func (m *PriorityViewModel) applySearchFilter(tasks []*PriorityTask) []*PriorityTask {
	if m.searchQuery == "" {
		return tasks
	}
	q := strings.ToLower(m.searchQuery)
	var result []*PriorityTask
	for _, pt := range tasks {
		if strings.Contains(strings.ToLower(pt.Task.Title), q) ||
			strings.Contains(strings.ToLower(pt.Task.Description), q) {
			result = append(result, pt)
		}
	}
	return result
}

// visibleScoredTasks returns m.scoredTasks filtered by the current search query.
func (m *PriorityViewModel) visibleScoredTasks() []*PriorityTask {
	return m.applySearchFilter(m.scoredTasks)
}

// filteredGroupsWithSubTab returns filtered groups further narrowed by the active sub-tab and search query.
func (m *PriorityViewModel) filteredGroupsWithSubTab() (overdue, today, thisWeek, later, backlog, waitingOn []*PriorityTask) {
	o, td, tw, l, bl, wo := m.filteredGroups()
	return m.applySearchFilter(m.filterBySubTab(o)),
		m.applySearchFilter(m.filterBySubTab(td)),
		m.applySearchFilter(m.filterBySubTab(tw)),
		m.applySearchFilter(m.filterBySubTab(l)),
		m.applySearchFilter(m.filterBySubTab(bl)),
		m.applySearchFilter(m.filterBySubTab(wo))
}

// recalculateNavigation updates totalTasks and groupStarts based on the active tab.
func (m *PriorityViewModel) recalculateNavigation() {
	m.resetViewScrollOffset()
	m.computeSubTabs()

	// By Score tab uses visibleScoredTasks (search-filtered)
	if m.tabs.Active == priorityTabByScore {
		m.subTabNames = nil
		m.totalTasks = len(m.visibleScoredTasks())
		m.groupStarts = []int{0}
		return
	}

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	// For the Blocked tab, waiting tasks are navigable (not collapsible)
	if m.tabs.Active == priorityTabBlocked {
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
