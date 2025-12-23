package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleNavigationKeys handles j/k, g/G, arrow keys, and page navigation.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleNavigationKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return true, nil
	case "down", "j":
		if m.cursor < len(visible)-1 {
			m.cursor++
		}
		return true, nil
	case "g":
		m.cursor = 0
		return true, nil
	case "G":
		if len(visible) > 0 {
			m.cursor = len(visible) - 1
		}
		return true, nil
	}
	return false, nil
}

// statusPredicate determines if a task is eligible for a status change.
type statusPredicate func(*models.Task) bool

// statusAction applies a status change to a task.
type statusAction func(*models.Task)

// statusValidator checks if a task can be changed; returns error message if blocked, empty string if allowed.
type statusValidator func(*models.Task) string

// handleStatusChange applies a status change to selected tasks or the current task.
// It handles multi-select mode by applying the change to all selected tasks that pass
// the predicate, then clears selection. For single-task mode, it applies the change
// to the task at the cursor if it passes the predicate.
func (m *ProjectTreeModel) handleStatusChange(
	visible []*TreeNode,
	predicate statusPredicate,
	action statusAction,
) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		for _, task := range m.getSelectedTasks() {
			if predicate(task) {
				action(task)
			}
		}
		m.clearSelection()
		return true, m.saveAndRefresh()
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		if predicate(task) {
			action(task)
			return true, m.saveAndRefresh()
		}
	}
	return true, nil
}

// handleStatusChangeWithValidation applies a status change with per-task validation.
// Unlike handleStatusChange, this variant can skip individual tasks based on validation
// and reports skipped tasks via an error message. Used for operations like complete
// where dependency checking may block individual tasks.
func (m *ProjectTreeModel) handleStatusChangeWithValidation(
	visible []*TreeNode,
	predicate statusPredicate,
	validator statusValidator,
	action statusAction,
	actionVerb string,
) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		var skippedIDs []string
		actionCount := 0
		for _, task := range m.getSelectedTasks() {
			if !predicate(task) {
				continue
			}
			if errMsg := validator(task); errMsg != "" {
				skippedIDs = append(skippedIDs, fmt.Sprintf("#%d", task.ShortID))
				continue
			}
			action(task)
			actionCount++
		}
		m.clearSelection()
		if len(skippedIDs) > 0 {
			var msg string
			if actionCount == 0 {
				msg = fmt.Sprintf("no tasks %s: %s have incomplete dependencies", actionVerb, strings.Join(skippedIDs, ", "))
			} else {
				msg = fmt.Sprintf("%s %d, skipped %s (incomplete dependencies)", actionVerb, actionCount, strings.Join(skippedIDs, ", "))
			}
			return true, tea.Batch(m.saveAndRefresh(), func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("%s", msg)}
			})
		}
		return true, m.saveAndRefresh()
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		if predicate(task) {
			if errMsg := validator(task); errMsg != "" {
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("%s", errMsg)}
				}
			}
			action(task)
			return true, m.saveAndRefresh()
		}
	}
	return true, nil
}

// handleStatusKeys handles s, m, b, x, o - status change operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleStatusKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "s":
		return m.handleStartTask(visible)
	case "m":
		return m.handleCompleteTask(visible)
	case "b":
		return m.handleBlockTask(visible)
	case "x":
		return m.handleCancelTask(visible)
	case "o":
		return m.handleReopenTask(visible)
	case "`":
		return m.handleCycleStatusForward(visible)
	case "~":
		return m.handleCycleStatusBackward(visible)
	}
	return false, nil
}

// handleStartTask starts a task (in_progress).
func (m *ProjectTreeModel) handleStartTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool {
			return t.Status == models.StatusTodo || t.Status == models.StatusBlocked
		},
		func(t *models.Task) { t.Start() },
	)
}

