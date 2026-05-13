package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/icons"
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

	// Reserve fixed height for bottom info section, then render it
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
			// Fixed height: reserve space for bottom pane first, top pane gets the rest
			bottomHeight = DashboardBottomPaneHeight + 1 // +1 for the gap newline
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
	title := fmt.Sprintf("Projects (%d)", len(visible))
	if m.config.StreakCount > 0 && isStreakActive(m.config.StreakLastDate) {
		title += fmt.Sprintf("  %s %dd", icons.Get("streak"), m.config.StreakCount)
	}
	return title
}

// isStreakActive returns true if the streak date is today or yesterday (still continuable).
func isStreakActive(dateStr string) bool {
	if dateStr == "" {
		return false
	}
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	return dateStr == today || dateStr == yesterday
}
