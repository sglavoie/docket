package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// workModeCycleOrder is the canonical five-item Work cycle driven by Tab /
// Shift+Tab. It is deliberately independent from the h/l workflow-perspective
// rotation: Tab moves between work horizons, h/l rotates GTD workflow lenses.
var workModeCycleOrder = []OutlinerModeID{
	ModeToday,
	ModeNext,
	ModeBacklog,
	ModeActive,
	ModeAll,
}

// activeOutlinerMode derives the current mode-catalog identity from the
// outliner's live state. It is the single source of truth for the mode label,
// tab-bar highlight, and Tab cycling, mapping the existing
// perspective/smart-focus/lens/priority machinery onto the unified catalog so
// no second selection variable can drift out of sync.
func (m *OutlinerModel) activeOutlinerMode() OutlinerModeID {
	switch {
	case m.reviewMode == reviewDaily:
		return ModeDailyReview
	case m.reviewMode == reviewActivity:
		return ModeActivity
	case m.workTodayMode:
		return ModeToday
	case m.workNextMode:
		return ModeNext
	case m.workBacklogMode:
		return ModeBacklog
	case m.project == nil && m.crossPriorityMode:
		return ModePriorityLens
	case m.lens == OutlinerLensDueDate:
		return ModeDueDateLens
	case m.smartFocus != nil:
		if mode, ok := OutlinerModeForSmartFocus(*m.smartFocus); ok {
			return mode.ID
		}
		return ModeAll
	default:
		if mode, ok := OutlinerModeForPerspective(m.activePerspective); ok {
			return mode.ID
		}
		return ModeAll
	}
}

// activeOutlinerModeEntry returns the catalog entry for the active mode.
func (m *OutlinerModel) activeOutlinerModeEntry() OutlinerMode {
	mode, _ := OutlinerModeByID(m.activeOutlinerMode())
	return mode
}

// modeCursorAnchor captures the cursor's project-qualified task identity plus
// the viewport (numeric row index and persistent scroll offset) before a mode
// transition rebuilds the row list, so continuity can be restored afterward.
type modeCursorAnchor struct {
	identity     TaskIdentity
	hasTask      bool
	prevCursor   int
	scrollOffset int
}

// captureModeCursor snapshots the selected task identity and viewport before a
// mode transition. The identity is project-qualified even in single-project mode
// (where task rows omit ProjectID) so restoreModeCursor can match on the same
// key the cross-project view uses.
func (m *OutlinerModel) captureModeCursor() modeCursorAnchor {
	anchor := modeCursorAnchor{prevCursor: m.cursor, scrollOffset: m.viewScrollOffset}
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		if row.isTaskRow() && row.task != nil && row.task.ID != "" {
			projectID := row.ProjectID
			if m.project != nil {
				projectID = m.project.ID
			}
			anchor.identity = TaskIdentity{ProjectID: projectID, TaskID: row.task.ID}
			anchor.hasTask = true
		}
	}
	return anchor
}

// restoreModeCursor re-seats the cursor after a mode transition rebuilt the
// rows. When the previously selected task is still visible its row is restored
// exactly (identity match, project-qualified in the cross-project view) and the
// captured scroll offset is preserved so the persistent viewport keeps it on
// screen without an unconditional top reset. When the task is absent the cursor
// falls back deterministically to the nearest navigable row at the pre-transition
// index, clamped into the (possibly differently shaped) new row list.
func (m *OutlinerModel) restoreModeCursor(anchor modeCursorAnchor) {
	m.viewScrollOffset = anchor.scrollOffset
	if anchor.hasTask {
		for i, row := range m.rows {
			if !row.isTaskRow() || row.task == nil || row.task.ID != anchor.identity.TaskID {
				continue
			}
			if m.project == nil && row.ProjectID != anchor.identity.ProjectID {
				continue
			}
			m.cursor = i
			return
		}
	}
	idx := anchor.prevCursor
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.rows) {
		idx = len(m.rows) - 1
	}
	if idx < 0 {
		idx = 0
	}
	m.cursor = idx
	m.clampCursor()
}