// handleCompleteTask marks a task as complete (done).
func (m *ProjectTreeModel) handleCompleteTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChangeWithValidation(visible,
		func(t *models.Task) bool { return t.Status.IsActive() },
		func(t *models.Task) string {
			incompleteDeps := m.project.GetIncompleteDependencies(t)
			if len(incompleteDeps) == 0 {
				return ""
			}
			depNames := make([]string, len(incompleteDeps))
			for i, dep := range incompleteDeps {
				depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
			}
			return fmt.Sprintf("cannot complete: %d incomplete dependencies (%s)",
				len(incompleteDeps), strings.Join(depNames, ", "))
		},
		func(t *models.Task) { t.Complete() },
		"completed",
	)
}

// handleBlockTask blocks a task.
func (m *ProjectTreeModel) handleBlockTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool {
			return t.Status.IsActive() && t.Status != models.StatusBlocked
		},
		func(t *models.Task) { t.Block() },
	)
}

// handleCancelTask cancels a task.
func (m *ProjectTreeModel) handleCancelTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool { return t.Status.IsActive() },
		func(t *models.Task) { t.Cancel() },
	)
}

// handleReopenTask reopens a task (back to todo).
func (m *ProjectTreeModel) handleReopenTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool { return t.Status != models.StatusTodo },
		func(t *models.Task) { t.Reopen() },
	)
}

// handleCycleStatusForward cycles status: todo -> in_progress -> done -> todo.
func (m *ProjectTreeModel) handleCycleStatusForward(visible []*TreeNode) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		var skippedIDs []string
		cycledCount := 0
		for _, task := range m.getSelectedTasks() {
			switch task.Status {
			case models.StatusTodo:
				task.Start()
				cycledCount++
			case models.StatusInProgress:
				if incompleteDeps := m.project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
					skippedIDs = append(skippedIDs, fmt.Sprintf("#%d", task.ShortID))
					continue
				}
				task.Complete()
				cycledCount++
			case models.StatusDone:
				task.Reopen()
				cycledCount++
			}
		}
		m.clearSelection()
		if len(skippedIDs) > 0 {
			var msg string
			if cycledCount == 0 {
				msg = fmt.Sprintf("no tasks cycled: %s have incomplete dependencies", strings.Join(skippedIDs, ", "))
			} else {
				msg = fmt.Sprintf("cycled %d, skipped %s (incomplete dependencies)", cycledCount, strings.Join(skippedIDs, ", "))
			}
			return true, tea.Batch(m.saveAndRefresh(), func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("%s", msg)}
			})
		}
		return true, m.saveAndRefresh()
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		switch task.Status {
		case models.StatusTodo:
			task.Start()
			return true, m.saveAndRefresh()
		case models.StatusInProgress:
			if incompleteDeps := m.project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
				depNames := make([]string, len(incompleteDeps))
				for i, dep := range incompleteDeps {
					depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
				}
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
						len(incompleteDeps), strings.Join(depNames, ", "))}
				}
			}
			task.Complete()
			return true, m.saveAndRefresh()
		case models.StatusDone:
			task.Reopen()
			return true, m.saveAndRefresh()
		}
	}
	return true, nil
}

// handleCycleStatusBackward cycles status backward: todo -> done -> in_progress -> todo.
func (m *ProjectTreeModel) handleCycleStatusBackward(visible []*TreeNode) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		for _, task := range m.getSelectedTasks() {
			switch task.Status {
			case models.StatusTodo:
				task.Complete()
			case models.StatusInProgress:
				task.Reopen()
			case models.StatusDone:
				task.Start()
			}
		}
		m.clearSelection()
		return true, m.saveAndRefresh()
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		switch task.Status {
		case models.StatusTodo:
			task.Complete()
			return true, m.saveAndRefresh()
		case models.StatusInProgress:
			task.Reopen()
			return true, m.saveAndRefresh()
		case models.StatusDone:
			task.Start()
			return true, m.saveAndRefresh()
		}
	}
	return true, nil
}

