package tui

import tea "charm.land/bubbletea/v2"

// View implements tea.Model. It renders the full outliner tree with connector
// prefixes computed on each render pass from the project's task hierarchy.
func (m *OutlinerModel) View() tea.View {
	if m.project == nil {
		// Spinner/loading states handled by crossProjectView.
		if m.crossSpinnerVisible || m.crossLoading {
			return tea.NewView(m.crossProjectView())
		}
		m.crossProjectRebuildRows()
		if m.sidebar.IsEnabled() {
			return tea.NewView(m.renderWithSidebar())
		}
		return tea.NewView(m.renderFullWidth())
	}

	// Rebuild flat row list with perspective filter and clamp cursor.
	m.rebuildRows()

	if m.sidebar.IsEnabled() {
		return tea.NewView(m.renderWithSidebar())
	}
	return tea.NewView(m.renderFullWidth())
}