// selectOutlinerMode applies the named catalog mode, translating it into the
// underlying perspective/smart-focus/lens/priority state. It is the sole entry
// point used by Tab cycling, the mode picker, and any command that jumps
// directly to a mode. It captures the selected task identity before rebuilding
// and restores it afterward (see restoreModeCursor) so an eligible task keeps
// focus and scroll position across the reshape.
func (m *OutlinerModel) selectOutlinerMode(id OutlinerModeID) (tea.Model, tea.Cmd) {
	mode, ok := OutlinerModeByID(id)
	if !ok {
		return m, nil
	}
	anchor := m.captureModeCursor()
	defer m.restoreModeCursor(anchor)

	transitionTarget := mode
	// The legacy single-project Priority action is implemented by the
	// High Priority tree focus with an explicit score sort. The global Outliner
	// has the true fixed-order Priority lens.
	if mode.PriorityLens && m.project != nil {
		transitionTarget, _ = OutlinerModeByID(ModeHighPriority)
	}
	m.applyOutlinerModeTransition(transitionTarget)

	switch mode.RowStrategy {
	case RowStrategyToday:
		m.applyWorkComputedMode(true, false, false)
	case RowStrategyBacklog:
		m.applyWorkComputedMode(false, false, true)
	case RowStrategyNext:
		// Global Next is a purpose-built computed horizon: one ungrouped
		// workspace-wide sequence ordered by the durable focus queue. A single
		// project keeps Next as its persisted perspective, whose applyFocusOrdering
		// already orders StateNext tasks by the focus queue within the tree.
		if m.project == nil {
			// Keep the persisted perspective mirror in sync for project-focus
			// consumers while the global/scoped Outliner renders the computed
			// focus-queue horizon.
			m.activatePerspective(mode.Perspective)
			m.applyWorkComputedMode(false, true, false)
		} else {
			m.activatePerspective(mode.Perspective)
			m.lens = OutlinerLensTree
		}
	case RowStrategyPerspective:
		m.activatePerspective(mode.Perspective)
		m.lens = OutlinerLensTree
	case RowStrategySmartFocus:
		m.applySmartFocusMode(*mode.SmartFocus)
	case RowStrategyLens:
		if mode.PriorityLens {
			m.applyPriorityLensMode()
			break
		}
		m.clearComputedModes()
		// The Due Date lens composes only with the always-on common filters
		// (search, pinned tag, sections, actionable, saved criteria). Clear the
		// smart focus so the schedule is not silently narrowed by a stale smart
		// predicate; the assignments below drop the computed, review, and
		// priority discriminators without re-entering another transition path.
		m.smartFocus = nil
		m.crossPriorityMode = false
		m.lens = OutlinerLensDueDate
		m.crossForceRebuild = true
	case RowStrategyReview:
		switch mode.ID {
		case ModeActivity:
			m.applyReviewMode(reviewActivity)
		default:
			m.applyReviewMode(reviewDaily)
		}
	}

	m.rebuildRows()
	return m, nil
}

// clearComputedModes drops the session-only computed horizons so a normal
// perspective/smart-focus/lens selection takes over cleanly.
func (m *OutlinerModel) clearComputedModes() {
	m.workTodayMode = false
	m.workNextMode = false
	m.workBacklogMode = false
	m.reviewMode = reviewNone
}

// applyWorkComputedMode switches into one of the three purpose-built work
// horizons (Today, Next, Backlog). These are flat, fixed-order lists computed from
// the daily plan / focus queue / cross-project ranker, so any active zoom or
// saved filter is cleared to avoid scoping the computed rows to a stale
// subtree.
func (m *OutlinerModel) applyWorkComputedMode(today, next, backlog bool) {
	m.smartFocus = nil
	m.crossPriorityMode = false
	m.activeSavedFilter = ""
	m.activeCriteriaFilter = nil
	m.lens = OutlinerLensTree
	m.workTodayMode = today
	m.workNextMode = next
	m.workBacklogMode = backlog
	m.reviewMode = reviewNone
	m.zoomStack = nil
	m.crossZoomStack = nil
	m.crossForceRebuild = true
	m.cursor = 0
}

// applySmartFocusMode switches to a smart-focus mode (Due Soon, High Priority,
// Blocked, Parked, Pinned, Recently Visited) reusing the shared matchesSmartFocus
// predicate.
func (m *OutlinerModel) applySmartFocusMode(focus OutlinerFocus) {
	m.clearComputedModes()
	m.crossPriorityMode = false
	m.activeSavedFilter = ""
	m.activeCriteriaFilter = nil
	m.lens = OutlinerLensTree
	f := focus
	m.smartFocus = &f
	m.crossForceRebuild = true
	m.cursor = 0
}

