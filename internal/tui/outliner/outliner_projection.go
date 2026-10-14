package outliner

import (
	"cmp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/query"
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

// projectRowGroup is one project's portion of a section, already assembled into
// a section-local forest.
type projectRowGroup struct {
	projectID string
	forest    sectionRowForest
}

// groupSectionRowsByProject splits an ordered section projection into
// per-project forests. The rows arrive sorted by the section's own order, so a
// group's earliest member is its best one, and a group ranks by that member
// under lead — the active sort, a deadline, a touch — with
// compareProjectGroupIDs, pinned projects first and then alphabetical, breaking
// a tie. lead compares two source indexes; nil ranks by position alone. No
// parent/child edge can cross a group boundary because a parent identity is
// always project-local, so splitting before building the forests is equivalent
// to building one forest over the whole section.
func (m *OutlinerModel) groupSectionRowsByProject(
	rows []OutlinerRow,
	opts sectionForestOptions,
	lead func(a, b int) int,
) []projectRowGroup {
	groups := make([]projectRowGroup, 0, len(rows))
	earliest := make([]int, 0, len(rows))
	indexes := make(map[string]int, len(rows))
	grouped := make([][]OutlinerRow, 0, len(rows))
	for sourceIndex, row := range rows {
		index, ok := indexes[row.ProjectID]
		if !ok {
			index = len(groups)
			indexes[row.ProjectID] = index
			groups = append(groups, projectRowGroup{projectID: row.ProjectID})
			earliest = append(earliest, sourceIndex)
			grouped = append(grouped, nil)
		}
		grouped[index] = append(grouped[index], row)
	}
	for index := range groups {
		groups[index].forest = m.buildSectionRowForestWith(grouped[index], opts)
	}
	order := make([]int, len(groups))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		left, right := order[i], order[j]
		if lead == nil {
			return earliest[left] < earliest[right]
		}
		if c := lead(earliest[left], earliest[right]); c != 0 {
			return c < 0
		}
		return m.compareProjectGroupIDs(groups[left].projectID, groups[right].projectID) < 0
	})
	ranked := make([]projectRowGroup, 0, len(groups))
	for _, index := range order {
		ranked = append(ranked, groups[index])
	}
	return ranked
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
	if m.Project != nil {
		if projectID == "" || m.Project.ID == projectID {
			return m.Project
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

// groupsRowsByProject reports whether the current projection splits its sections
// into per-project groups. Both single-project forms name their project in the
// chrome, so they emit no project header and the section alone is the group.
// This is the one predicate: the emitter and the group metadata read it together
// so they can never disagree about which groups exist.
func (m *OutlinerModel) groupsRowsByProject() bool {
	return m.Project == nil && !m.IsProjectScoped()
}

// rowGroupFor builds the structural identity of one group from the collapse keys
// the fold state already stores. A projection that is not project-grouped keeps
// both project halves empty, so a group never names a fold the projection cannot
// draw.
func (m *OutlinerModel) rowGroupFor(sectionKey, projectKey, projectID string) OutlinerRowGroup {
	group := OutlinerRowGroup{SectionKey: sectionKey}
	if m.groupsRowsByProject() {
		group.ProjectKey = projectKey
		group.ProjectID = projectID
	}
	return group
}

// appendProjectGroupHeader emits one synthetic project sub-header and reports
// whether the group's rows should follow. A project-scoped Outliner emits no
// header because its chrome already names the project, which is exactly the
// group carrying no project half.
//
// Every other project group draws its header, folded or expanded, with one
// exception: an expanded group holding exactly one top-level task merges into
// that task's row, which leads with the `[slug]` the header would have drawn
// and sits at the header's depth, so a group of one does not spend a row naming
// a project its only row can name. merged reports that case, and the caller
// hands the rows it then emits to mergeProjectGroupRows. Every row still
// carries the group, so discovery, folding, and navigation read it off the row
// rather than scanning for the header, and a folded header — singletons
// included — is the summary that carries the group's pressure counts and is
// what l expands and every project operation resolves from.
func (m *OutlinerModel) appendProjectGroupHeader(
	template OutlinerRow,
	group OutlinerRowGroup,
	tasks []*models.Task,
	topLevel int,
) (follow, merged bool) {
	if !group.ProjectGrouped() {
		return true, false
	}
	if topLevel == 1 && !m.sectionCollapsed(group.ProjectKey) {
		return true, true
	}
	header := template
	header.Group = group
	header.IsProjectHeader = true
	header.ProjectID = group.ProjectID
	header.SectionCount = len(tasks)
	header.CollapseKey = group.ProjectKey
	header.OverdueCount, header.TodayCount = m.projectGroupPressure(tasks, group.ProjectID)
	// A Today group is today-planned by construction, so the count there could
	// only restate the group's own size.
	if header.IsTodaySection {
		header.TodayCount = 0
	}
	m.Rows = append(m.Rows, header)
	return !m.IsSectionRowCollapsed(header), false
}

// mergeProjectGroupRows seats the rows one merged group emitted from start on
// at the depth its header would have held: the lone top-level task loses its
// connector and leads the group, and every descendant moves up the same one
// level, so the tree beneath it still reads as that task's own.
func (m *OutlinerModel) mergeProjectGroupRows(start int) {
	if start < 0 || start >= len(m.Rows) {
		return
	}
	for index := start; index < len(m.Rows); index++ {
		m.Rows[index].Prefix = trimTreePrefixLevel(m.Rows[index].Prefix)
	}
	m.Rows[start].LeadsMergedGroup = true
}

// trimTreePrefixLevel drops the outermost connector level of one tree prefix.
// Every level BuildTreePrefix writes is one four-column connector.
func trimTreePrefixLevel(prefix string) string {
	runes := []rune(prefix)
	level := len([]rune(connectorSpace))
	if len(runes) <= level {
		return ""
	}
	return string(runes[level:])
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
	if m.Runtime == nil {
		return nil
	}
	plan := m.Runtime.Planning().GetToday()
	identities := make(map[taskref.TaskIdentity]bool, len(plan))
	for _, item := range plan {
		identities[taskref.TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}] = true
	}
	return identities
}

// appendProjectGroupedForests emits every project group of one section, stamping
// each header and each member row with the same group identity. sectionKey is
// the owning section's collapse key, empty for a projection that draws no
// section header at all.
func (m *OutlinerModel) appendProjectGroupedForests(
	groups []projectRowGroup,
	template OutlinerRow,
	sectionKey string,
	collapseKeyFor func(projectID string) string,
) {
	for _, group := range groups {
		identity := m.rowGroupFor(sectionKey, collapseKeyFor(group.projectID), group.projectID)
		follow, merged := m.appendProjectGroupHeader(template, identity, group.forest.tasks(), len(group.forest.roots))
		if !follow {
			m.hideSectionRowNodes(group.forest.roots)
			continue
		}
		start := len(m.Rows)
		m.flattenSectionRowForest(group.forest, identity, &m.Rows)
		if merged {
			m.mergeProjectGroupRows(start)
		}
	}
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
		Group:           OutlinerRowGroup{SectionKey: collapseKey},
		IsSectionHeader: true,
		SectionName:     group.Label(),
		SectionCount:    total,
		StateGroup:      group,
		CollapseKey:     collapseKey,
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

// compareSectionTasks is the one order every Outliner section applies to its
// tasks: priority under the default sort, the explicit field and direction
// otherwise. A tie is 0, so a stable sort keeps the source order — stored,
// planning, or pin order — as the tiebreaker.
func (m *OutlinerModel) compareSectionTasks(a, b *models.Task) int {
	if m.Sort.Field == query.SortByDefault {
		return comparePriorityBand(a, b)
	}
	c := query.CompareTasks(a, b, m.Sort.Field)
	if c == 0 {
		return 0
	}
	if query.ApplySortDirection(c, m.Sort.Ascending) {
		return -1
	}
	return 1
}

// sortSectionRows stably orders a section's rows by compareSectionTasks ahead
// of grouping, so both the tasks and the project blocks follow the active order.
func (m *OutlinerModel) sortSectionRows(rows []OutlinerRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		return m.compareSectionTasks(rows[i].Task, rows[j].Task) < 0
	})
}

