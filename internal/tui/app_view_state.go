package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

// handleWindowSize updates dimensions for all views.
func (m *AppModel) handleWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height
	m.tooNarrow = msg.Width < chrome.MinTerminalWidth
	m.tooShort = msg.Height < chrome.MinTerminalHeight
	// All views (including full-height ones) are sized to height-2, matching the
	// open path: showView's full-height SetSize is immediately clobbered by
	// ViewRegistry.Register re-applying the registry's stored height-2. Views
	// self-pad their render to the terminal height, so the height-2 budget is
	// what every render is tuned against. Previously this loop re-applied the
	// full msg.Height to full-height views, diverging from the open path and
	// overflowing views (such as Notes) that size against their budget
	// rather than self-padding. Regression coverage: the *-resize-minimum-layout
	// scenarios under docs/tui-testing/scenarios/ (run by TestExampleScenariosPass).
	m.views.SetSize(msg.Width, msg.Height-2)
	if m.manageHub != nil {
		m.manageHub.SetSize(msg.Width, msg.Height-2)
	}
	if m.taskComposer != nil {
		m.taskComposer.SetSize(msg.Width, msg.Height-2)
	}
}

// delegateToCurrentView forwards messages to the active view model.
func (m *AppModel) delegateToCurrentView(msg tea.Msg) tea.Cmd { //nolint:gocyclo
	if m.activeTaskComposer() {
		updated, cmd := m.taskComposer.Update(msg)
		if composer, ok := updated.(*taskform.TaskComposerModel); ok {
			m.taskComposer = composer
			m.views.Register(presentation.ViewTaskComposer, composer)
		}
		return cmd
	}
	if m.activeManageHub() {
		updated, cmd := m.manageHub.Update(msg)
		if hub, ok := updated.(*projecthub.ManageHubModel); ok {
			m.manageHub = hub
		}
		return cmd
	}
	currentView := m.currentView()
	cmd := m.views.Update(currentView, msg)

	// Keep typed pointers in sync for views that need direct access
	switch currentView {
	case presentation.ViewHelp:
		if v := m.views.Get(presentation.ViewHelp); v != nil {
			m.helpView = v.(*help.HelpViewModel)
		}
	case presentation.ViewCrossProjectOutliner:
		if v, ok := m.views.Get(presentation.ViewCrossProjectOutliner).(*outliner.OutlinerModel); ok {
			m.crossOutliner = v
		}
	default:
	}

	return cmd
}

// getCurrentCursorTaskAndProject returns the task and project under the cursor in
// the current primary view, or (nil, nil) when no task is selected. The project
// pointer enables overlays that need project context.
func (m *AppModel) getCurrentCursorTaskAndProject() (*models.Task, *models.Project) {
	if hub, _, ok := m.manageHubPrimary(); ok && m.activeManageHub() {
		return hub.GetTaskRef()
	}
	type taskRefProvider interface {
		GetTaskRef() (*models.Task, *models.Project)
	}
	if v, ok := m.currentViewAsModel().(taskRefProvider); ok {
		return v.GetTaskRef()
	}
	return nil, nil
}
