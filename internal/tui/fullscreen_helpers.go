package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
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

	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	spacer := ThemeSpacer(gap)

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

	dividerLine := strings.Repeat("━", width)
	if cfg.rightIndicator != "" && len(cfg.rightIndicator) < width {
		dividerLine = strings.Repeat("━", width-len(cfg.rightIndicator)) + cfg.rightIndicator
	}

	return style.Render(dividerLine)
}

// renderWrappingTabBar renders a tab bar that wraps to multiple rows when tabs
// overflow the available width. Each row is filled with a border filler on the right.
// This differs from RenderTabBar which renders a single-line tab bar.
func renderWrappingTabBar(tabs []string, activeIdx, availableWidth int) string {
	plainStyle := lipgloss.NewStyle().Padding(0, 1)
	active := activeTabStyle(1, true)
	inactive := inactiveTabStyle(1, true)

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
			currentRow = append(currentRow, active.Render(name))
		} else {
			currentRow = append(currentRow, inactive.Render(name))
		}
		currentWidth += tabWidth
	}
	if len(currentRow) > 0 {
		rows = append(rows, finishWrappingTabRow(currentRow, availableWidth))
	}

	return strings.Join(rows, "\n")
}

// RenderScrollbar returns a viewportHeight-line string of ░/█ characters
// representing the scroll position. Returns "" when content fits (no scroll
// needed). Thumb height is proportional (viewportHeight²/totalItems, min 1).
// Thumb position maps scrollOffset linearly to the track range.
func RenderScrollbar(totalItems, viewportHeight, scrollOffset int) string {
	if totalItems <= viewportHeight || viewportHeight <= 0 {
		return ""
	}

	// Thumb size: proportional, min 1
	thumbHeight := max((viewportHeight*viewportHeight)/totalItems, 1)

	// Thumb position: map scrollOffset to track range
	trackRange := viewportHeight - thumbHeight
	maxOffset := totalItems - viewportHeight
	thumbTop := 0
	if maxOffset > 0 {
		thumbTop = (scrollOffset * trackRange) / maxOffset
	}
	if thumbTop > trackRange {
		thumbTop = trackRange
	}

	var b strings.Builder
	for i := range viewportHeight {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i >= thumbTop && i < thumbTop+thumbHeight {
			b.WriteString(ScrollbarThumbStyle.Render("█"))
		} else {
			b.WriteString(ScrollbarTrackStyle.Render("░"))
		}
	}
	return b.String()
}

// AttachScrollbar takes a content block and a scrollbar string (both newline-
// delimited), pads each content line to contentWidth, and zips them together.
// If scrollbar is empty, returns content unchanged. Content lines are padded
// using lipgloss width to handle ANSI escape sequences correctly.
func AttachScrollbar(content, scrollbar string) string {
	if scrollbar == "" {
		return content
	}

	contentLines := strings.Split(content, "\n")
	scrollLines := strings.Split(scrollbar, "\n")

	// Measure max visual width across all content lines so we can pad uniformly.
	maxWidth := 0
	for _, cl := range contentLines {
		if w := lipgloss.Width(cl); w > maxWidth {
			maxWidth = w
		}
	}

	var b strings.Builder
	maxLines := max(len(scrollLines), len(contentLines))

	for i := range maxLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		cl := ""
		if i < len(contentLines) {
			cl = contentLines[i]
		}
		// Pad content line to uniform width so scrollbar aligns vertically.
		if pad := maxWidth - lipgloss.Width(cl); pad > 0 {
			cl += strings.Repeat(" ", pad)
		}
		sl := ""
		if i < len(scrollLines) {
			sl = scrollLines[i]
		}
		b.WriteString(cl)
		b.WriteString(sl)
	}
	return b.String()
}

// renderFullscreenTitle renders a title line for fullscreen views with optional subtitle.
func renderFullscreenTitle(title, subtitle string) string {
	t := TitleStyle.Render(title)
	if subtitle != "" {
		t += "  " + MutedStyle.Render(subtitle)
	}
	return t
}

// renderInlineTags renders a compact inline tag string showing at most 2 tags,
// with per-tag truncation at 8 chars and an overflow count.
// Returns an empty string if there are no tags. Result is prefixed with a space.
func renderInlineTags(tags []string) string {
	s := renderTruncatedTags(tags, 2, 8)
	if s == "" {
		return ""
	}
	return " " + s
}

// renderPriorityBadge renders a priority indicator with color styling.
func renderPriorityBadge(priority int) string {
	return GetPriorityStyle(priority).Render(PriorityIcon(priority))
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
	return renderDueDateContentOpts(task, false)
}

// renderDueDateContentOpts builds the styled due date string. When hideRecurrence
// is true, the "Recurs:" line is omitted (useful when a dedicated Recurrence card
// is shown nearby).
func renderDueDateContentOpts(task *models.Task, hideRecurrence bool) string {
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

	if !hideRecurrence && task.Recurrence != nil {
		dueLine += "\n" + MutedStyle.Render("Recurs: "+task.Recurrence.String())
	}

	return dueLine
}