// sectionTaskLead is groupSectionRowsByProject's lead for a section sorted by
// sortSectionRows alone.
func (m *OutlinerModel) sectionTaskLead(rows []OutlinerRow) func(a, b int) int {
	return func(a, b int) int {
		return m.compareSectionTasks(rows[a].Task, rows[b].Task)
	}
}

// comparePriorityBand is the default order's only key. Unset priority sorts
// after every set one.
func comparePriorityBand(a, b *models.Task) int {
	return cmp.Compare(models.EffectivePriority(a.Priority), models.EffectivePriority(b.Priority))
}

// sortsByPriorityBand reports whether the tree's sibling runs are arranged by
// the default priority order, which is where J/K may only trade places inside
// one priority band: a swap across bands rewrites stored order and moves no row.
func (m *OutlinerModel) sortsByPriorityBand() bool {
	return m.Sort.Field == query.SortByDefault &&
		m.ActiveOutlinerModeEntry().SupportsManualSort()
}

// bestSectionTask returns the task that leads tasks under compareSectionTasks,
// the one a project block ranks by.
func (m *OutlinerModel) bestSectionTask(tasks []*models.Task) *models.Task {
	var best *models.Task
	for _, task := range tasks {
		if task != nil && (best == nil || m.compareSectionTasks(task, best) < 0) {
			best = task
		}
	}
	return best
}

