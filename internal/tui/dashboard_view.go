package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

func (m *DashboardModel) View() tea.View {
	var b strings.Builder

	filterBar, chromeHeight := m.renderFilterBar()
	b.WriteString(filterBar)

	strip, stripHeight := m.renderDashboardRecommendationStrip()
	b.WriteString(strip)
	chromeHeight += stripHeight

	visible := m.visibleProjects()
	b.WriteString(m.renderDashboard(visible, chromeHeight))

	return tea.NewView(b.String())
}

// renderDashboard dispatches between the configured dashboard layouts.
// Horizontal stacks the project list on top of the bottom info section,
// Vertical renders them side-by-side, and ListOnly hides the bottom section.
// Vertical layout falls back to horizontal on narrow terminals; an active
// search query or InfoPanelOff forces the list-only path regardless of layout.
func (m *DashboardModel) renderDashboard(visible []*models.Project, chromeHeight int) string {
	layout := m.config.GetDashboardBottomLayout()
	panelMode := m.config.GetInfoPanelMode()

	if layout == config.DashboardLayoutListOnly || panelMode == config.InfoPanelOff || m.SearchQuery() != "" {
		return m.renderDashboardListOnly(visible, chromeHeight)
	}

	if layout == config.DashboardLayoutVertical && m.width >= DashboardInfoSectionWideMinWidth {
		return m.renderDashboardVertical(visible, chromeHeight, panelMode)
	}
	return m.renderDashboardHorizontal(visible, chromeHeight, panelMode)
}

// renderDashboardListOnly renders the project list at full width with no bottom info section.
func (m *DashboardModel) renderDashboardListOnly(visible []*models.Project, chromeHeight int) string {
	cardWidth := max(m.width-2, MinCardWidth)
	cardTitle := m.projectCardTitle(visible)
	cardContent := m.renderProjectList(visible, chromeHeight, cardWidth, 0)
	cfg := DashboardCardConfig()
	return renderCard(cardTitle, cardContent, cardWidth, cfg)
}

// renderDashboardHorizontal renders the classic stacked layout: full-width
// project list on top, info section below. The info pane height is derived
// from GetDashboardHorizontalSplitPct (project-list share) and clamped against
// DashboardBottomPaneHeight as a floor.
func (m *DashboardModel) renderDashboardHorizontal(visible []*models.Project, chromeHeight int, panelMode config.InfoPanelMode) string {
	var b strings.Builder

	cardWidth := max(m.width-2, MinCardWidth)

	pct := m.config.GetDashboardHorizontalSplitPct()
	totalHeight := max(m.height-chromeHeight, 0)
	paneHeight := max(totalHeight*(100-pct)/100, DashboardBottomPaneHeight)

	var selected *models.Project
	if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
		selected = visible[m.cursor]
	}

	var infoContent string
	switch panelMode {
	case config.InfoPanelOff:
	case config.InfoPanelInfoCards:
		infoContent = m.renderInfoSection(cardWidth, paneHeight, selected)
	case config.InfoPanelProjectSummary:
		infoContent = m.renderProjectSummarySection(cardWidth, paneHeight, selected, false)
	}

	bottomHeight := 0
	if infoContent != "" {
		bottomHeight = lipgloss.Height(infoContent) + 1
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

// renderDashboardVertical renders the project list and info section side by side.
// Pane widths are derived from GetDashboardVerticalSplitPct (project-list share).
// Both panes share the full available height; the info column auto-stacks
// because its width sits below DashboardInfoSectionWideMinWidth in practice.
func (m *DashboardModel) renderDashboardVertical(visible []*models.Project, chromeHeight int, panelMode config.InfoPanelMode) string {
	totalWidth := max(m.width-2, MinCardWidth)
	pct := m.config.GetDashboardVerticalSplitPct()

	listWidth := max(min(max(totalWidth*pct/100, TreeMinWidth), totalWidth-MinCardWidth), TreeMinWidth)
	infoWidth := max(totalWidth-listWidth, MinCardWidth)

	paneHeight := max(m.height-chromeHeight-CardVerticalBorderOverhead, DashboardBottomPaneHeight)

	var selected *models.Project
	if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
		selected = visible[m.cursor]
	}

	var infoContent string
	switch panelMode {
	case config.InfoPanelOff:
	case config.InfoPanelInfoCards:
		infoContent = m.renderInfoSection(infoWidth, paneHeight, selected)
	case config.InfoPanelProjectSummary:
		infoContent = m.renderProjectSummarySection(infoWidth, paneHeight, selected, true)
	}

	cardTitle := m.projectCardTitle(visible)
	cardContent := m.renderProjectList(visible, chromeHeight, listWidth, 0)
	cfg := DashboardCardConfig()
	listCard := renderCard(cardTitle, cardContent, listWidth, cfg)

	if infoContent == "" {
		return listCard
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, listCard, infoContent)
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
