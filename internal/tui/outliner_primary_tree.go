package tui

import (
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

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

// primaryDefaultExclusion drops the two populations a projection does not load
// unless the query asks for them. It takes no mode: the rule is the query's, and
// a mode that would never show terminal work excludes it through its own
// membership rather than by narrowing this one.
func (m *OutlinerModel) primaryDefaultExclusion(now time.Time) func(*models.Task) bool {
	if now.IsZero() {
		now = time.Now()
	}
	includeDeferred := m.admitsDeferredPopulation()
	includeTerminal := m.admitsTerminalPopulation()
	return func(task *models.Task) bool {
		if task == nil || !task.State().IsValid() {
			return true
		}
		if task.IsDeferredAt(now) && !includeDeferred {
			return true
		}
		return task.IsTerminalState() && !includeTerminal
	}
}

func (m *OutlinerModel) allPrimaryGroupOrder() []models.WorkflowStateGroup {
	order := models.AllModeWorkflowStateGroups()
	if m.admitsTerminalPopulation() {
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
	if !m.sectionZoomAdmits(collapseKey) {
		return
	}
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		SectionName:     group.Label(),
		SectionCount:    total,
		StateGroup:      group,
		CollapseKey:     collapseKey,
	})
	if m.sectionCollapsed(collapseKey) {
		return
	}
	ordered := slices.Clone(forests)
	sort.SliceStable(ordered, func(i, j int) bool {
		leftID, rightID := "", ""
		if ordered[i].project != nil {
			leftID = ordered[i].project.ID
		}
		if ordered[j].project != nil {
			rightID = ordered[j].project.ID
		}
		return m.compareProjectGroupIDs(leftID, rightID) < 0
	})
	for _, item := range ordered {
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
		clone := taskref.CloneTaskDetachingDependencies(task)
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
		clone := taskref.CloneTaskDetachingDependencies(task)
		clone.Children = m.sortPrimaryTaskTree(task.Children)
		sorted[i] = &clone
	}
	return sorted
}

// flattenPrimaryProjectTrees renders one project group per tree. Pinned
// projects lead, followed by an alphabetical order, and each group keeps its
// own tree-prefix run, so the last root of a group closes with the terminal
// connector rather than continuing into the next project.
func (m *OutlinerModel) flattenPrimaryProjectTrees(trees []primaryProjectTree, mode chrome.OutlinerModeID) {
	ordered := slices.Clone(trees)
	sort.SliceStable(ordered, func(i, j int) bool {
		leftID, rightID := "", ""
		if ordered[i].project != nil {
			leftID = ordered[i].project.ID
		}
		if ordered[j].project != nil {
			rightID = ordered[j].project.ID
		}
		return m.compareProjectGroupIDs(leftID, rightID) < 0
	})
	for _, tree := range ordered {
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
	identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}
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
