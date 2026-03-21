package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// handleWithRefresh consolidates the common handler pattern of:
// pop view → reload project → refresh tree → set status message.
//
// Parameters:
//   - popView: whether to pop the current view from the stack
//   - reload: whether to reload the current project from storage
//   - statusMsg: optional status message to display (empty string skips)
//
// Returns a tea.Cmd for status timeout if statusMsg is provided, nil otherwise.
func (m *AppModel) handleWithRefresh(popView, reload bool, statusMsg string) tea.Cmd {
	if popView {
		m.popView()
	}
	if reload && m.currentProject != nil {
		_ = m.persister.Load(m.currentProject)
	}
	m.refreshProjectTree()
	if m.outliner != nil {
		m.outliner.rebuildRows()
	}
	if statusMsg != "" {
		m.setSuccessStatus(statusMsg)
		return defaultStatusTimeout()
	}
	return nil
}

// setSuccessStatus sets a status message with success styling (green checkmark).
func (m *AppModel) setSuccessStatus(msg string) {
	m.statusMessage = msg
	m.statusIsInfo = false
	m.messageLog.Append(msg, MessageLogLevelSuccess)
}

// setInfoStatus sets a status message with informational styling (neutral color, no checkmark).
func (m *AppModel) setInfoStatus(msg string) {
	m.statusMessage = msg
	m.statusIsInfo = true
	m.messageLog.Append(msg, MessageLogLevelInfo)
}

// clearStatus clears the status message and resets the info flag.
func (m *AppModel) clearStatus() {
	m.statusMessage = ""
	m.statusIsInfo = false
}

// saveConfigWarn attempts to save the config and logs a warning on failure.
// Use this in TUI code where a config save failure is non-fatal.
func saveConfigWarn(mgr *config.Manager, cfg *config.Config, context string) {
	if err := mgr.Save(cfg); err != nil {
		logging.Warn("failed to save config (%s): %v", context, err)
	}
}

// syncInboxProcessor updates the typed inboxProcessor pointer after view updates.
// This keeps the pointer in sync when the inbox processor view is active.
func (m *AppModel) syncInboxProcessor() {
	if m.currentView() == ViewInboxProcessor {
		if v := m.views.Get(ViewInboxProcessor); v != nil {
			m.inboxProcessor = v.(*InboxProcessorModel)
		}
	}
}
