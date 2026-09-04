package tui

import (
	"fmt"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func (m *AppModel) Update(msg tea.Msg) (outModel tea.Model, outCmd tea.Cmd) { //nolint:gocyclo
	// Every branch below returns early, so the one deferred closure is also the
	// single place the footer divider flash can compare status text before and
	// after dispatch. It must stay one defer: a second one would run first
	// under LIFO, before recovery and before outCmd is assigned.
	statusBefore := m.captureFooterFlashProbe()
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
			m.lensPicker = nil
			m.clearOperationValueStep()
			m.operationConfirm.Cancel()
			m.pendingOperationRequest = operation.OperationExecutionRequest{}
			m.statusMessage = ""
			outModel = m
			outCmd = nil
			// The recovery already rebuilt navigation; probing it again here
			// would risk a second panic with no recover left above it.
			return
		}
		if flash := m.armFooterFlashSince(statusBefore); flash != nil {
			outCmd = tea.Batch(outCmd, flash)
		}
		if expiry := m.scheduleViewStatusExpiry(statusBefore); expiry != nil {
			outCmd = tea.Batch(outCmd, expiry)
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
	return m, tea.Batch(m.delegateToCurrentView(msg))
}
