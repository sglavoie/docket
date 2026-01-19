package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// CardConfig holds configuration for card rendering.
type CardConfig struct {
	Style      lipgloss.Style
	TitleStyle lipgloss.Style
	Focused    bool
}

// DefaultCardConfig returns the default card configuration for sidebar cards.
func DefaultCardConfig() CardConfig {
	return CardConfig{
		Style:      SidebarCardStyle,
		TitleStyle: SidebarCardTitleStyle,
		Focused:    false,
	}
}

// DashboardCardConfig returns config for full-width dashboard cards.
func DashboardCardConfig() CardConfig {
	return CardConfig{
		Style:      DashboardCardStyle,
		TitleStyle: DashboardCardTitleStyle,
		Focused:    false,
	}
}

// renderCard renders a card with a titled border.
// Uses lipgloss styling for proper ANSI-aware rendering.
func renderCard(title, content string, width int, config CardConfig) string {
	if width < 10 {
		width = 10
	}

	// Choose border color based on focus state
	borderColor := ColorBorder
	if config.Focused {
		borderColor = ColorPrimary
	}

	// Apply border color to the style
	style := config.Style.BorderForeground(borderColor)

	// Render the content in a box with proper width handling
	rendered := style.
		Width(width - 2). // Account for border
		Render(content)

	// Split rendered into lines to replace top border with titled version
	lines := strings.Split(rendered, "\n")
	if len(lines) < 1 {
		return rendered
	}

	// Replace the top border line with titled version
	// The first line is: ╭────────────────╮
	// We want: ╭─ Title ────────╮
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	titleText := config.TitleStyle.Render(title)
	titleVisualWidth := lipgloss.Width(titleText)

	// Build new top border: ╭─ Title ─────╮
	// Inner width for dashes = width - 2 (corners)
	innerDashWidth := width - 2
	usedByTitle := 2 + titleVisualWidth + 1 // "─ " + title + " "
	remainingDashes := innerDashWidth - usedByTitle
	if remainingDashes < 1 {
		remainingDashes = 1
	}

	newTopBorder := borderStyle.Render("╭─ ") + titleText + borderStyle.Render(" "+strings.Repeat("─", remainingDashes)+"╮")
	lines[0] = newTopBorder

	return strings.Join(lines, "\n")
}
