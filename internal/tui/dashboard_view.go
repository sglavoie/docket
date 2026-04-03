package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func (m *DashboardModel) View() tea.View {
	var b strings.Builder

	filterBar, chromeHeight := m.renderFilterBar()
	b.WriteString(filterBar)

	visible := m.visibleProjects()
	b.WriteString(m.renderDashboard(visible, chromeHeight))

	return tea.NewView(b.String())
}

// renderDashboard renders the always-stacked dashboard: full-width projects on top,
// optional info section below.
func (m *DashboardModel) renderDashboard(visible []*models.Project, chromeHeight int) string {
	var b strings.Builder

	cardWidth := max(m.width-2, MinCardWidth)

	// Pre-render info section to measure its actual height
	var infoContent string
	bottomHeight := 0
	panelMode := m.config.GetInfoPanelMode()
	if panelMode != config.InfoPanelOff && m.SearchQuery() == "" {
		var selected *models.Project
		if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
			selected = visible[m.cursor]
		}
		switch panelMode {
		case config.InfoPanelInfoCards:
			infoContent = m.renderInfoSection(cardWidth, selected)
		case config.InfoPanelProjectSummary:
			infoContent = m.renderProjectSummarySection(cardWidth, selected)
		default:
		}
		if infoContent != "" {
			// +1 for the final line (Count gives separators), +1 for the gap newline
			bottomHeight = strings.Count(infoContent, "\n") + 2
		}
	}

	cardTitle := m.projectCardTitle(visible)
	cardContent := m.renderProjectList(visible, chromeHeight, cardWidth, bottomHeight)

	cfg := DashboardCardConfig()
	b.WriteString(renderCard(cardTitle, cardContent, cardWidth, cfg))

	if infoContent != "" {
		b.WriteString("\n")
		b.WriteString(infoContent)
	}

	return b.String()
}

// projectCardTitle returns the title for the project list card.
func (m *DashboardModel) projectCardTitle(visible []*models.Project) string {
	if m.showArchived {
		return fmt.Sprintf("Archived (%d)", len(visible))
	}
	return fmt.Sprintf("Projects (%d)", len(visible))
}
