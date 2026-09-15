package outliner

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// sectionTaskForest is a presentation-only hierarchy containing tasks from one
// authoritative state group. Cross-group children become roots while
// same-group parent/child relationships retain normal tree rendering.
type sectionTaskForest struct {
	roots    []*models.Task
	children map[string][]*models.Task
	count    int
}

// buildStateGroupTaskForests builds the canonical All-mode forests without
// passing through legacy section buckets. Parent/child edges survive only when
// both tasks share the same state group; a cross-group child becomes a root in
// its own group and carries an ancestor breadcrumb when flattened.
func buildStateGroupTaskForests(tasks []*models.Task) map[models.WorkflowStateGroup]*sectionTaskForest {
	return buildTaskForestsByStateGroup(tasks, theme.TaskStateGroup)
}

func buildTaskForestsByStateGroup(
	tasks []*models.Task,
	classify func(*models.Task) models.WorkflowStateGroup,
) map[models.WorkflowStateGroup]*sectionTaskForest {
	ordered := flattenTaskPointers(tasks)
	included := make(map[string]*models.Task, len(ordered))
	for _, task := range ordered {
		included[task.ID] = task
	}

	forests := make(map[models.WorkflowStateGroup]*sectionTaskForest)
	for _, task := range ordered {
		group := classify(task)
		if !group.IsValid() {
			continue
		}
		forest := forests[group]
		if forest == nil {
			forest = &sectionTaskForest{children: make(map[string][]*models.Task)}
			forests[group] = forest
		}
		forest.count++
		if task.ParentID != nil {
			if parent := included[*task.ParentID]; parent != nil && classify(parent) == group {
				forest.children[parent.ID] = append(forest.children[parent.ID], task)
				continue
			}
		}
		forest.roots = append(forest.roots, task)
	}
	return forests
}

// tasks returns every task the forest holds. It walks the forest's own child
// index rather than Task.Children so descendants belonging to another state
// group stay out.
func (f *sectionTaskForest) tasks() []*models.Task {
	if f == nil {
		return nil
	}
	tasks := make([]*models.Task, 0, f.count)
	var walk func([]*models.Task)
	walk = func(nodes []*models.Task) {
		for _, task := range nodes {
			tasks = append(tasks, task)
			walk(f.children[task.ID])
		}
	}
	walk(f.roots)
	return tasks
}

func flattenTaskPointers(tasks []*models.Task) []*models.Task {
	var result []*models.Task
	var walk func([]*models.Task)
	walk = func(nodes []*models.Task) {
		for _, task := range nodes {
			if task == nil {
				continue
			}
			result = append(result, task)
			walk(task.Children)
		}
	}
	walk(tasks)
	return result
}

func taskAncestorBreadcrumb(task *models.Task) string {
	return taskAncestorBreadcrumbAfter(task, "")
}

// taskAncestorBreadcrumbAfter returns the task's ancestor path below an
// optional boundary. The boundary itself and ancestors above it are omitted.
func taskAncestorBreadcrumbAfter(task *models.Task, boundaryID string) string {
	if task == nil || task.Parent == nil {
		return ""
	}
	var reversed []string
	seen := make(map[string]bool)
	for parent := task.Parent; parent != nil && !seen[parent.ID]; parent = parent.Parent {
		if boundaryID != "" && parent.ID == boundaryID {
			break
		}
		seen[parent.ID] = true
		reversed = append(reversed, parent.Title)
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	if len(reversed) == 0 {
		return ""
	}
	return strings.Join(reversed, " › ") + " ›"
}

func (m *OutlinerModel) flattenSectionForest(
	forest *sectionTaskForest,
	projectID string,
	group OutlinerRowGroup,
	rows *[]OutlinerRow,
) {
	if forest == nil {
		return
	}
	var walk func([]*models.Task, []bool)
	walk = func(tasks []*models.Task, ancestorIsLast []bool) {
		for i, task := range tasks {
			isLast := i == len(tasks)-1
			prefix := BuildTreePrefix(ancestorIsLast, isLast)
			children := forest.children[task.ID]
			breadcrumb := ""
			if len(ancestorIsLast) == 0 && task.ParentID != nil {
				breadcrumb = taskAncestorBreadcrumb(task)
			}
			*rows = append(*rows, OutlinerRow{
				Task:               task,
				Group:              group,
				Prefix:             prefix,
				Breadcrumb:         breadcrumb,
				HasVisibleChildren: len(children) > 0,
				ProjectID:          projectID,
			})
			identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}
			if identity.ProjectID == "" && m.Project != nil {
				identity.ProjectID = m.Project.ID
			}
			if len(children) > 0 && m.IsExpanded(identity) {
				next := append(append([]bool(nil), ancestorIsLast...), isLast)
				walk(children, next)
			}
		}
	}
	walk(forest.roots, nil)
}

