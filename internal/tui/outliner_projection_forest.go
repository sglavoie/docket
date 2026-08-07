package tui

import (
	"slices"
	"sort"
)

// sectionRowForest is a presentation-only hierarchy over rows that already
// belong to one displayed section. It never adds tasks: a parent/child edge is
// retained only when both qualified identities are present in the input.
type sectionRowForest struct {
	roots []*sectionRowNode
	count int
}

type sectionRowNode struct {
	row           outlinerRow
	children      []*sectionRowNode
	sourceIndex   int
	earliestIndex int
	subtreeCount  int
}

// sectionForestOptions selects how a section ranks its roots.
type sectionForestOptions struct {
	// rankRootsBySubtree hoists a root to the earliest source position any member
	// of its subtree holds. Sections whose order is manual rank a root by its own
	// source position instead, so a swap of two adjacent rows is exactly the swap
	// the user saw.
	rankRootsBySubtree bool
}

// buildSectionRowForest converts an ordered projection into a section-local
// tree. Roots are ranked by the earliest source member in their subtree so a
// child that ranks before its parent keeps the subtree at that earlier position.
// Direct siblings retain their source order.
func (m *OutlinerModel) buildSectionRowForest(rows []outlinerRow) sectionRowForest {
	return m.buildSectionRowForestWith(rows, sectionForestOptions{rankRootsBySubtree: true})
}

func (m *OutlinerModel) buildSectionRowForestWith(rows []outlinerRow, opts sectionForestOptions) sectionRowForest {
	forest := sectionRowForest{}
	nodes := make([]*sectionRowNode, 0, len(rows))
	byIdentity := make(map[TaskIdentity]*sectionRowNode, len(rows))

	for sourceIndex, row := range rows {
		if row.task == nil {
			continue
		}
		node := &sectionRowNode{
			row:           row,
			sourceIndex:   sourceIndex,
			earliestIndex: sourceIndex,
			subtreeCount:  1,
		}
		nodes = append(nodes, node)
		identity := m.sectionRowIdentity(row)
		if identity.Valid() {
			byIdentity[identity] = node
		}
	}

	for _, node := range nodes {
		identity := m.sectionRowIdentity(node.row)
		if node.row.task.ParentID != nil && identity.Valid() {
			parentIdentity := TaskIdentity{
				ProjectID: identity.ProjectID,
				TaskID:    *node.row.task.ParentID,
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
		node.subtreeCount = count
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

func (m *OutlinerModel) sectionRowIdentity(row outlinerRow) TaskIdentity {
	projectID := row.ProjectID
	if projectID == "" && m.project != nil {
		projectID = m.project.ID
	}
	if row.task == nil {
		return TaskIdentity{}
	}
	return TaskIdentity{ProjectID: projectID, TaskID: row.task.ID}
}

func sectionRowNodesCount(roots []*sectionRowNode) int {
	count := 0
	for _, root := range roots {
		count += root.subtreeCount
	}
	return count
}

func (m *OutlinerModel) flattenSectionRowForest(forest sectionRowForest, rows *[]outlinerRow) {
	m.flattenSectionRowRoots(forest.roots, rows)
}

func (m *OutlinerModel) flattenSectionRowRoots(roots []*sectionRowNode, rows *[]outlinerRow) {
	var walk func([]*sectionRowNode, []bool)
	walk = func(nodes []*sectionRowNode, ancestorIsLast []bool) {
		for index, node := range nodes {
			isLast := index == len(nodes)-1
			row := node.row
			row.prefix = buildTreePrefix(ancestorIsLast, isLast)
			row.hasVisibleChildren = len(node.children) > 0
			if len(ancestorIsLast) > 0 {
				row.Breadcrumb = ""
			} else if row.Breadcrumb == "" && row.task.ParentID != nil {
				row.Breadcrumb = taskAncestorBreadcrumb(row.task)
			}
			*rows = append(*rows, row)

			if len(node.children) == 0 || !m.isExpanded(m.sectionRowIdentity(row)) {
				continue
			}
			next := append(slices.Clone(ancestorIsLast), isLast)
			walk(node.children, next)
		}
	}
	walk(roots, nil)
}
