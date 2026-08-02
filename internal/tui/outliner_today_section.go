package tui

import "github.com/sglavoie/tt/internal/models"

const (
	todaySectionCollapseKey = "_planning:today"
	nextSectionCollapseKey  = "_planning:next"
)

type planningSectionBuild struct {
	rows       []outlinerRow
	suppressed map[TaskIdentity]bool
	group      models.WorkflowStateGroup
}

// buildTodaySection realizes explicit Today membership in durable order. Every
// visible task is a flat promoted row with an ancestor breadcrumb; descendants
// appear only when they have their own Today membership.
func (m *OutlinerModel) buildTodaySection(
	input OutlinerModeInput,
	pinned map[TaskIdentity]bool,
) planningSectionBuild {
	result := planningSectionBuild{
		suppressed: make(map[TaskIdentity]bool, len(input.TodayIDs)),
		group:      models.StateGroupToday,
	}
	projects := modeProjectIndex(input.Projects)
	for _, item := range input.TodayOrder {
		if item.Task == nil || item.Project == nil {
			continue
		}
		project := projects[item.Project.ID]
		if project == nil {
			continue
		}
		if task := project.GetTaskByID(item.Task.ID); planningTodayEligible(task) {
			result.suppressed[TaskIdentity{ProjectID: project.ID, TaskID: task.ID}] = true
		}
	}

	previousExclude := input.TaskExclude
	input.TaskExclude = func(task *models.Task, project *models.Project) bool {
		identity := taskIdentityFor(task, project)
		return pinned[identity] || (previousExclude != nil && previousExclude(task, project))
	}
	for _, modeRow := range BuildTodayModeRows(input) {
		row := m.outlinerRowFromModeRow(modeRow)
		row.IsTodaySection = true
		row.IsTodayMember = true
		result.rows = append(result.rows, row)
	}
	assignPlanningRowPrefixes(result.rows)
	return result
}

// buildNextSection realizes explicit Next membership in durable order. Today
// wins defensively if an inconsistent planning snapshot contains an identity in
// both lists, matching the locked All-mode group precedence.
func (m *OutlinerModel) buildNextSection(
	input OutlinerModeInput,
	pinned map[TaskIdentity]bool,
	today map[TaskIdentity]bool,
) planningSectionBuild {
	result := planningSectionBuild{
		suppressed: make(map[TaskIdentity]bool, len(input.NextOrder)),
		group:      models.StateGroupNext,
	}
	projects := modeProjectIndex(input.Projects)
	for _, locator := range input.NextOrder {
		project := projects[locator.ProjectID]
		if project == nil {
			continue
		}
		if task := project.GetTaskByID(locator.TaskID); task != nil && task.State().IsValid() && !task.IsTerminalState() {
			result.suppressed[TaskIdentity{ProjectID: locator.ProjectID, TaskID: locator.TaskID}] = true
		}
	}

	previousExclude := input.TaskExclude
	input.TaskExclude = func(task *models.Task, project *models.Project) bool {
		identity := taskIdentityFor(task, project)
		return pinned[identity] || today[identity] ||
			(previousExclude != nil && previousExclude(task, project))
	}
	for _, modeRow := range BuildNextModeRows(input) {
		row := m.outlinerRowFromModeRow(modeRow)
		row.IsNextSection = true
		result.rows = append(result.rows, row)
	}
	assignPlanningRowPrefixes(result.rows)
	return result
}

func assignPlanningRowPrefixes(rows []outlinerRow) {
	for i := range rows {
		rows[i].prefix = buildTreePrefix(nil, i == len(rows)-1)
		rows[i].hasVisibleChildren = false
	}
}

func (m *OutlinerModel) appendPlanningSection(section planningSectionBuild) {
	if len(section.rows) == 0 {
		return
	}
	isToday := section.group == models.StateGroupToday
	isNext := section.group == models.StateGroupNext
	collapseKey := nextSectionCollapseKey
	if isToday {
		collapseKey = todaySectionCollapseKey
	}
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		IsTodaySection:  isToday,
		IsNextSection:   isNext,
		SectionName:     section.group.Label(),
		SectionCount:    len(section.rows),
		StateGroup:      section.group,
		CollapseKey:     collapseKey,
	})
	if !m.collapsedSections[collapseKey] {
		m.rows = append(m.rows, section.rows...)
	}
}

// removePlanningRowsFromTree removes every explicit planning identity from a
// lifecycle forest while promoting non-planned descendants. This keeps All
// deduplicated without treating descendants as planned.
func removePlanningRowsFromTree(
	tasks []*models.Task,
	projectID string,
	suppressed map[TaskIdentity]bool,
) []*models.Task {
	if len(tasks) == 0 || len(suppressed) == 0 {
		return tasks
	}
	result := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		children := removePlanningRowsFromTree(task.Children, projectID, suppressed)
		if suppressed[TaskIdentity{ProjectID: projectID, TaskID: task.ID}] {
			result = append(result, children...)
			continue
		}
		clone := *task
		clone.Children = children
		result = append(result, &clone)
	}
	return result
}
