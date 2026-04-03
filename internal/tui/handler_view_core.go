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

	case CloseHelpMsg:
		m.popView()
		return true, nil

	case ShowTutorMsg:
		m.showView(ViewTutor, NewTutorModel())
		return true, nil

	case ShowAskMsg:
		askView := NewAskViewModel(m.currentProject, m.cliReference)
		m.showView(ViewAsk, askView)
		return true, askView.Init()

	case ShowMessageLogMsg:
		m.showView(ViewMessageLog, NewMessageLogViewModel(m.messageLog.Entries()))
		return true, nil

	case AppendToMessageLogMsg:
		m.messageLog.Append(msg.Message, msg.Level)
		return true, nil

	case StatusClearMsg:
		m.clearStatus()
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
