package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleKeyMsg processes key events in the outliner view.
func (m *OutlinerModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle score filter custom value input
	if m.scoreFilterInput.IsActive() {
		cmd := m.scoreFilterInput.ProcessKey(msg, m.config, m.configManager, &m.filter.ScoreMin, func() { m.rebuildRows() })
		return m, cmd
	}

	// Handle search mode input first
	if model, cmd, handled := m.HandleSearchMode(msg, m, m.rebuildRows); handled {
		return model, cmd
	}

	// Handle quick edit mode input first
	if m.quickEdit.IsActive() {
		return m.handleQuickEditMode(msg)
	}

	// Handle confirm dialog before anything else
	if m.confirm.IsActive() {
		return m.handleConfirmDialog(key)
	}

	// Sidebar-focused key handling (before prefix/action keys)
	if m.sidebar.IsFocused() {
		_, cmd := m.sidebar.HandleKeys(m, msg)
		// When sidebar is focused, consume all keys to prevent tree navigation
		return m, cmd
	}

	// Handle pending prefix keys first (which-key style two-key sequences)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Multi-select mode key handling (when active, intercept selection keys)
	if m.multiSelectMode {
		switch key {
		case "v":
			m.toggleMultiSelectMode()
			return m, nil
		case " ":
			if len(m.rows) > 0 && m.cursor < len(m.rows) && !m.rows[m.cursor].IsSectionHeader {
				m.ToggleSelection(m.rows[m.cursor].task.ID)
			}
			return m, nil
		case "a":
			m.selectAllVisible()
			return m, nil
		case "A":
			m.deselectAllVisible()
			return m, nil
		}
	}

	// Sidebar toggle
	if key == "tab" {
		task := m.SidebarSelectedTask()
		m.sidebar.CycleState(task)
		// When entering focused state, start in first non-empty linked section
		if m.sidebar.IsFocused() && task != nil {
			sections := sidebarGetAvailableSections(m.project, task)
			if len(sections) > 0 {
				m.sidebar.SetDependencySection(sections[0])
			}
		}
		return m, nil
	}

	// Prefix key activation (shared across primary views)
	onHeader := len(m.rows) > 0 && m.cursor < len(m.rows) && m.rows[m.cursor].IsSectionHeader
	if prefix, ok := activatePrefixKey(key, !onHeader); ok {
		m.pendingPrefix = prefix
		return m, nil
	}
	// Outliner-specific: P prefix for perspectives
	if key == "P" {
		m.pendingPrefix = "P"
		return m, nil
	}

	// Shared primary view keys (view switching, project cycling, secondary views, capture, etc.)
	if cmd, handled := handleCommonPrimaryViewKey(key, m); handled {
		return m, cmd
	}

	// Cursor movement
	if handleListNavigation(key, &m.cursor, len(m.rows), m.contentHeight()) {
		return m, nil
	}

	switch key {
	case "q":
		if len(m.zoomStack) == 0 {
			return m, func() tea.Msg {
				return CloseOutlinerMsg{}
			}
		}
		top := m.zoomStack[len(m.zoomStack)-1]
		m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
		m.cursor = top.CursorIndex
		return m, nil

	case "esc":
		if m.multiSelectMode {
			m.toggleMultiSelectMode()
			return m, nil
		}
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.rebuildRows()
			return m, nil
		}
		if len(m.zoomStack) > 0 {
			top := m.zoomStack[len(m.zoomStack)-1]
			m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
			m.cursor = top.CursorIndex
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseOutlinerMsg{}
		}

	// Task movement
	case "J":
		// Move task down (swap with sibling below)
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			siblings := m.getSiblings(task)
			idx := -1
			for i, s := range siblings {
				if s.ID == task.ID {
					idx = i
					break
				}
			}
			if idx >= 0 && idx < len(siblings)-1 {
				taskID := task.ID
				m.project.SwapTasks(task, siblings[idx+1])
				m.setUndoContext("Move Down", task.Title)
				m.rebuildRows()
				m.moveCursorToTask(taskID)
				return m, m.saveWithWorkflow()
			}
		}
		return m, nil
	case "K":
		// Move task up (swap with sibling above)
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			siblings := m.getSiblings(task)
			idx := -1
			for i, s := range siblings {
				if s.ID == task.ID {
					idx = i
					break
				}
			}
			if idx > 0 {
				taskID := task.ID
				m.project.SwapTasks(task, siblings[idx-1])
				m.setUndoContext("Move Up", task.Title)
				m.rebuildRows()
				m.moveCursorToTask(taskID)
				return m, m.saveWithWorkflow()
			}
		}
		return m, nil
	case "right":
		// Indent task (nest under task above in the flat list)
		if len(m.rows) > 1 && m.cursor > 0 && m.cursor < len(m.rows) && !onHeader {
			// The row above may be a section header (no task), skip if so
			if m.rows[m.cursor-1].task == nil {
				return m, nil
			}
			// Resolve cloned row tasks back to originals in project
			task := m.project.GetTaskByID(m.rows[m.cursor].task.ID)
			candidate := m.project.GetTaskByID(m.rows[m.cursor-1].task.ID)
			if task == nil || candidate == nil {
				return m, nil
			}
			if IndentTask(m.project, task, candidate) {
				m.setUndoContext("Indent", task.Title)
				m.validateZoomStack()
				m.rebuildRows()
				return m, m.saveWithWorkflow()
			}
		}
		return m, nil
	case "left":
		// Outdent task (to parent level)
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			// Resolve cloned row task back to original in project
			task := m.project.GetTaskByID(m.rows[m.cursor].task.ID)
			if task == nil {
				return m, nil
			}
			if OutdentTask(m.project, task) {
				m.setUndoContext("Outdent", task.Title)
				m.validateZoomStack()
				m.rebuildRows()
				return m, m.saveWithWorkflow()
			}
		}
		return m, nil

	// Open task detail view
	case "o":
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			project := m.project
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return m, nil

	case "e":
		// Open full editor (with multi-select bulk edit support)
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			return m, func() tea.Msg {
				return ShowBulkEditMsg{
					Tasks:   m.getSelectedTasks(),
					Project: m.project,
				}
			}
		}
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			return m, func() tea.Msg {
				return EditTaskMsg{Task: task}
			}
		}
		return m, nil

	case "E":
		// Quick edit with full modifier parsing (same as O)
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			return m, m.quickEdit.Enter(task, false)
		}
		return m, nil

	// Delete with confirmation
	case "D":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks? (y/n)", len(m.selectedTasks)))
			m.statusMessage = m.confirm.Prompt()
			return m, nil
		}
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			if task.IsRecurring() {
				m.confirm.Show("delete-recurring",
					fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
						task.ShortID, task.Title))
			} else {
				m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", task.ShortID))
			}
			m.statusMessage = m.confirm.Prompt()
			m.deleteTarget = task
		}
		return m, nil

	case "ctrl+f":
		return m, func() tea.Msg {
			return ShowFilterMsg{CommonFilter: &m.filter, Project: m.project}
		}

	// Enter: zoom into parent tasks or open detail for leaf tasks
	case "enter":
		if len(m.rows) == 0 {
			return m, nil
		}
		row := m.rows[m.cursor]

		// Handle section header toggle
		if row.IsSectionHeader {
			m.collapsedSections[row.SectionName] = !m.collapsedSections[row.SectionName]
			m.rebuildRows()
			return m, nil
		}

		// Handle regular task rows
		task := row.task
		if len(task.Children) > 0 {
			m.zoomStack = append(m.zoomStack, ZoomEntry{
				TaskID:      task.ID,
				CursorIndex: m.cursor,
			})
			m.cursor = 0
			return m, nil
		}
		return m, func() tea.Msg {
			return ShowTaskDetailMsg{Task: task, Project: m.project}
		}

	// Backspace: zoom out one level (no-op at root)
	case "backspace":
		if len(m.zoomStack) > 0 {
			top := m.zoomStack[len(m.zoomStack)-1]
			m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
			m.cursor = top.CursorIndex
		}
		return m, nil

	// Multi-select mode
	case "v":
		m.toggleMultiSelectMode()
		return m, nil

	// Clipboard operations
	case "y", "Y", "ctrl+y":
		if handled, cmd := handleClipboardKey(key,
			func() *models.Task {
				if len(m.rows) > 0 && m.cursor < len(m.rows) && !m.rows[m.cursor].IsSectionHeader {
					return m.rows[m.cursor].task
				}
				return nil
			},
			func() []*models.Task {
				if m.multiSelectMode && len(m.selectedTasks) > 0 {
					return m.getSelectedTasks()
				}
				return nil
			},
			m.clearMultiSelect,
		); handled {
			return m, cmd
		}
		return m, nil

	// Cut: copy title(s) to clipboard, then initiate delete
	case "x":
		return m.handleCutKey()

	// Filter keys
	case "T":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			return m, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: m.project}
			}
		}
		tags := BuildTagList(m.project.Tasks, false)
		return m, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewOutliner, Title: "Filter by Tags"}
		}

	case "n":
		m.jumpToNextSection(1)
		return m, nil

	case "N":
		m.jumpToNextSection(-1)
		return m, nil

	case "C":
		return m, func() tea.Msg {
			return ShowContextPickerMsg{}
		}

	// Additional views
	case "f":
		return m, func() tea.Msg {
			return ShowProjectFocusViewMsg{Project: m.project}
		}

	case "A":
		// Archive completed tasks when in Completed perspective (p8)
		if m.activePerspective == 8 {
			completedCount := m.project.CountCompletedTasks()
			if completedCount > 0 {
				m.confirm.Show("archive-tasks",
					fmt.Sprintf("Archive %d completed task(s) to completed.md? (y/n)", completedCount))
				m.statusMessage = m.confirm.Prompt()
			}
			return m, nil
		}
		// Otherwise open all tasks view
		return m, func() tea.Msg {
			return ShowAllTasksMsg{}
		}

	// Search
	case "/":
		return m, m.ActivateSearch()

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewOutliner,
				ViewName:   "Outliner",
				Items:      m.GetHelpItems(),
			}
		}

	// Type change keys
	case "<", ">", "H", "L":
		op, undoAction, ok := typeChangeOpForKey(key)
		if !ok {
			return m, nil
		}
		if len(m.rows) == 0 || m.cursor >= len(m.rows) || onHeader {
			return m, nil
		}
		task := m.rows[m.cursor].task
		m.setUndoContext(undoAction, task.Title)
		result := applyTypeChange(m.workflowPersistFn(), m.project, task.ID, op)
		if result.Err != nil {
			return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
		}
		if result.StatusMessage != "" {
			m.rebuildRows()
		}
		return m, result.Cmd

	// Toggle active flag (with multi-select support)
	case "*":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := len(m.selectedTasks)
			for _, task := range m.getSelectedTasks() {
				task.ToggleActive()
			}
			m.setUndoContext("Toggle Active", fmt.Sprintf("%d tasks", count))
			m.clearMultiSelect()
			m.rebuildRows()
			return m, m.saveWithWorkflow()
		}
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			m.setUndoContext("Toggle Active", task.Title)
			result := applyToggleActive(m.workflowPersistFn(), m.project, task.ID)
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.rebuildRows()
			return m, result.Cmd
		}
		return m, nil

	// Custom section tabs
	case "[", "]":
		if tabs := m.customSectionTabs(); tabs != nil {
			cycleCustomSectionTab(key, &m.customSectionTab, len(tabs), func() {
				m.rebuildRows()
			})
		}
		return m, nil

	// Archive/unarchive project
	case "ctrl+a":
		if m.project.Sandbox {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("cannot archive the sandbox project")}
			}
		}
		return m, m.archiveProject()

	// Tag manager
	case "ctrl+t":
		return m, func() tea.Msg {
			return ShowTagManagerMsg{Project: m.project}
		}

	// Section manager
	case "ctrl+s":
		return m, func() tea.Msg {
			return ShowSectionManagerMsg{ProjectTree: nil, Project: m.project}
		}

	// Sort keys: bare 1-5, 0
	case "1", "2", "3", "4", "5", "0":
		handleSortKey(key, &m.sortField, &m.sortAscending,
			func() { m.rebuildRows() },
			func(SortField) { m.rebuildRows() },
		)
		return m, nil
	}

	return m, nil
}

