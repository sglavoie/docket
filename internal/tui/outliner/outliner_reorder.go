package outliner

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func (m *OutlinerModel) ReorderTask(delta int) tea.Cmd {
	if len(m.Rows) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return nil
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return nil
	}

	task := row.Task
	siblings := m.visibleSiblings(m.Cursor)
	index := siblingIndex(siblings, task.ID)
	target := index + delta
	if index < 0 || target < 0 || target >= len(siblings) {
		return nil
	}
	other := siblings[target]
	project := m.ResolveProjectForRow(row)
	if project == nil {
		return nil
	}
	taskIdentity := taskref.For(task, project)
	targetIdentity := taskref.For(other, project)
	if m.taskItem.Wired() {
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
	_ = project.SwapSiblingTasksAt(task, other, time.Now())
	RebuildOutlinerPreservingCursor(m)
	return nil
}

// reorderTaskEdge sends the cursor task to the far end of its visible siblings
// in one transaction. Repeating an adjacent swap through visible peers is a
// rotation of the durable slots those peers occupy, so only the sub-range from
// the cursor to the edge is named and every sibling outside it — hidden by a
// filter, a collapsed parent, or another section — keeps its own slot.
func (m *OutlinerModel) ReorderTaskEdge(delta int) tea.Cmd {
	if len(m.Rows) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return nil
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return nil
	}
	siblings := m.visibleSiblings(m.Cursor)
	index := siblingIndex(siblings, row.Task.ID)
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
	rotated := edgeMoveOrder(row.Task, peers, delta)
	project := m.ResolveProjectForRow(row)
	if project == nil {
		return nil
	}
	if m.taskItem.Wired() {
		order := make([]taskref.TaskIdentity, 0, len(rotated))
		for _, task := range rotated {
			order = append(order, taskref.For(task, project))
		}
		return m.taskItem.ReorderSiblingOrder(taskref.For(row.Task, project), order, reorderEdgeLabel(delta))
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
	if err := project.ReorderSiblingTasksAt(resolved, time.Now()); err != nil {
		return nil
	}
	RebuildOutlinerPreservingCursor(m)
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
