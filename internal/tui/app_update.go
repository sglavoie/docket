package tui

import (
	"fmt"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
)

func (m *AppModel) Update(msg tea.Msg) (outModel tea.Model, outCmd tea.Cmd) { //nolint:gocyclo
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			logging.Warn("TUI panic in Update: %v\n%s", r, stack)
			m.err = fmt.Errorf("internal error: %v", r)
			if m.crossOutliner != nil {
				m.crossOutliner.cancelCrossProjectLoad()
			}
			m.navigation.ResetRoot()
			m.contextActionSheet = nil
			m.pendingTaskRecurrenceRequest = nil
			m.suspendedTaskActionSheet = nil
			m.clearOperationValueStep()
			m.operationConfirm.Cancel()
			m.pendingOperationRequest = OperationExecutionRequest{}
			m.isLoading = false
			m.pendingBulkTasks = nil
			m.pendingBulkSectionTasks = nil
			m.statusMessage = ""
			m.loadingMessage = ""
			outModel = m
			outCmd = nil
		}
	}()

	// Handle all trivial close-view messages via the closeViewMsg interface.
	// This eliminates individual case arms for ~35 Close*Msg types that only
	// pop the view stack.
	if cv, ok := msg.(closeViewMsg); ok {
		var cmd tea.Cmd
		switch cv.closePopStyle() {
		case popToAnchor:
			cmd = m.popToAnchorView()
		default:
			cmd = m.popView()
		}
		return m, cmd
	}

	if handled, cmd := m.handleGlobalMessage(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleManageHubMessages(msg); handled {
		return m, cmd
	}

	if handled, cmd := m.handleProjectMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTaskMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleViewMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTagMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleExportMessages(msg); handled {
		return m, cmd
	}

	return m, tea.Batch(m.delegateToCurrentView(msg))
}
