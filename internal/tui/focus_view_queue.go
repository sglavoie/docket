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

// persistQueue saves the current queue state to config.
func (m *FocusViewModel) persistQueue() {
	if m.configManager == nil {
		return
	}
	items := make([]config.FocusQueueItem, len(*m.queue))
	for i, ft := range *m.queue {
		items[i] = config.FocusQueueItem{
			TaskID:    ft.Task.ID,
			ProjectID: ft.Project.ID,
		}
	}
	m.config.SetFocusQueue(items)
	if err := m.configManager.Save(m.config); err != nil {
		m.err = err
	}
}

// persistDismissed saves the current dismissed IDs to config.
func (m *FocusViewModel) persistDismissed() {
	if m.configManager == nil {
		return
	}
	ids := make([]string, 0, len(m.dismissedIDs))
	for id := range m.dismissedIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	m.config.SetFocusDismissedIDs(ids)
	if err := m.configManager.Save(m.config); err != nil {
		m.err = err
	}
}
