package tui

import (
	tea "charm.land/bubbletea/v2"
)

// kanbanFlashClearMsg is sent after the flash-highlight timer expires.
type kanbanFlashClearMsg struct{}

// Update satisfies the tea.Model interface.
func (m *KanbanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustHorizontalScroll()
		m.clampCursorAllCols()
		return m, nil
	case kanbanFlashClearMsg:
		m.flashCardID = ""
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKanbanKey(msg)
	}
	return m, nil
}

// handleKanbanKey dispatches key press events for the kanban view.
//
//nolint:gocyclo // key dispatch hub
func (m *KanbanModel) handleKanbanKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.showAxisManager {
		return m.handleAxisManagerKey(msg)
	}

	key := msg.String()

	// Exit multi-select mode on Esc.
	if key == "esc" && m.multiSelectMode {
		m.ExitMultiSelect()
		m.statusMsg = ""
		return m, nil
	}

	// ── Pending prefix second-key ──────────────────────────────────────────
	if m.pendingPrefix != "" {
		return m.handleKanbanPrefixKey(key)
	}

	// ── Navigation ────────────────────────────────────────────────────────
	switch key {
	case "j", "down":
		m.moveCursorDown()
		return m, nil
	case "k", "up":
		m.moveCursorUp()
		return m, nil
	case "h":
		m.moveFocusPrevCol()
		return m, nil
	case "l":
		m.moveFocusNextCol()
		return m, nil
	case "ctrl+u":
		m.pageUp()
		return m, nil
	case "ctrl+d":
		m.pageDown()
		return m, nil
	case "G":
		m.jumpToBottom()
		return m, nil
	case "enter":
		if cmd := m.openTaskDetailCmd(); cmd != nil {
			return m, cmd
		}
		return m, nil
	}

	// ── H/L column move, J/K reorder ─────────────────────────────────────
	switch key {
	case "H":
		return m.moveTaskLeft()
	case "L":
		return m.moveTaskRight()
	case "J":
		m.reorderCardDown()
		return m, nil
	case "K":
		m.reorderCardUp()
		return m, nil
	}

	// ── View-mode keys ────────────────────────────────────────────────────
	switch key {
	case "tab":
		m.detailPaneVisible = !m.detailPaneVisible
		return m, nil
	case "`":
		m.cycleAxisMode()
		return m, nil
	case "~":
		m.cycleAxisModeBack()
		return m, nil
	case "@":
		m.showAxisManager = true
		m.axisCursor = 0
		return m, nil
	case "space":
		if card := m.getCursorCard(); card != nil {
			m.ToggleAndUpdateStatus(card.Task.ID, &m.statusMsg,
				"Multi-select: navigate and press space to select",
				"%d task(s) selected")
		}
		return m, nil
	case "V":
		m.ToggleMode(&m.statusMsg, "Multi-select mode — space: select, H/L: bulk move, esc: exit")
		return m, nil
	case "?":
		vt, vn := ViewKanban, "Kanban"
		if m.crossProject {
			vt, vn = ViewCrossProjectKanban, "Cross-Project Kanban"
		}
		return m, helpCmd(vt, vn, GetKeybindingRegistry().GetForView(vt))
	}

	// ── Direct action keys ────────────────────────────────────────────────
	hasCard := m.getCursorCard() != nil
	switch key {
	case "a":
		// Add task: open quick capture for current project(s).
		return m, func() tea.Msg {
			return ShowQuickCaptureMsg{
				Project:     m.project,
				AllProjects: m.config.GetActiveProjects(),
			}
		}
	case "D":
		return taskOpsDeleteTask(m)
	case "e":
		card := m.getCursorCard()
		if card != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: card.Task, Project: card.Project}
			}
		}
		return m, nil
	case "o":
		card := m.getCursorCard()
		if card != nil {
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: card.Task, Project: card.Project}
			}
		}
		return m, nil
	case "Z":
		card := m.getCursorCard()
		if card != nil {
			return m, func() tea.Msg {
				return SetSnoozeDateMsg{Task: card.Task, Project: card.Project}
			}
		}
		return m, nil
	case "r":
		m.reloadData()
		m.clampCursor()
		m.statusMsg = "Refreshed"
		return m, nil
	case "y", "Y", "ctrl+y":
		card := m.getCursorCard()
		if card != nil {
			if cmd, handled := clipboardCmd(key, card.Task); handled {
				return m, cmd
			}
		}
		return m, nil
	case "*":
		return taskOpsToggleActive(m)
	case "S":
		card := m.getCursorCard()
		if card != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: card.Task, Project: card.Project}
			}
		}
		return m, nil
	case "O":
		card := m.getCursorCard()
		if card != nil && card.Task.URL != "" {
			return m, openBrowserCmd(card.Task.URL)
		}
		return m, nil
	case "ctrl+.":
		card := m.getCursorCard()
		if card != nil {
			return m, func() tea.Msg {
				return OpenQuickNoteMsg{Task: card.Task, Project: card.Project}
			}
		}
		return m, nil
	case "ctrl+b":
		card := m.getCursorCard()
		if card != nil {
			return m, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{Task: card.Task, SourceProject: card.Project}
			}
		}
		return m, nil
	case "M":
		if m.mode == KanbanModeCustomSections {
			card := m.getCursorCard()
			if card != nil {
				projects := m.loadedProjects
				return m, func() tea.Msg {
					return ShowCustomSectionPickerMsg{Task: card.Task, Project: card.Project, Projects: projects}
				}
			}
		}
		return m, nil
	}

	// ── Prefix key activators ─────────────────────────────────────────────
	// "g" and "F" activate unconditionally; others require a card.
	if prefix, ok := activateSecondaryPrefix(key, "gF", ",!dsmct.", hasCard); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	return m, nil
}

