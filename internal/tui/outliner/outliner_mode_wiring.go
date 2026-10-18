package outliner

import (
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
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
		m.countDrawnSearchRows()
		m.finalizeModeRows()
		return
	}
	// Drop the previous projection's payloads before building its notes anew.
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
	m.appendProjectNotesSection()
	m.countDrawnSearchRows()
	m.finalizeModeRows()
}

// countDrawnSearchRows records how many task and note rows the projection just
// built draws for the held query. It runs ahead of finalizeModeRows, whose
// checklist expansion and focused header add rows that are no result.
func (m *OutlinerModel) countDrawnSearchRows() {
	m.Search.drawn = 0
	if !m.HasSearchProjection() {
		return
	}
	for _, row := range m.Rows {
		if row.IsTaskRow() || row.IsNoteRow {
			m.Search.drawn++
		}
	}
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
	m.indexSectionHeaderRows()
	m.pruneInvisibleSelections()
	m.ClampCursor()
}

// indexSectionHeaderRows records where each lane's header landed. It runs here
// rather than where a header is appended because three later writers move rows:
// the Pinned separator, the checklist expansion, and the focused header above,
// so this is the first point an index names the row it was taken from.
func (m *OutlinerModel) indexSectionHeaderRows() {
	clear(m.sectionHeaderRows)
	clear(m.projectHeaderRows)
	for index, row := range m.Rows {
		if (row.IsProjectHeader || row.LeadsMergedGroup) && row.Group.ProjectKey != "" {
			if m.projectHeaderRows == nil {
				m.projectHeaderRows = make(map[string]int)
			}
			m.projectHeaderRows[row.Group.ProjectKey] = index
		}
		if !row.IsSectionHeader || row.Group.SectionKey == "" {
			continue
		}
		if m.sectionHeaderRows == nil {
			m.sectionHeaderRows = make(map[string]int)
		}
		m.sectionHeaderRows[row.Group.SectionKey] = index
	}
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
			m.hideProjectRowGroups(groups)
			continue
		}
		m.appendProjectGroupedForests(groups, OutlinerRow{}, section.CollapseKey, func(projectID string) string {
			return modeSectionProjectCollapseKey(section.CollapseKey, projectID)
		})
	}
}
