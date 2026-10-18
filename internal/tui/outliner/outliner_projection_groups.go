package outliner

import (
	"cmp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

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
// and keeps the task's depth, so a group of one does not spend a row naming
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

// mergeProjectGroupRows lets the lone top-level task name its project without
// changing task columns. Its header connector becomes blank space; descendants
// keep their prefixes so their checkboxes also align with other project trees.
func (m *OutlinerModel) mergeProjectGroupRows(start int) {
	if start < 0 || start >= len(m.Rows) {
		return
	}
	m.Rows[start].Prefix = connectorSpace
	m.Rows[start].LeadsMergedGroup = true
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