// handleQuickEditMode handles input while in quick edit mode.
func (m *OutlinerModel) handleQuickEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.quickEdit.Exit()
		return m, nil

	case "enter":
		input := strings.TrimSpace(m.quickEdit.GetInput().Value())
		if input == "" {
			m.quickEdit.Exit()
			return m, nil
		}

		// Re-lookup task to avoid stale pointer after concurrent reload
		task := m.project.GetTaskByID(m.quickEdit.GetTask().ID)
		if task == nil {
			m.quickEdit.Exit()
			return m, nil
		}

		if m.quickEdit.IsRenameOnly() {
			// Rename-only mode: just update title
			m.project.MutateTask(task.ID, func(t *models.Task) {
				t.Title = input
			})
		} else {
			// Full quick edit mode: parse modifiers
			result, err := parseQuickEdit(input, task, m.project)
			if err != nil {
				return m, nil
			}
			applyQuickEdit(task, m.project, result)
		}

		m.quickEdit.Exit()
		return m, m.saveWithWorkflow()

	default:
		var cmd tea.Cmd
		inputModel := m.quickEdit.GetInput()
		inputModel, cmd = inputModel.Update(msg)
		m.quickEdit.UpdateInput(inputModel)

		// Update preview (only in full edit mode, not rename mode)
		if !m.quickEdit.IsRenameOnly() {
			inputValue := m.quickEdit.GetInput().Value()
			preview, _ := parseQuickEditForPreview(inputValue, m.quickEdit.GetTask(), m.project)
			m.quickEdit.SetPreview(preview)
		}

		return m, cmd
	}
}

// ShouldCaptureEsc returns true only when a transient mode is active or the
// view is zoomed in, so that the global handleEscKey() can pop the view when
// at the root level with no active mode.
func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.multiSelectMode || m.quickEdit.IsActive() || len(m.zoomStack) > 0 || m.IsSearchMode() || m.SearchQuery() != "" || m.scoreFilterInput.IsActive() || m.sidebar.ShouldCaptureEsc()
}

// HasConfirmDialog returns true when a confirmation dialog is active.
func (m *OutlinerModel) HasConfirmDialog() bool {
	return m.confirm.IsActive()
}

// HasTextInput returns true when the outliner has an active text input field.
func (m *OutlinerModel) HasTextInput() bool {
	return m.quickEdit.IsActive() || m.IsSearchMode() || m.scoreFilterInput.IsActive()
}
