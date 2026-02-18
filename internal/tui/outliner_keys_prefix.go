package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *OutlinerModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	// Handle filter sub-prefixes (Ft, Fs, Fd, Fa, Fas-Fap, FS, Fv)
	switch prefix {
	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		if prefix == "FS" && key == "C" {
			var custom *float64
			if m.config != nil {
				custom = m.config.ScoreFilterCustom
			}
			m.scoreFilterInput.Enter(custom)
			return m, nil
		}
		if dispatchFilterSubPrefix(prefix, key, &m.filter, m.config, func(p string) { m.pendingPrefix = p }) {
			m.rebuildRows()
		}
		return m, nil
	case "Fv":
		return m.handleSavedViewPrefix(key)
	}

	// Get cursor task (may be nil if cursor is out of bounds)
	var cursorTask *models.Task
	if len(m.rows) > 0 && m.cursor < len(m.rows) {
		cursorTask = m.rows[m.cursor].task
	}

	switch prefix {
	case ",":
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
			m.statusMessage = result.StatusMessage
			m.rebuildRows()
			return m, result.Cmd
		}

	case "t":
		// Try shared message-only handlers first (to, tc, ta, tl, td, tm)
		if cursorTask != nil {
			if handled, cmd := handleTaskPrefixMsgKey(key, cursorTask, m.project); handled {
				return m, cmd
			}
		}

		// Try shared action handlers (tn, tN, ts, tt)
		var focusedTask *models.Task
		if len(m.zoomStack) > 0 {
			topID := m.zoomStack[len(m.zoomStack)-1].TaskID
			focusedTask = m.project.GetTaskByID(topID)
		}
		getMultiTasks := func() []*models.Task {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				return m.getSelectedTasks()
			}
			return nil
		}
		if handled, cmd := handleTaskActionPrefixKey(key, cursorTask, m.project, focusedTask, getMultiTasks); handled {
			return m, cmd
		}

		// Rename task (tr)
		if key == "r" && cursorTask != nil {
			return m, m.quickEdit.Enter(cursorTask, true)
		}

		// Toggle pinned (tp)
		if key == "p" && cursorTask != nil {
			m.setUndoContext("Toggle Pin", cursorTask.Title)
			result := applyTogglePinned(m.workflowPersistFn(), m.project, cursorTask.ID)
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.rebuildRows()
			return m, result.Cmd
		}

		// Delete task (tD)
		if key == "D" {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks? (y/n)", len(m.selectedTasks)))
				m.statusMessage = m.confirm.Prompt()
				return m, nil
			}
			if cursorTask != nil {
				if cursorTask.IsRecurring() {
					m.confirm.Show("delete-recurring",
						fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
							cursorTask.ShortID, cursorTask.Title))
				} else {
					m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", cursorTask.ShortID))
				}
				m.statusMessage = m.confirm.Prompt()
				m.deleteTarget = cursorTask
			}
			return m, nil
		}

	case "p":
		// Perspective switching (p-prefix: p1-p9)
		if key >= "1" && key <= "9" {
			perspKey := int(key[0] - '0')
			m.activePerspective = perspKey
			m.rebuildRows()
			return m, nil
		}

	case "!":
		if priority, ok := PriorityForPrefixKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				tasks := m.getSelectedTasks()
				count := applyBulkPriority(m.project, tasks, priority)
				m.setUndoContext("Set Priority", fmt.Sprintf("P%d on %d tasks", priority, count))
				m.clearMultiSelect()
				m.rebuildRows()
				return m, m.saveWithWorkflow()
			}
			if cursorTask == nil {
				return m, nil
			}
			m.setUndoContext("Set Priority", fmt.Sprintf("P%d: %s", priority, cursorTask.Title))
			result := applyPriorityChange(m.workflowPersistFn(), m.project, cursorTask.ID, priority)
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.rebuildRows()
			return m, result.Cmd
		}

	case "d":
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				tasks := m.getSelectedTasks()
				dueDate, desc := dueDateFromDays(days, isClear)
				count := applyBulkDueDate(m.project, tasks, dueDate)
				m.setUndoContext("Set Due Date", fmt.Sprintf("%s on %d tasks", desc, count))
				m.clearMultiSelect()
				m.rebuildRows()
				return m, m.saveWithWorkflow()
			}
			if cursorTask == nil {
				return m, nil
			}
			var result TaskOpResult
			if isClear {
				m.setUndoContext("Set Due Date", fmt.Sprintf("cleared: %s", cursorTask.Title))
				result = applyClearDueDate(m.workflowPersistFn(), m.project, cursorTask.ID)
			} else {
				m.setUndoContext("Set Due Date", fmt.Sprintf("%s: %s", dueDateLabel(days), cursorTask.Title))
				result = applyQuickDueDate(m.workflowPersistFn(), m.project, cursorTask.ID, days)
			}
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.rebuildRows()
			return m, result.Cmd
		}
		if key == "d" {
			if cursorTask != nil {
				task := cursorTask
				project := m.project
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: task, Project: project}
				}
			}
			return m, nil
		}

	case "s":
		// Snooze quick actions
		if key == "d" {
			if cursorTask != nil {
				task := cursorTask
				project := m.project
				return m, func() tea.Msg {
					return SetSnoozeDateMsg{Task: task, Project: project}
				}
			}
			return m, nil
		}

		snoozeDate, desc, ok := resolveQuickSnoozeDate(key)
		if !ok {
			return m, nil
		}

		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := 0
			for id := range m.selectedTasks {
				if m.project.MutateTask(id, func(t *models.Task) {
					t.SnoozedUntil = snoozeDate
				}) {
					count++
				}
			}
			m.setUndoContext("Snooze", fmt.Sprintf("%s on %d tasks", desc, count))
			m.clearMultiSelect()
			m.rebuildRows()
			return m, m.saveWithWorkflow()
		}
		if cursorTask != nil {
			m.project.MutateTask(cursorTask.ID, func(t *models.Task) {
				t.SnoozedUntil = snoozeDate
			})
			m.setUndoContext("Snooze", fmt.Sprintf("%s: %s", desc, cursorTask.Title))
			m.rebuildRows()
			return m, m.saveWithWorkflow()
		}
		return m, nil

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
			m.ensureExpandedNodes()
			m.collapseAllParents(m.project.GetRootTasks())
			m.rebuildRows()
			return m, nil
		case "v":
			// Zoom into current task (reuse existing zoom)
			if cursorTask != nil && len(cursorTask.Children) > 0 {
				m.zoomStack = append(m.zoomStack, ZoomEntry{
					TaskID:      cursorTask.ID,
					CursorIndex: m.cursor,
				})
				m.cursor = 0
			}
			return m, nil
		}

	case "F":
		// Filter prefix
		return m.handleFilterPrefix(key)

	case "m":
		// Move to section
		if section, ok := sectionForMoveKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				count := 0
				for id := range m.selectedTasks {
					if m.project.MutateTask(id, func(t *models.Task) {
						t.Section = section
						if section == "" {
							t.Activate()
						}
					}) {
						m.project.RelocateTaskToEndOfSection(id, section)
						count++
					}
				}
				label := "Move to " + section
				if section == "" {
					label = "Move to Active"
				}
				m.setUndoContext(label, fmt.Sprintf("%d tasks", count))
				m.clearMultiSelect()
				m.rebuildRows()
				return m, m.saveWithWorkflow()
			}
			if cursorTask == nil {
				return m, nil
			}
			m.project.MutateTask(cursorTask.ID, func(t *models.Task) {
				t.Section = section
				if section == "" {
					t.Activate()
				}
			})
			m.project.RelocateTaskToEndOfSection(cursorTask.ID, section)
			label := "Move to " + section
			if section == "" {
				label = "Move to Active"
			}
			m.setUndoContext(label, cursorTask.Title)
			m.rebuildRows()
			return m, m.saveWithWorkflow()
		}
		// Move to top/bottom
		if key == "K" || key == "J" {
			if cursorTask == nil {
				return m, nil
			}
			return m.moveTaskToPosition(cursorTask, key == "K")
		}

	case "b":
		if cursorTask == nil {
			return m, nil
		}
		if handled, cmd := handleBlockerPrefixKey(key, cursorTask, m.project); handled {
			return m, cmd
		}

	case "c":
		if cursorTask == nil {
			return m, nil
		}
		if handled, cmd := handleLinkedTaskPrefixKey(key, cursorTask, m.project, true, true); handled {
			return m, cmd
		}

	case "g":
		switch key {
		case "g":
			m.cursor = 0
			return m, nil
		case "h":
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
		}
	}

	// Invalid prefix sequence - silently ignore
	return m, nil
}

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *OutlinerModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific filter keys
	switch key {
	case "v":
		m.pendingPrefix = "Fv"
		return m, nil
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "c":
		if m.activePerspective == 8 {
			m.activePerspective = 0
		} else {
			m.activePerspective = 8
		}
		m.rebuildRows()
		return m, nil
	case "@":
		return m, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{Project: m.project}
		}
	case "r":
		m.actionableFilter = !m.actionableFilter
		m.rebuildRows()
		return m, nil
	case "x":
		clearCommonFilters(&m.filter)
		m.actionableFilter = false
		m.ClearSearch()
		m.rebuildRows()
		return m, nil
	}

	// Common filter prefix keys
	result := handleCommonFilterPrefix(key, &m.filter, func() tea.Cmd {
		tags := BuildTagList(m.project.Tasks, false)
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

// handleSavedViewPrefix handles the third key after pressing Fv (saved views sub-prefix).
func (m *OutlinerModel) handleSavedViewPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		return m, func() tea.Msg {
			return ShowSaveViewDialogMsg{}
		}
	case "l":
		return m, func() tea.Msg {
			return ShowCommandPaletteMsg{PreFilter: ":sv "}
		}
	case "d":
		return m, func() tea.Msg {
			return ShowCommandPaletteMsg{PreFilter: ":sv "}
		}
	}
	return m, nil
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
