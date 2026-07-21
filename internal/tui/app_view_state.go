package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// minTerminalWidth is the minimum terminal width required to render the TUI.
const minTerminalWidth = 60

// minTerminalHeight is the minimum terminal height required to render the TUI.
const minTerminalHeight = 20

// handleWindowSize updates dimensions for all views.
func (m *AppModel) handleWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height
	m.tooNarrow = msg.Width < minTerminalWidth
	m.tooShort = msg.Height < minTerminalHeight
	// All views (including full-height ones) are sized to height-2, matching the
	// open path: showView's full-height SetSize is immediately clobbered by
	// ViewRegistry.Register re-applying the registry's stored height-2. Views
	// self-pad their render to the terminal height, so the height-2 budget is
	// what every render is tuned against. Previously this loop re-applied the
	// full msg.Height to full-height views, diverging from the open path and
	// overflowing views (Notes, Inbox Processor) that size against their budget
	// rather than self-padding. Regression coverage: the *-resize-minimum-layout
	// scenarios under docs/tui-testing/scenarios/ (run by TestExampleScenariosPass).
	m.views.SetSize(msg.Width, msg.Height-2)
	if m.taskWorkspaceView != nil {
		m.taskWorkspaceView.SetSize(msg.Width, msg.Height-2)
	}
	if m.reviewHub != nil {
		m.reviewHub.SetSize(msg.Width, msg.Height-2)
	}
	if m.manageHub != nil {
		m.manageHub.SetSize(msg.Width, msg.Height-2)
	}
	if m.taskDetail != nil {
		m.taskDetail.SetSize(msg.Width, msg.Height-2)
	}
	if m.taskComposer != nil {
		m.taskComposer.SetSize(msg.Width, msg.Height-2)
	}
	if m.pinnedPreview != nil {
		m.pinnedPreview.SetSize(msg.Width, msg.Height-2)
	}
}

// delegateToCurrentView forwards messages to the active view model.
func (m *AppModel) delegateToCurrentView(msg tea.Msg) tea.Cmd { //nolint:gocyclo
	if m.activeTaskComposer() {
		updated, cmd := m.taskComposer.Update(msg)
		if composer, ok := updated.(*TaskComposerModel); ok {
			m.taskComposer = composer
			m.views.Register(composer.presentation, composer)
		}
		return cmd
	}
	if m.activeTaskWorkspace() {
		updated, cmd := m.taskWorkspaceView.Update(msg)
		if view, ok := updated.(*TaskWorkspaceView); ok {
			m.taskWorkspaceView = view
		}
		if m.peekActive {
			if task, project := m.getCurrentCursorTaskAndProject(); task != nil {
				m.peekTask, m.peekProject = task, project
			} else {
				m.dismissPeek()
			}
		}
		return cmd
	}
	if m.activeReviewHub() {
		updated, cmd := m.reviewHub.Update(msg)
		if hub, ok := updated.(*ReviewHubModel); ok {
			m.reviewHub = hub
		}
		return cmd
	}
	if m.activeManageHub() {
		updated, cmd := m.manageHub.Update(msg)
		if hub, ok := updated.(*ManageHubModel); ok {
			m.manageHub = hub
		}
		return cmd
	}
	if m.activeTaskDetail() {
		updated, cmd := m.taskDetail.Update(msg)
		if detail, ok := updated.(*TaskDetailModel); ok {
			m.taskDetail = detail
		}
		return tea.Batch(cmd, m.syncTaskDetailTab(m.taskDetail.Tab()))
	}
	currentView := m.currentView()
	cmd := m.views.Update(currentView, msg)

	// Update peek task if active (cursor may have moved)
	if m.peekActive {
		if task, project := m.getCurrentCursorTaskAndProject(); task != nil {
			m.peekTask = task
			m.peekProject = project
		} else {
			m.dismissPeek()
		}
	}

	// Keep typed pointers in sync for views that need direct access
	switch currentView {
	case ViewDashboard:
		if v := m.views.Get(ViewDashboard); v != nil {
			m.dashboard = v.(*DashboardModel)
		}
	case ViewHelp:
		if v := m.views.Get(ViewHelp); v != nil {
			m.helpView = v.(*HelpViewModel)
		}
	case ViewTaskJumper:
		if v := m.views.Get(ViewTaskJumper); v != nil {
			m.taskJumper = v.(*TaskJumperModel)
		}
	case ViewInboxProcessor:
		if v := m.views.Get(ViewInboxProcessor); v != nil {
			m.inboxProcessor = v.(*InboxProcessorModel)
		}
	case ViewGlobalInboxProcessor:
		if v := m.views.Get(ViewGlobalInboxProcessor); v != nil {
			m.globalInboxProcessor = v.(*GlobalInboxProcessorModel)
		}
	case ViewAllTasks:
		if v := m.views.Get(ViewAllTasks); v != nil {
			m.allTasksView = v.(*AllTasksViewModel)
		}
	case ViewOutliner:
		if v := m.views.Get(ViewOutliner); v != nil {
			m.outliner = v.(*OutlinerModel)
		}
	case ViewCrossProjectOutliner:
		if v := m.views.Get(ViewCrossProjectOutliner); v != nil {
			m.crossOutliner = v.(*OutlinerModel)
		}
	default:
	}

	return cmd
}

