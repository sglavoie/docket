package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// UndoHistoryModel displays the undo history for a project.
type UndoHistoryModel struct {
	BaseView
	InteractiveViewState
	project *models.Project
	entries []storage.UndoEntry

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
			project: project,
			entries: []storage.UndoEntry{},
		}
		m.SetError(fmt.Errorf("failed to load undo history: %w", err))
		return m
	}

	m := &UndoHistoryModel{
		project: project,
		entries: entries,
	}
	m.BaseView.SetSize(80, 24)

	// Compute initial diff if there are entries
	if len(entries) > 0 {
		m.computeDiff()
	}

	return m
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
	if len(m.entries) == 0 || m.cursor >= len(m.entries) {
		m.diffLines = nil
		return
	}

	entry := m.entries[m.cursor]

	// Get current content
	currentContent, err := storage.GetCurrentContent(m.project)
	if err != nil {
		m.diffLines = []DiffLine{{Kind: DiffContext, Content: fmt.Sprintf("Error reading current: %v", err)}}
		return
	}

	// Get snapshot content
	snapshotContent, err := storage.GetSnapshotContent(m.project, entry.Num)
	if err != nil {
		m.diffLines = []DiffLine{{Kind: DiffContext, Content: fmt.Sprintf("Error reading snapshot: %v", err)}}
		return
	}

	// Compute diff (show what changed: snapshot → current, i.e., old → new)
	m.diffLines = ComputeContextualDiff(snapshotContent, currentContent, 3)

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

		// Reload project
		if err := storage.LoadProject(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reload after undo: %w", err)}
		}

		return UndoPerformedMsg{Action: actionDesc}
	}
}

func (m *UndoHistoryModel) StatusBarLabel() string { return "Undo History" }
