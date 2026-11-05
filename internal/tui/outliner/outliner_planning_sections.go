package outliner

import (
	"math"
	"slices"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/util"
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

// buildTodaySection combines the Today plan and tasks due today or overdue. Descendants
// qualify independently; the row appender restores hierarchy within the section.
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

	for _, modeRow := range BuildTodayModeRows(input) {
		identity := taskref.For(modeRow.Task, modeRow.Project)
		result.suppressed[identity] = true
		if pinned[identity] {
			continue
		}
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

func (m *OutlinerModel) appendPlanningSection(section planningSectionBuild, lifted LiftedCounts) {
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
		Group:           OutlinerRowGroup{SectionKey: collapseKey},
		IsSectionHeader: true,
		IsTodaySection:  isToday,
		IsNextSection:   isNext,
		SectionName:     section.group.Label(),
		SectionCount:    len(section.rows),
		StateGroup:      section.group,
		SectionLifted:   lifted,
		CollapseKey:     collapseKey,
	})
	if m.sectionCollapsed(collapseKey) {
		for _, row := range section.rows {
			header := &m.Rows[len(m.Rows)-1]
			header.attentionTasks = append(header.attentionTasks, row.Task)
		}
		m.hideProjectRowGroups(m.planningRowGroups(section.rows))
		return
	}
	m.appendPlanningRows(section.rows, section.group, collapseKey)
}

// removePlanningRowsFromTree removes identities claimed by Today or Next from a
// lifecycle forest while promoting other descendants. This keeps All
// deduplicated without claiming descendants implicitly. It is a pass over a tree
// the projection owns, so it shares what it leaves alone; see pruneOwnedTree.
func removePlanningRowsFromTree(
	tasks []*models.Task,
	projectID string,
	suppressed map[taskref.TaskIdentity]bool,
) []*models.Task {
	if len(tasks) == 0 || len(suppressed) == 0 {
		return tasks
	}
	pruned, _ := pruneOwnedTree(tasks, func(task *models.Task) bool {
		return suppressed[taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}]
	})
	return pruned
}

// todaySuggestionsCollapseKey is the Suggested lane's fold. It is its own
// namespace, since no other surface draws this lane, and like every fold it is
// session-only: a dismissed lane comes back at the next launch.
const todaySuggestionsCollapseKey = "_suggest:today"

// TodaySuggestionsSectionName is the lane's heading.
const TodaySuggestionsSectionName = "Suggested for today"

const (
	// todaySuggestionLimit bounds the lane: a short menu to choose from, not a
	// second Backlog.
	todaySuggestionLimit = 7
	// todayPlanFloor is the smallest day the lane plans toward, and
	// todayPlanCeiling the largest, whatever the completion history says.
	todayPlanFloor   = 3
	todayPlanCeiling = 8
	// todaySuggestionHorizonDays is how far ahead a deadline earns a place.
	todaySuggestionHorizonDays = 7
)

// todaySuggestionReason ranks why a task is offered, strongest first. Today
// already admits overdue work and work due today, so neither appears here.
type todaySuggestionReason int

const (
	suggestDoing todaySuggestionReason = iota
	suggestAttention
	suggestDueSoon
	suggestNext
	suggestTopPriority
	suggestNone
)

// todayPlanTarget is how many tasks a planned day holds before the Suggested
// lane steps aside: the 30-day median of completions, rounded, inside the floor
// and the ceiling, or the floor when there is no history to read.
func (m *OutlinerModel) todayPlanTarget() int {
	if !m.CompletionActivity.HasHistory() {
		return todayPlanFloor
	}
	target := int(math.Round(m.CompletionActivity.MedianPerDay()))
	return min(max(target, todayPlanFloor), todayPlanCeiling)
}

// todaySuggestionReasonFor names the strongest reason to plan the task today,
// or suggestNone. next is the Next plan's membership.
func todaySuggestionReasonFor(task *models.Task, project *models.Project, input OutlinerModeInput, next map[taskref.TaskIdentity]bool) todaySuggestionReason {
	now := input.now()
	switch {
	case task.State() == models.StateDoing:
		return suggestDoing
	case models.TaskNeedsAttention(task, now):
		return suggestAttention
	case task.DueDate != nil && util.DaysBetween(now, *task.DueDate) > 0 &&
		util.DaysBetween(now, *task.DueDate) <= todaySuggestionHorizonDays:
		return suggestDueSoon
	case next[taskref.For(task, project)]:
		return suggestNext
	case task.Priority == 1:
		return suggestTopPriority
	}
	return suggestNone
}