// handleTypeKeys handles h, l, H, L - type change operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTypeKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "h":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanPromote(); ok {
				task.Promote()
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "l":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanDemote(); ok {
				task.Demote()
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "H":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanCycleLeafType(true); ok {
				task.CycleLeafType(true)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "L":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanCycleLeafType(false); ok {
				task.CycleLeafType(false)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	}
	return false, nil
}

// handleMovementKeys handles K, J, left, right - task movement operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleMovementKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "K":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			siblings := m.project.GetSiblings(task)
			siblingIdx := m.project.FindSiblingIndex(task)
			if siblingIdx > 0 {
				siblingAbove := siblings[siblingIdx-1]
				if task.IsRoot() && task.IsComplete() && !siblingAbove.IsComplete() {
					return true, func() tea.Msg {
						return MoveBlockedMsg{Reason: "Cannot move completed task above uncompleted task"}
					}
				}
				m.project.MoveTaskUp(task)
				if m.cursor > 0 {
					m.cursor--
				}
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "J":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			siblings := m.project.GetSiblings(task)
			siblingIdx := m.project.FindSiblingIndex(task)
			if siblingIdx >= 0 && siblingIdx < len(siblings)-1 {
				siblingBelow := siblings[siblingIdx+1]
				if task.IsRoot() && !task.IsComplete() && siblingBelow.IsComplete() {
					return true, func() tea.Msg {
						return MoveBlockedMsg{Reason: "Cannot move uncompleted task below completed task"}
					}
				}
				m.project.MoveTaskDown(task)
				if m.cursor < len(visible)-1 {
					m.cursor++
				}
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "right":
		if CursorInBounds(m.cursor, len(visible)) && m.cursor > 0 {
			task := visible[m.cursor].Task
			// Walk up from the visually-above task to find a valid parent
			candidate := visible[m.cursor-1].Task
			for candidate != nil {
				if ok, _ := task.CanIndent(candidate); ok {
					task.ParentID = &candidate.ID
					m.project.BuildTaskTree()
					return true, m.saveAndRefresh()
				}
				candidate = candidate.Parent
			}
		}
		return true, nil
	case "left":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanOutdent(); ok {
				m.project.OutdentTask(task)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	}
	return false, nil
}

// handleSelectionKeys handles v, a, space - selection operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSelectionKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case " ":
		if m.multiSelectMode {
			if CursorInBounds(m.cursor, len(visible)) {
				m.toggleSelection(visible[m.cursor].Task)
			}
			return true, nil
		}
		if CursorInBounds(m.cursor, len(visible)) {
			node := visible[m.cursor]
			if len(node.Task.Children) > 0 {
				node.Expanded = !node.Expanded
				m.updateVisibility()
			}
		}
		return true, nil
	case "tab":
		if CursorInBounds(m.cursor, len(visible)) {
			node := visible[m.cursor]
			if len(node.Task.Children) > 0 {
				node.Expanded = !node.Expanded
				m.updateVisibility()
			}
		}
		return true, nil
	case "v":
		if m.multiSelectMode {
			m.clearSelection()
		} else {
			m.multiSelectMode = true
			if CursorInBounds(m.cursor, len(visible)) {
				m.toggleSelection(visible[m.cursor].Task)
			}
		}
		return true, nil
	case "a":
		if m.multiSelectMode {
			m.selectAllVisible()
			return true, nil
		}
		return false, nil
	case "A":
		if m.multiSelectMode {
			for _, node := range m.visibleNodes() {
				delete(m.selectedTasks, node.Task.ID)
			}
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

// handleFilterKeys handles d, P, t, S, T, N, u - filter operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "d":
		m.showCompleted = !m.showCompleted
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "P":
		if m.filterPriority == 0 {
			m.filterPriority = 1
		} else {
			m.filterPriority = 0
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "t":
		types := models.AllTaskTypes()
		if m.filterType == "" {
			m.filterType = types[0]
		} else {
			found := false
			for i, t := range types {
				if t == m.filterType {
					if i+1 < len(types) {
						m.filterType = types[i+1]
					} else {
						m.filterType = ""
					}
					found = true
					break
				}
			}
			if !found {
				m.filterType = ""
			}
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "S":
		statuses := models.AllTaskStatuses()
		if m.filterStatus == "" {
			m.filterStatus = statuses[0]
		} else {
			found := false
			for i, s := range statuses {
				if s == m.filterStatus {
					if i+1 < len(statuses) {
						m.filterStatus = statuses[i+1]
					} else {
						m.filterStatus = ""
					}
					found = true
					break
				}
			}
			if !found {
				m.filterStatus = ""
			}
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "T":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.tagInputMode = true
			m.tagInput.Focus()
			return true, textinput.Blink
		}
		return true, func() tea.Msg {
			return ShowTagFilterSelectorMsg{ProjectTree: m}
		}

	case "N":
		return true, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{ProjectTree: m}
		}

	case "u":
		m.filterDueDate = (m.filterDueDate + 1) % 5
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil
	}
	return false, nil
}

// handleSortKeys handles 1, 2, 3, 4, 0 - sort operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSortKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "1":
		if m.sortField == SortByPriority {
			m.sortAscending = !m.sortAscending
		} else {
			m.sortField = SortByPriority
			m.sortAscending = true
		}
		m.sortNodes()
		m.updateVisibility()
		return true, nil

	case "2":
		if m.sortField == SortByDueDate {
			m.sortAscending = !m.sortAscending
		} else {
			m.sortField = SortByDueDate
			m.sortAscending = true
		}
		m.sortNodes()
		m.updateVisibility()
		return true, nil

	case "3":
		if m.sortField == SortByCreatedAt {
			m.sortAscending = !m.sortAscending
		} else {
			m.sortField = SortByCreatedAt
			m.sortAscending = true
		}
		m.sortNodes()
		m.updateVisibility()
		return true, nil

	case "4":
		if m.sortField == SortByStatus {
			m.sortAscending = !m.sortAscending
		} else {
			m.sortField = SortByStatus
			m.sortAscending = true
		}
		m.sortNodes()
		m.updateVisibility()
		return true, nil

	case "0":
		m.sortField = SortByDefault
		m.sortAscending = true
		m.buildTree()
		return true, nil
	}
	return false, nil
}

