package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *ProjectTreeModel) View() tea.View {
	if m.project == nil {
		return tea.NewView("No project loaded")
	}

	// If sidebar is enabled, render split-pane layout
	if m.sidebar.IsEnabled() {
		return tea.NewView(m.renderWithSidebar())
	}

	// Otherwise, render traditional full-width view
	return tea.NewView(m.renderFullWidth())
}

// GetHelpItems returns the help items for the project tree view.
func (m *ProjectTreeModel) GetHelpItems() []HelpItem {
	if m.Dispatcher != nil {
		km := m.buildKeyMap()
		raw := km.HelpItems()
		out := make([]HelpItem, len(raw))
		for i, h := range raw {
			out[i] = HelpItem{Key: h.Key, Description: h.Description, Category: h.Category}
		}
		return out
	}
	archiveDesc := "Archive project"
	if m.project != nil && m.project.Archived {
		archiveDesc = "Unarchive project"
	}
	overrides := map[string]string{
		"ctrl+A": archiveDesc,
	}
	return GetKeybindingRegistry().GetForViewWithOverrides(ViewProjectTree, overrides)
}

// HandledKeys returns all keys ProjectTreeModel can handle, for sync testing.
func (m *ProjectTreeModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *ProjectTreeModel) selectPinnedTagTab(section string) {
	m.pinnedTagTab = 0
	tag := models.NormalizeTag(section)
	if tag == "" {
		return
	}

	if tabs := m.pinnedTagTabs(); tabs != nil {
		for i, tab := range tabs {
			if pinnedTagFromTab(tab) == tag {
				m.pinnedTagTab = i
				return
			}
		}
	}
}

func (m *ProjectTreeModel) selectViewForTask(task *models.Task) {
	m.pinnedTagTab = 0
	if task == nil {
		return
	}
	if task.IsSnoozed() {
		m.viewMode = ViewModeSnoozed
		return
	}
	if task.IsComplete() {
		m.viewMode = ViewModeCompleted
		return
	}
	switch task.Section {
	case models.SectionNext:
		m.viewMode = ViewModeNext
	case models.SectionInbox:
		m.viewMode = ViewModeInbox
	case models.SectionWaiting:
		m.viewMode = ViewModeWaiting
	case models.SectionSomeday:
		m.viewMode = ViewModeSomeday
	case models.SectionReview:
		m.viewMode = ViewModeReview
	case models.SectionStandby:
		m.viewMode = ViewModeStandby
	default:
		m.viewMode = ViewModeActive
	}
}

// NavigateToTask expands parent nodes and positions cursor on the given task.
// If the task is filtered out, it clears filters and switches view modes as needed.
// Returns a tea.Cmd that sends a status message if filters were cleared.
func (m *ProjectTreeModel) NavigateToTask(task *models.Task) tea.Cmd {
	if task == nil {
		return nil
	}

	// First, try to find in current nodes (task may exist but be collapsed)
	for _, node := range m.nodes {
		if node.Task.ID == task.ID {
			// Task exists in tree, just expand ancestors and navigate
			m.expandAncestors(task)
			m.updateVisibility()
			visible := m.visibleNodes()
			for i, n := range visible {
				if n.Task.ID == task.ID {
					m.cursor = i
					return nil
				}
			}
		}
	}

	// Task not in current tree - need to adjust view to show it
	// Check if any filters are active before clearing
	hadFilters := m.viewMode != ViewModeActive ||
		m.filter.Type != "" || m.filter.Status != "" || m.filter.Priority != 0 ||
		len(m.filter.Tags) > 0 || len(m.filter.Sections) > 0 || m.filter.DueDate != DueDateAll ||
		m.SearchQuery() != "" || m.focusedTask != nil || m.activePinnedTagFilter() != ""

	// Clear focus mode first
	if m.focusedTask != nil {
		m.focusedTask = nil
	}

	// Determine appropriate view mode based on task state
	m.selectViewForTask(task)

	// Clear all filters that might hide the task
	m.filter.Type = ""
	m.filter.Status = ""
	m.filter.Priority = 0
	m.filter.Tags = nil
	m.filter.Sections = nil
	m.filter.DueDate = DueDateAll
	m.ClearSearch()

	// Rebuild tree with cleared filters
	m.buildTree()

	// Now expand ancestors and find the task
	m.expandAncestors(task)
	m.updateVisibility()

	visible := m.visibleNodes()
	for i, node := range visible {
		if node.Task.ID == task.ID {
			m.cursor = i
			if hadFilters {
				return func() tea.Msg { return InfoStatusMsg("Filters cleared to reveal task") }
			}
			return nil
		}
	}
	return nil
}

// expandAncestors marks all ancestor nodes as expanded.
func (m *ProjectTreeModel) expandAncestors(task *models.Task) {
	// Walk up the parent chain
	current := task.Parent
	for current != nil {
		// Find the node for this parent and expand it
		for _, node := range m.nodes {
			if node.Task.ID == current.ID {
				node.Expanded = true
				break
			}
		}
		current = current.Parent
	}
}
