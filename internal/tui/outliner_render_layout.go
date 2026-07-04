package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// renderFullWidth renders the outliner tree using the full terminal width.
//
// The content is wrapped in Width(m.width), mirroring the split-pane path's
// Width(treeWidth) clamp: an over-wide header (long project name + perspective
// breadcrumb + multi-select badge) then wraps to the next line — whose extra
// height renderOutlinerTreeContent already reserves via titleExtra — instead of
// overflowing terminal width. Task rows are pre-clamped to rowWidth, so they
// never wrap here.
func (m *OutlinerModel) renderFullWidth() string {
	return lipgloss.NewStyle().Width(m.width).Render(m.renderOutlinerTreeContent(m.width))
}

// renderWithSidebar renders the split-pane view with tree on left and sidebar on right.
func (m *OutlinerModel) renderWithSidebar() string {
	treeWidth := max((m.width*55)/100, TreeMinWidth)

	dividerWidth := 1
	sidebarWidth := m.width - treeWidth - dividerWidth
	if sidebarWidth < SidebarMinWidth {
		sidebarWidth = SidebarMinWidth
		treeWidth = m.width - sidebarWidth - dividerWidth
	}

	treeContent := m.renderOutlinerTreeContent(treeWidth)
	sidebarContent := renderSidebarContent(
		m,
		m.selectedSidebarTask(),
		sidebarWidth-SidebarContentOverhead,
		m.height-CardVerticalBorderOverhead,
	)
	treeContentStyled := lipgloss.NewStyle().Width(treeWidth).Render(treeContent)
	divider := renderVerticalDivider(m.height-2, m.sidebar.IsFocused())

	if m.sidebar.IsFocused() {
		treeContentStyled = DimStyle.Render(treeContentStyled)
	} else {
		sidebarContent = DimStyle.Render(sidebarContent)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, treeContentStyled, divider, sidebarContent)
}

func (m *OutlinerModel) selectedSidebarTask() *models.Task {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}

	row := m.rows[m.cursor]
	if row.IsSectionHeader {
		return nil
	}

	return row.task
}
