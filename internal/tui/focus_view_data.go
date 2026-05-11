package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// loadTasks aggregates all actionable tasks from all active projects.
// Filters out completed, snoozed, and blocked tasks (via HasIncompleteDependencies).
func (m *FocusViewModel) loadTasks() {
	m.tasks = nil

	var failedProjects []string
	var loadedProjects []*models.Project
	for _, project := range m.config.GetActiveProjects() {
		if err := m.projectService.Load(project); err != nil {
			failedProjects = append(failedProjects, project.Name)
			continue
		}
		loadedProjects = append(loadedProjects, project)
	}

	m.tasks = focus.CollectActionableTasks(loadedProjects)

	if len(failedProjects) > 0 {
		m.setStatus(fmt.Sprintf("Failed to load: %s", strings.Join(failedProjects, ", ")))
	}

	// Reconcile queue task pointers after any task mutations in other views.
	if len(*m.queue) > 0 {
		taskByID := make(map[string]FocusTask, len(m.tasks))
		for _, ft := range m.tasks {
			taskByID[ft.Task.ID] = ft
		}

		q := *m.queue
		i := 0
		for i < len(q) {
			if fresh, ok := taskByID[q[i].Task.ID]; ok {
				q[i] = fresh
				i++
				continue
			}

			delete(m.queueIDs, q[i].Task.ID)
			q = append(q[:i], q[i+1:]...)
		}
		*m.queue = q
	}

	m.sortTasks()
	m.pruneDismissedIDs()
	m.recomputeSuggested()
}

// recomputeSuggested re-ranks suggestions, respecting showDismissed mode.
func (m *FocusViewModel) recomputeSuggested() {
	if m.showDismissed {
		all := focus.RankCrossProjectTasks(m.tasks, nil, m.config)
		m.ranked = nil
		for _, sft := range all {
			if m.dismissedIDs[sft.Task.ID] {
				m.ranked = append(m.ranked, sft)
			}
		}
	} else {
		m.ranked = focus.RankCrossProjectTasks(m.tasks, m.dismissedIDs, m.config)
	}

	m.applyFilter()
}

// applyFilter filters and sorts tasks based on the active tab, then applies sub-tab narrowing.
func (m *FocusViewModel) applyFilter() {
	now := time.Now()
	today := util.StartOfDay(now)

	switch m.tabs.Active {
	case focusTabQueue:
		m.applyQueueFilter()
	case focusTabSuggested:
		m.applySuggestedFilter()
	default:
		baseTasks := m.baseFilteredTasks()
		switch m.tabs.Active {
		case focusTabByProject:
			m.filtered = make([]FocusTask, len(baseTasks))
			copy(m.filtered, baseTasks)
			sort.SliceStable(m.filtered, func(i, j int) bool {
				return m.filtered[i].Project.Name < m.filtered[j].Project.Name
			})
		case focusTabByDueDate:
			m.filtered = nil
			for _, ft := range baseTasks {
				if ft.Task.DueDate == nil {
					continue
				}
				m.filtered = append(m.filtered, ft)
			}
		default:
			m.filtered = make([]FocusTask, len(baseTasks))
			copy(m.filtered, baseTasks)
		}
	}

	m.computeSubTabs()
	m.applySubTabFilter(now, today)
	m.expandFilteredChildren()
}

// computeSubTabs builds the dynamic sub-tab names based on active tab and filtered tasks.
func (m *FocusViewModel) computeSubTabs() {
	now := time.Now()
	today := util.StartOfDay(now)

	switch m.tabs.Active {
	case focusTabQueue, focusTabSuggested, focusTabByProject:
		seen := map[string]bool{}
		var names []string
		for _, ft := range m.filtered {
			if seen[ft.Project.Name] {
				continue
			}
			seen[ft.Project.Name] = true
			names = append(names, ft.Project.Name)
		}
		sort.Strings(names)
		m.subTabNames = names
	case focusTabByDueDate:
		m.computeDueDateSubTabs(now, today)
	default:
		m.subTabNames = nil
	}

	if m.subTab > len(m.subTabNames) {
		m.subTab = 0
	}
}

