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

	// Stacked layout: cards are joined vertically, so their heights add up.
	// cfg.Height is the budget for the whole section, not per card — divide it
	// across the cards so the total stays within budget instead of rendering
	// cfg.Height lines per card (which overflows narrow terminals).
	var parts []string
	for i, c := range cards {
		cardCfg := cfg
		if cfg.Height > 0 && len(cards) > 1 {
			perCard := max(cfg.Height/len(cards), CardVerticalBorderOverhead+1)
			cardCfg.Height = perCard
			if i == 0 {
				// Give any remainder to the first card so the heights sum to
				// the budget (only when the floor did not already exceed it).
				if remainder := cfg.Height - perCard*len(cards); remainder > 0 {
					cardCfg.Height = perCard + remainder
				}
			}
		}
		parts = append(parts, renderCard(c.title, c.content, width, cardCfg))
	}
	return strings.Join(parts, "\n")
}
