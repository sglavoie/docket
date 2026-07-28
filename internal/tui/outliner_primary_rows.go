package tui

import (
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
)

const stateGroupCollapsePrefix = "_state:"

type primaryProjectTree struct {
	project *models.Project
	roots   []*models.Task
}

type primaryTaskRoot struct {
	project *models.Project
	task    *models.Task
}

type stateGroupProjectForest struct {
	project *models.Project
	forest  *sectionTaskForest
}

// buildPrimaryModeRows is the common state-native pipeline for the four Tab
// modes. Scope and zoom choose the project trees once; common filters compose
// once; only membership, grouping, and canonical ordering vary by mode.
func (m *OutlinerModel) buildPrimaryModeRows() {
	mode := m.primaryMode
	if !isPrimaryOutlinerMode(mode) {
		return
	}

	projects := slices.Clone(m.workModeScopeProjects())
	if m.project == nil {
		sortProjectsByDisplayOrder(projects)
	}
	input := m.buildOutlinerModeInput(projects)

	if mode == ModeAll {
		m.buildAllPrimaryRows(projects, input)
		return
	}

	trees := m.buildPrimaryProjectTrees(projects, mode, input)
	roots := flattenPrimaryProjectRoots(trees)
	switch mode {
	case ModeNext:
		roots = sortPrimaryRootsByRank(roots, focusQueueIdentityRanks(input.FocusQueue))
	case ModeBacklog:
		roots = sortPrimaryRootsByRank(roots, m.backlogIdentityRanks(trees, input))
	case ModeInbox:
	default:
		return
	}
	m.flattenPrimaryRoots(roots)
}

func (m *OutlinerModel) buildAllPrimaryRows(projects []*models.Project, input OutlinerModeInput) {
	candidates := make(map[string][]*models.Task, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		candidates[project.ID] = flattenTaskPointers(m.primaryModeRoots(project))
	}
	today := m.buildTodaySection(projects, candidates)

	grouped := make(map[models.WorkflowStateGroup][]stateGroupProjectForest)
	for _, project := range projects {
		if project == nil {
			continue
		}
		roots := m.primaryModeRoots(project)
		roots = excludeTaskTreePromotingChildren(roots, m.primaryDefaultExclusion(ModeAll, input.Now))
		roots = filterTaskTree(roots, m.workModeFilterPredicate(project))
		roots = removeTodayRowsFromTree(roots, project.ID, today.suppressed)
		roots = m.sortPrimaryTaskTree(roots, project.ID)
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

	m.appendTodaySection(today)
	for _, group := range m.allPrimaryGroupOrder() {
		m.appendStateGroupForests(group, grouped[group])
	}
}

func (m *OutlinerModel) buildPrimaryProjectTrees(
	projects []*models.Project,
	mode OutlinerModeID,
	input OutlinerModeInput,
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
				return defaultExclude(task) || input.DailyPlanIDs[taskIdentityFor(task, project)]
			}
		}
		roots = excludeTaskTreePromotingChildren(roots, hardExclude)

		membership := func(task *models.Task) bool {
			switch mode {
			case ModeInbox:
				return task.IsInState(models.StateInbox)
			case ModeNext:
				return task.IsInState(models.StateNext)
			case ModeBacklog:
				return task.IsInState(models.StateReady) &&
					!input.DailyPlanIDs[taskIdentityFor(task, project)]
			default:
				return false
			}
		}
		// Inbox is a triage list, not a context-preserving search result. Once a
		// task leaves Inbox its row must disappear even when an Inbox descendant
		// survives. Promote that descendant to the nearest visible level while
		// retaining hierarchy between tasks that are themselves still in Inbox.
		if mode == ModeInbox {
			roots = excludeTaskTreePromotingChildren(roots, func(task *models.Task) bool {
				return !membership(task)
			})
		}
		visible := m.workModeFilterPredicate(project)
		roots = filterTaskTree(roots, func(task *models.Task) bool {
			return (mode == ModeInbox || membership(task)) && visible(task)
		})
		if len(roots) == 0 {
			continue
		}
		if mode == ModeInbox {
			roots = m.sortPrimaryTaskTree(roots, project.ID)
		}
		trees = append(trees, primaryProjectTree{project: project, roots: roots})
	}
	return trees
}

// primaryModeRoots applies the active single-project or qualified cross-project
// zoom without changing the surrounding project/context scope.
func (m *OutlinerModel) primaryModeRoots(project *models.Project) []*models.Task {
	if project == nil {
		return nil
	}
	roots := project.GetRootTasks()
	if m.project != nil {
		if len(m.zoomStack) == 0 {
			return roots
		}
		if task := project.GetTaskByID(m.zoomStack[len(m.zoomStack)-1].TaskID); task != nil {
			return task.Children
		}
		return nil
	}
	if len(m.crossZoomStack) == 0 {
		return roots
	}
	top := m.crossZoomStack[len(m.crossZoomStack)-1]
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
	if m.filter.Snoozed {
		return true
	}
	parsed := ParseSearchQuery(m.SearchQuery())
	return slices.Contains(parsed.IsState, "snoozed")
}

