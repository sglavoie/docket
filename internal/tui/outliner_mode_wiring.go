package tui

import (
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
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
	// Assigned above the ranked-list return so both branches leave a live wake
	// clock and child census behind: a presentation the user leaves must not
	// resume with a stale one.
	scope := m.workModeScopeProjects()
	m.nextDeferWake = earliestDeferWake(scope, time.Now())
	m.openChildCounts = openChildCounts(scope)
	if m.showsRankedList() {
		m.buildSearchProjectionRows()
		m.expandChecklistRows()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return
	}
	// Only the flat list has a notes tier, so the payloads it retained die with
	// it rather than outliving the rows that named them.
	m.searchNoteMatches = nil

	mode, ok := OutlinerModeByID(m.primaryMode)
	if !ok {
		return
	}

	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	input := m.buildModeCandidateInput(mode.ID, projects)
	// A narrowing query — a lens's saved one or the slash-search input's — is one
	// more facet over this mode's own projection, so it is composed into the
	// shared task predicate before any section is built. Every section emitter,
	// the pinned promotion, and the project grouping then narrow together, and the
	// mode's saved order is untouched.
	narrowing := m.compileActiveNarrowing(projects)
	input.TaskFilter = m.constrainModeFilterToNarrowing(input.TaskFilter, narrowing, projects)
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
	m.attributeNarrowingMatches(narrowing)
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
func openChildCounts(projects []*models.Project) map[taskref.TaskIdentity]int {
	counts := make(map[taskref.TaskIdentity]int)
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
			identity := taskref.For(task, project)
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
	// outright. Every other mode admits those populations only when the query
	// says so, and the query is the facets and the sigils together — one rule,
	// read here through the same two accessors every other consumer reads.
	input.IncludeDeferred = recent || m.admitsDeferredPopulation()
	input.IncludeTerminal = recent || m.admitsTerminalPopulation()
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
) map[taskref.TaskIdentity]time.Time {
	projection := m.jumpProjectionFingerprint()
	if m.recencyProjection != projection || m.recencyKeys == nil {
		m.recencyKeys = make(map[taskref.TaskIdentity]time.Time)
		m.recencyProjection = projection
	}
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range candidates[project.ID] {
			identity := taskref.For(task, project)
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
	excluded map[taskref.TaskIdentity]bool,
	taskFilter func(*models.Task, *models.Project) bool,
) {
	for _, section := range sections {
		if !m.sectionZoomAdmits(section.CollapseKey) {
			continue
		}
		rows := make([]OutlinerModeRow, 0, len(section.Rows))
		for _, row := range section.Rows {
			if row.Task != nil &&
				!excluded[taskref.For(row.Task, row.Project)] &&
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
		if m.sectionCollapsed(section.CollapseKey) {
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
func (m *OutlinerModel) resolveTodayIDs(projects []*models.Project) map[taskref.TaskIdentity]bool {
	ids := make(map[taskref.TaskIdentity]bool)
	if m.runtime == nil {
		return ids
	}
	for _, item := range m.runtime.Planning().GetToday() {
		for _, p := range projects {
			if p == nil || p.ID != item.ProjectID {
				continue
			}
			if t := p.GetTaskByID(item.TaskID); t != nil {
				ids[taskref.For(t, p)] = true
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

// facetPredicateAt is the one definition of the facet gate: the row half of the
// filter query answered by models.Query.Admits, ANDed with the hierarchy axis,
// evaluated at a single instant. Two consumers read it. The work-mode filter is
// it, and the search projection composes it into the candidate boundary before
// scoring. Free text is deliberately absent: it is one clause answered by one
// matcher in scoreSearchProjectionTask, reached through compileActiveNarrowing,
// so no second definition of "matches" exists to disagree with the tiers.
func (m *OutlinerModel) facetPredicateAt(now time.Time) func(*models.Task, *models.Project) bool {
	query := m.rowPredicateQuery()
	narrows := len(query.Clauses) > 0
	return func(t *models.Task, project *models.Project) bool {
		if t == nil {
			return false
		}
		if narrows && !query.Admits(t, m.filterQueryProject(project), now) {
			return false
		}
		return m.passesHierarchyFilter(t, project) && m.passesDependencyFilter(t, project)
	}
}

func (m *OutlinerModel) workModeTaskFilterAt(now time.Time) func(*models.Task, *models.Project) bool {
	return m.facetPredicateAt(now)
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

// passesDependencyFilter evaluates the f~ facet. It sits beside the hierarchy
// axis rather than inside the row predicate for the same reason: blocking is a
// fact about which other tasks name this one, and the reference index that knows
// it is the Outliner's, not the clause evaluator's. Blocked travels with it so
// one facet is answered in one place.
//
// Both readings are one hop and never transitive, and neither is a readiness
// verdict: a soft reference gates nothing, so this narrows what is on screen and
// changes no order, no rank, and no state.
func (m *OutlinerModel) passesDependencyFilter(task *models.Task, project *models.Project) bool {
	if m == nil || m.filter.Dependency == DependencyFilterAll {
		return true
	}
	if task == nil {
		return false
	}
	switch m.filter.Dependency {
	case DependencyFilterBlocked:
		return len(task.Dependencies) > 0
	case DependencyFilterBlocking:
		identity := taskref.For(task, m.filterQueryProject(project))
		subject := models.TaskReference{ProjectID: identity.ProjectID, TaskID: identity.TaskID}
		return len(m.dependencyIndex().Incoming(subject)) > 0
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
	return m.openChildCounts[taskref.For(task, project)]
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
