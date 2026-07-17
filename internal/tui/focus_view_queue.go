package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/config"
)

// reorderQueue swaps adjacent items in the queue (from -> to).
func (m *FocusViewModel) reorderQueue(from, to int) {
	q := *m.queue
	if from < 0 || from >= len(q) || to < 0 || to >= len(q) {
		return
	}
	q[from], q[to] = q[to], q[from]
	m.persistQueue()
}

// isInQueue checks if a task ID is in the work queue.
func (m *FocusViewModel) isInQueue(taskID string) bool {
	return m.queueIDs[taskID]
}

// persistQueue saves the current queue state through a checked planning commit.
// The Focus Queue is planning-domain state, committed to planning.json alone
// rather than through a whole-config save.
func (m *FocusViewModel) persistQueue() {
	items := make([]config.FocusQueueItem, len(*m.queue))
	for i, ft := range *m.queue {
		items[i] = config.FocusQueueItem{
			TaskID:    ft.Task.ID,
			ProjectID: ft.Project.ID,
		}
	}
	if err := commitPlanningToRuntime(m.runtime, func(p *config.PlanningState) {
		p.SetFocusQueue(items)
	}); err != nil {
		m.err = err
	}
}

// persistDismissed saves the current dismissed IDs through a checked planning
// commit. Dismissed focus suggestions are planning-domain state, committed to
// planning.json alone rather than through a whole-config save.
func (m *FocusViewModel) persistDismissed() {
	ids := make([]string, 0, len(m.dismissedIDs))
	for id := range m.dismissedIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if err := commitPlanningToRuntime(m.runtime, func(p *config.PlanningState) {
		p.SetFocusDismissedIDs(ids)
	}); err != nil {
		m.err = err
	}
}