// activeSubTabFilter returns the selected sub-tab value, or "" if no sub-tab filtering.
func (m *FocusViewModel) activeSubTabFilter() string {
	if len(m.subTabNames) <= 1 || m.subTab <= 0 || m.subTab > len(m.subTabNames) {
		return ""
	}
	return m.subTabNames[m.subTab-1]
}

// dueBucket categorizes a task into a due date bucket.
func (m *FocusViewModel) dueBucket(task *models.Task, _, today time.Time) string {
	if task.DueDate == nil {
		return "Later"
	}

	dueDay := util.StartOfDay(*task.DueDate)
	y1, m1, d1 := today.Date()
	y2, m2, d2 := dueDay.Date()
	todayUTC := time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)
	dueUTC := time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC)
	daysUntil := int(dueUTC.Sub(todayUTC).Hours() / 24)

	switch {
	case daysUntil < 0:
		return "Overdue"
	case daysUntil == 0:
		return "Today"
	case daysUntil == 1:
		return "Tomorrow"
	case daysUntil <= 7:
		return "This Week"
	default:
		return "Later"
	}
}

// sortTasks sorts tasks by priority (ascending, 1=highest), then by UpdatedAt (most recent first).
func (m *FocusViewModel) sortTasks() {
	sort.Slice(m.tasks, func(i, j int) bool {
		if m.tasks[i].Task.Priority != m.tasks[j].Task.Priority {
			return m.tasks[i].Task.Priority < m.tasks[j].Task.Priority
		}
		return m.tasks[i].Task.UpdatedAt.After(m.tasks[j].Task.UpdatedAt)
	})
}

func (m *FocusViewModel) applyQueueFilter() {
	pruned := false
	q := *m.queue
	i := 0
	for i < len(q) {
		if q[i].Task.Status.IsComplete() {
			delete(m.queueIDs, q[i].Task.ID)
			q = append(q[:i], q[i+1:]...)
			pruned = true
			continue
		}
		i++
	}

	if pruned {
		*m.queue = q
		m.persistQueue()
	}

	m.filtered = make([]FocusTask, len(*m.queue))
	copy(m.filtered, *m.queue)
}

func (m *FocusViewModel) applySuggestedFilter() {
	m.filtered = nil
	var queued []FocusTask
	for _, sft := range m.ranked {
		if hasActiveCommonFilters(&m.filter) && !passesCommonFilters(sft.Task, &m.filter, sft.Project.ID, m.config) {
			continue
		}
		if m.isInQueue(sft.Task.ID) {
			queued = append(queued, sft.FocusTask)
			continue
		}
		m.filtered = append(m.filtered, sft.FocusTask)
	}
	if len(queued) > 0 {
		m.filtered = append(m.filtered, queued...)
	}
}

func (m *FocusViewModel) baseFilteredTasks() []FocusTask {
	if !hasActiveCommonFilters(&m.filter) {
		return m.tasks
	}

	var baseTasks []FocusTask
	for _, ft := range m.tasks {
		if passesCommonFilters(ft.Task, &m.filter, ft.Project.ID, m.config) {
			baseTasks = append(baseTasks, ft)
		}
	}
	return baseTasks
}

func (m *FocusViewModel) applySubTabFilter(now, today time.Time) {
	filter := m.activeSubTabFilter()
	if filter == "" {
		return
	}

	var narrowed []FocusTask
	for _, ft := range m.filtered {
		switch m.tabs.Active {
		case focusTabQueue, focusTabSuggested, focusTabByProject:
			if ft.Project.Name == filter {
				narrowed = append(narrowed, ft)
			}
		case focusTabByDueDate:
			if m.dueBucket(ft.Task, now, today) == filter {
				narrowed = append(narrowed, ft)
			}
		}
	}
	m.filtered = narrowed
}

