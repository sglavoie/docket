package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// handleViewCoreMessages handles generic view navigation, status, and utility messages.
func (m *AppModel) handleViewCoreMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case status.BackMsg:
		return true, m.popView()

	case status.ErrorMsg:
		m.err = msg.Err
		return true, tea.Batch(m.delegateToCurrentView(msg), status.StatusTimeout(5*time.Second))

	case help.ShowHelpMsg:
		m.helpView = help.NewHelpViewModel(msg.Items, msg.ViewName)
		m.showView(presentation.ViewHelp, m.helpView)
		return true, nil

	case status.StatusClearMsg:
		if !msg.ScheduledAt.IsZero() && !m.statusUpdatedAt.Before(msg.ScheduledAt) {
			return true, nil
		}
		// A 2s plain-status tick may arrive after a newer setUndoableStatus
		// has installed a still-live toast (6s window). Don't dismiss a toast
		// that hasn't reached its own deadline yet — its dedicated
		// UndoToastTimeoutMsg will clear it on time.
		if m.statusUndoCb != nil && time.Now().Before(m.statusExpiresAt) {
			return true, nil
		}
		m.clearStatus()
		return true, nil

	case status.ViewStatusClearMsg:
		if m.currentView() != msg.View {
			return true, nil
		}
		if owner, ok := m.activeView().(viewStatusOwner); ok && owner.StatusStamp() == msg.Seq {
			owner.ClearStatus()
		}
		return true, nil

	case FooterFlashTickMsg:
		return true, m.advanceFooterFlash(msg.Seq)

	case status.UndoToastTimeoutMsg:
		// Only clear if the deadline that fired this tick is still the
		// active one. A newer toast (with later expiresAt) may have
		// replaced this one — leave its callback alone.
		if m.statusUndoCb != nil && !time.Now().Before(m.statusExpiresAt) {
			m.clearStatus()
		}
		return true, nil

	case status.WorkspaceUndoMsg:
		// The panels reach the workspace ledger through the same entry point the
		// Outliner's u and Ctrl+R press, so a live undo toast still outranks the
		// ledger and an exhausted ledger stays silent here too.
		if msg.Redo {
			return true, m.coreRedo()
		}
		return true, m.coreUndo()

	case status.StatusMsg:
		m.setSuccessStatus(string(msg))
		return true, status.DefaultStatusTimeout()

	case status.InfoStatusMsg:
		m.setInfoStatus(string(msg))
		return true, status.DefaultStatusTimeout()

	case outliner.OutlinerPlanningReorderMsg:
		return true, m.handleOutlinerPlanningReorder(msg)
	}

	return false, nil
}
