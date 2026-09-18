package chrome

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// HostedPanelLayout divides one content area between a bounded panel pinned to
// its bottom edge and the presentation that panel was opened over.
//
// HostRows is what the host keeps, and it is the frame less the rows the
// panel's own settled content asks for, bounded by that panel's ceiling and the
// host's floor. It moves when that content does, which is why the rows are a
// loan the host is told about rather than a resize: a list surface drops its
// scroll window every time it is resized, so a host repaying a loan restores
// the window it was reading instead of re-anchoring it under every keystroke. A
// panel that needs more room than its share takes the whole area for that frame
// — HostDrawn false — instead of shrinking the host again.
//
// The zero value is no panel at all: the view owns the content area outright,
// which is how one view declares both of its presentations without a second
// type.
type HostedPanelLayout struct {
	// HostRows is the content height the presentation underneath is rendered
	// into. Zero means there is no panel.
	HostRows int
	// HostDrawn reports whether that presentation is painted this frame.
	HostDrawn bool
}

// Active reports whether the layout describes a panel drawn over a host.
func (l HostedPanelLayout) Active() bool { return l.HostRows > 0 }

// PanelRows is the panel's own height inside a content area of the given
// height: its share when the host is drawn, and the whole area when it is not.
func (l HostedPanelLayout) PanelRows(height int) int {
	if !l.Active() || !l.HostDrawn {
		return max(height, 1)
	}
	return max(height-l.HostRows, 1)
}

// ComposeHostedPanel stacks a bounded panel under the host it is drawn over.
// The host is padded or cut to exactly hostRows first, so the panel sits on the
// same screen row whatever the host had to say.
func ComposeHostedPanel(host, panel string, hostRows int) string {
	panel = strings.TrimRight(panel, "\n")
	if hostRows < 1 {
		return panel
	}
	return FitBlockHeight(host, hostRows) + "\n" + panel
}

// RenderLabeledDivider renders a divider that names the block it opens:
// `── label ─────`. It is the heading form a bounded panel owns, where a
// fullscreen title row would spend a row the host needs. An empty label, or a
// width with no room for one, degrades to the plain divider.
func RenderLabeledDivider(label string, width int, opts ...DividerOption) string {
	if width <= 0 {
		width = 80
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return RenderHorizontalDivider(width, opts...)
	}

	cfg := dividerConfig{color: theme.ColorMuted, glyph: '─'}
	for _, opt := range opts {
		opt(&cfg)
	}
	const leadGlyphs = 2
	spent := leadGlyphs + lipgloss.Width(label) + 2
	if spent+1 > width {
		return RenderHorizontalDivider(width, opts...)
	}

	// Built here rather than taken from theme.SubtitleStyle: that style carries a
	// bottom margin, which on a single row is a newline through the middle of the
	// divider.
	rule := lipgloss.NewStyle().Foreground(cfg.color)
	name := lipgloss.NewStyle().Foreground(theme.ColorSecondary)
	return rule.Render(strings.Repeat(string(cfg.glyph), leadGlyphs)) +
		" " + name.Render(label) + " " +
		rule.Render(strings.Repeat(string(cfg.glyph), width-spent))
}
