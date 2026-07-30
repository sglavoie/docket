package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// CardConfig holds configuration for card rendering.
type CardConfig struct {
	Style       lipgloss.Style
	TitleStyle  lipgloss.Style
	Focused     bool
	AccentColor color.Color // optional section color for border
	Height      int         // optional fixed height (0 = auto)
}

// DefaultCardConfig returns the default card configuration for panel cards.
func DefaultCardConfig() CardConfig {
	return CardConfig{
		Style:      PanelCardStyle,
		TitleStyle: PanelCardTitleStyle,
		Focused:    false,
	}
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

	// Truncate - dense card content is allowed to clip, so use the intentional
	// ANSI-aware audited helper.
	truncated := truncateIntentional(line, maxWidth-3, "")
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
		// The titled border is header chrome, not card body data: a clipped
		// title/frame should be suspicious, so record a required-text audit.
		result = truncateRequired("card title border", result, width-1) + borderStyle.Render("╮")
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
		// Bottom titled border is footer chrome; audit clips like the top border.
		result = truncateRequired("card footer border", result, width-1) + borderStyle.Render("╯")
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

// renderClusterFrame renders a single outer card titled `label` that groups
// multiple bare inner columns side-by-side. Inner columns should be produced
// by renderClusterColumn (or any caller that pre-pads each column to its
// target visual width) and contain no card border of their own — the outer
// frame supplies the only border, so the cluster reads as one chunk under
// the Law of Proximity.
func renderClusterFrame(label string, innerColumns []string, width int) string {
	if len(innerColumns) == 0 {
		return ""
	}
	inner := lipgloss.JoinHorizontal(lipgloss.Top, innerColumns...)
	return renderCard(label, inner, width, DefaultCardConfig())
}

// renderClusterColumn formats a single column for use inside renderClusterFrame.
// It pads `content` to the given visual width and prepends a styled title
// heading so the column remains identifiable without its own border.
func renderClusterColumn(title, content string, width int) string {
	if width < 1 {
		width = 1
	}
	heading := PanelCardTitleStyle.Render(title)
	body := heading + "\n" + content
	return lipgloss.NewStyle().Width(width).Padding(0, 1).Render(body)
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

	// Render the content in a box with proper width handling. Lipgloss can let
	// explicit long lines expand a bordered block, so pre-wrap against the
	// inner body width before drawing the frame.
	content = boundToWidth(content, cardInnerWidth(style, width))
	renderStyle := style.Width(max(width, 1))
	rendered := renderStyle.Render(content)

	// Split rendered into lines to replace top border with titled version
	lines := strings.Split(rendered, "\n")
	if len(lines) < 1 {
		return rendered
	}

	// Enforce fixed height by padding or truncating lines, preserving the bottom border.
	if config.Height > 0 {
		targetHeight := max(config.Height, CardVerticalBorderOverhead)
		if len(lines) < targetHeight {
			bottom := lines[len(lines)-1]
			padLine := renderCardBlankLine(renderStyle)
			for len(lines) < targetHeight {
				lines = append(lines[:len(lines)-1], padLine, bottom)
			}
		}
		if len(lines) > targetHeight {
			truncated := make([]string, 0, targetHeight)
			truncated = append(truncated, lines[:targetHeight-1]...)
			truncated = append(truncated, lines[len(lines)-1])
			lines = truncated
		}
	}

	// Replace the top border line with titled version
	actualWidth := max(width, 1)
	if renderedWidth := lipgloss.Width(lines[0]); renderedWidth > 0 && renderedWidth < actualWidth {
		actualWidth = renderedWidth
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleText := titleStyle.Render(title)
	lines[0] = renderTitledTopBorder(titleText, actualWidth, borderStyle)

	return strings.Join(lines, "\n")
}

func cardInnerWidth(style lipgloss.Style, outerWidth int) int {
	return max(outerWidth-style.GetHorizontalFrameSize(), 1)
}

func renderCardBlankLine(style lipgloss.Style) string {
	lines := strings.Split(style.Render(""), "\n")
	if len(lines) >= 3 {
		return lines[1]
	}
	if len(lines) > 0 {
		return lines[0]
	}
	return ""
}
