package tui

// GetHelpItems returns help items for the Kanban view.
// Implements HelpProvider interface.
func (m *KanbanViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewKanban)
}