// projectBlock is one project's block of a tree-built section, carrying the
// best task it ranks by.
type projectBlock[T any] struct {
	item    T
	project *models.Project
	best    *models.Task
}

// rankProjectBlocks orders project blocks by their best task, with
// compareProjectGroupIDs breaking ties. A block holding no task ranks last.
func rankProjectBlocks[T any](m *OutlinerModel, blocks []projectBlock[T]) []projectBlock[T] {
	ranked := slices.Clone(blocks)
	sort.SliceStable(ranked, func(i, j int) bool {
		left, right := ranked[i], ranked[j]
		switch {
		case left.best != nil && right.best == nil:
			return true
		case left.best == nil && right.best != nil:
			return false
		case left.best != nil && right.best != nil:
			if c := m.compareSectionTasks(left.best, right.best); c != 0 {
				return c < 0
			}
		}
		leftID, rightID := "", ""
		if left.project != nil {
			leftID = left.project.ID
		}
		if right.project != nil {
			rightID = right.project.ID
		}
		return m.compareProjectGroupIDs(leftID, rightID) < 0
	})
	return ranked
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

// planningTodayEligible is kept local to the row pipeline so every visual
// caller uses the application layer's canonical planning invariant.
func planningTodayEligible(task *models.Task) bool {
	return task != nil && app.PlanningTodayEligible(task.State())
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

// buildPinnedModeProjection selects visible pinned tasks from the current
// scope, orders them newest-pin-first, and carries ancestor breadcrumbs for
// their eventual synthetic-section rendering. Session 22 owns attaching that
// section to each mode; this helper owns the visibility and ordering contract.
func buildPinnedModeProjection(input OutlinerModeInput) pinnedModeProjection {
	input.Now = input.now()
	projection := pinnedModeProjection{projected: make(map[taskref.TaskIdentity]bool)}
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
		if c := comparePinnedAt(left.row.Task, right.row.Task); c != 0 {
			return c < 0
		}
		return compareOrderedModeTasks(left, right) < 0
	})
	projection.rows = make([]OutlinerModeRow, 0, len(candidates))
	for _, candidate := range candidates {
		row := candidate.row
		row.Breadcrumb = taskAncestorBreadcrumb(row.Task)
		identity := taskref.For(row.Task, row.Project)
		projection.projected[identity] = true
		projection.rows = append(projection.rows, row)
	}
	return projection
}

// comparePinnedAt is pin order: newest pin first, a task without a pin instant
// last.
func comparePinnedAt(a, b *models.Task) int {
	switch {
	case a.PinnedAt != nil && b.PinnedAt == nil:
		return -1
	case a.PinnedAt == nil && b.PinnedAt != nil:
		return 1
	case a.PinnedAt == nil && b.PinnedAt == nil:
		return 0
	}
	return b.PinnedAt.Compare(*a.PinnedAt)
}

