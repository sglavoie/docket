package chrome

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/theme"
)

// FitBlockHeight truncates or pads a rendered block to an exact line count.
func FitBlockHeight(block string, height int) string {
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

// FullscreenContentHeightForBlocks returns the largest content-block height (in
// lines) such that the surrounding chrome/footer blocks, joined vertically with
// an h-line content block, still fit within totalHeight. Unlike
// fullscreenContentHeight, which assumes a fixed two-line footer, it budgets
// against the actual rendered height of every surrounding block — including a
// footer that wraps beyond two rows at narrow widths. Pass the already-rendered
// blocks that flank the content in the same lipgloss.JoinVertical the caller
// renders (e.g. title, tab bar, footer). This mirrors metricsContentHeight and
// settingsContentHeight, which now share this helper.
func FullscreenContentHeightForBlocks(totalHeight int, blocks ...string) int {
	for h := max(totalHeight, 1); h >= 1; h-- {
		sized := make([]string, 0, len(blocks)+1)
		sized = append(sized, blocks...)
		sized = append(sized, FitBlockHeight("", h))
		if lipgloss.Height(lipgloss.JoinVertical(lipgloss.Left, sized...)) <= totalHeight {
			return h
		}
	}
	return 1
}

// FooterHelpSeparator delimits complete key/action entries in structured
// guidance after they have been selected for the current row budget.
const FooterHelpSeparator = " · "

// DividerOption configures optional behavior for RenderHorizontalDivider.
type DividerOption func(*dividerConfig)

type dividerConfig struct {
	color color.Color
	glyph rune
}

// WithDividerColor overrides the default muted color.
func WithDividerColor(c color.Color) DividerOption {
	return func(cfg *dividerConfig) {
		cfg.color = c
	}
}

// WithDividerGlyph overrides the default thin horizontal line glyph.
func WithDividerGlyph(glyph rune) DividerOption {
	return func(cfg *dividerConfig) {
		cfg.glyph = glyph
	}
}

// RenderHorizontalDivider renders a full-width ─── line with optional styling.
// Use WithDividerColor and WithDividerGlyph to change its appearance.
func RenderHorizontalDivider(width int, opts ...DividerOption) string {
	if width <= 0 {
		width = 80
	}

	cfg := dividerConfig{
		color: theme.ColorMuted,
		glyph: '─',
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	style := lipgloss.NewStyle().Foreground(cfg.color)

	return style.Render(strings.Repeat(string(cfg.glyph), width))
}

// Spacing rule: 1 blank line between sections, 0 blank lines between rows
// within a section. Use RenderEmptyState for empty-state blocks so spacing is
// embedded and not duplicated at call sites.

// RenderEmptyState returns the standard empty-state block used when a view has
// no visible tasks. msg is rendered in MutedStyle; helpText (if non-empty) is
// appended below in HelpStyle with a blank-line separator.
func RenderEmptyState(msg string, helpText string) string {
	s := theme.MutedStyle.Render(msg)
	if helpText != "" {
		s += "\n\n" + theme.HelpStyle.Render(helpText)
	}
	return s
}
