package outliner

import (
	"slices"
	"sort"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// sectionRowForest is a presentation-only hierarchy over rows that already
// belong to one displayed section. It never adds tasks: a parent/child edge is
// retained only when both qualified identities are present in the input.
type sectionRowForest struct {
	roots []*sectionRowNode
	count int
}

type sectionRowNode struct {
	row           OutlinerRow
	children      []*sectionRowNode
	sourceIndex   int
	earliestIndex int
}

// sectionForestOptions selects how a section ranks its roots.
type sectionForestOptions struct {
	// rankRootsBySubtree hoists a root to the earliest source position any member
	// of its subtree holds. Sections whose order is manual rank a root by its own
	// source position instead, so a swap of two adjacent rows is exactly the swap
	// the user saw.
	rankRootsBySubtree bool
}

// buildSectionRowForestWith converts an ordered projection into a section-local
// tree. Under rankRootsBySubtree a root is ranked by the earliest source member
// in its subtree, so a child that ranks before its parent keeps the subtree at
// that earlier position. Direct siblings retain their source order.
func (m *OutlinerModel) buildSectionRowForestWith(rows []OutlinerRow, opts sectionForestOptions) sectionRowForest {
	forest := sectionRowForest{}
	nodes := make([]*sectionRowNode, 0, len(rows))
	byIdentity := make(map[taskref.TaskIdentity]*sectionRowNode, len(rows))

	for sourceIndex, row := range rows {
		if row.Task == nil {
			continue
		}
		node := &sectionRowNode{
			row:           row,
			sourceIndex:   sourceIndex,
			earliestIndex: sourceIndex,
		}
		nodes = append(nodes, node)
		identity := m.sectionRowIdentity(row)
		if identity.Valid() {
			byIdentity[identity] = node
		}
	}

	for _, node := range nodes {
		identity := m.sectionRowIdentity(node.row)
		if node.row.Task.ParentID != nil && identity.Valid() {
			parentIdentity := taskref.TaskIdentity{
				ProjectID: identity.ProjectID,
				TaskID:    *node.row.Task.ParentID,
			}
			if parent := byIdentity[parentIdentity]; parent != nil && parent != node {
				parent.children = append(parent.children, node)
				continue
			}
		}
		forest.roots = append(forest.roots, node)
	}

	var summarize func(*sectionRowNode) (int, int)
	summarize = func(node *sectionRowNode) (int, int) {
		earliest := node.sourceIndex
		count := 1
		for _, child := range node.children {
			childEarliest, childCount := summarize(child)
			earliest = min(earliest, childEarliest)
			count += childCount
		}
		node.earliestIndex = earliest
		return earliest, count
	}
	for _, root := range forest.roots {
		_, count := summarize(root)
		forest.count += count
	}
	sort.SliceStable(forest.roots, func(i, j int) bool {
		if opts.rankRootsBySubtree {
			return forest.roots[i].earliestIndex < forest.roots[j].earliestIndex
		}
		return forest.roots[i].sourceIndex < forest.roots[j].sourceIndex
	})
	return forest
}

func (m *OutlinerModel) sectionRowIdentity(row OutlinerRow) taskref.TaskIdentity {
	projectID := row.ProjectID
	if projectID == "" && m.Project != nil {
		projectID = m.Project.ID
	}
	if row.Task == nil {
		return taskref.TaskIdentity{}
	}
	return taskref.TaskIdentity{ProjectID: projectID, TaskID: row.Task.ID}
}

// tasks returns every task the forest holds, in flattened render order.
func (f sectionRowForest) tasks() []*models.Task {
	tasks := make([]*models.Task, 0, f.count)
	var walk func([]*sectionRowNode)
	walk = func(nodes []*sectionRowNode) {
		for _, node := range nodes {
			tasks = append(tasks, node.row.Task)
			walk(node.children)
		}
	}
	walk(f.roots)
	return tasks
}

func (m *OutlinerModel) flattenSectionRowForest(
	forest sectionRowForest,
	group OutlinerRowGroup,
	rows *[]OutlinerRow,
) {
	m.flattenSectionRowRoots(forest.roots, group, rows)
}

func (m *OutlinerModel) flattenSectionRowRoots(
	roots []*sectionRowNode,
	group OutlinerRowGroup,
	rows *[]OutlinerRow,
) {
	var walk func([]*sectionRowNode, []bool)
	walk = func(nodes []*sectionRowNode, ancestorIsLast []bool) {
		for index, node := range nodes {
			isLast := index == len(nodes)-1
			row := node.row
			row.Group = group
			row.Prefix = BuildTreePrefix(ancestorIsLast, isLast)
			row.HasVisibleChildren = len(node.children) > 0
			if len(ancestorIsLast) > 0 {
				row.Breadcrumb = ""
			} else if row.Breadcrumb == "" && row.Task.ParentID != nil {
				row.Breadcrumb = taskAncestorBreadcrumb(row.Task)
			}
			*rows = append(*rows, row)

			if len(node.children) == 0 {
				continue
			}
			if !m.IsExpanded(m.sectionRowIdentity(row)) {
				if rows == &m.Rows {
					m.hideSectionRowNodes(node.children)
				}
				continue
			}
			next := append(slices.Clone(ancestorIsLast), isLast)
			walk(node.children, next)
		}
	}
	walk(roots, nil)
}

// hiddenRowLayout is what the folds of one projection hide from the row
// layout: whether any hidden task would draw a fold glyph once shown, and the
// hidden tasks themselves, whose badges decide the badge columns. A projection
// measured from its drawn rows alone moves every title the moment a fold opens
// onto the first parent or the first prioritized task, so each collapsed branch
// of the row builders records here what it declined to emit. It is reset where
// the rows reset and read by the Outliner's own render alone.
type hiddenRowLayout struct {
	hasFold bool
	tasks   []*models.Task
}

func (h *hiddenRowLayout) reset() {
	h.hasFold = false
	h.tasks = h.tasks[:0]
}

// add records one hidden task. hasChildren is what HasVisibleChildren would
// have been on its row, so the fold answer matches rowDrawsFold exactly.
func (h *hiddenRowLayout) add(task *models.Task, hasChildren bool) {
	if task == nil {
		return
	}
	h.tasks = append(h.tasks, task)
	h.hasFold = h.hasFold || hasChildren || len(task.Checklist) > 0
}

// hideSectionRowNodes records every task of a section-local row forest a
// folded lane, project group, or parent left undrawn.
func (m *OutlinerModel) hideSectionRowNodes(nodes []*sectionRowNode) {
	for _, node := range nodes {
		m.hiddenLayout.add(node.row.Task, len(node.children) > 0)
		m.hideSectionRowNodes(node.children)
	}
}

// hideProjectRowGroups records every project group of one folded section.
func (m *OutlinerModel) hideProjectRowGroups(groups []projectRowGroup) {
	for _, group := range groups {
		m.hideSectionRowNodes(group.forest.roots)
	}
}

// hideSectionForestTasks records the given tasks of a lifecycle forest and
// everything beneath them, reading children the way flattenSectionForest does.
func (m *OutlinerModel) hideSectionForestTasks(forest *sectionTaskForest, tasks []*models.Task) {
	if forest == nil {
		return
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		children := forest.children[task.ID]
		m.hiddenLayout.add(task, len(children) > 0)
		m.hideSectionForestTasks(forest, children)
	}
}

// hidePrimaryTasks records the given tasks of a stored tree and everything
// beneath them, reading children the way flattenPrimaryTask does.
func (m *OutlinerModel) hidePrimaryTasks(tasks []*models.Task) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		m.hiddenLayout.add(task, len(task.Children) > 0)
		m.hidePrimaryTasks(task.Children)
	}
}