// renderMetadataContent builds the tags, section, custom section, and type-specific
// attributes content shared by sidebar and task detail.
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

	if task.Source != "" {
		lines = append(lines, MutedStyle.Render("Source:  ")+task.Source)
	}
	if task.URL != "" {
		display := util.Truncate(task.URL, innerWidth-5)
		link := Hyperlink(task.URL, display)
		lines = append(lines, MutedStyle.Render("URL:     ")+link)
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

// FormatRelativeDue formats a due date relative to today.
// When styled is true, overdue/today/normal ranges are wrapped in color styles.
func FormatRelativeDue(dueDate *time.Time, styled bool) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	daysUntil := util.DaysBetween(now, *dueDate)

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
	daysUntil := util.DaysBetween(now, *dueDate)

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

	if focused {
		bar = FocusIndicator + bar
	} else {
		bar = " " + bar
	}
	return bar
}

// renderTabFiller returns a bottom-border filler element that extends a tab row
// to the given width. Returns "" if width <= 0.
func renderTabFiller(width int) string {
	if width <= 0 {
		return ""
	}
	s := lipgloss.NewStyle().
		BorderBottom(true).
		BorderStyle(lipgloss.Border{Bottom: "─"}).
		BorderForeground(ColorBorder).
		Width(width)
	if themeBgColor != "" {
		s = s.Background(lipgloss.Color(themeBgColor))
	}
	return s.Render("")
}

// finishWrappingTabRow joins a row of rendered tabs and fills remaining width with a border.
func finishWrappingTabRow(tabs []string, availableWidth int) string {
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	remaining := availableWidth - lipgloss.Width(row)
	if filler := renderTabFiller(remaining); filler != "" {
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
		maxOffset := max(len(lines)-contentHeight, 0)
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
func renderFullscreenFooterBlock(statusMsg string, err error, footerLeft, footerRight string, width int) string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(renderStatusMessages(statusMsg, err))
	b.WriteString(renderStatusBarFooter(footerLeft, footerRight, width))
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

// FormatTimestampBracket returns "[2006-01-02 15:04] 3 hours ago".
func FormatTimestampBracket(t time.Time) string {
	return fmt.Sprintf("[%s] %s", t.Format("2006-01-02 15:04"), storage.FormatRelativeTime(t))
}

// FormatTimestampLabeled returns "Created: 2006-01-02 15:04 (3 hours ago)".
func FormatTimestampLabeled(label string, t time.Time) string {
	return fmt.Sprintf("%s: %s (%s)", label, t.Format("2006-01-02 15:04"), storage.FormatRelativeTime(t))
}

// renderGroupHeader renders a group header with count, using overdue styling when appropriate.
func renderGroupHeader(name string, count int, isOverdue bool) string {
	header := fmt.Sprintf("%s (%d)", name, count)
	if isOverdue {
		return OverdueStyle.Bold(true).Render(header)
	}
	return GroupHeaderStyle.Render(header)
}

// renderCollapsibleHeader renders a collapsible section/group header with cursor
// indicator, collapse toggle, name, count, and selection highlight.
func renderCollapsibleHeader(name string, count int, collapsed, isSelected, flash bool, style lipgloss.Style) string {
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
		selStyle := SelectedStyle
		if flash {
			selStyle = selStyle.Background(ColorFlashBg)
		}
		return selStyle.Render(styled)
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

// ClassifyDueDate returns a display string and style for a due date relative to today.
// Returns ("", lipgloss.NewStyle()) for nil dates. Used by formatDueDate in
// project_tree_filters.go and due_date_overview_render.go.
func ClassifyDueDate(dueDate *time.Time) (string, lipgloss.Style) {
	if dueDate == nil {
		return "", NormalStyle
	}

	now := time.Now()
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*dueDate)

	switch {
	case dueDay.Before(today):
		return dueDate.Format("Jan 2"), OverdueStyle
	case dueDay.Equal(today):
		return "Today", DueTodayStyle
	case dueDay.Equal(today.AddDate(0, 0, 1)):
		return "Tomorrow", DueDateStyle
	default:
		if dueDate.Year() != now.Year() {
			return dueDate.Format("Jan 2 '06"), DueDateStyle
		}
		return dueDate.Format("Jan 2"), DueDateStyle
	}
}

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

// renderDynamicSubTabBar renders the dynamic sub-tab row when there are
// multiple sub-tabs (prepends "All"), or returns "" if none.
func renderDynamicSubTabBar(subTabNames []string, subTab int, subTabRowFocused bool, width int) string {
	if len(subTabNames) <= 1 {
		return ""
	}

	tabs := make([]string, 0, len(subTabNames)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, subTabNames...)

	// Use wrapping tab bar with width reduced for focus indicator column
	bar := renderWrappingTabBar(tabs, subTab, width-2)

	lines := strings.Split(bar, "\n")
	if subTabRowFocused {
		lines[0] = FocusIndicator + lines[0]
	} else {
		lines[0] = " " + lines[0]
	}
	return strings.Join(lines, "\n")
}