func (m *FocusViewModel) computeDueDateSubTabs(now, today time.Time) {
	bucketOrder := []string{"Overdue", "Today", "Tomorrow", "This Week", "Later"}
	seen := map[string]bool{}
	for _, ft := range m.filtered {
		seen[m.dueBucket(ft.Task, now, today)] = true
	}

	m.subTabNames = nil
	for _, bucket := range bucketOrder {
		if seen[bucket] {
			m.subTabNames = append(m.subTabNames, bucket)
		}
	}
}

func (m *FocusViewModel) toggleDismiss() {
	if m.tabs.Active != focusTabSuggested {
		return
	}
	ft := m.getTaskAtCursor()
	if ft == nil {
		return
	}
	taskID := ft.Task.ID

	if m.dismissedIDs[taskID] {
		delete(m.dismissedIDs, taskID)
		m.setStatus(fmt.Sprintf("Restored: %s", ft.Task.Title))
	} else {
		m.dismissedIDs[taskID] = true
		m.setStatus(fmt.Sprintf("Dismissed: %s", ft.Task.Title))
	}
	m.persistDismissed()
	m.recomputeSuggested()
	m.clampCursor()
}

// expandFilteredChildren re-orders m.filtered so that child tasks appear
// indented under their parents using tree connectors. Tasks whose parent is
// also in m.filtered are skipped at the top level and reinserted as children.
func (m *FocusViewModel) expandFilteredChildren() {
	filteredIDs := make(map[string]bool, len(m.filtered))
	for _, ft := range m.filtered {
		filteredIDs[ft.Task.ID] = true
	}

	var expanded []FocusTask
	for _, ft := range m.filtered {
		if ft.Task.ParentID != nil && filteredIDs[*ft.Task.ParentID] {
			continue
		}
		expanded = append(expanded, ft)
		m.collectBacklogChildren(&expanded, ft.Task, ft.Project, nil, filteredIDs)
	}
	m.filtered = expanded
}

// collectBacklogChildren recursively appends children of parent that are
// present in filteredIDs, setting TreePrefix via buildBoardTreePrefix.
func (m *FocusViewModel) collectBacklogChildren(out *[]FocusTask, parent *models.Task, project *models.Project, ancestorIsLast []bool, filteredIDs map[string]bool) {
	var visible []*models.Task
	for _, child := range parent.Children {
		if filteredIDs[child.ID] {
			visible = append(visible, child)
		}
	}
	for i, child := range visible {
		isLast := i == len(visible)-1
		*out = append(*out, FocusTask{
			Task:       child,
			Project:    project,
			TreePrefix: buildBoardTreePrefix(ancestorIsLast, isLast),
		})
		next := make([]bool, len(ancestorIsLast)+1)
		copy(next, ancestorIsLast)
		next[len(ancestorIsLast)] = isLast
		m.collectBacklogChildren(out, child, project, next, filteredIDs)
	}
}

// parentRowIndices returns the indices of rows in m.filtered that are parent
// tasks (TreePrefix == ""), used by n/N navigation to jump between parents.
func (m *FocusViewModel) parentRowIndices() []int {
	var indices []int
	for i, ft := range m.filtered {
		if ft.TreePrefix == "" {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *FocusViewModel) pruneDismissedIDs() {
	if len(m.dismissedIDs) == 0 {
		return
	}

	actionableIDs := make(map[string]bool, len(m.tasks))
	for _, ft := range m.tasks {
		actionableIDs[ft.Task.ID] = true
	}

	pruned := false
	for id := range m.dismissedIDs {
		if actionableIDs[id] {
			continue
		}
		delete(m.dismissedIDs, id)
		pruned = true
	}

	if pruned {
		m.persistDismissed()
	}
}
