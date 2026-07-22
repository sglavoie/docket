package tui

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const crossDueDateCollapseKeyPrefix = "_xpd:"

type crossDueDateTask struct {
	task      *models.Task
	projectID string
}

type crossDueDateBucket struct {
	name  string
	slug  string
	tasks []crossDueDateTask
}

// buildCrossProjectDueDateRows restores the retired All Tasks schedule as a
// lens inside the canonical cross-project Outliner. Context-only ancestors
// retained by filterTaskTree are excluded from this flat presentation.
func (m *OutlinerModel) buildCrossProjectDueDateRows(groups []projectTasksGroup) {
	var tasks []crossDueDateTask
	for _, group := range groups {
		project := m.crossResolveProject(group.projectID)
		if project == nil {
			continue
		}
		walkTaskTree(group.tasks, func(task *models.Task) {
			if m.crossTaskMatchesAllFilters(task, project) {
				tasks = append(tasks, crossDueDateTask{task: task, projectID: group.projectID})
			}
		})
	}

	// Stable input order is project display order followed by source task order,
	// which becomes the deterministic tie-break for tasks sharing a due date.
	sort.SliceStable(tasks, func(i, j int) bool {
		left, right := tasks[i].task.DueDate, tasks[j].task.DueDate
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.Before(*right)
	})

	buckets := []crossDueDateBucket{
		{name: "Overdue", slug: "overdue"},
		{name: "Today", slug: "today"},
		{name: "This Week", slug: "this-week"},
		{name: "Later", slug: "later"},
		{name: "No Date", slug: "no-date"},
	}
	today := util.StartOfDay(time.Now())
	weekEnd := today.AddDate(0, 0, 7)
	for _, item := range tasks {
		index := 4
		if item.task.DueDate != nil {
			due := util.StartOfDay(*item.task.DueDate)
			switch {
			case due.Before(today):
				index = 0
			case due.Equal(today):
				index = 1
			case due.Before(weekEnd):
				index = 2
			default:
				index = 3
			}
		}
		buckets[index].tasks = append(buckets[index].tasks, item)
	}

	for _, bucket := range buckets {
		if len(bucket.tasks) == 0 {
			continue
		}
		collapseKey := crossDueDateCollapseKeyPrefix + bucket.slug
		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader: true,
			IsLensHeader:    true,
			SectionName:     bucket.name,
			SectionCount:    len(bucket.tasks),
			CollapseKey:     collapseKey,
		})
		if m.collapsedSections[collapseKey] {
			continue
		}
		for _, item := range bucket.tasks {
			m.rows = append(m.rows, outlinerRow{task: item.task, ProjectID: item.projectID})
		}
	}
}

func walkTaskTree(tasks []*models.Task, visit func(*models.Task)) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		visit(task)
		walkTaskTree(task.Children, visit)
	}
}

func (m *OutlinerModel) crossTaskMatchesAllFilters(task *models.Task, project *models.Project) bool {
	if task == nil || project == nil || !m.baseFocusFilter(project)(task) {
		return false
	}
	if tag := m.activePinnedTagFilter(); tag != "" && !task.HasTag(tag) {
		return false
	}
	if query := m.SearchQuery(); query != "" && !matchesSearch(task, query, m.runtime.Planning()) {
		return false
	}
	if hasActiveCommonFilters(&m.filter.CommonFilterState) &&
		!passesCommonFilters(task, &m.filter.CommonFilterState, project, m.runtime.Automation()) {
		return false
	}
	if m.activeCriteriaFilter != nil &&
		!passesSavedFilterCriteria(task, project, m.activeCriteriaFilter, m.runtime.Automation()) {
		return false
	}
	if len(m.filter.Sections) > 0 && !m.filter.Sections[taskSectionBucket(task)] {
		return false
	}
	return !m.filter.Actionable || taskIsActionableState(task, project)
}