const (
	pinnedSectionName                  = "Pinned"
	pinnedSectionCollapseKey           = "_synthetic:pinned"
	pinnedProjectSectionCollapsePrefix = "_pinned_project:"
)

// pinnedSectionBuild carries the transient Pinned project groups and the
// qualified identity set used to suppress those same tasks from the active mode
// below.
type pinnedSectionBuild struct {
	groups    []projectRowGroup
	count     int
	projected map[taskref.TaskIdentity]bool
}

// buildPinnedSection realizes Pinned from the same scoped candidates, reference
// time, and session filters as the active mode. Only modes declaring
// ShowsPinned call it. Projected relatives form a
// section-local tree; roots whose parent is outside Pinned keep their ancestor
// breadcrumb. Project groups get a synthetic header in the unscoped Outliner.
// A root ranks by its own pin position rather than its subtree's earliest one,
// so J/K swapping two adjacent rows moves exactly those rows.
func (m *OutlinerModel) buildPinnedSection(input OutlinerModeInput) pinnedSectionBuild {
	if input.TaskFilter == nil {
		input.TaskFilter = m.workModeTaskFilterAt(input.Now)
	}
	projection := buildPinnedModeProjection(input)
	rows := make([]OutlinerRow, 0, len(projection.rows))
	for _, modeRow := range projection.rows {
		row := m.outlinerRowFromModeRow(modeRow)
		row.HasVisibleChildren = false
		row.IsPinnedSection = true
		rows = append(rows, row)
	}
	return pinnedSectionBuild{
		groups:    m.groupSectionRowsByProject(rows, sectionForestOptions{}),
		count:     len(projection.rows),
		projected: projection.projected,
	}
}

func pinnedProjectSectionCollapseKey(projectID string) string {
	return pinnedProjectSectionCollapsePrefix + projectID
}

func (m *OutlinerModel) appendPinnedSection(section pinnedSectionBuild) {
	if section.count == 0 || !m.sectionZoomAdmits(pinnedSectionCollapseKey) {
		return
	}
	m.Rows = append(m.Rows, OutlinerRow{
		Group:           OutlinerRowGroup{SectionKey: pinnedSectionCollapseKey},
		IsSectionHeader: true,
		IsPinnedSection: true,
		SectionName:     pinnedSectionName,
		SectionCount:    section.count,
		CollapseKey:     pinnedSectionCollapseKey,
	})
	if m.sectionCollapsed(pinnedSectionCollapseKey) {
		return
	}

	m.appendProjectGroupedForests(
		section.groups,
		OutlinerRow{IsPinnedSection: true},
		pinnedSectionCollapseKey,
		pinnedProjectSectionCollapseKey,
	)
}

// isPinnedReorderRow reports whether J/K reorders the pin order rather than
// task hierarchy or a durable planning list. A pinned row answers before the
// active mode's own ordering, so Today's durable plan order does not claim it.
func (m *OutlinerModel) IsPinnedReorderRow() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	if m.MultiSelectMode {
		return false
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil || !row.IsPinnedSection {
		return false
	}
	return m.ResolveProjectForRow(row) != nil
}

// pinnedSectionParentAt returns the section-local parent of the pinned row at
// index, or the zero identity when the row sits at the section's root level.
// It reproduces the edge rule buildSectionRowForestWith applies: a parent link is
// retained only when the parent is itself projected into the same group, which
// the row's own group identity answers without reading a header. The backward
// scan is sufficient because the flattener always emits a parent above its
// children and only descends into expanded parents.
func (m *OutlinerModel) pinnedSectionParentAt(index int) taskref.TaskIdentity {
	if index < 0 || index >= len(m.Rows) {
		return taskref.TaskIdentity{}
	}
	row := m.Rows[index]
	if row.Task == nil || row.Task.ParentID == nil {
		return taskref.TaskIdentity{}
	}
	parent := taskref.TaskIdentity{
		ProjectID: m.TaskIdentityForRow(row).ProjectID,
		TaskID:    *row.Task.ParentID,
	}
	for i := index - 1; i >= 0; i-- {
		candidate := m.Rows[i]
		if !candidate.IsTaskRow() || candidate.Task == nil || candidate.Group != row.Group {
			return taskref.TaskIdentity{}
		}
		if m.TaskIdentityForRow(candidate) == parent {
			return parent
		}
	}
	return taskref.TaskIdentity{}
}

