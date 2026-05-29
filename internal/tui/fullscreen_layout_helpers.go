package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// fullscreenContentHeight calculates the available height for content in a
// fullscreen tabbed view. It subtracts the standard chrome overhead: title line (1),
// newline after title (1), and status bar footer (2), for a fixed cost of 4 lines.
// The tabBarOverhead parameter accounts for the tab bar height plus any additional
// header lines such as blank separators or filter badges.
func fullscreenContentHeight(totalHeight, tabBarOverhead int) int {
	const fixedOverhead = 4 // title(1) + newline(1) + footer(2)
	h := max(totalHeight-fixedOverhead-tabBarOverhead, 1)
	return h
}

// renderStatusBarFooter renders a horizontal divider followed by a left/right aligned
// status line. This is the standard footer pattern for fullscreen views: a separator
// line with left-aligned title and right-aligned help text beneath it.
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
	if leftWidth+rightWidth+1 > width {
		availableLeft := max(width-rightWidth-1, 1)
		left = ansi.Truncate(left, availableLeft, "")
		leftWidth = lipgloss.Width(left)
		if leftWidth+rightWidth+1 > width {
			right = ansi.Truncate(right, max(width-leftWidth-1, 1), "")
			rightWidth = lipgloss.Width(right)
		}
	}

	gap := max(width-leftWidth-rightWidth, 1)
	spacer := ThemeSpacer(gap)

	return fmt.Sprintf("%s\n%s", separator, lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right))
}

// DividerOption configures optional behavior for renderHorizontalDivider.
type DividerOption func(*dividerConfig)

type dividerConfig struct {
	color          color.Color
	rightIndicator string
}

// WithDividerColor overrides the default muted color.
func WithDividerColor(c color.Color) DividerOption {
	return func(cfg *dividerConfig) {
		cfg.color = c
	}
}

// WithRightIndicator places text right-aligned within the divider line.
func WithRightIndicator(indicator string) DividerOption {
	return func(c *dividerConfig) {
		c.rightIndicator = indicator
	}
}

// renderHorizontalDivider renders a full-width ─── line with optional styling and
// embedded content. Use WithDividerColor to change the color and WithRightIndicator
// to embed right-aligned text (e.g., a scroll position indicator).
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

	dividerLine := strings.Repeat("─", width)
	if indicatorWidth := lipgloss.Width(cfg.rightIndicator); cfg.rightIndicator != "" && indicatorWidth < width {
		dividerLine = strings.Repeat("─", width-indicatorWidth) + cfg.rightIndicator
	}

	return style.Render(dividerLine)
}

// renderFullscreenChrome renders the tab bar + optional filter badge + trailing
// blank line that sits between the title and the content area. It returns the
// rendered string and the number of lines consumed (for content height calc).
func renderFullscreenChrome(tabBar, filterBadge string, trailingBlank bool) (string, int) {
	var b strings.Builder
	b.WriteString(tabBar)
	b.WriteString("\n")

	if filterBadge != "" {
		b.WriteString(filterBadge)
		b.WriteString("\n")
	}
	if trailingBlank {
		b.WriteString("\n")
	}

	lines := lipgloss.Height(tabBar) // tab bar may wrap to multiple rows
	if filterBadge != "" {
		lines++
	}
	if trailingBlank {
		lines++
	}
	return b.String(), lines
}

// renderFullscreenFooterBlock renders the "\n\n" + status messages + footer bar
// sequence that most fullscreen views share. The caller supplies the status
// message, error, and the left/right footer strings already styled.
// Pass an optional accent color to tint the separator line.
func renderFullscreenFooterBlock(statusMsg string, err error, footerLeft, footerRight string, width int, accent ...color.Color) string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(renderStatusMessages(statusMsg, err))
	b.WriteString(renderStatusBarFooter(footerLeft, footerRight, width, accent...))
	return b.String()
}

// renderStatusMessages renders the standard status/error message block used by
// fullscreen views. Returns an empty string when neither message is set.
func renderStatusMessages(statusMessage string, err error) string {
	var b strings.Builder
	if statusMessage != "" {
		b.WriteString(SuccessStyle.Render(statusMessage))
		b.WriteString("\n")
	}
	if err != nil {
		b.WriteString(ValidationErrorStyle.Render(err.Error()))
		b.WriteString("\n")
	}
	return b.String()
}

// renderGroupHeader renders a group header with count, using overdue styling when appropriate.
// If width > 0, a trailing horizontal rule fills the remaining line width.
func renderGroupHeader(name string, count int, isOverdue bool, width int) string {
	header := fmt.Sprintf("%s (%d)", name, count)
	var styled string
	if isOverdue {
		styled = OverdueStyle.Bold(true).Render(header)
	} else {
		styled = GroupHeaderStyle.Render(header)
	}
	visibleWidth := lipgloss.Width(styled)
	if width > visibleWidth+2 {
		styled += " " + MutedStyle.Render(strings.Repeat("─", width-visibleWidth-1))
	}
	return styled
}

// renderCollapsibleHeader renders a collapsible section/group header.
// It is a thin wrapper around renderSectionHeader that also applies selection
// highlighting. Use renderSectionHeader directly when callers need to apply
// their own highlighting (e.g. ZebraLine, MutationFlashLine).
func renderCollapsibleHeader(name string, count int, collapsed, isSelected, flash bool, style lipgloss.Style, width int) string {
	line := renderSectionHeader(SectionHeaderOpts{
		Name:        name,
		Count:       count,
		Collapsed:   collapsed,
		AccentColor: style,
		Width:       width,
	})
	if isSelected {
		if flash {
			return SelectedFlashLine(line, 0)
		}
		return SelectedLine(line, 0)
	}
	return line
}

// Spacing rule: 1 blank line between sections, 0 blank lines between rows
// within a section. Use renderEmptyState / renderFilterAwareEmptyState for
// empty-state blocks so spacing is embedded and not duplicated at call sites.

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

// renderContextualEmptyState renders an empty-state block with view-specific
// action hints. hints are formatted as "key action" pairs joined by " · ".
// Only call this when there are no tasks, no active filters, and no pending prefix.
func renderContextualEmptyState(msg string, hints []string) string {
	s := MutedStyle.Render(msg)
	if len(hints) > 0 {
		s += "\n\n" + HelpStyle.Render(strings.Join(hints, " · "))
	}
	return s
}

// renderFilterAwareEmptyState returns an empty-state block that adapts its message
// based on whether filters are active. When hasFilters is true, it shows the
// standard "No tasks match current filters" message with filterHelpText; otherwise
// it shows baseMsg with baseHelpText.
func renderFilterAwareEmptyState(hasFilters bool, baseMsg, baseHelpText, filterHelpText string) string {
	if hasFilters {
		return renderEmptyState(EmptyStateFiltered, filterHelpText)
	}
	return renderEmptyState(baseMsg, baseHelpText)
}
