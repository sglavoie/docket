package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
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
			if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) && !m.rows[m.cursor].IsSectionHeader {
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

	// Cross-project mode: intercept creation keys before they reach the shared handler.
	if model, cmd, handled := m.crossInterceptCreationKeys(key); handled {
		return model, cmd
	}

	// Cross-project mode: C opens the context picker, including from the empty state.
	if model, cmd, handled := m.crossHandleContextPickerKey(key); handled {
		return model, cmd
	}

	// Sidebar toggle
	if key == "tab" {
		task := m.SidebarSelectedTask()
		m.sidebar.CycleState(task)
		// When entering focused state, start in first non-empty linked section
		if m.sidebar.IsFocused() && task != nil {
			if proj := m.SidebarProject(); proj != nil {
				sections := sidebarGetAvailableSections(proj, task)
				if len(sections) > 0 {
					m.sidebar.SetDependencySection(sections[0])
				}
			}
		}
		return m, nil
	}

	// Prefix key activation (shared across primary views)
	onHeader := len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].IsSectionHeader
	if prefix, ok := activatePrefixKey(key, !onHeader); ok {
		m.pendingPrefix = prefix
		return m, nil
	}
	if isPrefixKeySkippedOnHeader(key, !onHeader) {
		m.statusMessage = "Move cursor to a task first"
		return m, nil
	}
	// Outliner-specific: P prefix for perspectives
	if key == "P" {
		m.pendingPrefix = "P"
		return m, nil
	}

	// Cross-project mode: u performs multi-project undo from crossUndoCtx.
	if model, cmd, handled := m.crossHandleUndo(key); handled {
		return model, cmd
	}

	// Shared primary view keys (view switching, project cycling, secondary views, capture, etc.)
	if cmd, handled := handleCommonPrimaryViewKey(key, m); handled {
		return m, cmd
	}

	// Cursor movement
	prevCursor := m.cursor
	if handled, cmd := handleListNavigation(key, &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash); handled {
		return m.crossSkipSeparators(prevCursor, cmd)
	}

	// Perspective cycling (h/l) — analogous to ProjectTree's view-mode cycling
	if !m.sidebar.IsFocused() {
		switch key {
		case "h", "l":
			if m.project == nil {
				return m.crossHandlePerspectiveCycle(key)
			}
			perspCount := len(PerspectivePresets())
			if key == "h" {
				m.activePerspective = (m.activePerspective + perspCount - 1) % perspCount
			} else {
				m.activePerspective = (m.activePerspective + 1) % perspCount
			}
			m.rebuildRows()
			return m, nil
		}
	}

	switch key {
	case "q":
		// Cross-project zoom-out: pop one level from the cross zoom stack.
		if m.project == nil && len(m.crossZoomStack) > 0 {
			top := m.crossZoomStack[len(m.crossZoomStack)-1]
			m.crossZoomStack = m.crossZoomStack[:len(m.crossZoomStack)-1]
			m.cursor = top.CursorIndex
			return m, nil
		}
		// Cross-project at root: use the dedicated close message so
		// popToAnchorView is called and m.crossOutliner is not nil'd.
		if m.project == nil {
			return m, func() tea.Msg {
				return CloseCrossOutlinerMsg{}
			}
		}
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
		RunEscCascade(m.escSteps())
		return m, nil

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
			m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks?", len(m.selectedTasks)))
			return m, nil
		}
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			if task.IsRecurring() {
				m.confirm.Show("delete-recurring",
					fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
						task.ShortID, task.Title))
			} else {
				m.confirm.Show("delete", fmt.Sprintf("Delete task #%d?", task.ShortID))
			}
			m.deleteTarget = task
		}
		return m, nil

	case "ctrl+f":
		return m, func() tea.Msg {
			return ShowFilterMsg{CommonFilter: &m.filter.CommonFilterState, Project: m.project}
		}

	// Enter: zoom into parent tasks or open detail for leaf tasks
	case "enter":
		// Cross-project mode: resolve project from row.ProjectID and open detail.
		if model, cmd, handled := m.crossHandleEnterKey(); handled {
			return model, cmd
		}

		if len(m.rows) == 0 || m.cursor >= len(m.rows) {
			return m, nil
		}
		row := m.rows[m.cursor]

		// Handle section header toggle
		if row.IsSectionHeader {
			keyCollapse := row.effectiveCollapseKey()
			m.collapsedSections[keyCollapse] = !m.collapsedSections[keyCollapse]
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
		if m.project == nil && len(m.crossZoomStack) > 0 {
			top := m.crossZoomStack[len(m.crossZoomStack)-1]
			m.crossZoomStack = m.crossZoomStack[:len(m.crossZoomStack)-1]
			m.cursor = top.CursorIndex
			return m, nil
		}
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
		if handled, cmd := handleClipboardForPrefixableView(key, m); handled {
			return m, cmd
		}
		return m, nil

	// Cut: copy title(s) to clipboard, then initiate delete
	case "x":
		return m.handleCutKey()

	// Filter keys
	case "T":
		cmd, _ := handleTagKey(m, ViewOutliner, false, m.filter.Tags)
		return m, cmd

	case "n":
		return m, m.jumpToNextSection(1)

	case "N":
		return m, m.jumpToNextSection(-1)

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
		if m.activePerspective == PerspectiveCompleted {
			completedCount := m.project.CountCompletedTasks()
			if completedCount > 0 {
				m.confirm.Show("archive-tasks",
					fmt.Sprintf("Archive %d completed task(s) to completed.md?", completedCount))
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
		if m.project == nil {
			return m, helpCmd(ViewCrossProjectOutliner, "Cross-Project Outliner", m.GetHelpItems())
		}
		return m, helpCmd(ViewOutliner, "Outliner", m.GetHelpItems())

	// Type change keys
	case "<", ">", "H", "L":
		cmd, _ := handleTypeChangeKey(key, m)
		return m, cmd

	// Toggle active flag (with multi-select support)
	case "*":
		cmd, _ := handleToggleActiveKey(m)
		return m, cmd

	// Custom section tabs
	case "[", "]":
		if tabs := m.customSectionTabs(); tabs != nil {
			tab := m.customSectionTabIndex()
			cycleCustomSectionTab(key, &tab, len(tabs), func() {
				m.setCustomSectionTabIndex(tab)
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
		m.clearActiveSavedFilter()
		handleSortKey(key, &m.sort,
			func() { m.rebuildRows() },
			func(SortField) { m.rebuildRows() },
		)
		return m, nil
	}

	return m, nil
}

// handleQuickEditMode handles input while in quick edit mode.
func (m *OutlinerModel) handleQuickEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	_, cmd := handleQuickEditKey(msg, m.quickEdit, m.project, m.saveWithWorkflow, m.setUndoContext)
	return m, cmd
}

// escSteps defines the Esc dismissal cascade for Outliner:
// multi-select → search query → zoom stack (single-project or cross-project).
func (m *OutlinerModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.multiSelectMode }, Clear: func() {
			m.toggleMultiSelectMode()
		}},
		{Active: func() bool { return m.SearchQuery() != "" }, Clear: func() {
			m.ClearSearch()
			m.rebuildRows()
		}},
		{Active: func() bool { return len(m.zoomStack) > 0 }, Clear: func() {
			top := m.zoomStack[len(m.zoomStack)-1]
			m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
			m.cursor = top.CursorIndex
		}},
		{Active: func() bool { return len(m.crossZoomStack) > 0 }, Clear: func() {
			top := m.crossZoomStack[len(m.crossZoomStack)-1]
			m.crossZoomStack = m.crossZoomStack[:len(m.crossZoomStack)-1]
			m.cursor = top.CursorIndex
		}},
	}
}

// ShouldCaptureEsc returns true only when a transient mode is active or the
// view is zoomed in, so that the global handleEscKey() can pop the view when
// at the root level with no active mode.
func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return primaryViewShouldCaptureEsc(&m.InteractiveViewState, &m.BaseSearchableView, m.multiSelectMode, m.quickEdit.IsActive(), m.scoreFilterInput.IsActive()) ||
		AnyEscStepActive(m.escSteps()) || m.sidebar.ShouldCaptureEsc()
}

// HasConfirmDialog returns true when a confirmation dialog is active.
func (m *OutlinerModel) HasConfirmDialog() bool {
	return m.confirm.IsActive()
}

// HasTextInput returns true when the outliner has an active text input field.
func (m *OutlinerModel) HasTextInput() bool {
	return m.DefaultHasTextInput() || m.quickEdit.IsActive() || m.scoreFilterInput.IsActive()
}