// BuildTodaySuggestions is the Suggested lane's content: open work worth
// planning today that Today does not already hold, strongest reason first, then
// the soonest deadline, then priority, then stored order. It is empty once the
// day holds target tasks, so the lane steps aside as the day fills.
//
// planned and pinned name what Today and the Pinned lane already draw; a task
// either one shows is never offered twice.
func BuildTodaySuggestions(
	input OutlinerModeInput,
	planned, pinned map[taskref.TaskIdentity]bool,
	target int,
) []OutlinerModeRow {
	if len(planned) >= target {
		return nil
	}
	next := make(map[taskref.TaskIdentity]bool, len(input.NextOrder))
	for _, locator := range input.NextOrder {
		next[taskref.TaskIdentity{ProjectID: locator.ProjectID, TaskID: locator.TaskID}] = true
	}
	type candidate struct {
		ordered orderedModeTask
		reason  todaySuggestionReason
	}
	var candidates []candidate
	for _, ordered := range orderedModeTasks(input) {
		task, project := ordered.row.Task, ordered.row.Project
		if task == nil || project == nil || !planningTodayEligible(task) || !isComputedOpenWork(task, input) {
			continue
		}
		identity := taskref.For(task, project)
		if planned[identity] || pinned[identity] || input.TodayIDs[identity] {
			continue
		}
		if input.TaskExclude != nil && input.TaskExclude(task, project) {
			continue
		}
		if input.TaskFilter != nil && !input.TaskFilter(task, project) {
			continue
		}
		reason := todaySuggestionReasonFor(task, project, input, next)
		if reason == suggestNone {
			continue
		}
		candidates = append(candidates, candidate{ordered: ordered, reason: reason})
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		if a.reason != b.reason {
			return int(a.reason) - int(b.reason)
		}
		if a.reason == suggestDueSoon {
			if days := util.DaysBetween(*b.ordered.row.Task.DueDate, *a.ordered.row.Task.DueDate); days != 0 {
				return days
			}
		}
		if c := comparePriorityBand(a.ordered.row.Task, b.ordered.row.Task); c != 0 {
			return c
		}
		return compareOrderedModeTasks(a.ordered, b.ordered)
	})
	limit := min(len(candidates), todaySuggestionLimit)
	rows := make([]OutlinerModeRow, 0, limit)
	for _, item := range candidates[:limit] {
		row := item.ordered.row
		row.Breadcrumb = taskAncestorBreadcrumb(row.Task)
		rows = append(rows, row)
	}
	return rows
}

// appendTodaySuggestions draws the Suggested lane under Today's own rows while
// the day is still short of its target. Planning a suggestion with `wt` moves it
// into Today above, and the cursor carries on to the next suggestion, which is
// the ritual: a few keystrokes plan the day. A search narrowing the view owns it,
// so the lane stands down rather than mixing offers into results.
func (m *OutlinerModel) appendTodaySuggestions(input OutlinerModeInput, pinned map[taskref.TaskIdentity]bool) {
	if m.HasSearchProjection() || m.Focus.Zoom.Active() && !m.sectionZoomAdmits(todaySuggestionsCollapseKey) {
		return
	}
	planned := make(map[taskref.TaskIdentity]bool)
	for _, row := range BuildTodayModeRows(input) {
		planned[taskref.For(row.Task, row.Project)] = true
	}
	rows := BuildTodaySuggestions(input, planned, pinned, m.todayPlanTarget())
	if len(rows) == 0 {
		return
	}
	rank := make(map[*models.Task]int, len(rows))
	for index, row := range rows {
		rank[row.Task] = index
	}
	m.appendFilteredModeSections([]OutlinerModeSection{{
		Name:        TodaySuggestionsSectionName,
		CollapseKey: todaySuggestionsCollapseKey,
		Rows:        rows,
		Lead: func(a, b OutlinerModeRow) int {
			return rank[a.Task] - rank[b.Task]
		},
	}}, nil, nil)
}