// handleTaskCRUDKeys handles enter, n, e, D - task create/read/update/delete operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTaskCRUDKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "enter":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			m.clearSelection()
			return true, func() tea.Msg {
				return GlowViewMsg{Task: task}
			}
		}
		return true, nil

	case "n":
		if CursorInBounds(m.cursor, len(visible)) {
			return true, func() tea.Msg {
				return CreateTaskMsg{ParentTask: visible[m.cursor].Task}
			}
		}
		return true, func() tea.Msg {
			return CreateTaskMsg{ParentTask: nil}
		}

	case "O":
		// Create a new note (Shift+O)
		if CursorInBounds(m.cursor, len(visible)) {
			return true, func() tea.Msg {
				return CreateNoteMsg{ParentTask: visible[m.cursor].Task}
			}
		}
		return true, func() tea.Msg {
			return CreateNoteMsg{ParentTask: nil}
		}

	case "e":
		if CursorInBounds(m.cursor, len(visible)) {
			return true, func() tea.Msg {
				return EditTaskMsg{Task: visible[m.cursor].Task}
			}
		}
		return true, nil

	case "D":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.confirmAction = "bulk-delete"
			return true, nil
		}
		if CursorInBounds(m.cursor, len(visible)) {
			m.confirmAction = "delete"
			m.deleteTarget = visible[m.cursor].Task
		}
		return true, nil
	}
	return false, nil
}

