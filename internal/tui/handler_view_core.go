package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// handleViewCoreMessages handles generic view navigation, status, and utility messages.
func (m *AppModel) handleViewCoreMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case BackMsg:
		if m.currentView() == ViewProjectCreator {
			m.pendingProjectMoveTask = nil
			m.pendingProjectMoveSource = nil
		}
		m.popView()
		return true, nil

	case JumpDashboardMsg:
		return true, m.jumpToOutlinerRoot()

	case RefreshMsg:
		m.refreshLoadedTaskWorkspace()
		return true, nil

	case ErrorMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.err = msg.Err
		return true, statusTimeout(5 * time.Second)

	case NavigateToViewMsg:
		return m.handleNavigateToView(msg)
	case NavigatePaletteAliasMsg:
		// No searchable alias currently resolves to a distinct navigation
		// target; the message is retained as generic palette-alias plumbing.
		return true, nil

	case ShowHelpMsg:
		m.helpView = NewHelpViewModel(msg.Items, msg.SourceView, msg.ViewName)
		m.showView(ViewHelp, m.helpView)
		return true, nil

	case ShowTutorMsg:
		m.showView(ViewTutor, NewTutorModel())
		return true, nil

	case ShowAskMsg:
		return m.showViewInit(ViewAsk, NewAskViewModelForWorkspace(m.cliReference, m.runtime, m.appService, m.persister))

	case ShowMessageLogMsg:
		return true, m.activateManageHub(ManageParams{
			Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationLog,
		})

	case AppendToMessageLogMsg:
		m.messageLog.Append(msg.Message, msg.Level)
		m.refreshManageMessageLog()
		return true, nil

	case StatusClearMsg:
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

	case OutlinerPlanningReorderMsg:
		return true, m.handleOutlinerPlanningReorder(msg)
	}

	return false, nil
}
