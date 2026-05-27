package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// handleViewCoreMessages handles generic view navigation, status, and utility messages.
func (m *AppModel) handleViewCoreMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case BackMsg:
		m.popView()
		return true, nil

	case RefreshMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ErrorMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.err = msg.Err
		return true, statusTimeout(5 * time.Second)

	case NavigateHistoryBackMsg:
		return true, m.navigateHistoryBack()

	case NavigateHistoryForwardMsg:
		return true, m.navigateHistoryForward()

	case NavigateToViewMsg:
		return m.handleNavigateToView(msg)

	case ShowHelpMsg:
		m.helpView = NewHelpViewModel(msg.Items, msg.SourceView, msg.ViewName)
		m.showView(ViewHelp, m.helpView)
		return true, nil

	case ShowKeybindingExplorerMsg:
		explorer := NewKeybindingExplorerModel()
		m.showView(ViewKeybindingExplorer, explorer)
		return true, nil

	case ShowTutorMsg:
		m.showView(ViewTutor, NewTutorModel())
		return true, nil

	case ShowAskMsg:
		return m.showViewInit(ViewAsk, NewAskViewModel(m.currentProject, m.cliReference))

	case ShowMessageLogMsg:
		m.showView(ViewMessageLog, NewMessageLogViewModel(m.messageLog.Entries()))
		return true, nil

	case AppendToMessageLogMsg:
		m.messageLog.Append(msg.Message, msg.Level)
		return true, nil

	case StatusClearMsg:
		// A 2s plain-status tick may arrive after a newer setUndoableStatus
		// has installed a still-live toast (6s window). Don't dismiss a toast
		// that hasn't reached its own deadline yet — its dedicated
		// UndoToastTimeoutMsg will clear it on time.
		if m.statusUndoCb != nil && time.Now().Before(m.statusExpiresAt) {
			return true, nil
		}
		m.clearStatus()
		return true, nil

	case UndoToastTimeoutMsg:
		// Only clear if the deadline that fired this tick is still the
		// active one. A newer toast (with later expiresAt) may have
		// replaced this one — leave its callback alone.
		if m.statusUndoCb != nil && !time.Now().Before(m.statusExpiresAt) {
			m.clearStatus()
		}
		return true, nil

	case StatusMsg:
		m.setSuccessStatus(string(msg))
		return true, defaultStatusTimeout()

	case InfoStatusMsg:
		m.setInfoStatus(string(msg))
		return true, defaultStatusTimeout()

	case MoveBlockedMsg:
		m.setInfoStatus(msg.Reason)
		return true, defaultStatusTimeout()
	}

	return false, nil
}
