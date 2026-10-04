package outliner

import (
	"fmt"
	"slices"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
)

// buildOutlinerModeInput assembles the common scoped input all canonical mode
// strategies consume, resolving durable planning records once.
func (m *OutlinerModel) buildOutlinerModeInput(projects []*models.Project) OutlinerModeInput {
	now := time.Now()
	input := OutlinerModeInput{
		Projects:   projects,
		TodayIDs:   m.resolveTodayIDs(projects),
		TodayOrder: m.resolveTodayOrder(projects),
		NextOrder:  planningNextOrder(m.Runtime),
		Now:        now,
	}
	input.TaskFilter = m.workModeTaskFilterAt(now)
	return input
}

// projectsInCatalogOrder returns the supplied project subset in the workspace
// catalog's durable order. Unknown projects retain their input order after all
// cataloged projects so incomplete test/runtime snapshots remain deterministic.
func (m *OutlinerModel) ProjectsInCatalogOrder(projects []*models.Project) []*models.Project {
	ordered := slices.Clone(projects)
	if m == nil || m.Runtime == nil {
		return ordered
	}
	ranks := make(map[string]int, len(m.Runtime.Catalog().Projects))
	for index, project := range m.Runtime.Catalog().Projects {
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
	// Freeze the day before every branch, including ranked results and defensive
	// exits. A later periodic tick can then tell whether calendar-derived review
	// membership was answered on the day currently on screen.
	m.timeDrivenDay = time.Now()
	// Assigned above the ranked-list return so both branches leave a live wake
	// clock and child census behind: a presentation the user leaves must not
	// resume with a stale one.
	scope := m.WorkModeScopeProjects()
	m.NextDeferWake = earliestDeferWake(scope, time.Now())
	m.openChildCounts = query.OpenChildCounts(scope)
	if m.ShowsRankedList() {
		m.buildSearchProjectionRows()
		m.finalizeModeRows()
		return
	}
	// Only the flat list has a notes tier, so the payloads it retained die with
	// it rather than outliving the rows that named them.
	m.Search.clearNoteMatches()

	mode, ok := chrome.OutlinerModeByID(m.PrimaryMode)
	if !ok {
		m.finalizeModeRows()
		return
	}

	projects := m.ProjectsInCatalogOrder(m.WorkModeScopeProjects())
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
	pinnedBoundary := len(m.Rows)

	switch mode.RowStrategy {
	case chrome.RowStrategyAll, chrome.RowStrategyInbox, chrome.RowStrategyToday, chrome.RowStrategyNext, chrome.RowStrategyBacklog:
		m.buildPrimaryModeRowsWithInput(projects, input, pinned.projected)
	case chrome.RowStrategyDue:
		m.appendFilteredModeSections(BuildDueModeSections(input), pinned.projected, input.TaskFilter)
	case chrome.RowStrategyRecent:
		m.appendFilteredModeSections(BuildRecentModeSections(input), pinned.projected, input.TaskFilter)
	default:
		m.finalizeModeRows()
		return
	}
	m.separatePinnedSection(pinnedBoundary)
	m.attributeNarrowingMatches(narrowing)
	m.finalizeModeRows()
}

// TimeDrivenRefreshOwedAt reports whether the canonical rows were last answered
// on a different local calendar day. A zero day means no projection has been
// built yet; the first build establishes the baseline without making startup a
// synthetic refresh.
func (m *OutlinerModel) TimeDrivenRefreshOwedAt(now time.Time) bool {
	if m == nil || m.timeDrivenDay.IsZero() || now.IsZero() {
		return false
	}
	return util.DaysBetween(m.timeDrivenDay, now) != 0
}

// InvalidateTimeDrivenRows leaves one projection rebuild owed and remembers the
// cursor by mode identity. The render path remains the rebuild authority; when
// it consumes the invalidation, restoreTimeDrivenCursor keeps a row promoted by
// a newly arrived check-in from moving the cursor onto a neighbor.
func (m *OutlinerModel) InvalidateTimeDrivenRows() {
	if m == nil {
		return
	}
	if !m.timeDrivenCursorPending {
		m.timeDrivenCursor = m.captureModeCursor()
		m.timeDrivenCursorPending = true
	}
	m.Population.Invalidate()
}

func (m *OutlinerModel) restoreTimeDrivenCursor() {
	if m == nil || !m.timeDrivenCursorPending {
		return
	}
	anchor := m.timeDrivenCursor
	m.timeDrivenCursor = modeCursorAnchor{}
	m.timeDrivenCursorPending = false
	m.restoreModeCursor(anchor)
	m.ClampCursor()
}

// finalizeModeRows is the single row-finalization seam every buildCanonicalModeRows
// exit passes through — the ranked-search branch, the two defensive early
// returns, and the ordinary tail alike. Whatever a mode strategy emitted, the
// projection is closed the same way: checklist items expand under their owners,
// a focused projection gains its one project header above everything, selections
// drop the rows that are no longer visible, and the cursor lands inside the
// result. The header is prepended after the body is built so that every index a
// strategy computed while emitting — the pinned boundary above all — still names
// the row it was taken from.
func (m *OutlinerModel) finalizeModeRows() {
	m.expandChecklistRows()
	m.prependFocusedProjectHeader()
	m.pruneInvisibleSelections()
	m.ClampCursor()
}

// prependFocusedProjectHeader puts one actionable project header above the
// content of a focused projection, in both single-project forms: a direct
// project model and an explicit project scope. It is emitted from the resolved
// owner rather than from the rows, so an empty mode, a query that matches
// nothing, a lens, and a subtree focus all keep it — the header is context for
// the projection, not a search result inside it. An unresolvable scope owner
// emits nothing at all, so a missing project is never dressed up as an empty
// one.
func (m *OutlinerModel) prependFocusedProjectHeader() {
	project := m.FocusedProjectHeaderOwner()
	if project == nil {
		return
	}
	header := OutlinerRow{
		IsProjectHeader:        true,
		IsFocusedProjectHeader: true,
		ProjectID:              project.ID,
	}
	m.Rows = slices.Insert(m.Rows, 0, header)
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

// buildModeCandidateInput completes the shared mode input with the focus-narrowed
// candidates and the membership a mode owns rather than filters for.
func (m *OutlinerModel) buildModeCandidateInput(
	mode chrome.OutlinerModeID,
	projects []*models.Project,
) OutlinerModeInput {
	input := m.buildOutlinerModeInput(projects)
	recent := mode == chrome.ModeRecent
	// Recent reports what happened, so it owns terminal and deferred membership
	// outright. Every other mode admits those populations only when the query
	// says so, and the query is the facets and the sigils together — one rule,
	// read here through the same two accessors every other consumer reads.
	input.IncludeDeferred = recent || m.admitsDeferredPopulation()
	input.IncludeTerminal = recent || m.AdmitsTerminalPopulation()
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
	projects := m.ProjectsInCatalogOrder(m.WorkModeScopeProjects())
	// This is the one mode input built outside buildCanonicalModeRows, so the
	// census the hierarchy facet reads would otherwise be whichever scope ran
	// last.
	m.openChildCounts = query.OpenChildCounts(projects)
	input := m.buildModeCandidateInput(chrome.ModeRecent, projects)
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
	m.Recency.rebase(m.JumpProjectionFingerprint())
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range candidates[project.ID] {
			identity := taskref.For(task, project)
			if !identity.Valid() {
				continue
			}
			m.Recency.freeze(identity, task.UpdatedAt)
		}
	}
	return m.Recency.frozenKeys()
}

// separatePinnedSection closes Pinned with a rule so the promoted rows never
// read as the first group of the mode below them.
func (m *OutlinerModel) separatePinnedSection(boundary int) {
	if boundary == 0 || boundary >= len(m.Rows) {
		return
	}
	m.Rows = slices.Insert(m.Rows, boundary, OutlinerRow{IsSeparator: true})
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
		projectedRows := make([]OutlinerRow, 0, len(rows))
		for _, row := range rows {
			projectedRows = append(projectedRows, m.outlinerRowFromModeRow(row))
		}
		var lead func(a, b int) int
		if section.Lead != nil {
			lead = func(a, b int) int { return section.Lead(rows[a], rows[b]) }
		}
		groups := m.groupSectionRowsByProject(projectedRows, sectionForestOptions{rankRootsBySubtree: true}, lead)
		total := 0
		for _, group := range groups {
			total += group.forest.count
		}
		m.Rows = append(m.Rows, OutlinerRow{
			Group:            OutlinerRowGroup{SectionKey: section.CollapseKey},
			IsSectionHeader:  true,
			IsComputedHeader: true,
			SectionName:      section.Name,
			SectionCount:     total,
			CollapseKey:      section.CollapseKey,
		})
		if m.sectionCollapsed(section.CollapseKey) {
			continue
		}
		m.appendProjectGroupedForests(groups, OutlinerRow{}, section.CollapseKey, func(projectID string) string {
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
	if m.Runtime == nil {
		return ids
	}
	for _, item := range m.Runtime.Planning().GetToday() {
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
	if m.Runtime == nil {
		return nil
	}
	plan := m.Runtime.Planning().GetToday()
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
func (m *OutlinerModel) FacetPredicateAt(now time.Time) func(*models.Task, *models.Project) bool {
	return m.facetPredicateForRowsAt(m.rowPredicateQuery(), now)
}

// facetPredicateForRowsAt also evaluates a diagnostic's detached row query,
// preserving the live hierarchy and dependency restrictions.
func (m *OutlinerModel) facetPredicateForRowsAt(query models.Query, now time.Time) func(*models.Task, *models.Project) bool {
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
	return m.FacetPredicateAt(now)
}

// passesHierarchyFilter evaluates the hierarchy facet. The parent and leaf axes
// read the scope's open-direct-child census, the same source the ›N marker
// reads, so the facet and the marker agree by construction. The child and root
// axes read the stored ParentID, which survives cloning.
func (m *OutlinerModel) passesHierarchyFilter(task *models.Task, project *models.Project) bool {
	switch m.Filter.Hierarchy {
	case query.HierarchyChild:
		return task.ParentID != nil
	case query.HierarchyRoot:
		return task.ParentID == nil
	case query.HierarchyParent:
		return m.openChildCountFor(task, project) > 0
	case query.HierarchyLeaf:
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
	if m == nil || m.Filter.Dependency == query.DependencyFilterAll {
		return true
	}
	if task == nil {
		return false
	}
	switch m.Filter.Dependency {
	case query.DependencyFilterBlocked:
		return len(task.Dependencies) > 0
	case query.DependencyFilterBlocking:
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
		project = m.Project
	}
	return m.openChildCounts[taskref.For(task, project)]
}

// outlinerRowFromModeRow adapts a representation-neutral OutlinerModeRow to a
// concrete flat display row, carrying the owning project ID in cross-project
// scope so badges resolve.
func (m *OutlinerModel) outlinerRowFromModeRow(mr OutlinerModeRow) OutlinerRow {
	row := OutlinerRow{
		Task:               mr.Task,
		Breadcrumb:         mr.Breadcrumb,
		HasVisibleChildren: false,
	}
	if m.Project == nil && mr.Project != nil {
		row.ProjectID = mr.Project.ID
	}
	return row
}

// workModeCycleOrder is derived from the catalog so presentation identity and
// keyboard cycling cannot drift.
var workModeCycleOrder = func() []chrome.OutlinerModeID {
	modes := chrome.OutlinerTabModes()
	ids := make([]chrome.OutlinerModeID, len(modes))
	for i, mode := range modes {
		ids[i] = mode.ID
	}
	return ids
}()

var recentStaleFilterClearedStatus = fmt.Sprintf("Stale filter cleared: Recent only includes the last %d days", recentModeCutoffDays)

// clearStaleForRecentMode enforces the facet contract at every path that can
// activate Recent, including restoration of a captured project-scope frame.
func (m *OutlinerModel) clearStaleForRecentMode() {
	if m.PrimaryMode != chrome.ModeRecent || !m.Filter.Stale {
		return
	}
	m.ReplaceFilterClauses([]models.QueryField{models.QueryFieldStale}, nil)
	m.SetTransientStatus(recentStaleFilterClearedStatus)
}

// activeOutlinerMode returns the sole mode identity owned by the Outliner.
func (m *OutlinerModel) ActiveOutlinerMode() chrome.OutlinerModeID {
	if _, ok := chrome.OutlinerModeByID(m.PrimaryMode); ok {
		return m.PrimaryMode
	}
	return chrome.ModeAll
}

// activeOutlinerModeEntry returns the catalog entry for the active mode.
func (m *OutlinerModel) ActiveOutlinerModeEntry() chrome.OutlinerMode {
	mode, _ := chrome.OutlinerModeByID(m.ActiveOutlinerMode())
	return mode
}

// modeCursorAnchor captures the cursor's project-qualified task identity and
// viewport (numeric row index and persistent scroll offset) before a mode
// transition rebuilds the row list, so continuity can be restored afterward.
//
// It deliberately holds no group. A mode transition reshapes the structure
// itself, and the fold namespaces two modes emit overlap by design — All's Today
// lane and the Today mode group one project under the same key — so a group-wide
// seek across the transition would land the cursor on the destination's header
// rather than where the arithmetic of the old view put it. Group-driven restores
// belong to the fold toggles, which reshape one axis of a projection that stays
// the same mode.
type modeCursorAnchor struct {
	identity     taskref.TaskIdentity
	hasTask      bool
	prevCursor   int
	scrollOffset int
}

// captureModeCursor snapshots the selected task identity and viewport before a
// mode transition. The identity is project-qualified even in single-project mode
// (where task rows omit ProjectID) so restoreModeCursor can match on the same key
// the cross-project view uses.
func (m *OutlinerModel) captureModeCursor() modeCursorAnchor {
	anchor := modeCursorAnchor{prevCursor: m.Cursor, scrollOffset: m.ViewScrollOffset}
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) {
		row := m.Rows[m.Cursor]
		if row.IsTaskRow() && row.Task != nil && row.Task.ID != "" {
			projectID := row.ProjectID
			if m.Project != nil {
				projectID = m.Project.ID
			}
			anchor.identity = taskref.TaskIdentity{ProjectID: projectID, TaskID: row.Task.ID}
			anchor.hasTask = true
		}
	}
	return anchor
}

// restoreModeCursor re-seats the cursor after a mode transition rebuilt the
// rows. When the previously selected task is still visible its row is restored
// exactly (identity match, project-qualified in the cross-project view) and the
// captured scroll offset is preserved so the persistent viewport keeps it on
// screen without an unconditional top reset.
//
// When the task is absent — or the cursor never held one, as on a header row —
// the anchor degrades to the nearest navigable row at the pre-transition index,
// clamped into the (possibly differently shaped) new row list.
func (m *OutlinerModel) restoreModeCursor(anchor modeCursorAnchor) {
	m.ViewScrollOffset = anchor.scrollOffset
	if anchor.hasTask {
		for i, row := range m.Rows {
			if !row.IsTaskRow() || row.Task == nil || row.Task.ID != anchor.identity.TaskID {
				continue
			}
			if m.Project == nil && row.ProjectID != anchor.identity.ProjectID {
				continue
			}
			m.Cursor = i
			return
		}
	}
	idx := anchor.prevCursor
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.Rows) {
		idx = len(m.Rows) - 1
	}
	if idx < 0 {
		idx = 0
	}
	m.Cursor = idx
	m.ClampCursor()
}

// selectOutlinerMode applies the named catalog mode. It is the sole entry
// point used by Tab cycling, startup, and any command that jumps directly to a
// mode. It captures the selected task identity
// before rebuilding and restores it afterward (see restoreModeCursor) so an
// eligible task keeps focus and scroll position across the reshape.
// A held query never refuses a mode change: the query is a clause the new mode
// narrows with, so the mode keeps its own lanes and grouping and simply shows
// fewer rows.
func (m *OutlinerModel) SelectOutlinerMode(id chrome.OutlinerModeID) (tea.Model, tea.Cmd) {
	mode, ok := chrome.OutlinerModeByID(id)
	if !ok {
		return m, nil
	}
	// A status naming the mode it was raised in would otherwise outlive the
	// transition and misreport the new mode, up to a tick before it expires.
	m.ClearStatus()
	// The mode decides which rows exist and which operations apply to them, so a
	// menu resolved against the old one is describing a view that is about to stop
	// being on screen.
	m.closeMenuBar()
	anchor := m.captureModeCursor()
	defer m.restoreModeCursor(anchor)

	// Entering Recent re-reads activity, so gr over an already-open Recent is the
	// manual refresh.
	if mode.ID == chrome.ModeRecent {
		m.Recency.thaw()
	}
	// A zoom names a section of the mode it was taken in, so carrying it across
	// would silence every gated emitter of a mode that never emits that key.
	m.clearSectionZoom()
	m.applyOutlinerModeTransition(mode)
	m.PrimaryMode = mode.ID
	m.clearStaleForRecentMode()
	m.Population.Invalidate()
	m.RebuildRows()
	return m, nil
}

// focusLatestActivity enters Recent and lands the cursor on the newest task it
// admits. Entry re-reads activity, so the landing row is the task you touched
// last rather than the one that led the snapshot you have been triaging.
func (m *OutlinerModel) FocusLatestActivity() tea.Cmd {
	staleWasActive := m.Filter.Stale
	m.SelectOutlinerMode(chrome.ModeRecent)
	staleWasCleared := staleWasActive && !m.Filter.Stale
	task, project, sectionKey, ok := m.recentActivityLead()
	if !ok {
		if !staleWasCleared {
			m.SetTransientStatus(fmt.Sprintf("No activity in the last %d days", recentModeCutoffDays))
		}
		return nil
	}
	identity := taskref.For(task, project)
	index, found := m.RowIndexForTask(identity)
	if !found {
		m.revealRecentActivityLead(task, identity, sectionKey)
		m.RebuildRows()
		if index, found = m.RowIndexForTask(identity); !found {
			if !staleWasCleared {
				m.SetTransientStatus("Latest activity is unavailable in this projection")
			}
			return nil
		}
	}
	oldCursor := m.Cursor
	// The mode transition carries the origin's scroll offset over, which would
	// seat the lead against the viewport edge with the section and project
	// headers naming it scrolled off. Recent puts the lead near the top, so
	// reading the window from row zero keeps those headers on screen.
	m.ViewScrollOffset = 0
	m.Cursor = index
	if !staleWasCleared {
		m.SetTransientStatus("Latest activity: " + task.Title)
	}
	return listsurface.FlashIfJumped(oldCursor, index, &m.CursorFlash)
}

// revealRecentActivityLead opens the three things that can hide the lead: its
// bucket, its project group inside that bucket, and its ancestor chain. It runs
// only after a first attempt failed, so an unrelated fold is never disturbed.
func (m *OutlinerModel) revealRecentActivityLead(
	task *models.Task,
	identity taskref.TaskIdentity,
	sectionKey string,
) {
	m.Folds.clearSection(sectionKey)
	m.Folds.clearSection(modeSectionProjectCollapseKey(sectionKey, identity.ProjectID))
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.Folds.setNode(taskref.TaskIdentity{ProjectID: identity.ProjectID, TaskID: ancestor.ID}, true)
	}
}

// cycleWorkMode advances the active mode by delta within the exact seven-item
// cycle.
func (m *OutlinerModel) CycleWorkMode(delta int) (tea.Model, tea.Cmd) {
	current := m.ActiveOutlinerMode()
	idx := -1
	for i, id := range workModeCycleOrder {
		if id == current {
			idx = i
			break
		}
	}

	var target chrome.OutlinerModeID
	if idx < 0 {
		if delta >= 0 {
			target = workModeCycleOrder[0]
		} else {
			target = workModeCycleOrder[len(workModeCycleOrder)-1]
		}
	} else {
		next := (idx + delta) % len(workModeCycleOrder)
		if next < 0 {
			next += len(workModeCycleOrder)
		}
		target = workModeCycleOrder[next]
	}
	return m.SelectOutlinerMode(target)
}

func isPrimaryOutlinerMode(mode chrome.OutlinerModeID) bool {
	switch mode {
	case chrome.ModeAll, chrome.ModeInbox, chrome.ModeToday, chrome.ModeNext, chrome.ModeBacklog:
		return true
	default:
		return false
	}
}

// orderingUnavailableStatus explains why J/K and Ctrl+J/Ctrl+K cannot reorder
// rows in the active mode.
func (m *OutlinerModel) OrderingUnavailableStatus() {
	mode := m.ActiveOutlinerModeEntry()
	m.SetTransientStatus(fmt.Sprintf(
		"Ordering is automatic in %s mode — J/K and Ctrl+J/Ctrl+K reorder hierarchical or planning rows only",
		mode.Label,
	))
}

// workModeScopeProjects returns the projects the seven modes range over: the
// range over: the single project when scoped, otherwise the loaded projects
// admitted by the active Context.
func (m *OutlinerModel) WorkModeScopeProjects() []*models.Project {
	if m.Project != nil {
		return []*models.Project{m.Project}
	}
	return m.ScopeProjects()
}

// OutlinerModeRow is the representation-neutral output of a purpose-built row
// strategy. Membership and ordering remain flat here; the presentation layer
// later nests relatives that are both explicit members of the same section. A
// promoted child keeps its ancestor breadcrumb, and unselected descendants are
// never added implicitly.
type OutlinerModeRow struct {
	Task       *models.Task
	Project    *models.Project
	Breadcrumb string
}

// OutlinerModeInput bundles the scoped data a canonical row strategy needs.
// CandidateTasks optionally narrows eligibility to the current focus projection;
// a present project entry, including an empty one, is authoritative.
type OutlinerModeInput struct {
	Projects []*models.Project
	TodayIDs map[taskref.TaskIdentity]bool
	// CandidateTasks optionally narrows the tasks eligible for a projection per
	// project. A missing project entry falls back to Project.Tasks.
	CandidateTasks map[string][]*models.Task
	// TodayOrder and NextOrder are the durable project-qualified planning orders.
	TodayOrder  []PlanningTask
	NextOrder   []config.ProjectTaskLocator
	TaskFilter  func(task *models.Task, project *models.Project) bool
	TaskExclude func(task *models.Task, project *models.Project) bool
	// RecencyKeys freezes each task's Recent ordering timestamp so triaging a row
	// does not move it out from under the cursor. A task the map does not name
	// orders by its live UpdatedAt.
	RecencyKeys     map[taskref.TaskIdentity]time.Time
	Now             time.Time
	IncludeDeferred bool
	IncludeTerminal bool
}

func (in OutlinerModeInput) now() time.Time {
	if in.Now.IsZero() {
		return time.Now()
	}
	return in.Now
}

func (in OutlinerModeInput) recencyKey(task *models.Task, project *models.Project) time.Time {
	if frozen, ok := in.RecencyKeys[taskref.For(task, project)]; ok {
		return frozen
	}
	return task.UpdatedAt
}

// BuildTodayModeRows realizes the explicit ordered Today projection. Every row
// is a durable member; section-local hierarchy is added by the row appender.
func BuildTodayModeRows(in OutlinerModeInput) []OutlinerModeRow {
	projects := modeProjectIndex(in.Projects)
	now := in.now()
	rows := make([]OutlinerModeRow, 0, len(in.TodayOrder))
	seen := make(map[taskref.TaskIdentity]bool, len(in.TodayOrder))
	for _, item := range in.TodayOrder {
		if item.Task == nil || item.Project == nil {
			continue
		}
		project := projects[item.Project.ID]
		if project == nil {
			continue
		}
		task := project.GetTaskByID(item.Task.ID)
		identity := taskref.TaskIdentity{ProjectID: project.ID, TaskID: item.Task.ID}
		if task == nil || seen[identity] || !modeCandidateContains(in, identity) {
			continue
		}
		seen[identity] = true
		if !planningTodayEligible(task) || (task.IsDeferredAt(now) && !in.IncludeDeferred) {
			continue
		}
		if in.TaskExclude != nil && in.TaskExclude(task, project) {
			continue
		}
		if in.TaskFilter != nil && !in.TaskFilter(task, project) {
			continue
		}
		rows = append(rows, planningModeRow(task, project))
	}
	return rows
}

// BuildNextModeRows realizes the explicit ordered Next projection. Every row is
// a durable member; section-local hierarchy is added by the row appender.
func BuildNextModeRows(in OutlinerModeInput) []OutlinerModeRow {
	projects := modeProjectIndex(in.Projects)
	now := in.now()
	rows := make([]OutlinerModeRow, 0, len(in.NextOrder))
	seen := make(map[taskref.TaskIdentity]bool, len(in.NextOrder))
	for _, locator := range in.NextOrder {
		identity := taskref.TaskIdentity{ProjectID: locator.ProjectID, TaskID: locator.TaskID}
		if seen[identity] || !modeCandidateContains(in, identity) {
			continue
		}
		seen[identity] = true
		project := projects[locator.ProjectID]
		if project == nil {
			continue
		}
		task := project.GetTaskByID(locator.TaskID)
		if task == nil || !task.State().IsValid() || task.IsTerminalState() ||
			(task.IsDeferredAt(now) && !in.IncludeDeferred) {
			continue
		}
		if in.TaskExclude != nil && in.TaskExclude(task, project) {
			continue
		}
		if in.TaskFilter != nil && !in.TaskFilter(task, project) {
			continue
		}
		rows = append(rows, planningModeRow(task, project))
	}
	return rows
}

func modeProjectIndex(projects []*models.Project) map[string]*models.Project {
	index := make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		if project != nil {
			index[project.ID] = project
		}
	}
	return index
}

func modeCandidateContains(in OutlinerModeInput, identity taskref.TaskIdentity) bool {
	candidates, constrained := in.CandidateTasks[identity.ProjectID]
	if !constrained {
		return true
	}
	for _, task := range candidates {
		if task != nil && task.ID == identity.TaskID {
			return true
		}
	}
	return false
}

func planningModeRow(task *models.Task, project *models.Project) OutlinerModeRow {
	return OutlinerModeRow{
		Task:       task,
		Project:    project,
		Breadcrumb: taskAncestorBreadcrumb(task),
	}
}

// OutlinerModeSwitch carries the cross-mode state that must survive a mode
// change. Project scope, search, common filters, cursor identity,
// and compatible column state are preserved verbatim; only the sort is
// reconciled against the target mode's ordering policy.
//
// The switch keeps two sorts: TreeSort is the user's durable manual sort used by
// tree-order modes, and ActiveSort is the sort applied to the current mode.
// Entering a fixed-order mode deactivates manual sort without destroying
// TreeSort, so returning to a tree-order mode restores it.
type OutlinerModeSwitch struct {
	TreeSort   query.SortState
	ActiveSort query.SortState
}

// ApplyOutlinerModeTransition returns the state after switching from prev to
// target. Scope, search, cursor identity, and column state are
// carried over unchanged. The sort is reconciled: the current manual sort is
// stashed into TreeSort when leaving a tree-order mode, and restored (or
// deactivated) according to the target mode's ordering policy.
func ApplyOutlinerModeTransition(state OutlinerModeSwitch, prev, target chrome.OutlinerMode) OutlinerModeSwitch {
	next := state // preserves ProjectScope, Search, CursorTaskID, Columns

	// Persist the durable tree-mode sort before leaving a tree-order mode so a
	// later tree-order mode can restore it.
	if prev.SupportsManualSort() {
		next.TreeSort = state.ActiveSort
	}

	if target.SupportsManualSort() {
		next.ActiveSort = next.TreeSort
	} else {
		// Fixed-order modes own their ordering: manual sort is inactive, but the
		// stored tree-mode sort is left intact.
		next.ActiveSort = query.NewSortState()
	}
	return next
}

// applyOutlinerModeTransition is the model-level seam for reconciling the
// active render sort with the durable tree sort. Callers must invoke it before
// mutating the legacy fields consumed by activeOutlinerMode, so prev reflects
// the mode the user is actually leaving.
func (m *OutlinerModel) applyOutlinerModeTransition(target chrome.OutlinerMode) {
	prev := m.ActiveOutlinerModeEntry()
	next := ApplyOutlinerModeTransition(OutlinerModeSwitch{
		TreeSort:   m.TreeSort,
		ActiveSort: m.Sort,
	}, prev, target)
	m.Sort = next.ActiveSort
	m.TreeSort = next.TreeSort
}