// getCurrentCursorTaskAndProject returns the task and project under the cursor in
// the current primary view, or (nil, nil) when no task is selected. The project
// pointer enables overlays (e.g. peek) that need scoring profile lookups.
func (m *AppModel) getCurrentCursorTaskAndProject() (*models.Task, *models.Project) {
	if view, _, ok := m.taskWorkspacePrimary(); ok && m.activeTaskWorkspace() {
		return view.getTaskRef()
	}
	if hub, _, ok := m.reviewHubPrimary(); ok && m.activeReviewHub() {
		return hub.getTaskRef()
	}
	if hub, _, ok := m.manageHubPrimary(); ok && m.activeManageHub() {
		return hub.getTaskRef()
	}
	if detail, _, ok := m.taskDetailPrimary(); ok && m.activeTaskDetail() {
		return detail.getTaskRef()
	}
	type taskRefProvider interface {
		getTaskRef() (*models.Task, *models.Project)
	}
	if v, ok := m.currentViewAsModel().(taskRefProvider); ok {
		return v.getTaskRef()
	}
	return nil, nil
}

// getCurrentCursorTask returns the task under the cursor in the current primary view,
// or nil if no task is selected or the current view is not a primary project view.
func (m *AppModel) getCurrentCursorTask() *models.Task {
	if m.activeTaskWorkspace() && m.taskWorkspaceView != nil {
		return m.taskWorkspaceView.getCursorTask()
	}
	switch m.currentView() {
	case ViewOutliner:
		if m.outliner != nil {
			return m.outliner.getCursorTask()
		}
	case ViewTemporaryOutliner:
		if v := m.views.Get(ViewTemporaryOutliner); v != nil {
			if o, ok := v.(*OutlinerModel); ok {
				return o.getCursorTask()
			}
		}
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			return m.crossOutliner.getCursorTask()
		}
	case ViewInboxProcessor:
		if m.inboxProcessor != nil {
			task, _ := m.inboxProcessor.getTaskRef()
			return task
		}
	case ViewGlobalInboxProcessor:
		if m.globalInboxProcessor != nil {
			task, _ := m.globalInboxProcessor.getTaskRef()
			return task
		}
	case ViewKanban, ViewCrossProjectKanban:
		if v := m.views.Get(m.currentView()); v != nil {
			if kb, ok := v.(*KanbanModel); ok {
				if card := kb.getCursorCard(); card != nil {
					return card.Task
				}
			}
		}
	default:
		// Other view types don't have a cursor task.
	}
	return nil
}
