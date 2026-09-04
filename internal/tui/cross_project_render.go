package tui

import (
	"charm.land/lipgloss/v2"
)

// crossProjectView renders the cross-project outliner. A cold load shows a
// centered loading state. Later reloads retain the last accepted snapshot until
// its replacement is ready, so rendering never passes through an empty frame.
func (m *OutlinerModel) crossProjectView() string {
	if m.population.loading && !m.hasCrossProjectSnapshot() {
		return m.renderCrossLoadingState()
	}
	return m.renderCrossProjectContent()
}

// hasCrossProjectSnapshot reports whether there is stable content to render
// while a refresh is in flight. The loaded-projects fallback keeps older tests
// and callers that install an in-memory snapshot directly compatible.
func (m *OutlinerModel) hasCrossProjectSnapshot() bool {
	return m.population.snapshotReady || len(m.population.projects) > 0
}

// renderCrossLoadingState renders a non-empty cold-load frame immediately,
// adding the animated Dot spinner only after its short anti-flicker delay.
func (m *OutlinerModel) renderCrossLoadingState() string {
	label := "Loading projects…"
	if m.population.spinnerVisible {
		label = m.population.spinner.View() + " " + label
	}
	centered := lipgloss.NewStyle().
		Width(m.Width()).
		Height(m.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Render(label)
	return centered
}

// renderCrossProjectContent renders the aggregated cross-project outliner tree.
// It delegates to the unified renderOutlinerTreeContent after rebuilding rows.
func (m *OutlinerModel) renderCrossProjectContent() string {
	m.crossProjectRebuildRows()
	return m.renderFullWidth()
}

// reloadCrossProjects triggers a new load, incrementing the generation counter
// so any in-flight stale result is discarded on arrival.
