package tui

import tea "charm.land/bubbletea/v2"

// handlePrefixKey is a stub — prefix dispatch is now handled by m.Dispatcher.
// Kept to satisfy any lingering call sites during the migration.
func (m *ProjectTreeModel) handlePrefixKey(_ string, _ []*TreeNode) (tea.Model, tea.Cmd) {
	return m, nil
}

// handleTaskPrefix handles the second key after pressing t (task action prefix).
// Stub — now handled via m.Dispatcher.
func (m *ProjectTreeModel) handleTaskPrefix(_ string, _ []*TreeNode) (tea.Model, tea.Cmd) {
	return m, nil
}

// handleBlockerPrefix handles the second key after pressing b (blocker/dependency prefix).
// Stub — now handled via m.Dispatcher.
func (m *ProjectTreeModel) handleBlockerPrefix(_ string, _ []*TreeNode) (tea.Model, tea.Cmd) {
	return m, nil
}
