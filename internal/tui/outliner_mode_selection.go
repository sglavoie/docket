package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// workModeCycleOrder is derived from the catalog so presentation identity and
// keyboard cycling cannot drift.
var workModeCycleOrder = func() []OutlinerModeID {
	modes := OutlinerTabModes()
	ids := make([]OutlinerModeID, len(modes))
	for i, mode := range modes {
		ids[i] = mode.ID
	}
	return ids
}()

const recentStaleFilterClearedStatus = "Stale filter cleared: Recent only includes the last 30 days"

// clearStaleForRecentMode enforces the facet contract at every path that can
// activate Recent, including restoration of a captured project-scope frame.
func (m *OutlinerModel) clearStaleForRecentMode() {
	if m.primaryMode != ModeRecent || !m.filter.Stale {
		return
	}
	m.filter.Stale = false
	m.setTransientStatus(recentStaleFilterClearedStatus)
}

// activeOutlinerMode returns the sole mode identity owned by the Outliner.
func (m *OutlinerModel) activeOutlinerMode() OutlinerModeID {
	if _, ok := OutlinerModeByID(m.primaryMode); ok {
		return m.primaryMode
	}
	return ModeAll
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

// selectOutlinerMode applies the named catalog mode. It is the sole entry
// point used by Tab cycling, startup, and any command that jumps directly to a
// mode. It captures the selected task identity
// before rebuilding and restores it afterward (see restoreModeCursor) so an
// eligible task keeps focus and scroll position across the reshape.
func (m *OutlinerModel) selectOutlinerMode(id OutlinerModeID) (tea.Model, tea.Cmd) {
	if m.hasSearchProjection() {
		m.setTransientStatus("Exit search before changing modes")
		return m, nil
	}
	mode, ok := OutlinerModeByID(id)
	if !ok {
		return m, nil
	}
	// A status naming the mode it was raised in would otherwise outlive the
	// transition and misreport the new mode, up to a tick before it expires.
	m.clearStatus()
	m.updateVisualSelectionStatus()
	anchor := m.captureModeCursor()
	defer m.restoreModeCursor(anchor)

	// Entering Recent re-reads activity, so gr over an already-open Recent is the
	// manual refresh.
	if mode.ID == ModeRecent {
		m.recencyKeys = nil
		m.recencyProjection = ""
	}
	m.applyOutlinerModeTransition(mode)
	m.primaryMode = mode.ID
	m.clearStaleForRecentMode()
	m.crossForceRebuild = true
	m.rebuildRows()
	return m, nil
}

// focusLatestActivity enters Recent and lands the cursor on the newest task it
// admits. Entry re-reads activity, so the landing row is the task you touched
// last rather than the one that led the snapshot you have been triaging.
func (m *OutlinerModel) focusLatestActivity() tea.Cmd {
	staleWasActive := m.filter.Stale
	m.selectOutlinerMode(ModeRecent)
	staleWasCleared := staleWasActive && !m.filter.Stale
	task, project, sectionKey, ok := m.recentActivityLead()
	if !ok {
		if !staleWasCleared {
			m.setTransientStatus("No activity in the last 30 days")
		}
		return nil
	}
	identity := taskIdentityFor(task, project)
	index, found := m.rowIndexForTask(identity)
	if !found {
		m.revealRecentActivityLead(task, identity, sectionKey)
		m.rebuildRows()
		if index, found = m.rowIndexForTask(identity); !found {
			if !staleWasCleared {
				m.setTransientStatus("Latest activity is unavailable in this projection")
			}
			return nil
		}
	}
	oldCursor := m.cursor
	// The mode transition carries the origin's scroll offset over, which would
	// seat the lead against the viewport edge with the section and project
	// headers naming it scrolled off. Recent puts the lead near the top, so
	// reading the window from row zero keeps those headers on screen.
	m.viewScrollOffset = 0
	m.cursor = index
	if !staleWasCleared {
		m.setTransientStatus("Latest activity: " + task.Title)
	}
	return flashIfJumped(oldCursor, index, &m.cursorFlash)
}

// revealRecentActivityLead opens the three things that can hide the lead: its
// bucket, its project group inside that bucket, and its ancestor chain. It runs
// only after a first attempt failed, so an unrelated fold is never disturbed.
func (m *OutlinerModel) revealRecentActivityLead(
	task *models.Task,
	identity TaskIdentity,
	sectionKey string,
) {
	delete(m.collapsedSections, sectionKey)
	delete(m.collapsedSections, modeSectionProjectCollapseKey(sectionKey, identity.ProjectID))
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.expandedNodes[TaskIdentity{ProjectID: identity.ProjectID, TaskID: ancestor.ID}] = true
	}
}

// cycleWorkMode advances the active mode by delta within the exact seven-item
// cycle.
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

func isPrimaryOutlinerMode(mode OutlinerModeID) bool {
	switch mode {
	case ModeAll, ModeInbox, ModeToday, ModeNext, ModeBacklog:
		return true
	default:
		return false
	}
}

// orderingUnavailableStatus explains why J/K and Ctrl+J/Ctrl+K cannot reorder
// rows in the active mode.
func (m *OutlinerModel) orderingUnavailableStatus() {
	mode := m.activeOutlinerModeEntry()
	m.setTransientStatus(fmt.Sprintf(
		"Ordering is automatic in %s mode — J/K and Ctrl+J/Ctrl+K reorder hierarchical or planning rows only",
		mode.Label,
	))
}

// workModeScopeProjects returns the projects the seven modes range over: the
// range over: the single project when scoped, otherwise the loaded projects
// admitted by the active Context.
func (m *OutlinerModel) workModeScopeProjects() []*models.Project {
	if m.project != nil {
		return []*models.Project{m.project}
	}
	return m.scopeProjects()
}

func (m *OutlinerModel) workModeFilterPredicateAt(now time.Time) func(*models.Task) bool {
	query := m.searchTextQuery()
	caseSensitive := m.searchCaseSensitive()
	hasCommon := hasActiveCommonFilters(&m.filter.CommonFilterState)

	return func(t *models.Task) bool {
		if t == nil {
			return false
		}
		if query != "" && !TaskMatchesSearch(t, query, caseSensitive) {
			return false
		}
		if hasCommon && !passesCommonFiltersAt(t, &m.filter.CommonFilterState, now) {
			return false
		}
		return true
	}
}
