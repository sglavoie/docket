package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

// footerHelpSeparator delimits the individual key/action items inside a footer
// help string (e.g. "j/k nav · Enter open · ? help"). It is the wrap boundary
// renderStatusBarFooter uses to keep help text readable at narrow widths.
const footerHelpSeparator = " · "

// renderStatusBarFooter renders a horizontal divider followed by a left/right aligned
// status line. This is the standard footer pattern for fullscreen views: a separator
// line with left-aligned title and right-aligned help text beneath it.
//
// When the left/right pair does not fit on one line, the multi-item help side (the
// one joined by " · ") is wrapped onto additional lines on its own side rather than
// clipped, so required help text stays fully visible at narrow widths. The other
// side (a short title/indicator/label) keeps its place on the first line. Only a
// genuinely unwrappable pair (no " · " on either side, or a single help item wider
// than the terminal) falls back to last-resort audited truncation.
//
// Pass an optional accent color to tint the separator line.
func renderStatusBarFooter(left, right string, width int, accent ...color.Color) string {
	if width <= 0 {
		width = 80
	}
	var separatorStyle lipgloss.Style
	if len(accent) > 0 && accent[0] != nil {
		separatorStyle = lipgloss.NewStyle().Foreground(accent[0])
	} else {
		separatorStyle = MutedStyle
	}
	separator := separatorStyle.Render(strings.Repeat("─", width))

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)

	// Fast path: everything fits on one line. A gap is required only when both
	// sides are present; a width-fitted metadata line may legitimately consume
	// the entire row.
	minGap := 0
	if left != "" && right != "" {
		minGap = 1
	}
	if leftWidth+rightWidth+minGap <= width {
		gap := max(width-leftWidth-rightWidth, minGap)
		spacer := ThemeSpacer(gap)
		return fmt.Sprintf("%s\n%s", separator, lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right))
	}

	// Overflow: pick the wrappable help side (the one with more " · " items) and
	// wrap it instead of clipping. The other side is treated as a fixed label.
	leftHelp := strings.Count(ansi.Strip(left), footerHelpSeparator)
	rightHelp := strings.Count(ansi.Strip(right), footerHelpSeparator)

	// Neither side is a wrappable help string: fall back to last-resort audited
	// truncation of an indivisible label pair.
	if leftHelp == 0 && rightHelp == 0 {
		return separator + "\n" + truncateFooterLine(left, right, width)
	}

	helpOnLeft := leftHelp > rightHelp || (leftHelp == rightHelp && leftWidth >= rightWidth)
	help, label, labelWidth := right, left, leftWidth
	site, labelSite := "status footer right", "status footer left"
	if helpOnLeft {
		help, label, labelWidth = left, right, rightWidth
		site, labelSite = "status footer left", "status footer right"
	}

	// Constrain an over-wide fixed label before budgeting the first line. The
	// label shares row 0 with the first wrapped help chunk, which always keeps at
	// least one column plus a one-column gap; a label wider than width-2 would
	// otherwise retain its full width and push the joined row past the terminal.
	// Truncation here is audited so the render check flags chrome that could not
	// fit, matching the last-resort clip below.
	if label != "" && labelWidth > width-2 {
		label = truncateRequired(labelSite, label, max(width-2, 1))
		labelWidth = lipgloss.Width(label)
	}

	firstWidth := width
	if label != "" {
		firstWidth = max(width-labelWidth-1, 1)
	}
	helpLines := wrapStyledHelp(help, firstWidth, width)
	if len(helpLines) == 0 {
		helpLines = []string{""}
	}

	var b strings.Builder
	b.WriteString(separator)
	for i, line := range helpLines {
		b.WriteString("\n")

		// Last-resort clip of a single help item wider than its line budget.
		maxLine := width
		if i == 0 && label != "" {
			maxLine = firstWidth
		}
		if lipgloss.Width(line) > maxLine {
			line = truncateRequired(site, line, maxLine)
		}
		lineWidth := lipgloss.Width(line)

		if i == 0 && label != "" {
			gap := max(width-lineWidth-labelWidth, 1)
			spacer := ThemeSpacer(gap)
			if helpOnLeft {
				b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, line, spacer, label))
			} else {
				b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, label, spacer, line))
			}
			continue
		}
		if helpOnLeft {
			b.WriteString(line)
		} else {
			// Right-align help lines when the help sits on the right side.
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, ThemeSpacer(max(width-lineWidth, 0)), line))
		}
	}
	return b.String()
}

// truncateFooterLine renders the legacy single-line footer with last-resort
// audited truncation. It is used only when neither side can be wrapped on
// " · " boundaries (both sides are indivisible labels).
func truncateFooterLine(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth+1 > width {
		left = truncateRequired("status footer left", left, max(width-rightWidth-1, 1))
		leftWidth = lipgloss.Width(left)
		if leftWidth+rightWidth+1 > width {
			right = truncateRequired("status footer right", right, max(width-leftWidth-1, 1))
			rightWidth = lipgloss.Width(right)
		}
	}
	gap := max(width-leftWidth-rightWidth, 1)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, ThemeSpacer(gap), right)
}

// wrapStyledHelp splits an ANSI-styled footer help string on its " · " item
// boundaries and greedily packs items onto lines: the first line is bounded by
// firstWidth (to leave room for a same-line label), subsequent lines by width.
// It preserves the input's ANSI styling by slicing the original string with
// ansi.Cut on display-column boundaries, so no per-item restyling is required.
func wrapStyledHelp(styled string, firstWidth, width int) []string {
	if width <= 0 {
		width = 80
	}
	firstWidth = max(firstWidth, 1)

	sep := footerHelpSeparator
	sepW := lipgloss.Width(sep)
	items := strings.Split(ansi.Strip(styled), sep)
	if len(items) == 0 {
		return nil
	}

	var lines []string
	col := 0          // absolute display-column cursor into styled
	lineStartCol := 0 // start column of the line being built
	lineW := 0        // display width accumulated on the current line
	limit := firstWidth

	for _, item := range items {
		iw := lipgloss.Width(item)
		if lineW > 0 && lineW+sepW+iw > limit {
			lines = append(lines, ansi.Cut(styled, lineStartCol, col))
			col += sepW // skip the physical separator joining the two items
			lineStartCol = col
			col += iw
			lineW = iw
			limit = width
			continue
		}
		if lineW > 0 {
			col += sepW
			lineW += sepW
		}
		col += iw
		lineW += iw
	}
	lines = append(lines, ansi.Cut(styled, lineStartCol, col))
	return lines
}

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
