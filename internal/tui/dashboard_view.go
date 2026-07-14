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

	visible := m.visibleProjects()
	b.WriteString(m.renderDashboard(visible, chromeHeight))

	return tea.NewView(b.String())
}

// renderDashboard dispatches between the configured dashboard layouts.
// Horizontal stacks the project list on top of the bottom info section,
// Vertical renders them side-by-side, and ListOnly hides the bottom section.
// Vertical layout falls back to horizontal on narrow terminals; an active
// search state or InfoPanelOff forces the list-only path regardless of layout.
func (m *DashboardModel) renderDashboard(visible []*models.Project, chromeHeight int) string {
	layout := m.runtime.Machine().GetDashboardBottomLayout()
	panelMode := m.runtime.Machine().GetInfoPanelMode()

	if layout == config.DashboardLayoutListOnly || panelMode == config.InfoPanelOff || m.HasActiveSearch() {
		return m.renderDashboardListOnly(visible, chromeHeight)
	}

	if layout == config.DashboardLayoutVertical && m.width >= DashboardInfoSectionWideMinWidth {
		return m.renderDashboardVertical(visible, chromeHeight, panelMode)
	}
	return m.renderDashboardHorizontal(visible, chromeHeight, panelMode)
}

func (m *DashboardModel) projectListPageSize() int {
	if m == nil {
		return 1
	}
	_, chromeHeight := m.renderFilterBar()
	cardWidth, bottomHeight := m.projectListViewportInputs(chromeHeight)
	narrowMode := cardWidth < DashboardNarrowModeWidth
	return m.projectListMaxVisible(chromeHeight, bottomHeight, narrowMode)
}

func (m *DashboardModel) projectListViewportInputs(chromeHeight int) (cardWidth, bottomHeight int) {
	cardWidth = max(m.width-2, MinCardWidth)
	if m.runtime == nil {
		return cardWidth, 0
	}

	layout := m.runtime.Machine().GetDashboardBottomLayout()
	panelMode := m.runtime.Machine().GetInfoPanelMode()
	if layout == config.DashboardLayoutListOnly || panelMode == config.InfoPanelOff || m.HasActiveSearch() {
		return cardWidth, 0
	}

	if layout == config.DashboardLayoutVertical && m.width >= DashboardInfoSectionWideMinWidth {
		totalWidth := max(m.width-2, MinCardWidth)
		pct := m.runtime.Machine().GetDashboardVerticalSplitPct()
		listWidth := max(min(max(totalWidth*pct/100, TreeMinWidth), totalWidth-MinCardWidth), TreeMinWidth)
		return listWidth, 0
	}

	pct := m.runtime.Machine().GetDashboardHorizontalSplitPct()
	totalHeight := max(m.height-chromeHeight, 0)
	paneHeight := max(totalHeight*(100-pct)/100, DashboardBottomPaneHeight)

	selected := m.SelectedProject()
	var infoContent string
	switch panelMode {
	case config.InfoPanelOff:
	case config.InfoPanelInfoCards:
		infoContent = m.renderInfoSection(cardWidth, paneHeight, selected)
	case config.InfoPanelProjectSummary:
		infoContent = m.renderProjectSummarySection(cardWidth, paneHeight, selected, false)
	}
	if infoContent == "" {
		return cardWidth, 0
	}
	return cardWidth, lipgloss.Height(infoContent) + 1
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

	pct := m.runtime.Machine().GetDashboardHorizontalSplitPct()
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
	pct := m.runtime.Machine().GetDashboardVerticalSplitPct()

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
	if streak := m.runtime.Planning().GetStreakCount(); streak > 0 && isStreakActive(m.runtime.Planning().GetStreakLastDate()) {
		title += fmt.Sprintf("  %s %dd", icons.Get("streak"), streak)
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
