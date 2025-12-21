package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *ProjectTreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	visible := m.visibleNodes()

	switch msg := msg.(type) {
	case treeRefreshMsg:
		m.buildTree()
		return m, nil
	case tea.KeyMsg:
		// Handle confirmation prompts first
		if m.confirmAction != "" {
			switch msg.String() {
			case "y", "Y":
				if m.confirmAction == "archive" {
					m.confirmAction = ""
					return m, m.archiveProject()
				} else if m.confirmAction == "unarchive" {
					m.confirmAction = ""
					return m, m.unarchiveProject()
				} else if m.confirmAction == "delete" {
					m.confirmAction = ""
					return m, m.deleteTask()
				} else if m.confirmAction == "bulk-delete" {
					m.confirmAction = ""
					return m, m.bulkDeleteTasks()
				}
			case "n", "N", "esc":
				m.confirmAction = ""
				m.deleteTarget = nil
				return m, nil
			}
			return m, nil
		}

		// Handle search mode
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.searchQuery = ""
				m.updateVisibility()
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				m.searchQuery = m.searchInput.Value()
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.searchQuery = m.searchInput.Value()
				m.updateVisibility()
				// Reset cursor if out of bounds
				visible = m.visibleNodes()
				if m.cursor >= len(visible) {
					if len(visible) > 0 {
						m.cursor = len(visible) - 1
					} else {
						m.cursor = 0
					}
				}
				return m, cmd
			}
		}

		// Handle tag input mode for bulk tagging
		if m.tagInputMode {
			switch msg.String() {
			case "esc":
				m.tagInputMode = false
				m.tagInput.Blur()
				m.tagInput.SetValue("")
				return m, nil
			case "enter":
				tag := strings.TrimSpace(m.tagInput.Value())
				m.tagInputMode = false
				m.tagInput.Blur()
				m.tagInput.SetValue("")
				if tag == "" {
					return m, nil // Don't clear selection or save on empty tag
				}
				m.applyBulkTag(tag)
				m.clearSelection()
				return m, m.saveAndRefresh()
			default:
				var cmd tea.Cmd
				m.tagInput, cmd = m.tagInput.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "/":
			if m.confirmAction == "" {
				m.searchMode = true
				m.searchInput.Focus()
				return m, textinput.Blink
			}
		case "esc":
			// Exit multi-select mode if active
			if m.multiSelectMode {
				m.clearSelection()
				return m, nil
			}
			// Clear search filter if active
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.updateVisibility()
				return m, nil
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(visible)-1 {
				m.cursor++
			}
		case "enter":
			if len(visible) > 0 && m.cursor < len(visible) {
				m.clearSelection()
				return m, func() tea.Msg {
					return SelectTaskMsg{Task: visible[m.cursor].Task}
				}
			}
		case " ":
			// In multi-select mode, toggle selection
			if m.multiSelectMode {
				if len(visible) > 0 && m.cursor < len(visible) {
					m.toggleSelection(visible[m.cursor].Task)
				}
				return m, nil
			}
			// Normal mode: toggle expand/collapse
			if len(visible) > 0 && m.cursor < len(visible) {
				node := visible[m.cursor]
				if len(node.Task.Children) > 0 {
					node.Expanded = !node.Expanded
					m.updateVisibility()
				}
			}
		case "tab":
			// Toggle expand/collapse
			if len(visible) > 0 && m.cursor < len(visible) {
				node := visible[m.cursor]
				if len(node.Task.Children) > 0 {
					node.Expanded = !node.Expanded
					m.updateVisibility()
				}
			}
		case "v":
			// Toggle multi-select mode
			if m.multiSelectMode {
				m.clearSelection()
			} else {
				m.multiSelectMode = true
				// Select current task when entering multi-select
				if len(visible) > 0 && m.cursor < len(visible) {
					m.toggleSelection(visible[m.cursor].Task)
				}
			}
			return m, nil
		case "a":
			// Select all visible (only in multi-select mode)
			if m.multiSelectMode {
				m.selectAllVisible()
				return m, nil
			}
		case "E":
			// Expand all
			for _, node := range m.nodes {
				node.Expanded = true
			}
			m.updateVisibility()
		case "c":
			// Collapse all
			for _, node := range m.nodes {
				if node.Level > 0 {
					node.Expanded = false
				}
			}
			m.updateVisibility()
		case "d":
			// Toggle show completed
			m.showCompleted = !m.showCompleted
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()

		// Filter controls
		case "P":
			// Cycle priority filter: all -> high -> all
			if m.filterPriority == 0 {
				m.filterPriority = 1 // high priority only
			} else {
				m.filterPriority = 0 // all
			}
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()

		case "t":
			// Cycle type filter
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
							m.filterType = "" // back to all
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

		case "S":
			// Cycle status filter
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
							m.filterStatus = "" // back to all
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

		case "T":
			// In multi-select mode, open tag input for bulk tagging
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				m.tagInputMode = true
				m.tagInput.Focus()
				return m, textinput.Blink
			}
			// Open tag filter selector popup
			return m, func() tea.Msg {
				return ShowTagFilterSelectorMsg{ProjectTree: m}
			}

		case "u":
			// Cycle due date filter: all → overdue → today → this week → no due date → all
			m.filterDueDate = (m.filterDueDate + 1) % 5
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()

		// Sort controls
		case "1":
			// Sort by priority
			if m.sortField == SortByPriority {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortField = SortByPriority
				m.sortAscending = true
			}
			m.sortNodes()
			m.updateVisibility()

		case "2":
			// Sort by due date
			if m.sortField == SortByDueDate {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortField = SortByDueDate
				m.sortAscending = true
			}
			m.sortNodes()
			m.updateVisibility()

		case "3":
			// Sort by created date
			if m.sortField == SortByCreatedAt {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortField = SortByCreatedAt
				m.sortAscending = true
			}
			m.sortNodes()
			m.updateVisibility()

		case "4":
			// Sort by status
			if m.sortField == SortByStatus {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortField = SortByStatus
				m.sortAscending = true
			}
			m.sortNodes()
			m.updateVisibility()

		case "0":
			// Reset to default sort
			m.sortField = SortByDefault
			m.sortAscending = true
			m.buildTree()

		case "?":
			return m, func() tea.Msg {
				return ShowHelpMsg{
					SourceView: ViewProjectTree,
					ViewName:   "Project Tree",
					Items:      m.GetHelpItems(),
				}
			}
		case "g":
			// Jump to task (fuzzy finder)
			return m, func() tea.Msg {
				return ShowTaskJumperMsg{Project: m.project}
			}
		case "ctrl+f":
			return m, func() tea.Msg {
				return ShowFilterMsg{ProjectTree: m}
			}
		case "ctrl+t":
			return m, func() tea.Msg {
				return ShowTagManagerMsg{ProjectTree: m}
			}
		case "r":
			// Reload project
			m.clearSelection()
			storage.LoadProject(m.project)
			m.buildTree()
		case "R":
			// View task with glow pager
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return GlowViewMsg{Task: task}
				}
			}
		case "y":
			// Yank (copy) task title to clipboard
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				tasks := m.getSelectedTasks()
				m.clearSelection()
				return m, func() tea.Msg {
					return CopyTasksTitleMsg{Tasks: tasks}
				}
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return CopyTaskTitleMsg{Task: task}
				}
			}
		case "Y":
			// Yank (copy) full task details to clipboard
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				tasks := m.getSelectedTasks()
				m.clearSelection()
				return m, func() tea.Msg {
					return CopyTasksDetailsMsg{Tasks: tasks}
				}
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return CopyTaskDetailsMsg{Task: task}
				}
			}
		case "ctrl+o":
			// Edit title and description in external editor
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return EditTaskContentMsg{Task: task, Project: m.project}
				}
			}
		case "A":
			// In multi-select mode, deselect all visible
			if m.multiSelectMode {
				for _, node := range m.visibleNodes() {
					delete(m.selectedTasks, node.Task.ID)
				}
				return m, nil
			}
			// Archive/unarchive project
			if m.project.Archived {
				m.confirmAction = "unarchive"
			} else {
				m.confirmAction = "archive"
			}

		// Task status changes
		case "s":
			// Start task (in_progress)
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				for _, task := range m.getSelectedTasks() {
					if task.Status == models.StatusTodo || task.Status == models.StatusBlocked {
						task.Start()
					}
				}
				m.clearSelection()
				return m, m.saveAndRefresh()
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status == models.StatusTodo || task.Status == models.StatusBlocked {
					task.Start()
					return m, m.saveAndRefresh()
				}
			}
		case "m":
			// Mark complete (done)
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				var skippedIDs []string
				completedCount := 0
				for _, task := range m.getSelectedTasks() {
					if task.Status.IsActive() {
						if incompleteDeps := m.project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
							skippedIDs = append(skippedIDs, fmt.Sprintf("#%d", task.ShortID))
							continue
						}
						task.Complete()
						completedCount++
					}
				}
				m.clearSelection()
				if len(skippedIDs) > 0 {
					var msg string
					if completedCount == 0 {
						msg = fmt.Sprintf("no tasks completed: %s have incomplete dependencies", strings.Join(skippedIDs, ", "))
					} else {
						msg = fmt.Sprintf("completed %d, skipped %s (incomplete dependencies)", completedCount, strings.Join(skippedIDs, ", "))
					}
					return m, tea.Batch(m.saveAndRefresh(), func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("%s", msg)}
					})
				}
				return m, m.saveAndRefresh()
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status.IsActive() {
					// Block completion if task has incomplete dependencies
					if incompleteDeps := m.project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
						depNames := make([]string, len(incompleteDeps))
						for i, dep := range incompleteDeps {
							depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
						}
						return m, func() tea.Msg {
							return ErrorMsg{Err: fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
								len(incompleteDeps), strings.Join(depNames, ", "))}
						}
					}
					task.Complete()
					return m, m.saveAndRefresh()
				}
			}
		case "b":
			// Block task
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				for _, task := range m.getSelectedTasks() {
					if task.Status.IsActive() && task.Status != models.StatusBlocked {
						task.Block()
					}
				}
				m.clearSelection()
				return m, m.saveAndRefresh()
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status.IsActive() && task.Status != models.StatusBlocked {
					task.Block()
					return m, m.saveAndRefresh()
				}
			}
		case "x":
			// Cancel task
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				for _, task := range m.getSelectedTasks() {
					if task.Status.IsActive() {
						task.Cancel()
					}
				}
				m.clearSelection()
				return m, m.saveAndRefresh()
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status.IsActive() {
					task.Cancel()
					return m, m.saveAndRefresh()
				}
			}
		case "o":
			// Reopen task (back to todo)
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				for _, task := range m.getSelectedTasks() {
					if task.Status != models.StatusTodo {
						task.Reopen()
					}
				}
				m.clearSelection()
				return m, m.saveAndRefresh()
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if task.Status != models.StatusTodo {
					task.Reopen()
					return m, m.saveAndRefresh()
				}
			}
		case "`":
			// Cycle status forward: todo -> in_progress -> done -> todo
			// Skips blocked and cancelled (require explicit intent)
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
					return m, tea.Batch(m.saveAndRefresh(), func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("%s", msg)}
					})
				}
				return m, m.saveAndRefresh()
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				switch task.Status {
				case models.StatusTodo:
					task.Start()
					return m, m.saveAndRefresh()
				case models.StatusInProgress:
					if incompleteDeps := m.project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
						depNames := make([]string, len(incompleteDeps))
						for i, dep := range incompleteDeps {
							depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
						}
						return m, func() tea.Msg {
							return ErrorMsg{Err: fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
								len(incompleteDeps), strings.Join(depNames, ", "))}
						}
					}
					task.Complete()
					return m, m.saveAndRefresh()
				case models.StatusDone:
					task.Reopen()
					return m, m.saveAndRefresh()
				}
			}
		case "~":
			// Cycle status backward: todo -> done -> in_progress -> todo
			// Skips blocked and cancelled (require explicit intent)
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
				return m, m.saveAndRefresh()
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				switch task.Status {
				case models.StatusTodo:
					task.Complete()
					return m, m.saveAndRefresh()
				case models.StatusInProgress:
					task.Reopen()
					return m, m.saveAndRefresh()
				case models.StatusDone:
					task.Start()
					return m, m.saveAndRefresh()
				}
			}

		// Task type changes
		case "h":
			// Promote task type (Task -> Story -> Epic)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanPromote(); ok {
					task.Promote()
					return m, m.saveAndRefresh()
				}
			}
		case "l":
			// Demote task type (Epic -> Story -> Task)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanDemote(); ok {
					task.Demote()
					return m, m.saveAndRefresh()
				}
			}
		case "H":
			// Cycle leaf type left (Idea -> Bug -> Task)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanCycleLeafType(true); ok {
					task.CycleLeafType(true)
					return m, m.saveAndRefresh()
				}
			}
		case "L":
			// Cycle leaf type right (Task -> Bug -> Idea)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanCycleLeafType(false); ok {
					task.CycleLeafType(false)
					return m, m.saveAndRefresh()
				}
			}

		// Task movement operations
		case "K":
			// Move task up among siblings
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				siblings := m.project.GetSiblings(task)
				var siblingIdx int = -1
				for i, s := range siblings {
					if s.ID == task.ID {
						siblingIdx = i
						break
					}
				}
				if siblingIdx > 0 {
					m.project.MoveTaskUp(task)
					if m.cursor > 0 {
						m.cursor--
					}
					return m, m.saveAndRefresh()
				}
			}
		case "J":
			// Move task down among siblings
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				siblings := m.project.GetSiblings(task)
				var siblingIdx int = -1
				for i, s := range siblings {
					if s.ID == task.ID {
						siblingIdx = i
						break
					}
				}
				if siblingIdx >= 0 && siblingIdx < len(siblings)-1 {
					m.project.MoveTaskDown(task)
					if m.cursor < len(visible)-1 {
						m.cursor++
					}
					return m, m.saveAndRefresh()
				}
			}
		case "right":
			// Indent task (nest under task above)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				targetParent := m.project.GetTaskAbove(task)
				if targetParent != nil {
					if ok, _ := task.CanIndent(targetParent); ok {
						m.project.IndentTask(task)
						return m, m.saveAndRefresh()
					}
				}
			}
		case "left":
			// Outdent task (move to parent's level)
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				if ok, _ := task.CanOutdent(); ok {
					m.project.OutdentTask(task)
					return m, m.saveAndRefresh()
				}
			}

		// Task operations
		case "n":
			// Add new task
			if len(visible) > 0 && m.cursor < len(visible) {
				// Use selected task as potential parent
				return m, func() tea.Msg {
					return CreateTaskMsg{ParentTask: visible[m.cursor].Task}
				}
			}
			// No task selected, create root task
			return m, func() tea.Msg {
				return CreateTaskMsg{ParentTask: nil}
			}
		case "e":
			// Edit selected task
			if len(visible) > 0 && m.cursor < len(visible) {
				return m, func() tea.Msg {
					return EditTaskMsg{Task: visible[m.cursor].Task}
				}
			}
		case "D":
			// Delete task (requires confirmation)
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				m.confirmAction = "bulk-delete"
				return m, nil
			}
			if len(visible) > 0 && m.cursor < len(visible) {
				m.confirmAction = "delete"
				m.deleteTarget = visible[m.cursor].Task
			}
		case "ctrl+d":
			// Set due date for selected task
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: task, Project: m.project}
				}
			}
		case "B":
			// Manage dependencies for selected task
			if len(visible) > 0 && m.cursor < len(visible) {
				task := visible[m.cursor].Task
				return m, func() tea.Msg {
					return ManageDependenciesMsg{Task: task, Project: m.project}
				}
			}
		case "z":
			// Undo last operation
			return m, m.performUndo()
		}
	}

	return m, nil
}
