package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// currentViewAsModel returns the current view as a tea.Model for interface assertions.
func (m *AppModel) currentViewAsModel() tea.Model {
	return m.activeView()
}

type taskReferenceProvider interface {
	getTaskRef() (*models.Task, *models.Project)
}

var (
	createReferenceSnapshotFile = external.CreateReferenceSnapshotFile
	openReferenceFile           = external.OpenReferenceFile
)

func (m *AppModel) handleGlobalKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) { //nolint:gocyclo
	key := msg.String()
	if key == "ctrl+c" {
		m.exitAskView()
		m.saveProjectViewState()
		m.saveOutlinerFilterState()
		m.quitting = true
		return true, tea.Quit
	}
	if isCtrlEscKey(msg) {
		return true, m.jumpToOutlinerRoot()
	}
	// Esc is the canonical close/back operation. App-owned overlays and
	// confirmations receive it before routed controls; a routed control may
	// still capture Esc for its own inline state when no shell overlay is open.
	if key == "esc" {
		if m.pendingSandboxReset {
			m.pendingSandboxReset = false
			m.pendingSandboxProjectID = ""
			m.pendingSandboxTaskCount = 0
			m.setInfoStatus("Canceled")
			return true, defaultStatusTimeout()
		}
		if handled, cmd := m.handleProjectArchiveConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleTaskDeleteConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
			return true, cmd
		}
		if handled, cmd := m.handleEscKey(); handled {
			return true, cmd
		}
	}

	// Handle pending sandbox reset confirmation before clearing status
	if m.pendingSandboxReset {
		switch key {
		case "y", "Y":
			return true, m.executeSandboxReset()
		case "n", "N", "esc":
			m.pendingSandboxReset = false
			m.pendingSandboxProjectID = ""
			m.pendingSandboxTaskCount = 0
			m.setInfoStatus("Canceled")
			return true, defaultStatusTimeout()
		default:
			// Keep showing the confirmation prompt
			return true, nil
		}
	}

	if handled, cmd := m.handleProjectArchiveConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleTaskDeleteConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
		return true, cmd
	}

	// Intercept keys when peek overlay is active: handle scroll, pane focus,
	// and dismiss; block everything else so the underlying view is frozen.
	if m.peekActive {
		switch key {
		case "esc", "space":
			m.dismissPeek()
		case "tab", "shift+tab":
			m.peekFocusRight = !m.peekFocusRight
		case "up", "k":
			if m.peekFocusRight {
				m.peekRightOffset = clampPeekOffset(m.peekRightOffset-1, m.peekRightLines, m.peekPaneHeight)
			} else {
				m.peekLeftOffset = clampPeekOffset(m.peekLeftOffset-1, m.peekLeftLines, m.peekPaneHeight)
			}
		case "down", "j":
			if m.peekFocusRight {
				m.peekRightOffset = clampPeekOffset(m.peekRightOffset+1, m.peekRightLines, m.peekPaneHeight)
			} else {
				m.peekLeftOffset = clampPeekOffset(m.peekLeftOffset+1, m.peekLeftLines, m.peekPaneHeight)
			}
		}
		return true, nil
	}

	// Guard against stale currentProject (e.g., deleted by external CLI command)
	if handled, cmd := m.handleInvalidManageProjectScope(); handled {
		return true, cmd
	}
	if handled, cmd := m.validateCurrentProject(); handled {
		return true, tea.Batch(cmd, defaultStatusTimeout())
	}
	view := m.currentView()
	if key == ":" &&
		view != ViewCrossProjectOutliner &&
		!m.hasActiveTextInput() &&
		!m.hasActiveConfirmDialog() {
		return true, func() tea.Msg { return ShowFinderMsg{} }
	}
	if handled, cmd := m.handleSimplifiedInputKey(msg); handled {
		return true, cmd
	}
	return false, nil
}

// performGlobalTaskUndoRedoForProject applies the shell-owned ledger-vs-legacy
// restore selection for the cursor project. Operation handlers share this seam
// so action-sheet undo/redo cannot drift from the keyboard behavior.
//
// Application-service mutations commit to the durable workspace ledger, while a
// few remaining paths (task archive) still record legacy per-project snapshots.
// Restore through whichever backend holds the newer transaction so a committed
// application mutation is always reachable and a stale legacy entry never
// shadows it.
func (m *AppModel) performGlobalTaskUndoRedoForProject(project *models.Project, redo bool) tea.Cmd {
	if project == nil {
		if m.workspaceLedgerRestoreIsNewest(nil, redo) {
			return m.handleLedgerTaskUndoRedo(redo)
		}
		return nil
	}
	if m.workspaceLedgerRestoreIsNewest(project, redo) {
		return m.handleLedgerTaskUndoRedo(redo)
	}
	if redo {
		return performRedoForProjectsCmd(m.persister, project, activeUndoProjectsFromRuntime(m.runtime), m.repairTaskProjectRefsAfterRestore)
	}
	return performUndoForProjectsCmd(m.persister, project, activeUndoProjectsFromRuntime(m.runtime), m.repairTaskProjectRefsAfterRestore)
}

func (m *AppModel) currentViewSupportsGlobalTaskUndoRedo() bool {
	if m.activeTaskDetail() {
		return true
	}
	if m.activeReviewHub() {
		return m.reviewHub.SupportsTaskUndoRedo()
	}
	switch m.currentView() { //nolint:exhaustive
	case ViewDailyReview,
		ViewWeeklyReview,
		ViewRecentActivity,
		ViewTaskDetail:
		return true
	default:
		return false
	}
}

func (m *AppModel) handleProjectArchiveConfirmKey(key string) (bool, tea.Cmd) {
	if !m.projectArchiveConfirm.IsActive() {
		return false, nil
	}

	confirmed, handled := m.projectArchiveConfirm.HandleKey(key)
	if !handled {
		return false, nil
	}
	if confirmed {
		return true, func() tea.Msg { return projectArchiveConfirmedMsg{} }
	}
	switch key {
	case "n", "N", "esc":
		m.clearPendingProjectArchive()
	}
	return true, nil
}

func (m *AppModel) handleActiveKeyOverlay(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.operationValuePicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.operationValuePicker, msg)
		if dismiss {
			// The result message decides whether the sheet beneath should also
			// close; cancellation returns to it.
			m.operationValuePicker = nil
		}
		return true, cmd
	}
	if m.taskActionSheet != nil {
		dismiss, cmd := dispatchOverlayKey(m.taskActionSheet, msg)
		if dismiss {
			m.taskActionSheet = nil
		}
		return true, cmd
	}
	if m.quickNoteOverlay != nil {
		dismiss, cmd := dispatchOverlayKey(m.quickNoteOverlay, msg)
		if dismiss {
			m.quickNoteOverlay = nil
		}
		return true, cmd
	}
	if m.projectAdminMenu != nil {
		dismiss, cmd := dispatchOverlayKey(m.projectAdminMenu, msg)
		if dismiss {
			m.projectAdminMenu = nil
		}
		return true, cmd
	}
	return false, nil
}
