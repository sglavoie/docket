package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const (
	// HomeWideMinWidth is the fixed responsive breakpoint. Home never reads a
	// persisted dashboard layout or split percentage.
	HomeWideMinWidth = 100
	homeProjectsPct  = 45
)

func (m *DashboardModel) renderHome(visibleProjects []*models.Project, chromeHeight int) string {
	totalWidth := max(m.width-2, MinCardWidth)
	if m.width < HomeWideMinWidth {
		if m.homePane == homePaneAttention {
			return m.renderHomeAttentionPane(totalWidth, chromeHeight, true)
		}
		return m.renderHomeProjectsPane(visibleProjects, totalWidth, chromeHeight, true)
	}

	projectsWidth := max(totalWidth*homeProjectsPct/100, MinCardWidth)
	attentionWidth := max(totalWidth-projectsWidth, MinCardWidth)
	projects := m.renderHomeProjectsPane(visibleProjects, projectsWidth, chromeHeight, false)
	attention := m.renderHomeAttentionPane(attentionWidth, chromeHeight, false)
	return lipgloss.JoinHorizontal(lipgloss.Top, projects, attention)
}

func (m *DashboardModel) renderHomeProjectsPane(visible []*models.Project, width, chromeHeight int, narrow bool) string {
	title := m.projectCardTitle(visible)
	if m.homePane == homePaneProjects {
		title += " · active"
	}
	if narrow {
		title += " · h/l panes"
	}
	content := m.renderProjectList(visible, chromeHeight, width, 0)
	return renderCard(title, content, width, DashboardCardConfig())
}

func (m *DashboardModel) renderHomeAttentionPane(width, chromeHeight int, narrow bool) string {
	rows := m.visibleAttentionRows()
	title := fmt.Sprintf("Attention (%d)", len(rows))
	if m.homePane == homePaneAttention {
		title += " · active"
	}
	if narrow {
		title += " · h/l panes"
	}
	content := m.renderHomeAttentionRows(rows, width, chromeHeight)
	return renderCard(title, content, width, DashboardCardConfig())
}

func (m *DashboardModel) renderHomeAttentionRows(rows []homeAttentionRow, cardWidth, chromeHeight int) string {
	if len(rows) == 0 {
		if m.SearchQuery() != "" {
			return MutedStyle.Render("No Attention items match your search.")
		}
		return MutedStyle.Render("Nothing needs attention.")
	}

	rowWidth := max(cardWidth-4, 1)
	maxVisible := max(m.height-chromeHeight-CardVerticalBorderOverhead-1, 1)
	start, end := persistentIndexWindow(m.attentionCursor, len(rows), maxVisible, &m.attentionViewScrollOffset)

	var b strings.Builder
	for i := start; i < end; i++ {
		row := rows[i]
		category := MutedStyle.Render("[" + string(row.Category) + "]")
		label := row.Label
		if m.SearchQuery() != "" {
			label = HighlightSubstring(label, m.SearchQuery(), NormalStyle)
		} else {
			label = NormalStyle.Render(label)
		}
		line := category + " " + label
		if strings.TrimSpace(row.Detail) != "" {
			line += MutedStyle.Render(" · " + row.Detail)
		}
		line = truncateToVisualWidth(line, rowWidth-2)
		selected := m.homePane == homePaneAttention && i == m.attentionCursor
		b.WriteString(RenderSelectableRow(selected, false, m.cursorFlash, false, false, line, rowWidth))
		if i < end-1 {
			b.WriteByte('\n')
		}
	}

	scrollbar := RenderScrollbar(len(rows), maxVisible, start)
	return attachScrollbarAtWidth(b.String(), scrollbar, rowWidth)
}

func (m *DashboardModel) homePageSize() int {
	if m == nil {
		return 1
	}
	_, chromeHeight := m.renderFilterBar()
	return max(m.height-chromeHeight-CardVerticalBorderOverhead-1, 1)
}