// handleClipboardKeys handles y, Y - clipboard operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleClipboardKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "y":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			m.clearSelection()
			return true, func() tea.Msg {
				return CopyTasksTitleMsg{Tasks: tasks}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return CopyTaskTitleMsg{Task: task}
			}
		}
		return true, nil

	case "Y":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			m.clearSelection()
			return true, func() tea.Msg {
				return CopyTasksDetailsMsg{Tasks: tasks}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: task}
			}
		}
		return true, nil
	}
	return false, nil
}

// handleExpandCollapseKeys handles E, c - expand/collapse all operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleExpandCollapseKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "E":
		for _, node := range m.nodes {
			node.Expanded = true
		}
		m.updateVisibility()
		return true, nil

	case "c":
		for _, node := range m.nodes {
			if node.Level > 0 {
				node.Expanded = false
			}
		}
		m.updateVisibility()
		return true, nil
	}
	return false, nil
}

// handleFilterOverlayKeys handles ctrl+f, ctrl+t, ctrl+s - filter/manager overlay operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterOverlayKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "ctrl+f":
		return true, func() tea.Msg {
			return ShowFilterMsg{ProjectTree: m}
		}

	case "ctrl+t":
		return true, func() tea.Msg {
			return ShowTagManagerMsg{ProjectTree: m}
		}

	case "ctrl+s":
		return true, func() tea.Msg {
			return ShowSectionManagerMsg{ProjectTree: m}
		}
	}
	return false, nil
}

// handleTaskAttributeKeys handles ctrl+d, ctrl+n, B, ctrl+p, ctrl+o - task attribute modification.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTaskAttributeKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "ctrl+d":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return SetDueDateMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "ctrl+n":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowSectionPickerMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "B":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowDependencyViewerMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "ctrl+b":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ManageDependenciesMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "ctrl+p":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			for _, task := range m.getSelectedTasks() {
				task.TogglePinned()
			}
			m.clearSelection()
			return true, m.saveAndRefresh()
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			task.TogglePinned()
			return true, m.saveAndRefresh()
		}
		return true, nil

	case "*":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			for _, task := range m.getSelectedTasks() {
				task.ToggleActive()
			}
			m.clearSelection()
			return true, m.saveAndRefresh()
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			task.ToggleActive()
			return true, m.saveAndRefresh()
		}
		return true, nil

	case "ctrl+o":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return EditTaskContentMsg{Task: task, Project: m.project}
			}
		}
		return true, nil
	}
	return false, nil
}

// handleMiscActionKeys handles z, R, ?, \, r - miscellaneous action operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleMiscActionKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "z":
		return true, m.performUndo()

	case "?":
		return true, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewProjectTree,
				ViewName:   "Project Tree",
				Items:      m.GetHelpItems(),
			}
		}

	case "\\":
		return true, func() tea.Msg {
			return ShowTaskJumperMsg{Project: m.project}
		}

	case "r":
		m.clearSelection()
		storage.LoadProject(m.project)
		m.buildTree()
		return true, nil
	}
	return false, nil
}

// handleActionKeys routes action keys to specialized sub-handlers.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleActionKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	if handled, cmd := m.handleTaskCRUDKeys(key, visible); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleClipboardKeys(key, visible); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleExpandCollapseKeys(key); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleFilterOverlayKeys(key); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleTaskAttributeKeys(key, visible); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleMiscActionKeys(key, visible); handled {
		return handled, cmd
	}
	return false, nil
}

// handleArchiveKey handles A - archive/unarchive project (requires special handling for multi-select deselect).
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleArchiveKey(visible []*TreeNode) (bool, tea.Cmd) {
	if m.multiSelectMode {
		for _, node := range m.visibleNodes() {
			delete(m.selectedTasks, node.Task.ID)
		}
		return true, nil
	}
	if m.project.Archived {
		m.confirmAction = "unarchive"
	} else {
		m.confirmAction = "archive"
	}
	return true, nil
}
