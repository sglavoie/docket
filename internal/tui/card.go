package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// CardConfig holds configuration for card rendering.
type CardConfig struct {
	Style       lipgloss.Style
	TitleStyle  lipgloss.Style
	Focused     bool
	AccentColor color.Color // optional section color for border
	Height      int         // optional fixed height (0 = auto)
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

// renderOverlaySplitPane renders a split-pane layout for overlays (list on left, preview on right).
// Uses a styled vertical separator between panes.
func renderOverlaySplitPane(leftContent, rightContent string, leftWidth, rightWidth, height int) string {
	// Split into lines
	leftLines := strings.Split(leftContent, "\n")
	rightLines := strings.Split(rightContent, "\n")

	// Right pane has 1 char padding, so content area is rightWidth-1
	rightContentWidth := rightWidth - 1

	// Ensure we have exactly 'height' lines for each pane
	for len(leftLines) < height {
		leftLines = append(leftLines, "")
	}
	if len(leftLines) > height {
		leftLines = leftLines[:height]
	}
	for len(rightLines) < height {
		rightLines = append(rightLines, "")
	}
	if len(rightLines) > height {
		rightLines = rightLines[:height]
	}

	// Build the split pane line by line
	var result strings.Builder
	sep := OverlaySplitStyle.Render("┃")

	for i := range height {
		// Truncate left line to exact width and pad with spaces
		leftLine := truncateAndPad(leftLines[i], leftWidth)
		// Truncate right line to content width and pad with spaces (then add 1 space for padding)
		rightLine := " " + truncateAndPad(rightLines[i], rightContentWidth)

		result.WriteString(leftLine)
		result.WriteString(sep)
		result.WriteString(rightLine)
		if i < height-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// truncateAndPad truncates a line to exactly maxWidth visual characters,
// padding with spaces if shorter, truncating with "..." if longer.
func truncateAndPad(line string, maxWidth int) string {
	if maxWidth < 4 {
		maxWidth = 4
	}

	visualWidth := lipgloss.Width(line)

	if visualWidth == maxWidth {
		return line
	}

	if visualWidth < maxWidth {
		// Pad with spaces
		return line + strings.Repeat(" ", maxWidth-visualWidth)
	}

	// Truncate - use ansi.Truncate for ANSI-aware truncation
	truncated := ansi.Truncate(line, maxWidth-3, "")
	truncatedWidth := lipgloss.Width(truncated)
	// Add ellipsis and pad to exact width
	return truncated + "..." + strings.Repeat(" ", maxWidth-truncatedWidth-3)
}

// renderTitledTopBorder builds "╭─ Title ─────╮" for the given width.
// titleText should already be styled; borderStyle controls the frame characters.
func renderTitledTopBorder(titleText string, width int, borderStyle lipgloss.Style) string {
	titleVisualWidth := lipgloss.Width(titleText)
	remainingDashes := max(width-5-titleVisualWidth, 1)
	left := borderStyle.Render("╭─ ")
	right := borderStyle.Render(" " + strings.Repeat("─", remainingDashes) + "╮")
	result := left + titleText + right
	if lipgloss.Width(result) > width {
		result = ansi.Truncate(result, width-1, "") + borderStyle.Render("╮")
	}
	return result
}

// renderTitledBottomBorder builds "╰─ Footer ─────╯" for the given width.
// footerText should already be styled; borderStyle controls the frame characters.
func renderTitledBottomBorder(footerText string, width int, borderStyle lipgloss.Style) string {
	footerVisualWidth := lipgloss.Width(footerText)
	remainingDashes := max(width-5-footerVisualWidth, 1)
	left := borderStyle.Render("╰─ ")
	right := borderStyle.Render(" " + strings.Repeat("─", remainingDashes) + "╯")
	result := left + footerText + right
	if lipgloss.Width(result) > width {
		result = ansi.Truncate(result, width-1, "") + borderStyle.Render("╯")
	}
	return result
}

// renderTitledBox renders content inside a rounded-border box with a titled top
// border and a titled bottom border (footer). title and footer must already be
// styled; borderStyle controls the frame characters and BorderForeground color.
func renderTitledBox(content string, innerWidth int, title, footer string, borderStyle lipgloss.Style) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderStyle.GetForeground()).
		Padding(0, 1).
		Width(innerWidth)

	rendered := boxStyle.Render(content)

	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}

	actualWidth := lipgloss.Width(lines[0])
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	lines[len(lines)-1] = renderTitledBottomBorder(footer, actualWidth, borderStyle)

	return strings.Join(lines, "\n")
}

// renderCard renders a card with a titled border.
// Uses lipgloss styling for proper ANSI-aware rendering.
// When AccentColor is set, uses it for borders. When Focused with AccentColor,
// shows a blue title indicator while keeping the accent border.
func renderCard(title, content string, width int, config CardConfig) string {
	if width < 10 {
		width = 10
	}

	// Determine border and title colors based on accent and focus state
	var borderColor color.Color
	var titleStyle lipgloss.Style

	hasAccent := config.AccentColor != nil

	if hasAccent {
		// Use accent color for border
		borderColor = config.AccentColor
		if config.Focused {
			// Focused with accent: use blue title to indicate focus
			titleStyle = TitleStyle
		} else {
			// Accent but not focused: use accent color for title too
			titleStyle = lipgloss.NewStyle().Foreground(config.AccentColor)
		}
	} else {
		// No accent: use standard focus behavior
		if config.Focused {
			borderColor = ColorPrimary
			titleStyle = config.TitleStyle
		} else {
			borderColor = ColorBorder
			titleStyle = config.TitleStyle
		}
	}

	// Apply border color to the style
	style := config.Style.BorderForeground(borderColor)

	// Render the content in a box with proper width handling
	rendered := style.
		Width(max(width, 1)).
		Render(content)

	// Split rendered into lines to replace top border with titled version
	lines := strings.Split(rendered, "\n")
	if len(lines) < 1 {
		return rendered
	}

	// Enforce fixed height by truncating lines, preserving the bottom border
	if config.Height > 0 && len(lines) > config.Height {
		truncated := make([]string, 0, config.Height)
		truncated = append(truncated, lines[:config.Height-1]...)
		truncated = append(truncated, lines[len(lines)-1])
		lines = truncated
	}

	// Replace the top border line with titled version
	actualWidth := lipgloss.Width(lines[0])
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleText := titleStyle.Render(title)
	lines[0] = renderTitledTopBorder(titleText, actualWidth, borderStyle)

	return strings.Join(lines, "\n")
}
