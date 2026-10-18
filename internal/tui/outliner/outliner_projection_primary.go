package outliner

import (
	"slices"
	"sort"
	"time"

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
	if m.Project != nil {
		if len(m.Focus.Stack) == 0 {
			return roots
		}
		if task := project.GetTaskByID(m.Focus.Stack[len(m.Focus.Stack)-1].TaskID); task != nil {
			return task.Children
		}
		return nil
	}
	if len(m.Focus.CrossStack) == 0 {
		return roots
	}
	top := m.Focus.CrossStack[len(m.Focus.CrossStack)-1]
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
	includeTerminal := m.AdmitsTerminalPopulation()
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
	if m.AdmitsTerminalPopulation() {
		order = append(order, models.StateGroupDone, models.StateGroupCanceled)
	}
	return order
}

func (m *OutlinerModel) appendStateGroupForests(
	group models.WorkflowStateGroup,
	forests []stateGroupProjectForest,
	lifted int,
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
	m.Rows = append(m.Rows, OutlinerRow{
		Group:              OutlinerRowGroup{SectionKey: collapseKey},
		IsSectionHeader:    true,
		SectionName:        group.Label(),
		SectionCount:       total,
		StateGroup:         group,
		SectionLiftedCount: lifted,
		CollapseKey:        collapseKey,
	})
	if m.sectionCollapsed(collapseKey) {
		for _, item := range forests {
			m.hideSectionForestTasks(item.forest, item.forest.roots)
		}
		return
	}
	blocks := make([]projectBlock[stateGroupProjectForest], 0, len(forests))
	for _, item := range forests {
		blocks = append(blocks, projectBlock[stateGroupProjectForest]{
			item: item, project: item.project, best: m.bestSectionTask(item.forest.tasks()),
		})
	}
	for _, block := range rankProjectBlocks(m, blocks) {
		item := block.item
		projectID := ""
		if m.Project == nil {
			projectID = item.project.ID
		}
		identity := m.rowGroupFor(collapseKey, projectSectionCollapseKey(projectID, group), projectID)
		follow, merged := m.appendProjectGroupHeader(OutlinerRow{}, identity, item.forest.tasks(), len(item.forest.roots))
		if !follow {
			m.hideSectionForestTasks(item.forest, item.forest.roots)
			continue
		}
		start := len(m.Rows)
		m.flattenSectionForest(item.forest, projectID, identity, &m.Rows)
		if merged {
			m.mergeProjectGroupRows(start)
		}
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

// sortPrimaryTaskTree orders every sibling run of a tree-order projection by
// compareSectionTasks. The sort is stable, so stored order still decides a tie
// and J/K still arranges the run inside one priority band.
func (m *OutlinerModel) sortPrimaryTaskTree(tasks []*models.Task) []*models.Task {
	if len(tasks) == 0 {
		return tasks
	}
	sorted := slices.Clone(tasks)
	sort.SliceStable(sorted, func(i, j int) bool {
		return m.compareSectionTasks(sorted[i], sorted[j]) < 0
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

// flattenPrimaryProjectTrees renders one project group per tree. A group ranks
// by its best task, pinned projects and then an alphabetical order breaking
// ties, and each group keeps its own tree-prefix run, so the last root of a
// group closes with the terminal connector rather than continuing into the next
// project.
func (m *OutlinerModel) flattenPrimaryProjectTrees(trees []primaryProjectTree, mode chrome.OutlinerModeID) {
	blocks := make([]projectBlock[primaryProjectTree], 0, len(trees))
	for _, tree := range trees {
		blocks = append(blocks, projectBlock[primaryProjectTree]{
			item: tree, project: tree.project, best: m.bestSectionTask(flattenTaskPointers(tree.roots)),
		})
	}
	for _, block := range rankProjectBlocks(m, blocks) {
		tree := block.item
		if tree.project == nil {
			continue
		}
		// These modes emit no section header, so the project group is the whole
		// group identity and the section half stays empty.
		identity := m.rowGroupFor("", modeProjectCollapseKey(mode, tree.project.ID), tree.project.ID)
		follow, merged := m.appendProjectGroupHeader(OutlinerRow{}, identity, flattenTaskPointers(tree.roots), len(tree.roots))
		if !follow {
			m.hidePrimaryTasks(tree.roots)
			continue
		}
		start := len(m.Rows)
		for i, task := range tree.roots {
			m.flattenPrimaryTask(task, tree.project, identity, nil, i == len(tree.roots)-1)
		}
		if merged {
			m.mergeProjectGroupRows(start)
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
	if m.Project != nil {
		if len(m.Focus.Stack) > 0 && m.Project.ID == project.ID {
			return m.Focus.Stack[len(m.Focus.Stack)-1].TaskID
		}
		return ""
	}
	if len(m.Focus.CrossStack) == 0 {
		return ""
	}
	top := m.Focus.CrossStack[len(m.Focus.CrossStack)-1]
	if top.ProjectID == project.ID {
		return top.TaskID
	}
	return ""
}

func (m *OutlinerModel) flattenPrimaryTask(
	task *models.Task,
	project *models.Project,
	group OutlinerRowGroup,
	ancestorIsLast []bool,
	isLast bool,
) {
	if task == nil {
		return
	}
	projectID := ""
	if m.Project == nil && project != nil {
		projectID = project.ID
	}
	breadcrumb := ""
	if len(ancestorIsLast) == 0 && task.ParentID != nil {
		breadcrumb = taskAncestorBreadcrumbAfter(task, m.primaryModeFocusBoundary(project))
	}
	m.Rows = append(m.Rows, OutlinerRow{
		Task:               task,
		Group:              group,
		Prefix:             BuildTreePrefix(ancestorIsLast, isLast),
		Breadcrumb:         breadcrumb,
		HasVisibleChildren: len(task.Children) > 0,
		ProjectID:          projectID,
	})
	identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}
	if identity.ProjectID == "" && m.Project != nil {
		identity.ProjectID = m.Project.ID
	}
	if len(task.Children) == 0 {
		return
	}
	if !m.IsExpanded(identity) {
		m.hidePrimaryTasks(task.Children)
		return
	}
	nextAncestors := append(slices.Clone(ancestorIsLast), isLast)
	for i, child := range task.Children {
		m.flattenPrimaryTask(child, project, group, nextAncestors, i == len(task.Children)-1)
	}
}

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
	projected map[taskref.TaskIdentity]bool
}
