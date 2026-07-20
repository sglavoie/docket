package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// handleFocusBoardReorder dispatches an adjacent reorder to the active planning
// horizon: the Today plan on the Today tab, the Next shortlist on the Next tab.
// It is a no-op on the Backlog/Done/All tabs, whose order is derived (ranked or
// closed-today), not user-editable.
func (m *FocusBoardModel) handleFocusBoardReorder(delta int) (tea.Model, tea.Cmd) {
	switch m.tabs.Active {
	case boardTabToday:
		return m.handleDailyPlanReorder(delta)
	case boardTabNext:
		return m.handleNextReorder(delta)
	default:
		return m, nil
	}
}

// handleNextReorder moves the selected StateNext task one slot within the Next
// shortlist ordering. It swaps the cursor row with its view-adjacent neighbor,
// then persists the full canonical Next order — every live StateNext task, in
// order, with no stale non-Next entry — so previously-unindexed members gain
// deterministic order entries without dropping any other member. Membership
// (StateNext) is untouched; only the ordering changes.
func (m *FocusBoardModel) handleNextReorder(delta int) (tea.Model, tea.Cmd) {
	if m.tabs.Active != boardTabNext {
		return m, nil
	}
	if m.reorderBlockedBySecondarySort("Next order") {
		return m, nil
	}
	to := m.cursor + delta
	if m.cursor < 0 || m.cursor >= len(m.filtered) || to < 0 || to >= len(m.filtered) {
		return m, nil
	}
	curRow := m.filtered[m.cursor]
	targetRow := m.filtered[to]
	if curRow.Task == nil || targetRow.Task == nil {
		return m, nil
	}

	// Reorder inside the full canonical Next order, not just the (possibly
	// filtered) visible slice, so the persisted index always lists every live
	// StateNext task. Swapping the two view-adjacent tasks by ID moves them past
	// each other even when a filter hides intervening members.
	ordered := m.orderedNextRows()
	curIdx := nextRowIndexByID(ordered, curRow.Task.ID)
	targetIdx := nextRowIndexByID(ordered, targetRow.Task.ID)
	if curIdx < 0 || targetIdx < 0 {
		return m, nil
	}
	ordered[curIdx], ordered[targetIdx] = ordered[targetIdx], ordered[curIdx]

	queue, err := m.persistNextOrder(ordered)
	if err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return m, nil
	}
	if m.focusQueue != nil {
		*m.focusQueue = queue
	}

	// Publish the visible move only after the candidate queue is durable.
	m.filtered[m.cursor], m.filtered[to] = m.filtered[to], m.filtered[m.cursor]
	m.cursor = to
	return m, nil
}

// orderedNextRows returns every StateNext row (m.nextRows) in the canonical Next
// order: indexed members first by their stored rank, then unindexed StateNext
// tasks in their stable insertion order.
func (m *FocusBoardModel) orderedNextRows() []boardRow {
	ordered := make([]boardRow, len(m.nextRows))
	copy(ordered, m.nextRows)
	m.sortRowsByQueueOrder(ordered)
	return ordered
}

// nextRowIndexByID returns the index of the row whose task has the given ID, or
// -1 when no such row exists.
func nextRowIndexByID(rows []boardRow, id string) int {
	for i, r := range rows {
		if r.Task != nil && r.Task.ID == id {
			return i
		}
	}
	return -1
}

// persistNextOrder builds and commits a complete candidate Next order without
// touching the shared queue. Every StateNext row gains an explicit entry and no
// non-Next entry survives, mirroring the dense-snapshot contract of
// `tt focus queue move`.
func (m *FocusBoardModel) persistNextOrder(ordered []boardRow) ([]FocusTask, error) {
	queue := make([]FocusTask, 0, len(ordered))
	items := make([]config.FocusQueueItem, 0, len(ordered))
	for _, r := range ordered {
		if r.Task == nil || r.Project == nil {
			continue
		}
		queue = append(queue, FocusTask{Task: r.Task, Project: r.Project})
		items = append(items, config.FocusQueueItem{TaskID: r.Task.ID, ProjectID: r.Project.ID})
	}
	err := m.commitPlanning(func(p *config.PlanningState) {
		p.SetFocusQueue(items)
	})
	return queue, err
}