// applyPriorityLensMode selects the Priority lens. In the cross-project outliner
// this is the grouped session-only Priority mode; in a single project it is the
// High Priority focus sorted by score, mirroring the historical `p` shortcut.
func (m *OutlinerModel) applyPriorityLensMode() {
	m.clearComputedModes()
	if m.project == nil {
		if !m.crossPriorityMode {
			m.enterCrossPriorityMode()
		}
		return
	}
	m.crossPriorityMode = false
	m.activeSavedFilter = ""
	m.activeCriteriaFilter = nil
	m.lens = OutlinerLensTree
	focus := OutlinerHighPriority
	m.smartFocus = &focus
	m.sort = SortState{Field: SortByScore, Ascending: true}
	m.treeSort = m.sort
	m.crossForceRebuild = true
	m.cursor = 0
}

// cycleWorkMode advances the active mode by delta within the five-item Work
// cycle. Entering from a mode outside the cycle starts at Today when moving
// forward and All when moving backward, matching the pinned tab-bar order.
func (m *OutlinerModel) cycleWorkMode(delta int) (tea.Model, tea.Cmd) {
	current := m.activeOutlinerMode()
	idx := -1
	for i, id := range workModeCycleOrder {
		if id == current {
			idx = i
			break
		}
	}

	var target OutlinerModeID
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
	return m.selectOutlinerMode(target)
}

// isComputedOrderMode reports whether the rows are currently a flat, computed
// list whose order the engine owns, so a structural J/K move cannot apply. It
// keys off the live render state rather than the catalog OrderPolicy. The global
// Today and Next horizons are computed-order too, but their J/K reorders the
// durable planning list (see isPlanningReorderMode) rather than being unavailable,
// so callers that gate J/K must consult isPlanningReorderMode first.
func (m *OutlinerModel) isComputedOrderMode() bool {
	return m.workTodayMode || m.workNextMode || m.workBacklogMode || m.reviewModeActive() ||
		(m.project == nil && m.crossPriorityMode) ||
		m.lens == OutlinerLensDueDate
}

// orderingUnavailableStatus explains why J/K cannot reorder rows in the active
// computed mode.
func (m *OutlinerModel) orderingUnavailableStatus() {
	mode := m.activeOutlinerModeEntry()
	m.setStatus(fmt.Sprintf(
		"Ordering is automatic in %s mode — J/K reorder tasks only in tree modes (Active, All, workflow, smart)",
		mode.Label,
	))
}

// workModeScopeProjects returns the projects the computed work horizons and
// tab-bar counts range over: the single project when scoped, otherwise every
// loaded project narrowed by the active multi-project (F P) filter.
//
// The F P filter is honored only when unscoped: inside a single-project scope
// the filter is disabled and scopeProjects already returns the lone scoped
// project, so the tree, Priority, and computed paths agree on the same visible
// project set.
func (m *OutlinerModel) workModeScopeProjects() []*models.Project {
	if m.project != nil {
		return []*models.Project{m.project}
	}
	projects := m.scopeProjects()
	if m.scopedProjectID != "" || len(m.filterProjects) == 0 {
		return projects
	}
	filtered := make([]*models.Project, 0, len(projects))
	for _, p := range projects {
		if p != nil && m.filterProjects[p.ID] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// workModeFilterPredicate returns the per-task predicate applying the active
// common filters, search, pinned tag, saved criteria, section, and actionable
// constraints. It mirrors crossPriorityRowPredicate minus the priority
// eligibility gate so computed work horizons honor the same visible filters as
// the tree modes.
func (m *OutlinerModel) workModeFilterPredicate(proj *models.Project) func(*models.Task) bool {
	pinnedTag := m.activePinnedTagFilter()
	query := m.SearchQuery()
	hasCommon := hasActiveCommonFilters(&m.filter.CommonFilterState)
	criteria := m.activeCriteriaFilter
	sections := m.filter.Sections
	actionable := m.filter.Actionable

	return func(t *models.Task) bool {
		if t == nil {
			return false
		}
		if pinnedTag != "" && !t.HasTag(pinnedTag) {
			return false
		}
		if query != "" && !matchesSearch(t, query, m.runtime.Planning()) {
			return false
		}
		if hasCommon && !passesCommonFilters(t, &m.filter.CommonFilterState, proj, m.runtime.Automation()) {
			return false
		}
		if criteria != nil && !passesSavedFilterCriteria(t, proj, criteria, m.runtime.Automation()) {
			return false
		}
		if len(sections) > 0 && !sections[taskSectionBucket(t)] {
			return false
		}
		if actionable && !taskIsActionableState(t, proj) {
			return false
		}
		return true
	}
}
