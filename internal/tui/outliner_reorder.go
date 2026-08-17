package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *OutlinerModel) reorderTask(delta int) tea.Cmd {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil {
		return nil
	}

	task := row.task
	siblings := m.visibleSiblings(m.cursor)
	index := siblingIndex(siblings, task.ID)
	target := index + delta
	if index < 0 || target < 0 || target >= len(siblings) {
		return nil
	}
	other := siblings[target]
	project := m.resolveProjectForRow(row)
	if project == nil {
		return nil
	}
	taskIdentity := taskIdentityFor(task, project)
	targetIdentity := taskIdentityFor(other, project)
	if m.taskItem.task != nil {
		return m.taskItem.ReorderSiblings(taskIdentity, targetIdentity)
	}

	// Isolated Outliner models have no transaction coordinator. Preserve their
	// in-memory structural behavior for focused rendering/navigation tests while
	// production always takes the checked application transaction above.
	task = project.GetTaskByID(task.ID)
	other = project.GetTaskByID(other.ID)
	if task == nil || other == nil {
		return nil
	}
	project.SwapTasks(task, other)
	rebuildOutlinerPreservingCursor(m)
	return nil
}

// reorderTaskEdge sends the cursor task to the far end of its visible siblings
// in one transaction. Repeating an adjacent swap through visible peers is a
// rotation of the durable slots those peers occupy, so only the sub-range from
// the cursor to the edge is named and every sibling outside it — hidden by a
// filter, a collapsed parent, or another section — keeps its own slot.
func (m *OutlinerModel) reorderTaskEdge(delta int) tea.Cmd {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil {
		return nil
	}
	siblings := m.visibleSiblings(m.cursor)
	index := siblingIndex(siblings, row.task.ID)
	if index < 0 {
		return nil
	}
	peers := siblings[index+1:]
	if delta < 0 {
		peers = reversed(siblings[:index])
	}
	if len(peers) == 0 {
		return nil
	}
	rotated := edgeMoveOrder(row.task, peers, delta)
	project := m.resolveProjectForRow(row)
	if project == nil {
		return nil
	}
	if m.taskItem.task != nil {
		order := make([]TaskIdentity, 0, len(rotated))
		for _, task := range rotated {
			order = append(order, taskIdentityFor(task, project))
		}
		return m.taskItem.ReorderSiblingOrder(taskIdentityFor(row.task, project), order, reorderEdgeLabel(delta))
	}

	// Isolated Outliner models have no transaction coordinator. Preserve their
	// in-memory structural behavior for focused rendering/navigation tests while
	// production always takes the checked application transaction above.
	resolved := make([]*models.Task, 0, len(rotated))
	for _, task := range rotated {
		canonical := project.GetTaskByID(task.ID)
		if canonical == nil {
			return nil
		}
		resolved = append(resolved, canonical)
	}
	if err := project.ReorderSiblingTasks(resolved); err != nil {
		return nil
	}
	rebuildOutlinerPreservingCursor(m)
	return nil
}

// edgeMoveOrder returns the final display order of the run an edge move rewrites:
// the moving element plus the peers between it and the edge, given nearest
// first. The mover ends up past all of them and each peer shifts one slot toward
// the cursor's old position, which is exactly what repeating an adjacent swap
// produces. Every ordering domain shares the permutation, only the slots it is
// written into differ.
func edgeMoveOrder[T any](mover T, peers []T, delta int) []T {
	moved := make([]T, 0, len(peers)+1)
	if delta < 0 {
		moved = append(moved, mover)
		return append(moved, reversed(peers)...)
	}
	moved = append(moved, peers...)
	return append(moved, mover)
}

func reversed[T any](items []T) []T {
	out := make([]T, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		out = append(out, items[i])
	}
	return out
}

// reorderEdgeLabel names the edge an edge move heads for, for status text.
func reorderEdgeLabel(delta int) string {
	if delta < 0 {
		return "top"
	}
	return "bottom"
}

func siblingIndex(siblings []*models.Task, taskID string) int {
	for index, sibling := range siblings {
		if sibling != nil && sibling.ID == taskID {
			return index
		}
	}
	return -1
}
