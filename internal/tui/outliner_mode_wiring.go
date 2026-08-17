package tui

import (
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// buildOutlinerModeInput assembles the common scoped input all canonical mode
// strategies consume, resolving durable planning records once.
func (m *OutlinerModel) buildOutlinerModeInput(projects []*models.Project) OutlinerModeInput {
	now := time.Now()
	input := OutlinerModeInput{
		Projects:   projects,
		TodayIDs:   m.resolveTodayIDs(projects),
		TodayOrder: m.resolveTodayOrder(projects),
		NextOrder:  planningNextOrder(m.runtime),
		Now:        now,
	}
	input.TaskFilter = m.workModeTaskFilterAt(now)
	return input
}

// projectsInCatalogOrder returns the supplied project subset in the workspace
// catalog's durable order. Unknown projects retain their input order after all
// cataloged projects so incomplete test/runtime snapshots remain deterministic.
func (m *OutlinerModel) projectsInCatalogOrder(projects []*models.Project) []*models.Project {
	ordered := slices.Clone(projects)
	if m == nil || m.runtime == nil {
		return ordered
	}
	ranks := make(map[string]int, len(m.runtime.Catalog().Projects))
	for index, project := range m.runtime.Catalog().Projects {
		ranks[project.ID] = index
	}
	const unranked = int(^uint(0) >> 1)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := unranked, unranked
		if ordered[i] != nil {
			if rank, ok := ranks[ordered[i].ID]; ok {
				left = rank
			}
		}
		if ordered[j] != nil {
			if rank, ok := ranks[ordered[j].ID]; ok {
				right = rank
			}
		}
		return left < right
	})
	return ordered
}

// buildCanonicalModeRows is the single row-pipeline entry point for all six
// canonical modes. Scope, focus candidates, common filters, search, and fixed
// mode order are assembled once; the catalog strategy changes only membership
// and grouping.
func (m *OutlinerModel) buildCanonicalModeRows() {
	// Assigned above the search-projection return so both branches leave a live
	// wake clock and child census behind: a projection the user leaves must not
	// resume with a stale one.
	scope := m.workModeScopeProjects()
	m.nextDeferWake = earliestDeferWake(scope, time.Now())
	m.openChildCounts = openChildCounts(scope)
	if m.hasSearchProjection() {
		m.buildSearchProjectionRows()
		m.expandChecklistRows()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return
	}

	mode, ok := OutlinerModeByID(m.primaryMode)
	if !ok {
		return
	}

	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	input := m.buildModeCandidateInput(mode.ID, projects)
	var pinned pinnedSectionBuild
	if mode.ShowsPinned {
		pinned = m.buildPinnedSection(input)
		m.appendPinnedSection(pinned)
	}
	pinnedBoundary := len(m.rows)

	switch mode.RowStrategy {
	case RowStrategyAll, RowStrategyInbox, RowStrategyToday, RowStrategyNext, RowStrategyBacklog:
		m.buildPrimaryModeRowsWithInput(projects, input, pinned.projected)
	case RowStrategyDue:
		m.appendFilteredModeSections(BuildDueModeSections(input), pinned.projected, input.TaskFilter)
	case RowStrategyRecent:
		m.appendFilteredModeSections(BuildRecentModeSections(input), pinned.projected, input.TaskFilter)
	default:
		return
	}
	m.separatePinnedSection(pinnedBoundary)
	m.expandChecklistRows()
	m.pruneInvisibleSelections()
	m.clampCursor()
}

// earliestDeferWake returns the soonest instant at which a still-pending
// deferral in the given projects lapses, or the zero time when none is pending.
func earliestDeferWake(projects []*models.Project, now time.Time) time.Time {
	var earliest time.Time
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || !task.IsDeferredAt(now) {
				continue
			}
			if earliest.IsZero() || task.DeferredUntil.Before(earliest) {
				earliest = *task.DeferredUntil
			}
		}
	}
	return earliest
}

// openChildCounts censuses each task's open direct children across the scope.
// It reads the authoritative flat project.Tasks, which already holds every
// nested child, because a row's own task can be a filter-pipeline clone whose
// Children were rewritten — the marker would then mean one thing in All and
// another in a flat mode.
func openChildCounts(projects []*models.Project) map[TaskIdentity]int {
	counts := make(map[TaskIdentity]int)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			open := len(task.IncompleteChildren())
			if open == 0 {
				continue
			}
			identity := taskIdentityFor(task, project)
			if !identity.Valid() {
				continue
			}
			counts[identity] = open
		}
	}
	return counts
}

