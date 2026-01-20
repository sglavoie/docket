package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// CardConfig holds configuration for card rendering.
type CardConfig struct {
	Style       lipgloss.Style
	TitleStyle  lipgloss.Style
	Focused     bool
	AccentColor lipgloss.Color // optional section color for border
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

// OverlayCardConfig holds configuration for overlay card rendering.
type OverlayCardConfig struct {
	Title        string // Main title in the border
	Subtitle     string // Optional subtitle below title (e.g., "42 tasks")
	Width        int    // Desired width (0 = auto)
	Height       int    // Desired height (0 = auto)
	Centered     bool   // Whether to center in viewport
	NoTruncation bool   // If true, don't truncate lines (for forms with textareas)
}

// renderOverlayCard renders an overlay card with titled border and optional subtitle.
// This is used for popup/modal views like command palette, task jumper, filter view, etc.
// Returns the rendered card string.
func renderOverlayCard(content string, viewWidth, viewHeight int, config OverlayCardConfig) string {
	cardWidth := config.Width
	if cardWidth == 0 || cardWidth > viewWidth-4 {
		cardWidth = viewWidth - 4
	}
	if cardWidth < 20 {
		cardWidth = 20
	}

	// OverlayCardStyle has Border (2 chars) + Padding(1, 2) (4 chars horizontal) = 6 chars total
	contentWidth := cardWidth - 6

	// Build the styled content with subtitle if present
	var fullContent strings.Builder
	if config.Subtitle != "" {
		if config.NoTruncation {
			fullContent.WriteString(OverlaySubtitleStyle.Render(config.Subtitle))
		} else {
			fullContent.WriteString(truncateAndPad(OverlaySubtitleStyle.Render(config.Subtitle), contentWidth))
		}
		fullContent.WriteString("\n\n")
	}

	if config.NoTruncation {
		// For forms with textareas, don't truncate - let lipgloss handle width
		fullContent.WriteString(content)
	} else {
		// Truncate each line of content to prevent overflow
		contentLines := strings.Split(content, "\n")
		for i, line := range contentLines {
			contentLines[i] = truncateAndPad(line, contentWidth)
		}
		fullContent.WriteString(strings.Join(contentLines, "\n"))
	}

	// Render the content in a box
	var rendered string
	if config.NoTruncation {
		// For forms, use Width() to let lipgloss handle the box sizing
		rendered = OverlayCardStyle.Width(contentWidth).Render(fullContent.String())
	} else {
		// For pre-formatted content (split-panes, lists), don't use Width()
		// as it would cause word-wrapping that breaks our carefully constructed lines
		rendered = OverlayCardStyle.Render(fullContent.String())
	}

	// Split into lines to replace top border with titled version
	lines := strings.Split(rendered, "\n")
	if len(lines) < 1 {
		return rendered
	}

	// Measure the actual rendered width of the original border line
	actualWidth := lipgloss.Width(lines[0])

	// Build titled top border: ╭─ Title ─────╮
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	titleText := OverlayCardTitleStyle.Render(config.Title)
	titleVisualWidth := lipgloss.Width(titleText)

	// Calculate dashes to fill the remaining width
	// Format: ╭─ Title ─────╮
	// usedByTitle: "╭─ " (3) + title + " " (1) + "╮" (1) = 5 + titleVisualWidth
	usedByTitle := 5 + titleVisualWidth
	remainingDashes := actualWidth - usedByTitle
	if remainingDashes < 1 {
		remainingDashes = 1
	}

	newTopBorder := borderStyle.Render("╭─ ") + titleText + borderStyle.Render(" "+strings.Repeat("─", remainingDashes)+"╮")
	lines[0] = newTopBorder

	box := strings.Join(lines, "\n")

	// Center if requested
	if config.Centered {
		boxHeight := lipgloss.Height(box)
		topPadding := max((viewHeight-boxHeight)/2, 0)
		leftPadding := max((viewWidth-lipgloss.Width(box))/2, 0)

		var result strings.Builder
		for i := 0; i < topPadding; i++ {
			result.WriteString("\n")
		}
		padding := strings.Repeat(" ", leftPadding)
		for _, line := range strings.Split(box, "\n") {
			result.WriteString(padding)
			result.WriteString(line)
			result.WriteString("\n")
		}
		return result.String()
	}

	return box
}

// truncateLineVisual truncates a single line to maxWidth visual characters.
// It handles ANSI escape codes properly using charmbracelet/x/ansi.Truncate.
func truncateLineVisual(line string, maxWidth int) string {
	if maxWidth < 4 {
		maxWidth = 4
	}
	visualWidth := lipgloss.Width(line)
	if visualWidth <= maxWidth {
		return line
	}

	// Use ansi.Truncate which properly handles ANSI escape codes
	// It adds the tail ("...") and accounts for its width
	return ansi.Truncate(line, maxWidth, "...")
}

// truncateLines truncates each line in content to maxWidth visual characters.
// This ensures content doesn't wrap when rendered in a fixed-width pane.
func truncateLines(content string, maxWidth int) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = truncateLineVisual(line, maxWidth)
	}
	return strings.Join(lines, "\n")
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
	sep := OverlaySplitStyle.Render("│")

	for i := 0; i < height; i++ {
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

// renderCard renders a card with a titled border.
// Uses lipgloss styling for proper ANSI-aware rendering.
// When AccentColor is set, uses it for borders. When Focused with AccentColor,
// shows a blue title indicator while keeping the accent border.
func renderCard(title, content string, width int, config CardConfig) string {
	if width < 10 {
		width = 10
	}

	// Determine border and title colors based on accent and focus state
	var borderColor lipgloss.Color
	var titleStyle lipgloss.Style

	hasAccent := config.AccentColor != ""

	if hasAccent {
		// Use accent color for border
		borderColor = config.AccentColor
		if config.Focused {
			// Focused with accent: use blue title to indicate focus
			titleStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
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
		Width(width - 2). // Account for border
		Render(content)

	// Split rendered into lines to replace top border with titled version
	lines := strings.Split(rendered, "\n")
	if len(lines) < 1 {
		return rendered
	}

	// Measure the actual rendered width of the original border line
	actualWidth := lipgloss.Width(lines[0])

	// Replace the top border line with titled version
	// The first line is: ╭────────────────╮
	// We want: ╭─ Title ────────╮
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	titleText := titleStyle.Render(title)
	titleVisualWidth := lipgloss.Width(titleText)

	// Calculate dashes to fill the remaining width
	// Format: ╭─ Title ─────╮
	// usedByTitle: "╭─ " (3) + title + " " (1) + "╮" (1) = 5 + titleVisualWidth
	usedByTitle := 5 + titleVisualWidth
	remainingDashes := actualWidth - usedByTitle
	if remainingDashes < 1 {
		remainingDashes = 1
	}

	newTopBorder := borderStyle.Render("╭─ ") + titleText + borderStyle.Render(" "+strings.Repeat("─", remainingDashes)+"╮")
	lines[0] = newTopBorder

	return strings.Join(lines, "\n")
}
