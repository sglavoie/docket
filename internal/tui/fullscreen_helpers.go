package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderStatusBarFooter renders a horizontal divider followed by a left/right aligned
// status line. This is the standard footer pattern for fullscreen views: a separator
// line with left-aligned title and right-aligned help text beneath it.
func renderStatusBarFooter(left, right string, width int) string {
	separator := MutedStyle.Render(strings.Repeat("─", width))

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	spacer := lipgloss.NewStyle().Width(gap).Render("")

	return fmt.Sprintf("%s\n%s", separator, lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right))
}

// DividerOption configures optional behavior for renderHorizontalDivider.
type DividerOption func(*dividerConfig)

type dividerConfig struct {
	color          lipgloss.TerminalColor
	rightIndicator string
}

// WithDividerColor overrides the default muted color.
func WithDividerColor(color lipgloss.TerminalColor) DividerOption {
	return func(c *dividerConfig) {
		c.color = color
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
	if cfg.rightIndicator != "" && len(cfg.rightIndicator) < width {
		dividerLine = strings.Repeat("─", width-len(cfg.rightIndicator)) + cfg.rightIndicator
	}

	return style.Render(dividerLine)
}

// renderWrappingTabBar renders a tab bar that wraps to multiple rows when tabs
// overflow the available width. Each row is filled with a border filler on the right.
// This differs from RenderTabBar which renders a single-line tab bar.
func renderWrappingTabBar(tabs []string, activeIdx, availableWidth int) string {
	plainStyle := lipgloss.NewStyle().Padding(0, 1)

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1).
		BorderBottom(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(ColorPrimary)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1).
		BorderBottom(true).
		BorderStyle(lipgloss.Border{Bottom: "─"}).
		BorderForeground(ColorBorder)

	// Build rows of tabs that fit within available width
	var rows []string
	var currentRow []string
	currentWidth := 0

	for i, name := range tabs {
		tabWidth := lipgloss.Width(plainStyle.Render(name))
		if currentWidth+tabWidth > availableWidth && len(currentRow) > 0 {
			rows = append(rows, finishWrappingTabRow(currentRow, availableWidth))
			currentRow = nil
			currentWidth = 0
		}
		if i == activeIdx {
			currentRow = append(currentRow, activeStyle.Render(name))
		} else {
			currentRow = append(currentRow, inactiveStyle.Render(name))
		}
		currentWidth += tabWidth
	}
	if len(currentRow) > 0 {
		rows = append(rows, finishWrappingTabRow(currentRow, availableWidth))
	}

	return strings.Join(rows, "\n")
}

// renderFullscreenTitle renders a title line for fullscreen views with optional subtitle.
func renderFullscreenTitle(title, subtitle string) string {
	t := TitleStyle.Render(title)
	if subtitle != "" {
		t += "  " + MutedStyle.Render(subtitle)
	}
	return t
}

// renderPriorityBadge renders a priority indicator with color styling.
func renderPriorityBadge(priority int) string {
	badge := fmt.Sprintf("!%d", priority)
	switch priority {
	case 1:
		return PriorityHighStyle.Render(badge)
	case 2:
		return PriorityMediumStyle.Render(badge)
	default:
		return PriorityLowStyle.Render(badge)
	}
}

// formatRelativeDue formats a due date relative to today with appropriate styling.
func formatRelativeDue(dueDate *time.Time) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, now.Location())

	daysUntil := int(dueDay.Sub(today).Hours() / 24)

	switch {
	case daysUntil < -1:
		return OverdueStyle.Render(fmt.Sprintf("%d days ago", -daysUntil))
	case daysUntil == -1:
		return OverdueStyle.Render("yesterday")
	case daysUntil == 0:
		return DueTodayStyle.Render("today")
	case daysUntil == 1:
		return DueDateStyle.Render("tomorrow")
	case daysUntil < 7:
		return DueDateStyle.Render(dueDate.Format("Mon"))
	default:
		if dueDate.Year() != now.Year() {
			return DueDateStyle.Render(dueDate.Format("Jan 2 '06"))
		}
		return DueDateStyle.Render(dueDate.Format("Jan 2"))
	}
}

// renderSubTabBarWithFocus renders a sub-tab bar row with a ▐ focus indicator.
// tabs is the list of tab names, activeIdx is the active tab, focused indicates
// whether this row has focus (shows ▐) or not (shows space), and width is the
// available width for the tab bar.
func renderSubTabBarWithFocus(tabs []string, activeIdx int, focused bool, width int) string {
	bar := RenderTabBar(tabs, activeIdx, width-2, 1)

	focusIndicator := lipgloss.NewStyle().Foreground(ColorPrimary).Render("▐")
	if focused {
		bar = focusIndicator + bar
	} else {
		bar = " " + bar
	}
	return bar
}

// finishWrappingTabRow joins a row of rendered tabs and fills remaining width with a border.
func finishWrappingTabRow(tabs []string, availableWidth int) string {
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	tabWidth := lipgloss.Width(row)
	remaining := availableWidth - tabWidth
	if remaining > 0 {
		filler := lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderForeground(ColorBorder).
			Width(remaining).
			Render("")
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, filler)
	}
	return row
}
