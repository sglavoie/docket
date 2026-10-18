package outliner

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

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
// clamped into the (possibly differently shaped) new row list. A cursor that
// held a task keeps holding one: the index is walked to the nearest task row,
// forward first, so a task key pressed next is not refused on a header the
// reshape happened to put there.
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
	if anchor.hasTask {
		m.seatNearestTaskRow()
	}
}

// seatNearestTaskRow moves the cursor to the nearest task row when it rests on
// anything else, looking forward before backward, and leaves it alone when the
// projection draws no task at all.
func (m *OutlinerModel) seatNearestTaskRow() {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) || m.Rows[m.Cursor].IsTaskRow() {
		return
	}
	for i := m.Cursor + 1; i < len(m.Rows); i++ {
		if m.Rows[i].IsTaskRow() && m.Rows[i].Task != nil {
			m.Cursor = i
			return
		}
	}
	for i := m.Cursor - 1; i >= 0; i-- {
		if m.Rows[i].IsTaskRow() && m.Rows[i].Task != nil {
			m.Cursor = i
			return
		}
	}
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
