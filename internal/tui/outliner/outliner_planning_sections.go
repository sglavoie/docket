package outliner

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

const (
	todaySectionCollapseKey       = "_planning:today"
	nextSectionCollapseKey        = "_planning:next"
	planningProjectCollapsePrefix = "_planning_project:"
)

type planningSectionBuild struct {
	rows       []OutlinerRow
	suppressed map[taskref.TaskIdentity]bool
	group      models.WorkflowStateGroup
}

// buildTodaySection realizes explicit Today membership in durable order.
// Descendants appear only when they have their own Today membership; the row
// appender restores hierarchy among relatives that share the section.
func (m *OutlinerModel) buildTodaySection(
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
) planningSectionBuild {
	result := planningSectionBuild{
		suppressed: make(map[taskref.TaskIdentity]bool, len(input.TodayIDs)),
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
			result.suppressed[taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID}] = true
		}
	}

	previousExclude := input.TaskExclude
	input.TaskExclude = func(task *models.Task, project *models.Project) bool {
		return pinned[taskref.For(task, project)] ||
			(previousExclude != nil && previousExclude(task, project))
	}
	for _, modeRow := range BuildTodayModeRows(input) {
		row := m.outlinerRowFromModeRow(modeRow)
		row.IsTodaySection = true
		result.rows = append(result.rows, row)
	}
	return result
}

// buildNextSection realizes explicit Next membership in durable order. Today
// wins defensively if an inconsistent planning snapshot contains an identity in
// both lists, matching the locked All-mode group precedence.
func (m *OutlinerModel) buildNextSection(
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
	today map[taskref.TaskIdentity]bool,
) planningSectionBuild {
	result := planningSectionBuild{
		suppressed: make(map[taskref.TaskIdentity]bool, len(input.NextOrder)),
		group:      models.StateGroupNext,
	}
	projects := modeProjectIndex(input.Projects)
	for _, locator := range input.NextOrder {
		project := projects[locator.ProjectID]
		if project == nil {
			continue
		}
		if task := project.GetTaskByID(locator.TaskID); task != nil && task.State().IsValid() && !task.IsTerminalState() {
			result.suppressed[taskref.TaskIdentity{ProjectID: locator.ProjectID, TaskID: locator.TaskID}] = true
		}
	}

	previousExclude := input.TaskExclude
	input.TaskExclude = func(task *models.Task, project *models.Project) bool {
		identity := taskref.For(task, project)
		return pinned[identity] || today[identity] ||
			(previousExclude != nil && previousExclude(task, project))
	}
	for _, modeRow := range BuildNextModeRows(input) {
		row := m.outlinerRowFromModeRow(modeRow)
		row.IsNextSection = true
		result.rows = append(result.rows, row)
	}
	return result
}

func planningProjectCollapseKey(projectID string, group models.WorkflowStateGroup) string {
	return planningProjectCollapsePrefix + string(group) + ":" + projectID
}

// appendPlanningRows orders planning rows by compareSectionTasks, the
// workspace-wide durable order breaking ties, while restoring project context in
// the unscoped cross-project Outliner. A subtree occupies its earliest member's
// rank, a project block ranks by its earliest member, and all of a project's
// roots are rendered under one header.
func (m *OutlinerModel) appendPlanningRows(
	rows []OutlinerRow,
	group models.WorkflowStateGroup,
	sectionKey string,
) {
	if len(rows) == 0 {
		return
	}
	m.appendProjectGroupedForests(
		m.planningRowGroups(rows),
		OutlinerRow{
			IsTodaySection: group == models.StateGroupToday,
			IsNextSection:  group == models.StateGroupNext,
			StateGroup:     group,
		},
		sectionKey,
		func(projectID string) string {
			return planningProjectCollapseKey(projectID, group)
		},
	)
}

// planningRowGroups is the project grouping one planning lane draws, built the
// same way whether the lane is drawn or folded, so what a fold hides is measured
// over the tree the open lane would have shown.
func (m *OutlinerModel) planningRowGroups(rows []OutlinerRow) []projectRowGroup {
	rows = slices.Clone(rows)
	m.sortSectionRows(rows)
	return m.groupSectionRowsByProject(rows, sectionForestOptions{rankRootsBySubtree: true}, m.sectionTaskLead(rows))
}

func (m *OutlinerModel) appendPlanningSection(section planningSectionBuild, lifted int) {
	if len(section.rows) == 0 {
		return
	}
	isToday := section.group == models.StateGroupToday
	isNext := section.group == models.StateGroupNext
	collapseKey := nextSectionCollapseKey
	if isToday {
		collapseKey = todaySectionCollapseKey
	}
	if !m.sectionZoomAdmits(collapseKey) {
		return
	}
	m.Rows = append(m.Rows, OutlinerRow{
		Group:              OutlinerRowGroup{SectionKey: collapseKey},
		IsSectionHeader:    true,
		IsTodaySection:     isToday,
		IsNextSection:      isNext,
		SectionName:        section.group.Label(),
		SectionCount:       len(section.rows),
		StateGroup:         section.group,
		SectionLiftedCount: lifted,
		CollapseKey:        collapseKey,
	})
	if m.sectionCollapsed(collapseKey) {
		m.hideProjectRowGroups(m.planningRowGroups(section.rows))
		return
	}
	m.appendPlanningRows(section.rows, section.group, collapseKey)
}

// removePlanningRowsFromTree removes every explicit planning identity from a
// lifecycle forest while promoting non-planned descendants. This keeps All
// deduplicated without treating descendants as planned.
func removePlanningRowsFromTree(
	tasks []*models.Task,
	projectID string,
	suppressed map[taskref.TaskIdentity]bool,
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
		if suppressed[taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}] {
			result = append(result, children...)
			continue
		}
		clone := taskref.CloneTaskDetachingDependencies(task)
		clone.Children = children
		result = append(result, &clone)
	}
	return result
}
