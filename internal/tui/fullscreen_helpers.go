package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// ScrollUpIndicator returns a muted "↑ N more" string for scroll overflow above.
func ScrollUpIndicator(n int) string {
	return MutedStyle.Render(fmt.Sprintf("↑ %d more", n))
}

// ScrollDownIndicator returns a muted "↓ N more" string for scroll overflow below.
func ScrollDownIndicator(n int) string {
	return MutedStyle.Render(fmt.Sprintf("↓ %d more", n))
}

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

// renderInlineTags renders a compact inline tag string showing at most 2 tags.
// Returns an empty string if there are no tags.
func renderInlineTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	display := tags
	if len(display) > 2 {
		display = display[:2]
	}
	return TagInlineStyle.Render(" #" + strings.Join(display, " #"))
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

// renderTaskHeaderContent builds the common task header content:
// title, type badge + status line, priority badge + ID, and optional snooze line.
func renderTaskHeaderContent(task *models.Task, hasIncompleteDeps bool, showSnooze bool) string {
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	if hasIncompleteDeps {
		statusStyle = BlockedByDepsStyle
	}

	title := SidebarHeaderStyle.Render(task.Title)
	typeLine := GetTypeBadge(string(task.Type), task.Type.Icon()) + " " + typeStyle.Render(task.Type.HeadingPrefix())
	statusLine := statusStyle.Render(task.Status.Icon() + " " + string(task.Status))
	priorityBadge := renderPriorityBadge(task.Priority)
	idLine := fmt.Sprintf("#%d %s", task.ShortID, priorityBadge)

	content := title + "\n" + typeLine + " " + statusLine + " " + idLine

	if showSnooze && task.IsSnoozed() {
		snoozeStr := task.SnoozedUntil.Format("Jan 2")
		if task.SnoozedUntil.Year() != time.Now().Year() {
			snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
		}
		content += "\n" + MutedStyle.Render("zzz "+snoozeStr)
	}

	return content
}

// renderDueDateContent builds the styled due date string with overdue/today/normal
// coloring, and appends a recurrence line when the task has one.
func renderDueDateContent(task *models.Task) string {
	dueAbsolute := task.DueDate.Format(util.DateFormat)
	dueRelative := formatRelativeDueVerbose(task.DueDate)
	dueLine := fmt.Sprintf("%s (%s)", dueAbsolute, dueRelative)

	if task.IsOverdue() {
		dueLine = OverdueStyle.Render(dueLine)
	} else if task.IsDueToday() {
		dueLine = DueTodayStyle.Render(dueLine)
	} else {
		dueLine = DueDateStyle.Render(dueLine)
	}

	if task.Recurrence != nil {
		dueLine += "\n" + MutedStyle.Render("Recurs: "+task.Recurrence.String())
	}

	return dueLine
}

// renderMetadataContent builds the tags, section, custom section, and type-specific
// attributes content shared by sidebar, task detail, and kanban info cards.
func renderMetadataContent(task *models.Task, project *models.Project, innerWidth int, showAttrs ...bool) string {
	var lines []string

	if len(task.Tags) > 0 {
		var tagParts []string
		for _, tag := range task.Tags {
			tagParts = append(tagParts, TagInlineStyle.Render("#"+tag))
		}
		lines = append(lines, strings.Join(tagParts, " "))
	}

	sectionName := task.Section
	if sectionName == "" {
		sectionName = "Active"
	}
	sectionColor := project.GetSectionColor(task.Section)
	sectionStyle := GetSectionColorStyle(sectionColor)
	sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
	lines = append(lines, sectionLine)

	if task.CustomSection != "" {
		csColor := project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		csLine := MutedStyle.Render("Custom:  ") + csStyle.Render(task.CustomSection)
		lines = append(lines, csLine)
	}

	if len(showAttrs) == 0 || showAttrs[0] {
		attrLines := formatTaskAttributes(task, innerWidth)
		if len(attrLines) > 0 {
			lines = append(lines, "")
			lines = append(lines, attrLines...)
		}
	}

	return strings.Join(lines, "\n")
}

// formatRelativeDue formats a due date relative to today.
// When styled is true, overdue/today/normal ranges are wrapped in color styles.
func formatRelativeDue(dueDate *time.Time, styled bool) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*dueDate)

	daysUntil := int(dueDay.Sub(today).Hours() / 24)

	var text string
	var style lipgloss.Style

	switch {
	case daysUntil < -1:
		text = fmt.Sprintf("%d days ago", -daysUntil)
		style = OverdueStyle
	case daysUntil == -1:
		text = "yesterday"
		style = OverdueStyle
	case daysUntil == 0:
		text = "today"
		style = DueTodayStyle
	case daysUntil == 1:
		text = "tomorrow"
		style = DueDateStyle
	case daysUntil < 7:
		text = dueDate.Format("Mon")
		style = DueDateStyle
	default:
		if dueDate.Year() != now.Year() {
			text = dueDate.Format("Jan 2 '06")
		} else {
			text = dueDate.Format("Jan 2")
		}
		style = DueDateStyle
	}

	if styled {
		return style.Render(text)
	}
	return text
}

