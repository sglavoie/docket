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
		return tea.NewView(m.renderFullWidth())
	}

	// Rebuild the mode's flat row list and clamp the cursor.
	m.rebuildRows()

	return tea.NewView(m.renderFullWidth())
}