// buildModeCandidateInput completes the shared mode input with the focus-narrowed
// candidates and the membership a mode owns rather than filters for.
func (m *OutlinerModel) buildModeCandidateInput(
	mode OutlinerModeID,
	projects []*models.Project,
) OutlinerModeInput {
	input := m.buildOutlinerModeInput(projects)
	recent := mode == ModeRecent
	// Recent reports what happened, so it owns terminal and deferred membership
	// instead of waiting for an explicit facet to admit them.
	input.IncludeDeferred = recent || m.hasExplicitDeferredFilter()
	input.IncludeTerminal = recent || m.hasExplicitTerminalFilter()
	input.CandidateTasks = make(map[string][]*models.Task, len(projects))
	for _, project := range projects {
		if project != nil {
			input.CandidateTasks[project.ID] = flattenTaskPointers(m.primaryModeRoots(project))
		}
	}
	if recent {
		input.RecencyKeys = m.freezeRecencyKeys(input.CandidateTasks, projects)
	}
	return input
}

// recentActivityLead returns the newest task Recent admits, along with the
// collapse key of the section holding it. The lead is read from the flat
// section order rather than from the built rows, because grouping later nests a
// promoted child under an older parent and the first row on screen is then not
// the newest task. Returning the section's own key keeps the bucket vocabulary
// in one place.
func (m *OutlinerModel) recentActivityLead() (*models.Task, *models.Project, string, bool) {
	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	// This is the one mode input built outside buildCanonicalModeRows, so the
	// census the hierarchy facet reads would otherwise be whichever scope ran
	// last.
	m.openChildCounts = openChildCounts(projects)
	input := m.buildModeCandidateInput(ModeRecent, projects)
	for _, section := range BuildRecentModeSections(input) {
		for _, row := range section.Rows {
			if row.Task == nil || row.Project == nil {
				continue
			}
			if input.TaskFilter != nil && !input.TaskFilter(row.Task, row.Project) {
				continue
			}
			return row.Task, row.Project, section.CollapseKey, true
		}
	}
	return nil, nil, "", false
}

// freezeRecencyKeys returns Recent's ordering timestamps, recording each task's
// UpdatedAt the first time it is seen. The keys are held against the projection
// fingerprint that already invalidates relocation jump history, so a scope,
// filter, search, sort, or fold change re-reads the world while ordinary
// mutations leave the list the user is triaging in place.
func (m *OutlinerModel) freezeRecencyKeys(
	candidates map[string][]*models.Task,
	projects []*models.Project,
) map[TaskIdentity]time.Time {
	projection := m.jumpProjectionFingerprint()
	if m.recencyProjection != projection || m.recencyKeys == nil {
		m.recencyKeys = make(map[TaskIdentity]time.Time)
		m.recencyProjection = projection
	}
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range candidates[project.ID] {
			identity := taskIdentityFor(task, project)
			if !identity.Valid() {
				continue
			}
			if _, frozen := m.recencyKeys[identity]; !frozen {
				m.recencyKeys[identity] = task.UpdatedAt
			}
		}
	}
	return m.recencyKeys
}

// separatePinnedSection closes Pinned with a rule so the promoted rows never
// read as the first group of the mode below them.
func (m *OutlinerModel) separatePinnedSection(boundary int) {
	if boundary == 0 || boundary >= len(m.rows) {
		return
	}
	m.rows = slices.Insert(m.rows, boundary, outlinerRow{IsSeparator: true})
}

