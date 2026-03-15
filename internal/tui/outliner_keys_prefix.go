package tui

import (
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
			m.statusMessage = result.StatusMessage
			m.rebuildRows()
			return m, result.Cmd
		}

	case ",D":
		if cmd, ok := handleDeferRecurrence(key, m); ok {
			return m, cmd
		}

	case "t":
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
			m.activePerspective = perspKey
			m.rebuildRows()
			return m, nil
		}

	case ".":
		if cmd, ok := handleStagePrefix(key, m, m.config.Stages); ok {
			return m, cmd
		}

	case "!":
		if cmd, ok := handlePriorityPrefix(key, m); ok {
			return m, cmd
		}

	case "d":
		if cmd, ok := handleDueDatePrefix(key, m); ok {
			return m, cmd
		}

	case "s":
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
		case "p":
			m.previewMode = !m.previewMode
			return m, nil
		}

	case "F":
		// Filter prefix
		return m.handleFilterPrefix(key)

	case "m":
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
			old := m.cursor
			m.cursor = 0
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
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
		case "z":
			// Jump directly to Snoozed perspective (P0)
			m.activePerspective = PerspectiveSnoozed
			m.rebuildRows()
			return m, nil
		case "l":
			// Go to project recent activity
			project := m.project
			return m, func() tea.Msg {
				return ShowProjectRecentActivityMsg{Project: project}
			}
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
		return m, func() tea.Msg {
			return ShowColumnPickerMsg{Source: ViewOutliner, ColumnConfig: m.columnConfig}
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
