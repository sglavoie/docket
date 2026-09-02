package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

const (
	stateGroupCollapsePrefix     = "_state:"
	projectSectionCollapsePrefix = "_project:"
)

// projectSectionCollapseKey identifies one project's task group within a
// shared cross-project state section. Keeping the state group in the key lets
// users fold (for example) Alpha's Backlog rows without hiding Alpha's Inbox
// rows as well.
func projectSectionCollapseKey(projectID string, group models.WorkflowStateGroup) string {
	return projectSectionCollapsePrefix + projectID + ":" + string(group)
}

type primaryProjectTree struct {
	project *models.Project
	roots   []*models.Task
}

type stateGroupProjectForest struct {
	project *models.Project
	forest  *sectionTaskForest
}

// pinnedModeProjection is a transient view over authoritative project tasks.
// projected records qualified identities so later sections can suppress the
// exact rows promoted into Pinned without maintaining a second task collection.
type pinnedModeProjection struct {
	rows      []OutlinerModeRow
	projected map[TaskIdentity]bool
}

// buildPinnedModeProjection selects visible pinned tasks from the current
// scope, orders them newest-pin-first, and carries ancestor breadcrumbs for
// their eventual synthetic-section rendering. Session 22 owns attaching that
// section to each mode; this helper owns the visibility and ordering contract.
func buildPinnedModeProjection(input OutlinerModeInput) pinnedModeProjection {
	input.Now = input.now()
	projection := pinnedModeProjection{projected: make(map[TaskIdentity]bool)}
	var candidates []orderedModeTask
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		if task == nil || project == nil || !task.Pinned || !isComputedOpenWork(task, input) {
			continue
		}
		if input.TaskExclude != nil && input.TaskExclude(task, project) {
			continue
		}
		if input.TaskFilter != nil && !input.TaskFilter(task, project) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		switch {
		case left.row.Task.PinnedAt != nil && right.row.Task.PinnedAt == nil:
			return true
		case left.row.Task.PinnedAt == nil && right.row.Task.PinnedAt != nil:
			return false
		case left.row.Task.PinnedAt != nil && right.row.Task.PinnedAt != nil &&
			!left.row.Task.PinnedAt.Equal(*right.row.Task.PinnedAt):
			return left.row.Task.PinnedAt.After(*right.row.Task.PinnedAt)
		default:
			return compareOrderedModeTasks(left, right) < 0
		}
	})
	projection.rows = make([]OutlinerModeRow, 0, len(candidates))
	for _, candidate := range candidates {
		row := candidate.row
		row.Breadcrumb = taskAncestorBreadcrumb(row.Task)
		identity := taskIdentityFor(row.Task, row.Project)
		projection.projected[identity] = true
		projection.rows = append(projection.rows, row)
	}
	return projection
}

func (m *OutlinerModel) buildPrimaryModeRowsWithInput(
	projects []*models.Project,
	input OutlinerModeInput,
	pinned map[TaskIdentity]bool,
) {
	mode := m.primaryMode
	if !isPrimaryOutlinerMode(mode) {
		return
	}

	if mode == ModeAll {
		m.buildAllPrimaryRows(projects, input, pinned)
		return
	}
	if mode == ModeToday || mode == ModeNext {
		m.appendPlanningModeRows(mode, input, pinned)
		return
	}

	switch mode {
	case ModeInbox, ModeBacklog:
	default:
		return
	}
	m.flattenPrimaryProjectTrees(m.buildPrimaryProjectTrees(projects, mode, input, pinned), mode)
}

func (m *OutlinerModel) buildAllPrimaryRows(
	projects []*models.Project,
	input OutlinerModeInput,
	pinned map[TaskIdentity]bool,
) {
	today := m.buildTodaySection(input, pinned)
	next := m.buildNextSection(input, pinned, today.suppressed)
	review := m.buildReviewSection(input, pinned, today.suppressed, next.suppressed)
	doing := projectedDoingTasks(input)
	stale := m.buildStaleSection(
		input, pinned, today.suppressed, next.suppressed, review.projected, doing,
	)

	grouped := make(map[models.WorkflowStateGroup][]stateGroupProjectForest)
	for _, project := range projects {
		if project == nil {
			continue
		}
		roots := m.primaryModeRoots(project)
		roots = excludeTaskTreePromotingChildren(roots, m.primaryDefaultExclusion(input.Now))
		roots = filterTaskTreePromotingChildren(roots, func(task *models.Task) bool {
			return input.TaskFilter == nil || input.TaskFilter(task, project)
		})
		roots = removePlanningRowsFromTree(roots, project.ID, today.suppressed)
		roots = removePlanningRowsFromTree(roots, project.ID, next.suppressed)
		roots = removeProjectedRowsFromTree(roots, project.ID, pinned)
		roots = removeProjectedRowsFromTree(roots, project.ID, review.projected)
		roots = removeProjectedRowsFromTree(roots, project.ID, stale.projected)
		roots = m.sortPrimaryTaskTree(roots)
		for group, forest := range buildStateGroupTaskForests(roots) {
			if forest == nil || forest.count == 0 {
				continue
			}
			grouped[group] = append(grouped[group], stateGroupProjectForest{
				project: project,
				forest:  forest,
			})
		}
	}

	for _, group := range m.allPrimaryGroupOrder() {
		switch group {
		case models.StateGroupToday:
			m.appendPlanningSection(today)
		case models.StateGroupNext:
			m.appendPlanningSection(next)
		default:
			m.appendStateGroupForests(group, grouped[group])
		}
		if group == models.StateGroupDoing {
			m.appendReviewSection(review)
			m.appendStaleSection(stale)
		}
	}
}

