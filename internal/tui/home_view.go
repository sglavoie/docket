package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

const (
	// homeMinCardHeight is the smallest usable card height: top and bottom
	// borders plus one content row. Both stacked panes are clamped to this so
	// each keeps a legible row even at very constrained terminal heights.
	homeMinCardHeight = CardVerticalBorderOverhead + 1
	// homeCardSeparatorLines is the blank line rendered between the two stacked
	// cards. It is reserved out of the body budget before splitting.
	homeCardSeparatorLines = 1
)

// renderHome always stacks the Projects card above the Attention card at every
// supported width. The two cards split the post-chrome body by the machine-local
// HomeProjectsHeightPct, minus one inter-card separator line. Focus (h/l) still
// switches the active pane even though both are always visible.
func (m *DashboardModel) renderHome(visibleProjects []*models.Project, chromeHeight int) string {
	totalWidth := max(m.width-2, MinCardWidth)
	projectsHeight, attentionHeight := m.homePaneHeights(chromeHeight)
	projects := m.renderHomeProjectsPane(visibleProjects, totalWidth, projectsHeight)
	attention := m.renderHomeAttentionPane(totalWidth, attentionHeight)
	return lipgloss.JoinVertical(lipgloss.Left, projects, "", attention)
}

// homePaneHeights allocates full card heights (borders included) for the stacked
// Projects and Attention cards. The Projects card receives the configured
// percentage of the body; Attention receives the remainder. Both are clamped to
// homeMinCardHeight, borrowing from the sibling so the total does not exceed the
// available body except when the terminal is too short to honor both minimums.
func (m *DashboardModel) homePaneHeights(chromeHeight int) (projectsHeight, attentionHeight int) {
	// Reserve one trailing status line (as the legacy layout did) and one
	// inter-card separator before splitting the body between the two cards.
	cardsTotal := m.height - chromeHeight - 1 - homeCardSeparatorLines
	if cardsTotal < 2*homeMinCardHeight {
		cardsTotal = 2 * homeMinCardHeight
	}

	pct := m.homeProjectsHeightPct()
	// Deterministic round-half-up; Attention takes the exact remainder so the
	// pair always sums to cardsTotal before clamping.
	projectsHeight = (cardsTotal*pct + 50) / 100
	attentionHeight = cardsTotal - projectsHeight

	if projectsHeight < homeMinCardHeight {
		attentionHeight -= homeMinCardHeight - projectsHeight
		projectsHeight = homeMinCardHeight
	}
	if attentionHeight < homeMinCardHeight {
		projectsHeight -= homeMinCardHeight - attentionHeight
		attentionHeight = homeMinCardHeight
		if projectsHeight < homeMinCardHeight {
			projectsHeight = homeMinCardHeight
		}
	}
	return projectsHeight, attentionHeight
}

// homeProjectsHeightPct returns the clamped machine-local split percentage.
func (m *DashboardModel) homeProjectsHeightPct() int {
	if m == nil || m.runtime == nil {
		return config.DefaultHomeProjectsHeightPct
	}
	return m.runtime.Machine().GetHomeProjectsHeightPct()
}

func (m *DashboardModel) renderHomeProjectsPane(visible []*models.Project, width, cardHeight int) string {
	title := m.projectCardTitle(visible)
	if m.homePane == homePaneProjects {
		title += " · active"
	}
	contentHeight := max(cardHeight-CardVerticalBorderOverhead, 1)
	content := m.renderProjectList(visible, contentHeight, width)
	cfg := DashboardCardConfig()
	cfg.Height = cardHeight
	return renderCard(title, content, width, cfg)
}

func (m *DashboardModel) renderHomeAttentionPane(width, cardHeight int) string {
	rows := m.visibleAttentionRows()
	title := fmt.Sprintf("Attention (%d)", len(rows))
	if m.homePane == homePaneAttention {
		title += " · active"
	}
	contentHeight := max(cardHeight-CardVerticalBorderOverhead, 1)
	content := m.renderHomeAttentionRows(rows, width, contentHeight)
	cfg := DashboardCardConfig()
	cfg.Height = cardHeight
	return renderCard(title, content, width, cfg)
}

func (m *DashboardModel) renderHomeAttentionRows(rows []homeAttentionRow, cardWidth, contentHeight int) string {
	if len(rows) == 0 {
		if m.SearchQuery() != "" {
			return MutedStyle.Render("No Attention items match your search.")
		}
		return MutedStyle.Render("Nothing needs attention.")
	}

	rowWidth := max(cardWidth-4, 1)
	maxVisible := max(contentHeight, 1)
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

// homePageSize returns the number of rows a half/full page jump should move for
// the currently focused pane, matching that pane's visible-row window.
func (m *DashboardModel) homePageSize() int {
	if m == nil {
		return 1
	}
	_, chromeHeight := m.renderFilterBar()
	projectsHeight, attentionHeight := m.homePaneHeights(chromeHeight)
	if m.homePane == homePaneAttention {
		return max(attentionHeight-CardVerticalBorderOverhead, 1)
	}
	contentHeight := max(projectsHeight-CardVerticalBorderOverhead, 1)
	narrowMode := max(m.width-2, MinCardWidth) < DashboardNarrowModeWidth
	return max(m.projectListMaxVisible(contentHeight, narrowMode), 1)
}
