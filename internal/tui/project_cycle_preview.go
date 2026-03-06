package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderCyclePreviewBackground renders a dimmed task tree of the highlighted
// project as background content behind the cycle overlay popup.
func (m *AppModel) renderCyclePreviewBackground() string {
	if !m.showProjectCycle || len(m.projectCycleItems) == 0 {
		return ""
	}

	proj := m.projectCycleItems[m.projectCycleCursor].Project
	if m.projectCyclePreviewID != proj.ID {
		m.projectCyclePreview = NewProjectTreeModel(proj, m.config, m.configManager)
		m.projectCyclePreview.SetSize(m.width, m.height-2)
		m.projectCyclePreviewID = proj.ID
	}

	content := m.projectCyclePreview.View()

	// Dim each line so the preview visually recedes behind the overlay
	dim := lipgloss.NewStyle().Faint(true)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = dim.Render(line)
	}
	return strings.Join(lines, "\n")
}

// invalidateCyclePreview clears the cached preview tree.
func (m *AppModel) invalidateCyclePreview() {
	m.projectCyclePreview = nil
	m.projectCyclePreviewID = ""
}
