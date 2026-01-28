package tui

import tea "github.com/charmbracelet/bubbletea"

// handleTagMessages handles tag-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleTagMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTagManagerMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewTagManager, NewTagManagerModel(msg.ProjectTree))
		}
		return true, nil

	case CloseTagManagerMsg:
		m.popView()
		return true, nil

	case ShowTagSelectorMsg:
		selector := NewTagSelectorModel(msg.Task, msg.Project, msg.CurrentTags)
		m.showView(ViewTagSelector, selector)
		return true, nil

	case ShowBulkTagSelectorMsg:
		selector := NewBulkTagSelectorModel(msg.Tasks, msg.Project)
		m.showView(ViewBulkTagSelector, selector)
		return true, nil

	case TagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		// Check if we came from project tree - apply tags directly
		if m.currentView() == ViewProjectTree && msg.Task != nil {
			return true, m.applyTagsToTask(msg.Task, msg.Project, msg.SelectedTags)
		}
		// Otherwise forward to current view (inbox processor, task editor, etc.)
		cmd := m.views.Update(m.currentView(), msg)
		if v := m.views.Get(ViewInboxProcessor); v != nil {
			m.inboxProcessor = v.(*InboxProcessorModel)
		}
		return true, cmd

	case BulkTagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		return true, m.applyTagsToTasks(msg.Tasks, msg.Project, msg.SelectedTags)

	case ShowTagFilterSelectorMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewTagFilterSelector, NewTagFilterSelectorModel(msg.ProjectTree))
		}
		return true, nil

	case TagFilterSelectorDoneMsg:
		m.popView()
		if !msg.Canceled && m.projectTree != nil {
			m.projectTree.filter.Tags = make(map[string]bool)
			for _, tag := range msg.SelectedTags {
				m.projectTree.filter.Tags[tag] = true
			}
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		}
		return true, nil

	case ShowKanbanTagFilterSelectorMsg:
		if m.kanban != nil {
			m.showView(ViewKanbanTagFilterSelector, NewKanbanTagFilterSelectorModel(m.kanban))
		}
		return true, nil

	case KanbanTagFilterSelectorDoneMsg:
		m.popView()
		if !msg.Canceled && m.kanban != nil {
			m.kanban.filter.Tags = make(map[string]bool)
			for _, tag := range msg.SelectedTags {
				m.kanban.filter.Tags[tag] = true
			}
			m.kanban.loadTasks()
		}
		return true, nil

	case TagRenamedMsg:
		m.popView()
		m.reloadAndRefreshProjectTree()
		return true, nil

	case ShowGlobalTagManagerMsg:
		m.showView(ViewGlobalTagManager, NewGlobalTagManagerModel(m.config))
		return true, nil

	case CloseGlobalTagManagerMsg:
		m.popView()
		return true, nil

	case GlobalTagRenamedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case GlobalTagNavigateMsg:
		m.popView()
		m.switchToProject(msg.Project, false) // Don't restore state - apply specific tag filter
		m.projectTree.filter.Tags = map[string]bool{msg.Tag: true}
		m.projectTree.updateVisibility()
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil
	}
	return false, nil
}
