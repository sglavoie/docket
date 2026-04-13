package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// UndoHistoryModel displays the undo/redo history for a project.
type UndoHistoryModel struct {
	BaseView
	InteractiveViewState
	project     *models.Project
	entries     []storage.UndoEntry // undo entries, most-recent-first
	redoEntries []storage.UndoEntry // redo entries, most-recent-first

	// Diff preview
	diffLines   []DiffLine
	diffScroll  int
	diffCursor  int // Currently selected diff line when diff pane focused
	diffHScroll int // Horizontal scroll offset for diff pane

	// Detail view
	showingDetail bool
	detailLine    DiffLine // The line being shown in detail

	// Layout
	focusedPane int // 0 = entry list, 1 = diff preview
}

// NewUndoHistoryModel creates a new undo history model.
func NewUndoHistoryModel(project *models.Project) *UndoHistoryModel {
	entries, err := storage.LoadUndoHistory(project)
	if err != nil {
		m := &UndoHistoryModel{
			project:     project,
			entries:     []storage.UndoEntry{},
			redoEntries: []storage.UndoEntry{},
		}
		m.SetError(fmt.Errorf("failed to load undo history: %w", err))
		return m
	}

	redoEntries, err := storage.LoadRedoHistory(project)
	if err != nil {
		redoEntries = []storage.UndoEntry{}
	}

	m := &UndoHistoryModel{
		project:     project,
		entries:     entries,
		redoEntries: redoEntries,
	}
	m.BaseView.SetSize(80, 24)

	// Start cursor on the most recent actionable entry.
	// If undo entries exist, select the most recent undo (first after redo section).
	// If only redo entries exist, select the most recent redo (index 0).
	if len(entries) > 0 {
		m.cursor = len(redoEntries) // first undo entry
	} else if len(redoEntries) > 0 {
		m.cursor = 0 // most recent redo entry
	}

	// Compute initial diff if there are any entries
	if m.totalEntries() > 0 {
		m.computeDiff()
	}

	return m
}

// totalEntries returns the total number of selectable entries (redo + undo).
func (m *UndoHistoryModel) totalEntries() int {
	return len(m.redoEntries) + len(m.entries)
}

// isOnRedoSide returns true when the cursor is on a redo entry.
func (m *UndoHistoryModel) isOnRedoSide() bool {
	return m.cursor < len(m.redoEntries)
}

// redoIndex returns the index into redoEntries for the current cursor (only valid when isOnRedoSide).
func (m *UndoHistoryModel) redoIndex() int {
	return m.cursor
}

// undoIndex returns the index into entries for the current cursor (only valid when !isOnRedoSide).
func (m *UndoHistoryModel) undoIndex() int {
	return m.cursor - len(m.redoEntries)
}

// SetSize sets the view dimensions.
func (m *UndoHistoryModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
}

func (m *UndoHistoryModel) Init() tea.Cmd {
	return nil
}

// ShouldCaptureEsc returns true when detail view is active or a pending prefix is set.
func (m *UndoHistoryModel) ShouldCaptureEsc() bool {
	return m.showingDetail || m.DefaultShouldCaptureEsc()
}

// ensureDiffCursorVisible adjusts diffScroll to keep the cursor visible.
func (m *UndoHistoryModel) ensureDiffCursorVisible() {
	availableHeight := max(m.Height()-TabbedViewChrome, 1)
	visibleLines := max(availableHeight-5, 1)

	// If cursor is above visible area, scroll up
	if m.diffCursor < m.diffScroll {
		m.diffScroll = m.diffCursor
	}
	// If cursor is below visible area, scroll down
	if m.diffCursor >= m.diffScroll+visibleLines {
		m.diffScroll = m.diffCursor - visibleLines + 1
	}
}

// computeDiff computes the diff for the currently selected entry.
func (m *UndoHistoryModel) computeDiff() {
	if m.totalEntries() == 0 || m.cursor >= m.totalEntries() {
		m.diffLines = nil
		return
	}

	currentContent, err := storage.GetCurrentContent(m.project)
	if err != nil {
		m.diffLines = []DiffLine{{Kind: DiffContext, Content: fmt.Sprintf("Error reading current: %v", err)}}
		return
	}

	if m.isOnRedoSide() {
		// Redo side: show current → redo snapshot (what applying redo would restore)
		entry := m.redoEntries[m.redoIndex()]
		redoContent, err := storage.GetRedoSnapshotContent(m.project, entry.Num)
		if err != nil {
			m.diffLines = []DiffLine{{Kind: DiffContext, Content: fmt.Sprintf("Error reading redo snapshot: %v", err)}}
			return
		}
		m.diffLines = ComputeContextualDiff(currentContent, redoContent, 3)
	} else {
		// Undo side: show snapshot → current (what undoing would restore)
		entry := m.entries[m.undoIndex()]
		snapshotContent, err := storage.GetSnapshotContent(m.project, entry.Num)
		if err != nil {
			m.diffLines = []DiffLine{{Kind: DiffContext, Content: fmt.Sprintf("Error reading snapshot: %v", err)}}
			return
		}
		m.diffLines = ComputeContextualDiff(snapshotContent, currentContent, 3)
	}

	// Filter to show only changes if too many lines
	if len(m.diffLines) > 100 {
		m.diffLines = FilterDiffToChanges(m.diffLines)
	}
}

// performUndo performs the undo operation.
func (m *UndoHistoryModel) performUndo() tea.Cmd {
	project := m.project
	return func() tea.Msg {
		actionDesc, err := storage.PerformUndo(project)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("undo failed: %w", err)}
		}

		if err := storage.LoadProject(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reload after undo: %w", err)}
		}

		if err := reapplyStorageSideEffects(project); err != nil {
			return ErrorMsg{Err: err}
		}

		return UndoPerformedMsg{Action: actionDesc}
	}
}

// performRedo performs the redo operation.
func (m *UndoHistoryModel) performRedo() tea.Cmd {
	project := m.project
	return func() tea.Msg {
		actionDesc, err := storage.PerformRedo(project)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("redo failed: %w", err)}
		}

		if err := storage.LoadProject(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reload after redo: %w", err)}
		}

		if err := reapplyStorageSideEffects(project); err != nil {
			return ErrorMsg{Err: err}
		}

		return RedoPerformedMsg{Action: actionDesc}
	}
}

// reloadEntries reloads both undo and redo entries from disk and recomputes the diff.
func (m *UndoHistoryModel) reloadEntries() {
	entries, err := storage.LoadUndoHistory(m.project)
	if err != nil {
		entries = []storage.UndoEntry{}
	}

	redoEntries, err := storage.LoadRedoHistory(m.project)
	if err != nil {
		redoEntries = []storage.UndoEntry{}
	}

	m.entries = entries
	m.redoEntries = redoEntries

	// Clamp cursor to valid range, preferring first undo entry
	total := m.totalEntries()
	if total == 0 {
		m.cursor = 0
	} else if m.cursor >= total {
		m.cursor = total - 1
	}

	m.computeDiff()
	m.resetDiffSelection(true)
}

func (m *UndoHistoryModel) StatusBarLabel() string { return "Undo / Redo Stack" }