// formatRelativeDueVerbose returns a verbose relative due date string
// (e.g. "in 3 weeks", "in 2 months") for use in detail/sidebar panels.
func formatRelativeDueVerbose(dueDate *time.Time) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*dueDate)
	daysUntil := int(dueDay.Sub(today).Hours() / 24)

	switch {
	case daysUntil < -1:
		return fmt.Sprintf("%d days ago", -daysUntil)
	case daysUntil == -1:
		return "yesterday"
	case daysUntil == 0:
		return "today"
	case daysUntil == 1:
		return "tomorrow"
	case daysUntil < 7:
		return fmt.Sprintf("in %d days", daysUntil)
	case daysUntil < 14:
		return "next week"
	case daysUntil < 30:
		weeks := daysUntil / 7
		return fmt.Sprintf("in %d weeks", weeks)
	default:
		months := daysUntil / 30
		return fmt.Sprintf("in %d months", months)
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

// renderWrappingHelpLine renders a help line that wraps to multiple lines when width is exceeded.
// Items should be separated by " · " in the input. The function splits them across lines as needed.
func renderWrappingHelpLine(helpText string, width int) string {
	if width <= 0 {
		width = 80
	}

	separator := " · "
	items := strings.Split(helpText, separator)
	if len(items) == 0 {
		return ""
	}

	var lines []string
	var currentLine strings.Builder
	currentWidth := 0

	for i, item := range items {
		itemWidth := len(item)
		sepWidth := 0
		if currentLine.Len() > 0 {
			sepWidth = len(separator)
		}

		// Check if adding this item would exceed width
		if currentLine.Len() > 0 && currentWidth+sepWidth+itemWidth > width {
			// Start a new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentWidth = 0
		}

		// Add separator if not first item on this line
		if currentLine.Len() > 0 {
			currentLine.WriteString(separator)
			currentWidth += len(separator)
		}

		currentLine.WriteString(item)
		currentWidth += itemWidth

		// Handle last item
		if i == len(items)-1 && currentLine.Len() > 0 {
			lines = append(lines, currentLine.String())
		}
	}

	return strings.Join(lines, "\n")
}

// calcScrollOffset returns the scroll offset needed to keep cursorIdx visible
// within a window of maxVisible items. Used by views that iterate over items
// with index-based windowing (for i := offset; i < offset+maxVisible; ...).
func calcScrollOffset(cursorIdx, maxVisible int) int {
	if cursorIdx >= maxVisible {
		return cursorIdx - maxVisible + 1
	}
	return 0
}

// applyScroll returns the visible slice of lines for a non-persistent scroll.
// cursorIdx is the line index (in the lines slice) where the cursor sits;
// contentHeight is the number of lines that fit on screen.
func applyScroll(lines []string, cursorIdx, contentHeight int) []string {
	scrollOffset := calcScrollOffset(cursorIdx, contentHeight)

	visible := lines
	if scrollOffset > 0 && scrollOffset < len(lines) {
		visible = lines[scrollOffset:]
	}
	if len(visible) > contentHeight {
		visible = visible[:contentHeight]
	}
	return visible
}

// applyPersistentScroll adjusts *scrollOffset to keep cursorIdx visible, then
// returns the visible slice of lines. Unlike applyScroll, the scroll position
// is remembered across renders so that small cursor movements don't jump to
// cursor-anchored scrolling.
func applyPersistentScroll(lines []string, cursorIdx, contentHeight int, scrollOffset *int) []string {
	if contentHeight > 0 {
		if cursorIdx < *scrollOffset {
			*scrollOffset = cursorIdx
		}
		if cursorIdx >= *scrollOffset+contentHeight {
			*scrollOffset = cursorIdx - contentHeight + 1
		}
		maxOffset := len(lines) - contentHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		if *scrollOffset > maxOffset {
			*scrollOffset = maxOffset
		}
	}

	visible := lines
	if *scrollOffset > 0 && *scrollOffset < len(lines) {
		visible = lines[*scrollOffset:]
	}
	if len(visible) > contentHeight {
		visible = visible[:contentHeight]
	}
	return visible
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

// renderCollapsibleHeader renders a collapsible section/group header with cursor
// indicator, collapse toggle, name, count, and selection highlight.
func renderCollapsibleHeader(name string, count int, collapsed, isSelected bool, style lipgloss.Style) string {
	indicator := "▼"
	if collapsed {
		indicator = "▶"
	}
	cursor := "  "
	if isSelected {
		cursor = "> "
	}
	header := fmt.Sprintf("%s%s %s (%d)", cursor, indicator, name, count)
	styled := style.Bold(true).Render(header)
	if isSelected {
		return SelectedStyle.Render(styled)
	}
	return styled
}

// renderTabBarWithOptionalSubTabs renders a tab bar that switches between a
// wrapping tab bar and a sub-tab-bar-with-focus layout depending on whether
// sub-tabs are present.
func renderTabBarWithOptionalSubTabs(tabNames []string, activeTab int, subTabNames []string, subTabRowFocused bool, width int) string {
	if len(subTabNames) > 1 {
		return renderSubTabBarWithFocus(tabNames, activeTab, !subTabRowFocused, width)
	}
	return renderWrappingTabBar(tabNames, activeTab, width)
}

// renderDynamicSubTabBar renders the dynamic sub-tab row when there are
// multiple sub-tabs (prepends "All"), or returns "" if none.
func renderDynamicSubTabBar(subTabNames []string, subTab int, subTabRowFocused bool, width int) string {
	if len(subTabNames) <= 1 {
		return ""
	}

	tabs := make([]string, 0, len(subTabNames)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, subTabNames...)

	return renderSubTabBarWithFocus(tabs, subTab, subTabRowFocused, width)
}