// handleKanbanPrefixKey handles the second key in a prefix sequence.
func (m *KanbanModel) handleKanbanPrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

	if key == "esc" {
		return m, nil // Cancel prefix
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, nil); handled {
		return model, cmd
	}

	return m, nil
}

// ─── Scroll/cursor helpers ────────────────────────────────────────────────────

// visibleBodyRows returns the number of card rows the column body can display.
// Mirrors the fixedOverhead constant in kanbanRender.
func (m *KanbanModel) visibleBodyRows() int {
	const fixedOverhead = 4
	rows := m.height - fixedOverhead
	if m.detailPaneVisible {
		rows -= 1 + m.detailPaneHeight()
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

// countDisplayRows returns the total number of display rows for a column,
// including project group headers in cross-project mode.
func (m *KanbanModel) countDisplayRows(colIdx int) int {
	if colIdx < 0 || colIdx >= len(m.columns) {
		return 0
	}
	col := &m.columns[colIdx]
	if !m.crossProject || len(col.Tasks) == 0 {
		return len(col.Tasks)
	}
	count := 0
	lastProj := ""
	for _, card := range col.Tasks {
		projName := ""
		if card.Project != nil {
			projName = card.Project.Name
		}
		if projName != lastProj {
			count++ // header row
			lastProj = projName
		}
		count++
	}
	return count
}

// displayRowForTask returns the display row index for the given task index,
// accounting for project group headers in cross-project mode.
func (m *KanbanModel) displayRowForTask(colIdx, taskIdx int) int {
	if colIdx < 0 || colIdx >= len(m.columns) {
		return taskIdx
	}
	col := &m.columns[colIdx]
	if !m.crossProject || len(col.Tasks) == 0 {
		return taskIdx
	}
	dispRow := 0
	lastProj := ""
	for ti := 0; ti <= taskIdx && ti < len(col.Tasks); ti++ {
		projName := ""
		if col.Tasks[ti].Project != nil {
			projName = col.Tasks[ti].Project.Name
		}
		if projName != lastProj {
			dispRow++ // header row
			lastProj = projName
		}
		if ti < taskIdx {
			dispRow++
		}
	}
	return dispRow
}

// clampScrollToVisible adjusts col.ScrollOffset so that col.Cursor is rendered
// as an actual card (not hidden behind the overflow indicator).
//
// ScrollOffset is in display-row space (includes project group headers in
// cross-project mode). The cursor is in task-index space and is mapped to
// display-row space via displayRowForTask.
func (m *KanbanModel) clampScrollToVisible(colIdx int) {
	if colIdx < 0 || colIdx >= len(m.columns) {
		return
	}
	col := &m.columns[colIdx]
	bodyRows := m.visibleBodyRows()
	totalDispRows := m.countDisplayRows(colIdx)

	// Reserve one row for the potential overflow indicator.
	visibleCards := bodyRows - 1
	if visibleCards < 1 {
		visibleCards = 1
	}

	// Get the display row of the cursor task.
	cursorDispRow := m.displayRowForTask(colIdx, col.Cursor)

	// Cursor above visible area — scroll up.
	if cursorDispRow < col.ScrollOffset {
		col.ScrollOffset = cursorDispRow
	}

	// Cursor below visible area — scroll down.
	if cursorDispRow >= col.ScrollOffset+visibleCards {
		col.ScrollOffset = cursorDispRow - visibleCards + 1
	}

	if col.ScrollOffset < 0 {
		col.ScrollOffset = 0
	}

	// Don't scroll past end.
	maxOff := totalDispRows - bodyRows
	if maxOff < 0 {
		maxOff = 0
	}
	if col.ScrollOffset > maxOff {
		col.ScrollOffset = maxOff
	}
}

// adjustHorizontalScroll ensures colScrollOff keeps activeCol within the
// visible column range. Mirrors the width computation in kanbanRender.
func (m *KanbanModel) adjustHorizontalScroll() {
	numCols := len(m.columns)
	if numCols == 0 || m.width == 0 {
		return
	}

	const minColWidth = 20
	maxVisible := (m.width + 1) / (minColWidth + 1)
	if maxVisible < 1 {
		maxVisible = 1
	}
	numVisible := min(numCols, maxVisible)

	// Active column scrolled past right edge.
	if m.activeCol >= m.colScrollOff+numVisible {
		m.colScrollOff = m.activeCol - numVisible + 1
	}
	// Active column scrolled past left edge.
	if m.activeCol < m.colScrollOff {
		m.colScrollOff = m.activeCol
	}

	// Clamp to valid range.
	if m.colScrollOff < 0 {
		m.colScrollOff = 0
	}
	maxOff := numCols - numVisible
	if maxOff < 0 {
		maxOff = 0
	}
	if m.colScrollOff > maxOff {
		m.colScrollOff = maxOff
	}
}

// moveCursorDown moves the cursor down one card within the active column (j).
func (m *KanbanModel) moveCursorDown() {
	if m.activeCol >= len(m.columns) {
		return
	}
	col := &m.columns[m.activeCol]
	if len(col.Tasks) == 0 {
		return
	}
	if col.Cursor < len(col.Tasks)-1 {
		col.Cursor++
	}
	m.clampScrollToVisible(m.activeCol)
}

// moveCursorUp moves the cursor up one card within the active column (k).
func (m *KanbanModel) moveCursorUp() {
	if m.activeCol >= len(m.columns) {
		return
	}
	col := &m.columns[m.activeCol]
	if col.Cursor > 0 {
		col.Cursor--
	}
	m.clampScrollToVisible(m.activeCol)
}

// moveFocusNextCol moves focus to the next column to the right (l).
func (m *KanbanModel) moveFocusNextCol() {
	if m.activeCol < len(m.columns)-1 {
		m.activeCol++
		m.adjustHorizontalScroll()
	}
}

// moveFocusPrevCol moves focus to the previous column to the left (h).
func (m *KanbanModel) moveFocusPrevCol() {
	if m.activeCol > 0 {
		m.activeCol--
		m.adjustHorizontalScroll()
	}
}

// pageDown moves the cursor down by half a page within the active column (ctrl+d).
func (m *KanbanModel) pageDown() {
	if m.activeCol >= len(m.columns) {
		return
	}
	col := &m.columns[m.activeCol]
	if len(col.Tasks) == 0 {
		return
	}
	half := m.visibleBodyRows() / 2
	if half < 1 {
		half = 1
	}
	col.Cursor += half
	if col.Cursor >= len(col.Tasks) {
		col.Cursor = len(col.Tasks) - 1
	}
	m.clampScrollToVisible(m.activeCol)
}

// pageUp moves the cursor up by half a page within the active column (ctrl+u).
func (m *KanbanModel) pageUp() {
	if m.activeCol >= len(m.columns) {
		return
	}
	col := &m.columns[m.activeCol]
	if len(col.Tasks) == 0 {
		return
	}
	half := m.visibleBodyRows() / 2
	if half < 1 {
		half = 1
	}
	col.Cursor -= half
	if col.Cursor < 0 {
		col.Cursor = 0
	}
	m.clampScrollToVisible(m.activeCol)
}

// jumpToBottom moves the cursor to the last card in the active column (G).
func (m *KanbanModel) jumpToBottom() {
	if m.activeCol >= len(m.columns) {
		return
	}
	col := &m.columns[m.activeCol]
	if len(col.Tasks) == 0 {
		return
	}
	col.Cursor = len(col.Tasks) - 1
	m.clampScrollToVisible(m.activeCol)
}

// openTaskDetailCmd returns a command that sends ShowTaskDetailMsg for the
// currently focused card. Returns nil if no card is focused.
func (m *KanbanModel) openTaskDetailCmd() tea.Cmd {
	card := m.getCursorCard()
	if card == nil {
		return nil
	}
	task := card.Task
	project := card.Project
	return func() tea.Msg {
		return ShowTaskDetailMsg{Task: task, Project: project}
	}
}
