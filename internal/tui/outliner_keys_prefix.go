package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *OutlinerModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) { //nolint:gocyclo
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	// Handle filter sub-prefixes (Ft, Fs, Fd, Fa, Fas-Fap, FS, Fv)
	switch prefix {
	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS", "F.":
		if prefix == "FS" && key == "C" {
			var custom *float64
			if m.config != nil {
				custom = m.config.ScoreFilterCustom
			}
			m.scoreFilterInput.Enter(custom)
			return m, nil
		}
		if dispatchFilterSubPrefix(prefix, key, &m.filter.CommonFilterState, m.config, func(p string) { m.pendingPrefix = p }) {
			m.rebuildRows()
		}
		return m, nil
	case "Fv":
		return m.handleSavedFilterPrefix(key)
	}

	// Get cursor task (may be nil if cursor is out of bounds)
	var cursorTask *models.Task
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
		cursorTask = m.rows[m.cursor].task
	}

	switch prefix {
	case ",":
		// Cross-project: route to resolved project; recurrence ops need single-project context.
		if m.project == nil {
			if model, cmd, handled := m.crossHandleStatusMutation(key); handled {
				return model, cmd
			}
			return m, nil
		}
		if key == "k" {
			cmd, _ := handleSkipRecurrence(m)
			return m, cmd
		}
		if key == "D" {
			m.pendingPrefix = ",D"
			return m, nil
		}
		if status, ok := StatusForPrefixKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				return m.applyBulkStatusChange(status)
			}
			if cursorTask == nil {
				return m, nil
			}
			pf := m.workflowPersistFn()
			result := applyStatusChange(pf, m.project, cursorTask.ID, status)
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.setStatus(result.StatusMessage)
			m.rebuildRows()
			return m, result.Cmd
		}

	case ",D":
		if m.project == nil {
			return m, nil
		}
		if cmd, ok := handleDeferRecurrence(key, m); ok {
			return m, cmd
		}

	case "t":
		// Cross-project: resolve project from cursor, delegate to shared handlers.
		if m.project == nil {
			// Block task creation (same as a/i/I)
			if key == "n" || key == "N" {
				m.setStatus("use go to open project outliner for task creation")
				return m, nil
			}
			row, proj, valid := m.crossGetMutationTarget()
			if !valid {
				return m, nil
			}
			task := row.task
			// to, tc, ta, tl, te, tm → message-only handlers
			if handled, cmd := handleTaskPrefixMsgKey(key, task, proj); handled {
				return m, cmd
			}
			// ts, tt → action handlers (no focusedTask in cross-project)
			if handled, cmd := handleTaskActionPrefixKey(key, task, proj, nil, func() []*models.Task {
				return m.getMultiSelectedTasks()
			}); handled {
				return m, cmd
			}
			// tr → rename via quickEdit
			if key == "r" {
				m.crossQuickEditProject = proj
				return m, m.quickEdit.Enter(task, true)
			}
			// tp → pin/unpin
			if key == "p" {
				return m.crossTogglePinned(row, proj)
			}
			// td → dim/undim
			if key == "d" {
				return m.crossToggleDimmed(row, proj)
			}
			// tD → delete
			if key == "D" {
				if task.IsRecurring() {
					m.confirm.Show("delete-recurring",
						fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
							task.ShortID, task.Title))
				} else {
					m.confirm.Show("delete", fmt.Sprintf("Delete task #%d?", task.ShortID))
				}
				m.deleteTarget = task
				return m, nil
			}
			return m, nil
		}
		var focusedTask *models.Task
		if len(m.zoomStack) > 0 {
			topID := m.zoomStack[len(m.zoomStack)-1].TaskID
			focusedTask = m.project.GetTaskByID(topID)
		}
		if cmd, handled := handleSharedTaskPrefix(key, m, focusedTask, func(task *models.Task) tea.Cmd {
			return m.quickEdit.Enter(task, true)
		}); handled {
			return m, cmd
		}

	case "P":
		// Perspective switching (P-prefix: P0-P9)
		if key >= "0" && key <= "9" {
			perspKey := int(key[0] - '0')
			if m.project == nil {
				return m.crossHandlePerspectiveJump(perspKey)
			}
			m.activePerspective = perspKey
			m.rebuildRows()
			return m, nil
		}

	case ".":
		if m.project == nil {
			if cmd, ok := m.crossHandleStageMutation(key); ok {
				return m, cmd
			}
			return m, nil
		}
		if cmd, ok := handleStagePrefix(key, m, m.config.Stages); ok {
			return m, cmd
		}

	case "!":
		if m.project == nil {
			if cmd, ok := m.crossHandlePriorityMutation(key); ok {
				return m, cmd
			}
			return m, nil
		}
		if cmd, ok := handlePriorityPrefix(key, m); ok {
			return m, cmd
		}

	case "d":
		if m.project == nil {
			if cmd, ok := m.crossHandleDueDateMutation(key); ok {
				return m, cmd
			}
			return m, nil
		}
		if cmd, ok := handleDueDatePrefix(key, m); ok {
			return m, cmd
		}

	case "s":
		if m.project == nil {
			if cmd, ok := m.crossHandleSnoozeMutation(key); ok {
				return m, cmd
			}
			return m, nil
		}
		if cmd, ok := handleSnoozePrefix(key, m); ok {
			return m, cmd
		}

	case "z":
		// Fold operations
		switch key {
		case "a":
			// Toggle fold
			if cursorTask != nil && len(cursorTask.Children) > 0 {
				m.ensureExpandedNodes()
				if m.isExpanded(cursorTask.ID) {
					m.expandedNodes[cursorTask.ID] = false
				} else {
					m.expandedNodes[cursorTask.ID] = true
				}
				m.rebuildRows()
			}
			return m, nil
		case "o":
			// Expand
			if cursorTask != nil && len(cursorTask.Children) > 0 {
				m.ensureExpandedNodes()
				m.expandedNodes[cursorTask.ID] = true
				m.rebuildRows()
			}
			return m, nil
		case "c":
			// Collapse
			if cursorTask != nil && len(cursorTask.Children) > 0 {
				m.ensureExpandedNodes()
				m.expandedNodes[cursorTask.ID] = false
				m.rebuildRows()
			}
			return m, nil
		case "R":
			// Expand all
			m.expandedNodes = make(map[string]bool)
			m.rebuildRows()
			return m, nil
		case "M":
			// Collapse all - mark all parent tasks as collapsed
			if m.project == nil {
				m.ensureExpandedNodes()
				for _, p := range m.loadedProjects {
					m.collapseAllParents(p.GetRootTasks())
				}
				m.rebuildRows()
				return m, nil
			}
			m.ensureExpandedNodes()
			m.collapseAllParents(m.project.GetRootTasks())
			m.rebuildRows()
			return m, nil
		case "v":
			// Zoom into current task.
			if cursorTask != nil && len(cursorTask.Children) > 0 {
				if m.project == nil {
					// Cross-project mode: push CrossProjectZoomEntry scoped to the
					// owning project so UUID collisions across projects cannot cause
					// incorrect zoom resolution.
					projectID := m.rows[m.cursor].ProjectID
					m.crossZoomStack = append(m.crossZoomStack, CrossProjectZoomEntry{
						TaskID:      cursorTask.ID,
						ProjectID:   projectID,
						CursorIndex: m.cursor,
					})
					m.cursor = 0
				} else {
					m.zoomStack = append(m.zoomStack, ZoomEntry{
						TaskID:      cursorTask.ID,
						CursorIndex: m.cursor,
					})
					m.cursor = 0
				}
			}
			return m, nil
		case "p":
			m.previewMode = !m.previewMode
			return m, nil
		}

	case "F":
		// Filter prefix
		return m.handleFilterPrefix(key)

	case "m":
		if m.project == nil {
			return m.crossHandleSectionMove(key, cursorTask)
		}
		if model, cmd, handled := handleSectionMovePrefix(key, SectionMoveCallbacks{
			MoveToSection: func(section string) (tea.Model, tea.Cmd) {
				return m.sendTaskToSection(cursorTask, section)
			},
			BulkMoveToSection: func(section string) (tea.Model, tea.Cmd) {
				return m.sendTasksToSectionBulk(section)
			},
			MoveToTop: func() (tea.Model, tea.Cmd) {
				if cursorTask == nil {
					return m, nil
				}
				return m.moveTaskToPosition(cursorTask, true)
			},
			MoveToBottom: func() (tea.Model, tea.Cmd) {
				if cursorTask == nil {
					return m, nil
				}
				return m.moveTaskToPosition(cursorTask, false)
			},
			IsMultiSelect: func() bool { return m.multiSelectMode && len(m.selectedTasks) > 0 },
		}); handled {
			return model, cmd
		}

	case "b":
		if m.project == nil {
			row, proj, valid := m.crossGetMutationTarget()
			if !valid {
				return m, nil
			}
			if handled, cmd := handleBlockerPrefixKey(key, row.task, proj); handled {
				return m, cmd
			}
			return m, nil
		}
		if cursorTask == nil {
			return m, nil
		}
		if handled, cmd := handleBlockerPrefixKey(key, cursorTask, m.project); handled {
			return m, cmd
		}

	case "c":
		if m.project == nil {
			row, proj, valid := m.crossGetMutationTarget()
			if !valid {
				return m, nil
			}
			if handled, cmd := handleLinkedTaskPrefixKey(key, row.task, proj, true, true); handled {
				return m, cmd
			}
			return m, nil
		}
		if cursorTask == nil {
			return m, nil
		}
		if handled, cmd := handleLinkedTaskPrefixKey(key, cursorTask, m.project, true, true); handled {
			return m, cmd
		}

	case "g":
		switch key {
		case "g":
			old := m.cursor
			m.cursor = 0
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		case "h":
			if m.project == nil {
				return m, nil
			}
			if cursorTask != nil {
				task := cursorTask
				project := m.project
				return m, func() tea.Msg {
					return OpenTaskActivityMsg{Task: task, Project: project}
				}
			}
			return m, nil
		case "X":
			// Deselect all visible in multi-select mode
			if m.multiSelectMode {
				m.deselectAllVisible()
			}
			return m, nil
		case "d":
			// Cycle due date filter
			m.cycleDueDateFilter()
			m.rebuildRows()
			return m, nil
		case "z":
			// Jump directly to Snoozed perspective (P0)
			m.activePerspective = PerspectiveSnoozed
			m.rebuildRows()
			return m, nil
		case "l":
			if m.project == nil {
				return m, nil
			}
			// Go to project recent activity
			project := m.project
			return m, func() tea.Msg {
				return ShowProjectRecentActivityMsg{Project: project}
			}
		case "o":
			// Drill down to the single-project outliner for the cursor task's project
			if m.project == nil {
				return m.crossHandleGoDrillDown()
			}
			return m, nil
		}
	}

	// Invalid prefix sequence - silently ignore
	return m, nil
}

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *OutlinerModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// Any manual filter change clears the saved filter indicator
	if key != "v" {
		m.clearActiveSavedFilter()
	}

	// View-specific filter keys
	switch key {
	case "v":
		m.pendingPrefix = "Fv"
		return m, nil
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "c":
		if m.activePerspective == PerspectiveCompleted {
			m.activePerspective = PerspectiveSnoozed
		} else {
			m.activePerspective = PerspectiveCompleted
		}
		m.rebuildRows()
		return m, nil
	case "@":
		return m, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{Project: m.project}
		}
	case "r":
		m.filter.Actionable = !m.filter.Actionable
		m.rebuildRows()
		return m, nil
	case "D":
		source := ViewOutliner
		if m.project == nil {
			source = ViewCrossProjectOutliner
		}
		return m, func() tea.Msg {
			return ShowColumnPickerMsg{Source: source, ColumnConfig: m.columnConfig}
		}
	case "x":
		clearCommonFilters(&m.filter.CommonFilterState)
		m.filter.Actionable = false
		m.filter.Sections = make(map[string]bool)
		m.ClearSearch()
		m.rebuildRows()
		return m, nil
	}

	// Common filter prefix keys
	result := handleCommonFilterPrefix(key, &m.filter.CommonFilterState, func() tea.Cmd {
		var taskSlice []*models.Task
		if m.project != nil {
			taskSlice = m.project.Tasks
		} else {
			taskSlice = m.crossMergedTaskSlice()
		}
		tags := BuildTagList(taskSlice, false)
		return func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewOutliner, Title: "Filter by Tags"}
		}
	})
	if cmd := applyCommonFilterResult(result, func(p string) { m.pendingPrefix = p }, func() { m.rebuildRows() }); cmd != nil {
		return m, cmd
	}
	if result.Changed {
		return m, nil
	}
	return m, nil
}

// handleSavedFilterPrefix handles the third key after pressing Fv (saved filters sub-prefix).
func (m *OutlinerModel) handleSavedFilterPrefix(key string) (tea.Model, tea.Cmd) {
	cmd, _ := handleSavedFilterPrefixKey(key)
	return m, cmd
}

// cycleDueDateFilter cycles through due date filter modes.
func (m *OutlinerModel) cycleDueDateFilter() {
	switch m.filter.DueDate {
	case DueDateAll:
		m.filter.DueDate = DueDateOverdue
	case DueDateOverdue:
		m.filter.DueDate = DueDateToday
	case DueDateToday:
		m.filter.DueDate = DueDateThisWeek
	case DueDateThisWeek:
		m.filter.DueDate = DueDateNone
	default:
		m.filter.DueDate = DueDateAll
	}
}
