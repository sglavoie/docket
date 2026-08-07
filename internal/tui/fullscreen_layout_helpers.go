package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// fitBlockHeight truncates or pads a rendered block to an exact line count.
func fitBlockHeight(block string, height int) string {
	if height <= 0 {
		return ""
	}
	block = strings.TrimRight(block, "\n")
	lines := []string{""}
	if block != "" {
		lines = strings.Split(block, "\n")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// fullscreenContentHeightForBlocks returns the largest content-block height (in
// lines) such that the surrounding chrome/footer blocks, joined vertically with
// an h-line content block, still fit within totalHeight. Unlike
// fullscreenContentHeight, which assumes a fixed two-line footer, it budgets
// against the actual rendered height of every surrounding block — including a
// footer that wraps beyond two rows at narrow widths. Pass the already-rendered
// blocks that flank the content in the same lipgloss.JoinVertical the caller
// renders (e.g. title, tab bar, footer). This mirrors metricsContentHeight and
// settingsContentHeight, which now share this helper.
func fullscreenContentHeightForBlocks(totalHeight int, blocks ...string) int {
	for h := max(totalHeight, 1); h >= 1; h-- {
		sized := make([]string, 0, len(blocks)+1)
		sized = append(sized, blocks...)
		sized = append(sized, fitBlockHeight("", h))
		if lipgloss.Height(lipgloss.JoinVertical(lipgloss.Left, sized...)) <= totalHeight {
			return h
		}
	}
	return 1
}

// footerHelpSeparator delimits complete key/action entries in structured
// guidance after they have been selected for the current row budget.
const footerHelpSeparator = " · "

// DividerOption configures optional behavior for renderHorizontalDivider.
type DividerOption func(*dividerConfig)

type dividerConfig struct {
	color color.Color
}

// WithDividerColor overrides the default muted color.
func WithDividerColor(c color.Color) DividerOption {
	return func(cfg *dividerConfig) {
		cfg.color = c
	}
}

// renderHorizontalDivider renders a full-width ─── line with optional styling.
// Use WithDividerColor to change the color.
func renderHorizontalDivider(width int, opts ...DividerOption) string {
	if width <= 0 {
		width = 80
	}

	cfg := dividerConfig{
		color: ColorMuted,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	style := lipgloss.NewStyle().Foreground(cfg.color)

	return style.Render(strings.Repeat("─", width))
}

// Spacing rule: 1 blank line between sections, 0 blank lines between rows
// within a section. Use renderEmptyState for empty-state blocks so spacing is
// embedded and not duplicated at call sites.

// renderEmptyState returns the standard empty-state block used when a view has
// no visible tasks. msg is rendered in MutedStyle; helpText (if non-empty) is
// appended below in HelpStyle with a blank-line separator.
func renderEmptyState(msg string, helpText string) string {
	s := MutedStyle.Render(msg)
	if helpText != "" {
		s += "\n\n" + HelpStyle.Render(helpText)
	}
	return s
}
