package tui

import (
	"cmp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/pulse"
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
	row           outlinerRow
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
func (m *OutlinerModel) buildSectionRowForestWith(rows []outlinerRow, opts sectionForestOptions) sectionRowForest {
	forest := sectionRowForest{}
	nodes := make([]*sectionRowNode, 0, len(rows))
	byIdentity := make(map[taskref.TaskIdentity]*sectionRowNode, len(rows))

	for sourceIndex, row := range rows {
		if row.task == nil {
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
		if node.row.task.ParentID != nil && identity.Valid() {
			parentIdentity := taskref.TaskIdentity{
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

func (m *OutlinerModel) sectionRowIdentity(row outlinerRow) taskref.TaskIdentity {
	projectID := row.ProjectID
	if projectID == "" && m.project != nil {
		projectID = m.project.ID
	}
	if row.task == nil {
		return taskref.TaskIdentity{}
	}
	return taskref.TaskIdentity{ProjectID: projectID, TaskID: row.task.ID}
}

// projectRowGroup is one project's portion of a section, already assembled into
// a section-local forest.
type projectRowGroup struct {
	projectID string
	forest    sectionRowForest
}

// groupSectionRowsByProject splits an ordered section projection into
// per-project forests. Groups are sorted with pinned projects first, then by
// their display label alphabetically, so every section keeps one consistent
// project-header order. No parent/child edge can cross a group boundary because
// a parent identity is always project-local, so splitting before building the
// forests is equivalent to building one forest over the whole section.
func (m *OutlinerModel) groupSectionRowsByProject(
	rows []outlinerRow,
	opts sectionForestOptions,
) []projectRowGroup {
	groups := make([]projectRowGroup, 0, len(rows))
	indexes := make(map[string]int, len(rows))
	grouped := make([][]outlinerRow, 0, len(rows))
	for _, row := range rows {
		index, ok := indexes[row.ProjectID]
		if !ok {
			index = len(groups)
			indexes[row.ProjectID] = index
			groups = append(groups, projectRowGroup{projectID: row.ProjectID})
			grouped = append(grouped, nil)
		}
		grouped[index] = append(grouped[index], row)
	}
	for index := range groups {
		groups[index].forest = m.buildSectionRowForestWith(grouped[index], opts)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return m.compareProjectGroupIDs(groups[i].projectID, groups[j].projectID) < 0
	})
	return groups
}

func (m *OutlinerModel) compareProjectGroupIDs(leftID, rightID string) int {
	leftProject := m.projectByGroupID(leftID)
	rightProject := m.projectByGroupID(rightID)
	leftPinned := leftProject != nil && leftProject.Pinned
	rightPinned := rightProject != nil && rightProject.Pinned
	switch {
	case leftPinned && !rightPinned:
		return -1
	case !leftPinned && rightPinned:
		return 1
	}
	if c := cmp.Compare(
		projectGroupSortLabel(leftProject, leftID),
		projectGroupSortLabel(rightProject, rightID),
	); c != 0 {
		return c
	}
	return cmp.Compare(leftID, rightID)
}

func (m *OutlinerModel) projectByGroupID(projectID string) *models.Project {
	if m == nil {
		return nil
	}
	if m.project != nil {
		if projectID == "" || m.project.ID == projectID {
			return m.project
		}
		return nil
	}
	return m.crossResolveProject(projectID)
}

func projectGroupSortLabel(project *models.Project, fallbackID string) string {
	if project != nil {
		for _, value := range []string{project.Slug, project.Name, project.ID} {
			if key := strings.ToLower(strings.TrimSpace(value)); key != "" {
				return key
			}
		}
	}
	return strings.ToLower(strings.TrimSpace(fallbackID))
}

// appendProjectGroupHeader emits one synthetic project sub-header and reports
// whether the group's rows should follow. A project-scoped Outliner emits no
// header because its chrome already names the project.
func (m *OutlinerModel) appendProjectGroupHeader(
	template outlinerRow,
	projectID string,
	tasks []*models.Task,
	collapseKey string,
) bool {
	if m.project != nil || m.isProjectScoped() {
		return true
	}
	header := template
	header.IsProjectHeader = true
	header.ProjectID = projectID
	header.SectionCount = len(tasks)
	header.CollapseKey = collapseKey
	header.OverdueCount, header.TodayCount = m.projectGroupPressure(tasks, projectID)
	// A Today group is today-planned by construction, so the count there could
	// only restate the group's own size.
	if header.IsTodaySection {
		header.TodayCount = 0
	}
	m.rows = append(m.rows, header)
	return !m.isSectionRowCollapsed(header)
}

// projectGroupPressure counts the overdue and Today-planned work one project
// group holds, so a header still names that pressure once the group is folded.
func (m *OutlinerModel) projectGroupPressure(tasks []*models.Task, projectID string) (overdue, today int) {
	now := time.Now()
	planned := m.todayPlanIdentities()
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if pulse.TaskIsOverdueAt(task, now) {
			overdue++
		}
		if planned[taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}] {
			today++
		}
	}
	return overdue, today
}

// todayPlanIdentities reads the durable Today membership directly, so a header
// counts the same plan every projection resolves from without threading the
// mode input through the row appenders.
func (m *OutlinerModel) todayPlanIdentities() map[taskref.TaskIdentity]bool {
	if m.runtime == nil {
		return nil
	}
	plan := m.runtime.Planning().GetToday()
	identities := make(map[taskref.TaskIdentity]bool, len(plan))
	for _, item := range plan {
		identities[taskref.TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}] = true
	}
	return identities
}

// appendProjectGroupedForests emits every project group of one section.
func (m *OutlinerModel) appendProjectGroupedForests(
	groups []projectRowGroup,
	template outlinerRow,
	collapseKeyFor func(projectID string) string,
) {
	for _, group := range groups {
		if m.appendProjectGroupHeader(
			template, group.projectID, group.forest.tasks(), collapseKeyFor(group.projectID),
		) {
			m.flattenSectionRowForest(group.forest, &m.rows)
		}
	}
}

// tasks returns every task the forest holds, in flattened render order.
func (f sectionRowForest) tasks() []*models.Task {
	tasks := make([]*models.Task, 0, f.count)
	var walk func([]*sectionRowNode)
	walk = func(nodes []*sectionRowNode) {
		for _, node := range nodes {
			tasks = append(tasks, node.row.task)
			walk(node.children)
		}
	}
	walk(f.roots)
	return tasks
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
