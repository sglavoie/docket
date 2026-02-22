package tui

import (
	"fmt"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// groupTasks groups tasks based on the current groupBy setting.
func (m *AllTasksViewModel) groupTasks(tasks []AllTasksItem) []AllTasksGroup {
	switch m.groupBy {
	case ATGroupByProject:
		return m.groupByProject(tasks)
	case ATGroupByStatus:
		return m.groupByStatus(tasks)
	case ATGroupByPriority:
		return m.groupByPriority(tasks)
	case ATGroupBySection:
		return m.groupBySection(tasks)
	case ATGroupByDueDate:
		return m.groupByDueDate(tasks)
	case ATGroupByFlat:
		if len(tasks) == 0 {
			return nil
		}
		return []AllTasksGroup{{Name: "All Tasks", Tasks: tasks}}
	default:
		return m.groupByProject(tasks)
	}
}

// groupByProject groups tasks by their parent project name.
func (m *AllTasksViewModel) groupByProject(tasks []AllTasksItem) []AllTasksGroup {
	groups := make(map[string][]AllTasksItem)
	var order []string

	for _, item := range tasks {
		name := item.Project.Name
		if _, exists := groups[name]; !exists {
			order = append(order, name)
		}
		groups[name] = append(groups[name], item)
	}

	sort.Strings(order)

	var result []AllTasksGroup
	for _, name := range order {
		result = append(result, AllTasksGroup{Name: name, Tasks: groups[name]})
	}
	return result
}

// groupByStatus groups tasks by their workflow status in a defined order.
func (m *AllTasksViewModel) groupByStatus(tasks []AllTasksItem) []AllTasksGroup {
	statusOrder := []models.TaskStatus{
		models.StatusTodo,
		models.StatusInProgress,
		models.StatusReview,
		models.StatusBlocked,
		models.StatusDone,
		models.StatusCanceled,
	}

	groups := make(map[models.TaskStatus][]AllTasksItem)
	for _, item := range tasks {
		groups[item.Task.Status] = append(groups[item.Task.Status], item)
	}

	var result []AllTasksGroup
	for _, status := range statusOrder {
		if items, ok := groups[status]; ok && len(items) > 0 {
			result = append(result, AllTasksGroup{
				Name:  string(status),
				Tasks: items,
			})
		}
	}
	return result
}

// groupByPriority groups tasks by priority level.
func (m *AllTasksViewModel) groupByPriority(tasks []AllTasksItem) []AllTasksGroup {
	groups := make(map[int][]AllTasksItem)
	for _, item := range tasks {
		groups[item.Task.Priority] = append(groups[item.Task.Priority], item)
	}

	// Sort priority keys
	var priorities []int
	for p := range groups {
		priorities = append(priorities, p)
	}
	sort.Ints(priorities)

	var result []AllTasksGroup
	for _, p := range priorities {
		name := fmt.Sprintf("P%d", p)
		if p == 0 || p > 5 {
			name = "Unset"
		}
		result = append(result, AllTasksGroup{Name: name, Tasks: groups[p]})
	}
	return result
}

// groupBySection groups tasks by their GTD section.
func (m *AllTasksViewModel) groupBySection(tasks []AllTasksItem) []AllTasksGroup {
	groups := make(map[string][]AllTasksItem)
	var order []string

	for _, item := range tasks {
		section := item.Task.Section
		if section == "" {
			section = "(none)"
		}
		if _, exists := groups[section]; !exists {
			order = append(order, section)
		}
		groups[section] = append(groups[section], item)
	}

	sort.Strings(order)

	var result []AllTasksGroup
	for _, section := range order {
		result = append(result, AllTasksGroup{Name: section, Tasks: groups[section]})
	}
	return result
}

// groupByDueDate groups tasks into due date buckets.
func (m *AllTasksViewModel) groupByDueDate(tasks []AllTasksItem) []AllTasksGroup {
	now := time.Now()
	today := util.StartOfDay(now)
	weekEnd := today.AddDate(0, 0, 7)

	var overdue, todayTasks, thisWeek, later, noDate []AllTasksItem

	for _, item := range tasks {
		if item.Task.DueDate == nil {
			noDate = append(noDate, item)
			continue
		}
		dueDay := util.StartOfDay(*item.Task.DueDate)
		switch {
		case dueDay.Before(today):
			overdue = append(overdue, item)
		case dueDay.Equal(today):
			todayTasks = append(todayTasks, item)
		case dueDay.Before(weekEnd):
			thisWeek = append(thisWeek, item)
		default:
			later = append(later, item)
		}
	}

	var result []AllTasksGroup
	if len(overdue) > 0 {
		result = append(result, AllTasksGroup{Name: "Overdue", Tasks: overdue})
	}
	if len(todayTasks) > 0 {
		result = append(result, AllTasksGroup{Name: "Today", Tasks: todayTasks})
	}
	if len(thisWeek) > 0 {
		result = append(result, AllTasksGroup{Name: "This Week", Tasks: thisWeek})
	}
	if len(later) > 0 {
		result = append(result, AllTasksGroup{Name: "Later", Tasks: later})
	}
	if len(noDate) > 0 {
		result = append(result, AllTasksGroup{Name: "No Date", Tasks: noDate})
	}
	return result
}

// cycleGroupBy cycles through grouping modes.
func (m *AllTasksViewModel) cycleGroupBy() {
	m.groupBy = (m.groupBy + 1) % (ATGroupByFlat + 1)
	m.applyFiltersAndGroup()
	m.clampCursor()
	m.statusMessage = fmt.Sprintf("Grouped by: %s", m.groupBy)
}

// toggleGroupCollapse toggles the collapse state of the group containing the cursor.
func (m *AllTasksViewModel) toggleGroupCollapse() {
	if len(m.groups) == 0 {
		return
	}

	// Walk groups tracking position to find which one contains the cursor
	pos := 0
	for _, group := range m.groups {
		headerPos := pos
		pos++ // past header

		var sectionEnd int
		if m.collapsedGroups[group.Name] {
			sectionEnd = pos
		} else {
			sectionEnd = pos + len(group.Tasks)
		}

		if m.cursor >= headerPos && m.cursor < sectionEnd {
			ToggleSection(m.collapsedGroups, group.Name)
			// On collapse, snap cursor to header position
			if m.collapsedGroups[group.Name] {
				m.cursor = headerPos
			}
			m.calculateNavigation()
			m.clampCursor()
			return
		}

		pos = sectionEnd
	}
}