// projectedDoingTasks names the qualified rows claimed by the lifecycle Doing
// lane so derived sections can apply precedence without conflating state age
// with touch age. The section builders apply their own visibility guards.
func projectedDoingTasks(input OutlinerModeInput) map[TaskIdentity]bool {
	projected := make(map[TaskIdentity]bool)
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		if task == nil || project == nil || task.State() != models.StateDoing {
			continue
		}
		projected[taskIdentityFor(task, project)] = true
	}
	return projected
}

func (m *OutlinerModel) appendPlanningModeRows(
	mode OutlinerModeID,
	input OutlinerModeInput,
	pinned map[TaskIdentity]bool,
) {
	// Today and Next render their planning body ungrouped, under no header of
	// their own, so no zoom key can ever name it. Today's Pinned section is the
	// one zoom reachable here and it must leave the body out entirely.
	if m.zoom.active() {
		return
	}
	var modeRows []OutlinerModeRow
	switch mode {
	case ModeToday:
		modeRows = BuildTodayModeRows(input)
	case ModeNext:
		modeRows = BuildNextModeRows(input)
	default:
		return
	}

	rows := make([]outlinerRow, 0, len(modeRows))
	for _, modeRow := range modeRows {
		if modeRow.Task == nil || modeRow.Project == nil ||
			pinned[taskIdentityFor(modeRow.Task, modeRow.Project)] {
			continue
		}
		row := m.outlinerRowFromModeRow(modeRow)
		row.IsTodaySection = mode == ModeToday
		row.IsNextSection = mode == ModeNext
		rows = append(rows, row)
	}
	group := models.StateGroupToday
	if mode == ModeNext {
		group = models.StateGroupNext
	}
	m.appendPlanningRows(rows, group)
}

func (m *OutlinerModel) buildPrimaryProjectTrees(
	projects []*models.Project,
	mode OutlinerModeID,
	input OutlinerModeInput,
	pinned map[TaskIdentity]bool,
) []primaryProjectTree {
	var trees []primaryProjectTree
	for _, project := range projects {
		if project == nil {
			continue
		}
		roots := m.primaryModeRoots(project)
		hardExclude := m.primaryDefaultExclusion(input.Now)
		if mode == ModeBacklog {
			defaultExclude := hardExclude
			hardExclude = func(task *models.Task) bool {
				return defaultExclude(task) || input.TodayIDs[taskIdentityFor(task, project)]
			}
		}
		roots = excludeTaskTreePromotingChildren(roots, hardExclude)

		membership := func(task *models.Task) bool {
			switch mode {
			case ModeInbox:
				return task.IsInState(models.StateInbox)
			case ModeBacklog:
				return task.IsInState(models.StateBacklog) &&
					!input.TodayIDs[taskIdentityFor(task, project)]
			default:
				return false
			}
		}
		visible := func(task *models.Task) bool {
			return input.TaskFilter == nil || input.TaskFilter(task, project)
		}
		// Facets are task predicates, not ancestry requests. Apply them before
		// mode membership so every row that survives the later hierarchy
		// projection satisfies every active facet.
		roots = filterTaskTreePromotingChildren(roots, visible)
		// Inbox is a triage list, not a context-preserving search result. Once a
		// task leaves Inbox its row must disappear even when an Inbox descendant
		// survives. Promote that descendant to the nearest visible level while
		// retaining hierarchy between tasks that are themselves still in Inbox.
		if mode == ModeInbox {
			roots = excludeTaskTreePromotingChildren(roots, func(task *models.Task) bool {
				return !membership(task)
			})
		} else {
			// Backlog retains mode context for qualifying descendants, but the
			// strict facet projection above prevents nonmatching context rows.
			roots = filterTaskTreeKeepingAncestors(roots, membership)
		}
		roots = removeProjectedRowsFromTree(roots, project.ID, pinned)
		if len(roots) == 0 {
			continue
		}
		if mode == ModeInbox || mode == ModeBacklog {
			roots = m.sortPrimaryTaskTree(roots)
		}
		trees = append(trees, primaryProjectTree{project: project, roots: roots})
	}
	return trees
}

// removeProjectedRowsFromTree removes each project-qualified identity promoted
// into a synthetic section while preserving every non-promoted descendant in
// its original relative order.
func removeProjectedRowsFromTree(
	tasks []*models.Task,
	projectID string,
	projected map[TaskIdentity]bool,
) []*models.Task {
	if len(tasks) == 0 || len(projected) == 0 {
		return tasks
	}
	result := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		children := removeProjectedRowsFromTree(task.Children, projectID, projected)
		if projected[TaskIdentity{ProjectID: projectID, TaskID: task.ID}] {
			result = append(result, children...)
			continue
		}
		clone := cloneTaskDetachingDependencies(task)
		clone.Children = children
		result = append(result, &clone)
	}
	return result
}