// appendFilteredModeSections composes the shared per-task filter predicate over
// a mode's fixed grouped order, emitting only non-empty sections.
func (m *OutlinerModel) appendFilteredModeSections(
	sections []OutlinerModeSection,
	excluded map[TaskIdentity]bool,
	taskFilter func(*models.Task, *models.Project) bool,
) {
	for _, section := range sections {
		rows := make([]OutlinerModeRow, 0, len(section.Rows))
		for _, row := range section.Rows {
			if row.Task != nil &&
				!excluded[taskIdentityFor(row.Task, row.Project)] &&
				(taskFilter == nil || taskFilter(row.Task, row.Project)) {
				rows = append(rows, row)
			}
		}
		if len(rows) == 0 {
			continue
		}
		projectedRows := make([]outlinerRow, 0, len(rows))
		for _, row := range rows {
			projectedRows = append(projectedRows, m.outlinerRowFromModeRow(row))
		}
		groups := m.groupSectionRowsByProject(projectedRows, sectionForestOptions{rankRootsBySubtree: true})
		total := 0
		for _, group := range groups {
			total += group.forest.count
		}
		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader:  true,
			IsComputedHeader: true,
			SectionName:      section.Name,
			SectionCount:     total,
			CollapseKey:      section.CollapseKey,
		})
		if m.collapsedSections[section.CollapseKey] {
			continue
		}
		m.appendProjectGroupedForests(groups, outlinerRow{}, func(projectID string) string {
			return modeSectionProjectCollapseKey(section.CollapseKey, projectID)
		})
	}
}

func planningNextOrder(runtime *config.RuntimeState) []config.ProjectTaskLocator {
	if runtime == nil {
		return nil
	}
	return runtime.Planning().GetNext()
}

// resolveTodayIDs returns the identity set of daily-planned tasks that
// exist in the scope projects. Derived from the durable plan so it does not
// depend on any externally-maintained indicator cache.
func (m *OutlinerModel) resolveTodayIDs(projects []*models.Project) map[TaskIdentity]bool {
	ids := make(map[TaskIdentity]bool)
	if m.runtime == nil {
		return ids
	}
	for _, item := range m.runtime.Planning().GetToday() {
		for _, p := range projects {
			if p == nil || p.ID != item.ProjectID {
				continue
			}
			if t := p.GetTaskByID(item.TaskID); t != nil {
				ids[taskIdentityFor(t, p)] = true
			}
			break
		}
	}
	return ids
}

// resolveTodayOrder returns the daily-plan tasks in their durable order,
// scoped to loaded projects.
func (m *OutlinerModel) resolveTodayOrder(projects []*models.Project) []PlanningTask {
	if m.runtime == nil {
		return nil
	}
	plan := m.runtime.Planning().GetToday()
	order := make([]PlanningTask, 0, len(plan))
	for _, item := range plan {
		for _, p := range projects {
			if p == nil || p.ID != item.ProjectID {
				continue
			}
			if t := p.GetTaskByID(item.TaskID); t != nil {
				order = append(order, PlanningTask{Task: t, Project: p})
			}
			break
		}
	}
	return order
}

func (m *OutlinerModel) workModeTaskFilterAt(now time.Time) func(*models.Task, *models.Project) bool {
	predicate := m.workModeFilterPredicateAt(now)
	return func(t *models.Task, project *models.Project) bool {
		return predicate(t) && m.passesHierarchyFilter(t, project)
	}
}

// passesHierarchyFilter evaluates the hierarchy facet. The parent and leaf axes
// read the scope's open-direct-child census, the same source the ›N marker
// reads, so the facet and the marker agree by construction. The child and root
// axes read the stored ParentID, which survives cloning.
func (m *OutlinerModel) passesHierarchyFilter(task *models.Task, project *models.Project) bool {
	switch m.filter.Hierarchy {
	case HierarchyChild:
		return task.ParentID != nil
	case HierarchyRoot:
		return task.ParentID == nil
	case HierarchyParent:
		return m.openChildCountFor(task, project) > 0
	case HierarchyLeaf:
		return m.openChildCountFor(task, project) == 0
	default:
		return true
	}
}

// openChildCountFor resolves a task's census entry, falling back to the scoped
// project the way rowIdentity does. A census miss is 0, which reads as leaf.
func (m *OutlinerModel) openChildCountFor(task *models.Task, project *models.Project) int {
	if project == nil {
		project = m.project
	}
	return m.openChildCounts[taskIdentityFor(task, project)]
}

// outlinerRowFromModeRow adapts a representation-neutral OutlinerModeRow to a
// concrete flat display row, carrying the owning project ID in cross-project
// scope so badges resolve.
func (m *OutlinerModel) outlinerRowFromModeRow(mr OutlinerModeRow) outlinerRow {
	row := outlinerRow{
		task:               mr.Task,
		Breadcrumb:         mr.Breadcrumb,
		hasVisibleChildren: false,
	}
	if m.project == nil && mr.Project != nil {
		row.ProjectID = mr.Project.ID
	}
	return row
}
