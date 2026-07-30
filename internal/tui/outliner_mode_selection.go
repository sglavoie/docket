package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
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
	mode, ok := OutlinerModeByID(id)
	if !ok {
		return m, nil
	}
	anchor := m.captureModeCursor()
	defer m.restoreModeCursor(anchor)

	m.applyOutlinerModeTransition(mode)
	m.primaryMode = mode.ID
	m.crossForceRebuild = true
	m.rebuildRows()
	return m, nil
}

// cycleWorkMode advances the active mode by delta within the exact five-item
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
	case ModeAll, ModeInbox, ModeNext, ModeBacklog:
		return true
	default:
		return false
	}
}

// orderingUnavailableStatus explains why J/K cannot reorder rows in the active
// mode.
func (m *OutlinerModel) orderingUnavailableStatus() {
	if m != nil && m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].IsPinnedSection {
		m.setStatus("Pinned order is newest first — J/K is unavailable")
		return
	}
	mode := m.activeOutlinerModeEntry()
	m.setStatus(fmt.Sprintf(
		"Ordering is automatic in %s mode — J/K reorder hierarchical or planning rows only",
		mode.Label,
	))
}

// workModeScopeProjects returns the projects the five modes and Today section
// range over: the single project when scoped, otherwise the loaded projects
// admitted by the active Context.
func (m *OutlinerModel) workModeScopeProjects() []*models.Project {
	if m.project != nil {
		return []*models.Project{m.project}
	}
	return m.scopeProjects()
}

func (m *OutlinerModel) workModeFilterPredicateWithResolverAt(
	proj *models.Project,
	resolver *app.WorkspaceResolver,
	now time.Time,
) func(*models.Task) bool {
	pinnedTag := m.activePinnedTagFilter()
	query := m.SearchQuery()
	hasCommon := hasActiveCommonFilters(&m.filter.CommonFilterState)
	actionable := m.filter.Actionable

	return func(t *models.Task) bool {
		if t == nil {
			return false
		}
		if pinnedTag != "" && !t.HasTag(pinnedTag) {
			return false
		}
		if query != "" && !TaskMatchesSearch(t, query) {
			return false
		}
		if hasCommon && !passesCommonFiltersWithResolverAt(
			t,
			&m.filter.CommonFilterState,
			proj,
			resolver,
			now,
		) {
			return false
		}
		if actionable && !taskIsActionableState(t, proj) {
			return false
		}
		return true
	}
}
