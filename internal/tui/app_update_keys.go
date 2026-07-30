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
		return m.executeSimplifiedGlobalKey(key)
	}
	if isCtrlEscKey(msg) {
		// Ctrl+Esc was the legacy hard reset. It is intentionally inert in the
		// final grammar, including while a subordinate route is open.
		return true, nil
	}
	// Esc is the canonical close/back operation. App-owned overlays and
	// confirmations receive it before routed controls; a routed control may
	// still capture Esc for its own inline state when no shell overlay is open.
	if key == "esc" {
		if handled, cmd := m.handleProjectArchiveConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleTaskDeleteConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleOperationConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
			return true, cmd
		}
		if handled, cmd := m.handleEscKey(); handled {
			return true, cmd
		}
	}

	if handled, cmd := m.handleProjectArchiveConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleTaskDeleteConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleOperationConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
		return true, cmd
	}

	// A narrow inspector is a frozen snapshot with no keymap of its own.
	// Esc is handled by the canonical overlay-close cascade above; every other
	// key is swallowed so the covered Outliner cannot move or mutate.
	if m.peekActive {
		return true, nil
	}

	// Guard against stale currentProject (e.g., deleted by external CLI command)
	if handled, cmd := m.handleInvalidManageProjectScope(); handled {
		return true, cmd
	}
	if handled, cmd := m.validateCurrentProject(); handled {
		return true, tea.Batch(cmd, defaultStatusTimeout())
	}
	if handled, cmd := m.handleSimplifiedInputKey(msg); handled {
		return true, cmd
	}
	return false, nil
}

// performGlobalTaskUndoRedoForProject applies the single workspace-wide history.
// The project argument remains part of the operation-handler seam until the
// surrounding task-target adapters are simplified.
func (m *AppModel) performGlobalTaskUndoRedoForProject(_ *models.Project, redo bool) tea.Cmd {
	if m.workspaceLedgerRestoreIsNewest(redo) {
		return m.handleLedgerTaskUndoRedo(redo)
	}
	return nil
}

func (m *AppModel) currentViewSupportsGlobalTaskUndoRedo() bool {
	if m.activeTaskDetail() {
		return true
	}
	switch m.currentView() { //nolint:exhaustive
	case ViewTaskDetail:
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
	if m.operationTextPrompt != nil {
		dismiss, cmd := dispatchOverlayKey(m.operationTextPrompt, msg)
		if dismiss {
			m.operationTextPrompt = nil
		}
		return true, cmd
	}
	if m.operationValuePicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.operationValuePicker, msg)
		if dismiss {
			// The result message decides whether the sheet beneath should also
			// close; cancellation returns to it.
			m.operationValuePicker = nil
		}
		return true, cmd
	}
	if m.contextActionSheet != nil {
		dismiss, cmd := dispatchOverlayKey(m.contextActionSheet, msg)
		if dismiss {
			m.contextActionSheet = nil
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
