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
			if m.currentView() != ViewDashboard {
				m.viewStack = []ViewType{ViewDashboard}
			}
			m.pendingSemicolonPrefix = false
			m.pendingSemicolonTPrefix = false
			m.sectionQuickPicker = nil
			m.linkTypeQuickPicker = nil
			m.linkModeQuickPicker = nil
			m.isLoading = false
			m.showQuickSwitch = false
			m.quickSwitchFiltering = false
			m.quickSwitchShowCompleted = false
			m.showPinnedPopup = false
			m.pinnedPopupFiltering = false
			m.showProjectCyclePicker = false
			m.showProjectCycle = false
			m.pendingSandboxReset = false
			m.pendingBulkTasks = nil
			m.pendingBulkSectionTasks = nil
			m.pendingBulkNotes = nil
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
		switch cv.closePopStyle() {
		case popToAnchor:
			m.popToAnchorView()
		default:
			m.popView()
		}
		return m, nil
	}

	if handled, cmd := m.handleGlobalMessage(msg); handled {
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
	if handled, cmd := m.handleSectionMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleInboxMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleProjectCycleMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleExportMessages(msg); handled {
		return m, cmd
	}

	return m, tea.Batch(m.delegateToCurrentView(msg))
}
