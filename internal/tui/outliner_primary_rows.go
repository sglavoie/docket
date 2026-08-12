package tui

import (
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/app"
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
	review := m.buildReviewSection(input, pinned)
	today := m.buildTodaySection(input, pinned, review.projected)
	next := m.buildNextSection(input, pinned, today.suppressed)

	grouped := make(map[models.WorkflowStateGroup][]stateGroupProjectForest)
	for _, project := range projects {
		if project == nil {
			continue
		}
		roots := m.primaryModeRoots(project)
		roots = excludeTaskTreePromotingChildren(roots, m.primaryDefaultExclusion(ModeAll, input.Now))
		roots = filterTaskTreePromotingChildren(roots, func(task *models.Task) bool {
			return input.TaskFilter == nil || input.TaskFilter(task, project)
		})
		roots = removePlanningRowsFromTree(roots, project.ID, today.suppressed)
		roots = removePlanningRowsFromTree(roots, project.ID, next.suppressed)
		roots = removeProjectedRowsFromTree(roots, project.ID, pinned)
		roots = removeProjectedRowsFromTree(roots, project.ID, review.projected)
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
		}
	}
}

func (m *OutlinerModel) appendPlanningModeRows(
	mode OutlinerModeID,
	input OutlinerModeInput,
	pinned map[TaskIdentity]bool,
) {
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
		hardExclude := m.primaryDefaultExclusion(mode, input.Now)
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
		clone := *task
		clone.Children = children
		result = append(result, &clone)
	}
	return result
}

// primaryModeRoots applies the active single-project or qualified cross-project
// focus without changing the surrounding project/context scope.
func (m *OutlinerModel) primaryModeRoots(project *models.Project) []*models.Task {
	if project == nil {
		return nil
	}
	roots := project.GetRootTasks()
	if m.project != nil {
		if len(m.focusStack) == 0 {
			return roots
		}
		if task := project.GetTaskByID(m.focusStack[len(m.focusStack)-1].TaskID); task != nil {
			return task.Children
		}
		return nil
	}
	if len(m.crossFocusStack) == 0 {
		return roots
	}
	top := m.crossFocusStack[len(m.crossFocusStack)-1]
	if top.ProjectID != project.ID {
		return nil
	}
	if task := project.GetTaskByID(top.TaskID); task != nil {
		return task.Children
	}
	return nil
}

func (m *OutlinerModel) primaryDefaultExclusion(mode OutlinerModeID, now time.Time) func(*models.Task) bool {
	if now.IsZero() {
		now = time.Now()
	}
	includeSnoozed := m.hasExplicitSnoozedFilter()
	includeTerminal := mode == ModeAll && m.hasExplicitTerminalFilter()
	return func(task *models.Task) bool {
		if task == nil || !task.State().IsValid() {
			return true
		}
		if task.IsSnoozedAt(now) && !includeSnoozed {
			return true
		}
		return task.IsTerminalState() && !includeTerminal
	}
}

func (m *OutlinerModel) hasExplicitSnoozedFilter() bool {
	return m.filter.Snoozed
}

func (m *OutlinerModel) hasExplicitTerminalFilter() bool {
	if m.filter.hasStateFilter() {
		for _, state := range m.filter.States.Values() {
			if state.IsTerminal() {
				return true
			}
		}
	}
	return false
}

func (m *OutlinerModel) allPrimaryGroupOrder() []models.WorkflowStateGroup {
	order := models.AllModeWorkflowStateGroups()
	if m.hasExplicitTerminalFilter() {
		order = append(order, models.StateGroupDone, models.StateGroupCanceled)
	}
	return order
}

func (m *OutlinerModel) appendStateGroupForests(
	group models.WorkflowStateGroup,
	forests []stateGroupProjectForest,
) {
	if len(forests) == 0 {
		return
	}
	total := 0
	for _, item := range forests {
		total += item.forest.count
	}
	collapseKey := stateGroupCollapsePrefix + string(group)
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		SectionName:     group.Label(),
		SectionCount:    total,
		StateGroup:      group,
		CollapseKey:     collapseKey,
	})
	if m.collapsedSections[collapseKey] {
		return
	}
	for _, item := range forests {
		projectID := ""
		if m.project == nil {
			projectID = item.project.ID
		}
		if !m.appendProjectGroupHeader(
			outlinerRow{}, projectID, item.forest.tasks(), projectSectionCollapseKey(projectID, group),
		) {
			continue
		}
		m.flattenSectionForest(item.forest, projectID, &m.rows)
	}
}