func (m *OutlinerModel) hasExplicitTerminalFilter() bool {
	if m.filter.hasStateFilter() {
		for _, state := range m.filter.States.Values() {
			if state.IsTerminal() {
				return true
			}
		}
	}
	parsed := ParseSearchQuery(m.SearchQuery())
	for _, state := range parsed.States {
		if state.IsTerminal() {
			return true
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
			m.rows = append(m.rows, outlinerRow{
				IsProjectHeader: true,
				ProjectID:       projectID,
			})
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

func (m *OutlinerModel) sortPrimaryTaskTree(tasks []*models.Task, projectID string) []*models.Task {
	if m.sort.Field == SortByDefault || len(tasks) == 0 {
		return tasks
	}
	var automation *config.AutomationConfig
	if m.runtime != nil {
		automation = m.runtime.Automation()
	}
	sorted := slices.Clone(tasks)
	sort.SliceStable(sorted, func(i, j int) bool {
		cmp := CompareTasksCrossProject(
			sorted[i],
			sorted[j],
			m.sort.Field,
			projectID,
			projectID,
			automation,
		)
		return ApplySortDirection(cmp, m.sort.Ascending)
	})
	for i, task := range sorted {
		if len(task.Children) == 0 {
			continue
		}
		clone := *task
		clone.Children = m.sortPrimaryTaskTree(task.Children, projectID)
		sorted[i] = &clone
	}
	return sorted
}

func flattenPrimaryProjectRoots(trees []primaryProjectTree) []primaryTaskRoot {
	var roots []primaryTaskRoot
	for _, tree := range trees {
		for _, task := range tree.roots {
			if task != nil {
				roots = append(roots, primaryTaskRoot{project: tree.project, task: task})
			}
		}
	}
	return roots
}

func focusQueueIdentityRanks(items []config.FocusQueueItem) map[TaskIdentity]int {
	ranks := make(map[TaskIdentity]int, len(items))
	for i, item := range items {
		ranks[TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}] = i
	}
	return ranks
}

func (m *OutlinerModel) backlogIdentityRanks(
	trees []primaryProjectTree,
	input OutlinerModeInput,
) map[TaskIdentity]int {
	var candidates []focus.FocusTask
	for _, tree := range trees {
		visible := m.workModeFilterPredicate(tree.project)
		for _, task := range flattenTaskPointers(tree.roots) {
			if task.IsInState(models.StateReady) &&
				!input.DailyPlanIDs[taskIdentityFor(task, tree.project)] &&
				visible(task) {
				candidates = append(candidates, focus.FocusTask{Task: task, Project: tree.project})
			}
		}
	}
	ranked := focus.RankCrossProjectTasks(candidates, nil, input.Scorer)
	ranks := make(map[TaskIdentity]int, len(ranked))
	for i, task := range ranked {
		ranks[taskIdentityFor(task.Task, task.Project)] = i
	}
	return ranks
}

func sortPrimaryRootsByRank(
	roots []primaryTaskRoot,
	ranks map[TaskIdentity]int,
) []primaryTaskRoot {
	sorted := slices.Clone(roots)
	for i := range sorted {
		sorted[i].task = sortPrimaryTaskTreeByRank(sorted[i].task, sorted[i].project, ranks)
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return minimumPrimaryTaskRank(sorted[i].task, sorted[i].project, ranks) <
			minimumPrimaryTaskRank(sorted[j].task, sorted[j].project, ranks)
	})
	return sorted
}

func sortPrimaryTaskTreeByRank(
	task *models.Task,
	project *models.Project,
	ranks map[TaskIdentity]int,
) *models.Task {
	if task == nil {
		return nil
	}
	clone := *task
	clone.Children = slices.Clone(task.Children)
	for i, child := range clone.Children {
		clone.Children[i] = sortPrimaryTaskTreeByRank(child, project, ranks)
	}
	sort.SliceStable(clone.Children, func(i, j int) bool {
		return minimumPrimaryTaskRank(clone.Children[i], project, ranks) <
			minimumPrimaryTaskRank(clone.Children[j], project, ranks)
	})
	return &clone
}

func minimumPrimaryTaskRank(
	task *models.Task,
	project *models.Project,
	ranks map[TaskIdentity]int,
) int {
	const unranked = int(^uint(0) >> 1)
	if task == nil || project == nil {
		return unranked
	}
	rank, ok := ranks[taskIdentityFor(task, project)]
	if !ok {
		rank = unranked
	}
	for _, child := range task.Children {
		if childRank := minimumPrimaryTaskRank(child, project, ranks); childRank < rank {
			rank = childRank
		}
	}
	return rank
}

func (m *OutlinerModel) flattenPrimaryRoots(roots []primaryTaskRoot) {
	for i, root := range roots {
		m.flattenPrimaryTask(root.task, root.project, nil, i == len(roots)-1)
	}
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
	m.rows = append(m.rows, outlinerRow{
		task:               task,
		prefix:             buildTreePrefix(ancestorIsLast, isLast),
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
