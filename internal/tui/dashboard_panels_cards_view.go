package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type dashboardPanelCard struct {
	title   string
	content string
	width   int
}

func renderDashboardCards(width int, wide bool, cfg CardConfig, cards []dashboardPanelCard) string {
	if len(cards) == 0 {
		return ""
	}
	if wide && len(cards) > 1 {
		var rendered []string
		usedWidth := 0
		for i, c := range cards {
			cardWidth := c.width
			if i == len(cards)-1 {
				cardWidth = width - usedWidth
			}
			usedWidth += cardWidth
			rendered = append(rendered, renderCard(c.title, c.content, cardWidth, cfg))
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	}

	var parts []string
	for _, c := range cards {
		parts = append(parts, renderCard(c.title, c.content, width, cfg))
	}
	return strings.Join(parts, "\n")
}