func (m *OutlinerModel) buildPrimaryModeRowsWithInput(
	projects []*models.Project,
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
) {
	mode := m.PrimaryMode
	if !isPrimaryOutlinerMode(mode) {
		return
	}

	if mode == chrome.ModeAll {
		m.buildAllPrimaryRows(projects, input, pinned)
		return
	}
	if mode == chrome.ModeToday || mode == chrome.ModeNext {
		m.appendPlanningModeRows(mode, input, pinned)
		return
	}

	switch mode {
	case chrome.ModeInbox, chrome.ModeBacklog:
	default:
		return
	}
	m.flattenPrimaryProjectTrees(m.buildPrimaryProjectTrees(projects, mode, input, pinned), mode)
}

func (m *OutlinerModel) buildAllPrimaryRows(
	projects []*models.Project,
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
) {
	today := m.buildTodaySection(input, pinned)
	next := m.buildNextSection(input, pinned, today.suppressed)
	doing := projectedDoingTasks(input)
	review := m.buildReviewSection(input, pinned, today.suppressed, next.suppressed, doing)

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

// projectedDoingTasks names the qualified rows claimed by the lifecycle Doing
// lane so derived sections can apply precedence without conflating state age
// with touch age. The section builders apply their own visibility guards.
func projectedDoingTasks(input OutlinerModeInput) map[taskref.TaskIdentity]bool {
	projected := make(map[taskref.TaskIdentity]bool)
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		if task == nil || project == nil || task.State() != models.StateDoing {
			continue
		}
		projected[taskref.For(task, project)] = true
	}
	return projected
}

func (m *OutlinerModel) appendPlanningModeRows(
	mode chrome.OutlinerModeID,
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
) {
	// Today and Next render their planning body ungrouped, under no header of
	// their own, so no zoom key can ever name it. Today's Pinned section is the
	// one zoom reachable here and it must leave the body out entirely.
	if m.Focus.Zoom.Active() {
		return
	}
	var modeRows []OutlinerModeRow
	switch mode {
	case chrome.ModeToday:
		modeRows = BuildTodayModeRows(input)
	case chrome.ModeNext:
		modeRows = BuildNextModeRows(input)
	default:
		return
	}

	rows := make([]OutlinerRow, 0, len(modeRows))
	for _, modeRow := range modeRows {
		if modeRow.Task == nil || modeRow.Project == nil ||
			pinned[taskref.For(modeRow.Task, modeRow.Project)] {
			continue
		}
		row := m.outlinerRowFromModeRow(modeRow)
		row.IsTodaySection = mode == chrome.ModeToday
		row.IsNextSection = mode == chrome.ModeNext
		rows = append(rows, row)
	}
	group := models.StateGroupToday
	if mode == chrome.ModeNext {
		group = models.StateGroupNext
	}
	// The dedicated modes draw no section header, so their groups name the
	// planning project fold alone.
	m.appendPlanningRows(rows, group, "")
}

func (m *OutlinerModel) buildPrimaryProjectTrees(
	projects []*models.Project,
	mode chrome.OutlinerModeID,
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
) []primaryProjectTree {
	var trees []primaryProjectTree
	for _, project := range projects {
		if project == nil {
			continue
		}
		roots := m.primaryModeRoots(project)
		hardExclude := m.primaryDefaultExclusion(input.Now)
		if mode == chrome.ModeBacklog {
			defaultExclude := hardExclude
			hardExclude = func(task *models.Task) bool {
				return defaultExclude(task) || input.TodayIDs[taskref.For(task, project)]
			}
		}
		roots = excludeTaskTreePromotingChildren(roots, hardExclude)

		membership := func(task *models.Task) bool {
			switch mode {
			case chrome.ModeInbox:
				return task.IsInState(models.StateInbox)
			case chrome.ModeBacklog:
				return task.IsInState(models.StateBacklog) &&
					!input.TodayIDs[taskref.For(task, project)]
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
		if mode == chrome.ModeInbox {
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
		if mode == chrome.ModeInbox || mode == chrome.ModeBacklog {
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
	projected map[taskref.TaskIdentity]bool,
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
		if projected[taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}] {
			result = append(result, children...)
			continue
		}
		clone := taskref.CloneTaskDetachingDependencies(task)
		clone.Children = children
		result = append(result, &clone)
	}
	return result
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