func excludeTaskTreePromotingChildren(
	tasks []*models.Task,
	exclude func(*models.Task) bool,
) []*models.Task {
	if len(tasks) == 0 {
		return nil
	}
	result := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		children := excludeTaskTreePromotingChildren(task.Children, exclude)
		if exclude(task) {
			result = append(result, children...)
			continue
		}
		clone := *task
		clone.Children = children
		result = append(result, &clone)
	}
	return result
}

func (m *OutlinerModel) sortPrimaryTaskTree(tasks []*models.Task) []*models.Task {
	if m.sort.Field == SortByDefault || len(tasks) == 0 {
		return tasks
	}
	sorted := slices.Clone(tasks)
	sort.SliceStable(sorted, func(i, j int) bool {
		cmp := CompareTasks(sorted[i], sorted[j], m.sort.Field)
		return ApplySortDirection(cmp, m.sort.Ascending)
	})
	for i, task := range sorted {
		if len(task.Children) == 0 {
			continue
		}
		clone := *task
		clone.Children = m.sortPrimaryTaskTree(task.Children)
		sorted[i] = &clone
	}
	return sorted
}

// flattenPrimaryProjectTrees renders one project group per tree. Trees arrive in
// catalog order and each keeps its own tree-prefix run, so the last root of a
// group closes with the terminal connector rather than continuing into the next
// project.
func (m *OutlinerModel) flattenPrimaryProjectTrees(trees []primaryProjectTree, mode OutlinerModeID) {
	for _, tree := range trees {
		if tree.project == nil {
			continue
		}
		if !m.appendProjectGroupHeader(
			outlinerRow{}, tree.project.ID, flattenTaskPointers(tree.roots), modeProjectCollapseKey(mode, tree.project.ID),
		) {
			continue
		}
		for i, task := range tree.roots {
			m.flattenPrimaryTask(task, tree.project, nil, i == len(tree.roots)-1)
		}
	}
}

// primaryModeFocusBoundary returns the task whose children form the current
// hierarchical mode root. Breadcrumbs for promoted descendants stop at this
// boundary because the Outliner header already renders the focus path.
func (m *OutlinerModel) primaryModeFocusBoundary(project *models.Project) string {
	if m == nil || project == nil {
		return ""
	}
	if m.project != nil {
		if len(m.focusStack) > 0 && m.project.ID == project.ID {
			return m.focusStack[len(m.focusStack)-1].TaskID
		}
		return ""
	}
	if len(m.crossFocusStack) == 0 {
		return ""
	}
	top := m.crossFocusStack[len(m.crossFocusStack)-1]
	if top.ProjectID == project.ID {
		return top.TaskID
	}
	return ""
}

func (m *OutlinerModel) flattenPrimaryTask(
	task *models.Task,
	project *models.Project,
	ancestorIsLast []bool,
	isLast bool,
) {
	if task == nil {
		return
	}
	projectID := ""
	if m.project == nil && project != nil {
		projectID = project.ID
	}
	breadcrumb := ""
	if len(ancestorIsLast) == 0 && task.ParentID != nil {
		breadcrumb = taskAncestorBreadcrumbAfter(task, m.primaryModeFocusBoundary(project))
	}
	m.rows = append(m.rows, outlinerRow{
		task:               task,
		prefix:             buildTreePrefix(ancestorIsLast, isLast),
		Breadcrumb:         breadcrumb,
		hasVisibleChildren: len(task.Children) > 0,
		ProjectID:          projectID,
	})
	identity := TaskIdentity{ProjectID: projectID, TaskID: task.ID}
	if identity.ProjectID == "" && m.project != nil {
		identity.ProjectID = m.project.ID
	}
	if len(task.Children) == 0 || !m.isExpanded(identity) {
		return
	}
	nextAncestors := append(slices.Clone(ancestorIsLast), isLast)
	for i, child := range task.Children {
		m.flattenPrimaryTask(child, project, nextAncestors, i == len(task.Children)-1)
	}
}

// planningTodayEligible is kept local to the row pipeline so every visual
// caller uses the application layer's canonical planning invariant.
func planningTodayEligible(task *models.Task) bool {
	return task != nil && app.PlanningTodayEligible(task.State())
}