// visiblePinnedSiblings returns every visible pinned row index in the delta
// direction that shares the cursor's section-local parent, nearest first. Rows
// belonging to a sibling's expanded subtree are stepped over; anything that is
// not a task row and any task row outside the cursor's group bounds the search,
// so a move can never leave the cursor's Pinned project group.
func (m *OutlinerModel) visiblePinnedSiblings(delta int) []int {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return nil
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	cursorRow := m.Rows[m.Cursor]
	cursorParent := m.pinnedSectionParentAt(m.Cursor)
	var siblings []int
	for i := m.Cursor + step; i >= 0 && i < len(m.Rows); i += step {
		row := m.Rows[i]
		if !row.IsTaskRow() || row.Task == nil || row.Group != cursorRow.Group {
			break
		}
		if m.pinnedSectionParentAt(i) == cursorParent {
			siblings = append(siblings, i)
		}
	}
	return siblings
}

// adjacentVisiblePinnedSibling returns the index of the nearest visible pinned
// sibling in the delta direction.
func (m *OutlinerModel) adjacentVisiblePinnedSibling(delta int) (int, bool) {
	siblings := m.visiblePinnedSiblings(delta)
	if len(siblings) == 0 {
		return 0, false
	}
	return siblings[0], true
}

// pinnedReorderMove swaps the cursor's pin order with the adjacent visible
// pinned sibling. The identity that must end up displayed first is passed
// first: pinned rows sort PinnedAt-descending and the application layer
// resolves an exact tie in favor of its first argument.
func (m *OutlinerModel) PinnedReorderMove(delta int) tea.Cmd {
	if !m.IsPinnedReorderRow() || !m.taskItem.Wired() {
		return nil
	}
	targetIndex, ok := m.adjacentVisiblePinnedSibling(delta)
	if !ok {
		return nil
	}
	cursor := m.TaskIdentityForRow(m.Rows[m.Cursor])
	target := m.TaskIdentityForRow(m.Rows[targetIndex])
	if !cursor.Valid() || !target.Valid() {
		return nil
	}
	if delta < 0 {
		return m.taskItem.ReorderPinned(cursor, target)
	}
	return m.taskItem.ReorderPinned(target, cursor)
}

// pinnedReorderEdge sends the cursor's pin to the far end of its visible pinned
// siblings in one transaction. The order handed down is display order, and it
// names only the peers the cursor would have swapped past one at a time, so the
// redistributed timestamps stay inside that run.
func (m *OutlinerModel) PinnedReorderEdge(delta int) tea.Cmd {
	if !m.IsPinnedReorderRow() || !m.taskItem.Wired() {
		return nil
	}
	rows := m.visiblePinnedSiblings(delta)
	if len(rows) == 0 {
		return nil
	}
	peers := make([]taskref.TaskIdentity, 0, len(rows))
	for _, index := range rows {
		peers = append(peers, m.TaskIdentityForRow(m.Rows[index]))
	}
	moved := m.TaskIdentityForRow(m.Rows[m.Cursor])
	order := edgeMoveOrder(moved, peers, delta)
	for _, identity := range order {
		if !identity.Valid() {
			return nil
		}
	}
	return m.taskItem.ReorderPinnedOrder(moved, order, reorderEdgeLabel(delta))
}

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

