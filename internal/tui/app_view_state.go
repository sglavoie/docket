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
	m.views.SetSize(msg.Width, msg.Height-2)
	// Re-apply full terminal height for views that render their own footer
	for _, vt := range m.views.Types() {
		v := m.views.Get(vt)
		if v == nil {
			continue
		}
		if caps, ok := v.(ViewCapabilities); ok && caps.IsFullHeight() {
			v.SetSize(msg.Width, msg.Height)
		}
	}
	if m.projectCyclePreview != nil {
		m.projectCyclePreview.SetSize(msg.Width, msg.Height-2)
	}
	if m.pinnedPreview != nil {
		m.pinnedPreview.SetSize(msg.Width, msg.Height-2)
	}
}

// delegateToCurrentView forwards messages to the active view model.
func (m *AppModel) delegateToCurrentView(msg tea.Msg) tea.Cmd {
	currentView := m.currentView()
	cmd := m.views.Update(currentView, msg)

	// Update peek task if active (cursor may have moved)
	if m.peekActive {
		if task := m.getCurrentCursorTask(); task != nil {
			m.peekTask = task
		} else {
			m.peekTask = nil
			m.peekActive = false
		}
	}

	// Keep typed pointers in sync for views that need direct access
	switch currentView {
	case ViewDashboard:
		if v := m.views.Get(ViewDashboard); v != nil {
			m.dashboard = v.(*DashboardModel)
		}
	case ViewProjectTree:
		if v := m.views.Get(ViewProjectTree); v != nil {
			m.projectTree = v.(*ProjectTreeModel)
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

// getCurrentCursorTask returns the task under the cursor in the current primary view,
// or nil if no task is selected or the current view is not a primary project view.
func (m *AppModel) getCurrentCursorTask() *models.Task {
	switch m.currentView() {
	case ViewProjectTree:
		if m.projectTree != nil {
			return m.projectTree.getCursorTask()
		}
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
