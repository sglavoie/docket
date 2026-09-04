package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/status"
)

// currentViewAsModel returns the current view as a tea.Model for interface assertions.
func (m *AppModel) currentViewAsModel() tea.Model {
	return m.activeView()
}

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
		if handled, cmd := m.handleProjectDeleteConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleProjectParkConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleTaskDeleteConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleOutlineDeleteConfirmKey(key); handled {
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

	if handled, cmd := m.handleProjectDeleteConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleProjectParkConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleTaskDeleteConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleOutlineDeleteConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleOperationConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
		return true, cmd
	}

	// Guard against stale currentProject (e.g., deleted by external CLI command)
	if handled, cmd := m.handleInvalidManageProjectScope(); handled {
		return true, cmd
	}
	if handled, cmd := m.validateCurrentProject(); handled {
		return true, tea.Batch(cmd, status.DefaultStatusTimeout())
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

func (m *AppModel) handleProjectArchiveConfirmKey(key string) (bool, tea.Cmd) {
	return projectConfirmKey(m.project.resolveArchiveKey(key), projecthub.ProjectArchiveConfirmedMsg{})
}

func (m *AppModel) handleProjectDeleteConfirmKey(key string) (bool, tea.Cmd) {
	return projectConfirmKey(m.project.resolveDeleteKey(key), projecthub.ProjectDeleteConfirmedMsg{})
}

func (m *AppModel) handleProjectParkConfirmKey(key string) (bool, tea.Cmd) {
	return projectConfirmKey(m.project.resolveParkKey(key), projecthub.ProjectParkConfirmedMsg{})
}

func (m *AppModel) handleActiveKeyOverlay(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.operations.textPrompt != nil {
		dismiss, cmd := dispatchOverlayKey(m.operations.textPrompt, msg)
		if dismiss {
			m.operations.textPrompt = nil
		}
		return true, cmd
	}
	if m.operations.valuePicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.operations.valuePicker, msg)
		if dismiss {
			// The result message decides whether the sheet beneath should also
			// close; cancellation returns to it.
			m.operations.valuePicker = nil
		}
		return true, cmd
	}
	if m.operations.actionSheet != nil {
		dismiss, cmd := dispatchOverlayKey(m.operations.actionSheet, msg)
		if dismiss {
			m.operations.actionSheet = nil
		}
		return true, cmd
	}
	if m.lensPicker != nil {
		// Last, because every overlay above nests over the picker rather than
		// replacing it: the picker only sees a key once nothing is on top.
		dismiss, cmd := dispatchOverlayKey(m.lensPicker, msg)
		if dismiss {
			m.lensPicker = nil
		}
		return true, cmd
	}
	if m.conflictBrowser != nil {
		// Nothing ever opens above the browser, so its position among the arms is
		// free; it is last because it is the only overlay that cannot be reached
		// from any of the others.
		dismiss, cmd := dispatchOverlayKey(m.conflictBrowser, msg)
		if dismiss {
			m.conflictBrowser = nil
		}
		return true, cmd
	}
	return false, nil
}