// buildTodaySection realizes explicit Today membership in durable order.
// Descendants appear only when they have their own Today membership; the row
// appender restores hierarchy among relatives that share the section.
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

	previousExclude := input.TaskExclude
	input.TaskExclude = func(task *models.Task, project *models.Project) bool {
		return pinned[taskref.For(task, project)] ||
			(previousExclude != nil && previousExclude(task, project))
	}
	for _, modeRow := range BuildTodayModeRows(input) {
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

// appendPlanningRows derives subtree rank from the workspace-wide durable order
// while restoring project context in the unscoped cross-project Outliner. A
// subtree occupies its earliest member's rank. Pinned projects are grouped
// first, then project groups are ordered alphabetically, and all of a
// project's roots are rendered under one header while retaining their relative
// durable order.
func (m *OutlinerModel) appendPlanningRows(
	rows []OutlinerRow,
	group models.WorkflowStateGroup,
	sectionKey string,
) {
	if len(rows) == 0 {
		return
	}
	m.appendProjectGroupedForests(
		m.groupSectionRowsByProject(rows, sectionForestOptions{rankRootsBySubtree: true}),
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

func (m *OutlinerModel) appendPlanningSection(section planningSectionBuild) {
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
		CollapseKey:     collapseKey,
	})
	if !m.sectionCollapsed(collapseKey) {
		m.appendPlanningRows(section.rows, section.group, collapseKey)
	}
}

// removePlanningRowsFromTree removes every explicit planning identity from a
// lifecycle forest while promoting non-planned descendants. This keeps All
// deduplicated without treating descendants as planned.
func removePlanningRowsFromTree(
	tasks []*models.Task,
	projectID string,
	suppressed map[taskref.TaskIdentity]bool,
) []*models.Task {
	if len(tasks) == 0 || len(suppressed) == 0 {
		return tasks
	}
	result := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		children := removePlanningRowsFromTree(task.Children, projectID, suppressed)
		if suppressed[taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}] {
			result = append(result, children...)
			continue
		}
		clone := taskref.CloneTaskDetachingDependencies(task)
		clone.Children = children
		result = append(result, &clone)
	}
	return result
}

// reviewLaneKey names the Review lane on the synthetic rows that carry it. It
// is a lane identity, not a tag: review is a stored boolean on the task, and
// nothing here reads a tag list.
const reviewLaneKey = "review"

const (
	reviewSectionCollapseKey           = "_promoted:review"
	reviewProjectSectionCollapsePrefix = "_promoted_project:review:"
)

type reviewSectionBuild struct {
	groups    []projectRowGroup
	count     int
	projected map[taskref.TaskIdentity]bool
}

// buildReviewSection promotes tasks that currently need attention into the
// sparse All-mode Review lane. Pinning and explicit Today/Next membership are
// stronger claims for every candidate. Doing is stronger only for a scheduled
// check-in: a manual review flag keeps its established claim over lifecycle,
// while an automatic reminder stays visible in place on active work.
func (m *OutlinerModel) buildReviewSection(
	input OutlinerModeInput,
	pinned, today, next, doing map[taskref.TaskIdentity]bool,
) reviewSectionBuild {
	now := input.now()
	result := reviewSectionBuild{projected: make(map[taskref.TaskIdentity]bool)}
	var rows []OutlinerRow
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		identity := taskref.For(task, project)
		if task == nil || project == nil ||
			(task.IsDeferredAt(now) && !input.IncludeDeferred) ||
			!models.TaskNeedsAttention(task, now) ||
			pinned[identity] || today[identity] || next[identity] ||
			(!task.Review && doing[identity]) {
			continue
		}
		if input.TaskExclude != nil && input.TaskExclude(task, project) {
			continue
		}
		if input.TaskFilter != nil && !input.TaskFilter(task, project) {
			continue
		}
		row := m.outlinerRowFromModeRow(candidate.row)
		row.PromotedTag = reviewLaneKey
		row.Breadcrumb = taskAncestorBreadcrumb(task)
		row.HasVisibleChildren = false
		result.projected[identity] = true
		result.count++
		rows = append(rows, row)
	}
	result.groups = m.groupSectionRowsByProject(rows, sectionForestOptions{rankRootsBySubtree: true})
	return result
}

func (m *OutlinerModel) appendReviewSection(section reviewSectionBuild) {
	if section.count == 0 || !m.sectionZoomAdmits(reviewSectionCollapseKey) {
		return
	}
	m.Rows = append(m.Rows, OutlinerRow{
		Group:           OutlinerRowGroup{SectionKey: reviewSectionCollapseKey},
		IsSectionHeader: true,
		SectionName:     "Review",
		SectionCount:    section.count,
		CollapseKey:     reviewSectionCollapseKey,
		PromotedTag:     reviewLaneKey,
	})
	if m.sectionCollapsed(reviewSectionCollapseKey) {
		return
	}
	m.appendProjectGroupedForests(
		section.groups,
		OutlinerRow{PromotedTag: reviewLaneKey},
		reviewSectionCollapseKey,
		func(projectID string) string {
			return reviewProjectSectionCollapsePrefix + projectID
		},
	)
}
