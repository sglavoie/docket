package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/alerts"
)

var (
	alertUrgentDot  = lipgloss.NewStyle().Foreground(ColorError).Render("● ")
	alertWarningDot = lipgloss.NewStyle().Foreground(ColorWarning).Render("● ")
	alertInfoDot    = MutedStyle.Render("○ ")
)

// alertCenterItem is a flattened item for cursor-based navigation.
type alertCenterItem struct {
	isHeader  bool   // true for group headers (not selectable)
	group     int    // index into the active snapshot's Groups
	alert     int    // index into group.Alerts (-1 for headers)
	label     string // pre-rendered display text (title only for alerts)
	details   string // reasoning/context appended after title
	dismissed bool   // true if this alert is in the dismissed set (only when showDismissed)
}

// NotificationCenterModel shows proactive alerts grouped by category.
type NotificationCenterModel struct {
	BaseView
	InteractiveViewState

	snapshot      *alerts.AlertSnapshot // filtered snapshot (what's normally shown)
	rawSnapshot   *alerts.AlertSnapshot // unfiltered snapshot (includes dismissed)
	dismissedSet  map[string]bool       // keys of currently dismissed alerts
	showDismissed bool                  // true = show all alerts including dismissed
	items         []alertCenterItem     // flattened items for navigation
	sectionStarts []int                 // cursor indices of first selectable item per section
	expandedItem  int                   // index of expanded item, -1 = none
}

// NewNotificationCenterModel creates an alert center from the current snapshot.
func NewNotificationCenterModel(filtered, raw *alerts.AlertSnapshot, dismissedSet map[string]bool) *NotificationCenterModel {
	m := &NotificationCenterModel{
		snapshot:     filtered,
		rawSnapshot:  raw,
		dismissedSet: dismissedSet,
		expandedItem: -1,
	}
	m.buildItems()
	// Position cursor on first selectable item
	for i, item := range m.items {
		if !item.isHeader {
			m.cursor = i
			break
		}
	}
	return m
}

// activeSnapshot returns the snapshot to use for rendering: raw when showing
// dismissed alerts, filtered otherwise.
func (m *NotificationCenterModel) activeSnapshot() *alerts.AlertSnapshot {
	if m.showDismissed && m.rawSnapshot != nil {
		return m.rawSnapshot
	}
	return m.snapshot
}

// buildItems flattens alert groups into navigable items, ordered by severity (urgent first).
func (m *NotificationCenterModel) buildItems() {
	m.items = nil
	m.sectionStarts = nil
	snap := m.activeSnapshot()
	if snap == nil || len(snap.Groups) == 0 {
		return
	}

	// Sort groups by max severity descending
	groups := make([]alerts.AlertGroup, len(snap.Groups))
	copy(groups, snap.Groups)
	sort.SliceStable(groups, func(i, j int) bool {
		return maxSeverity(groups[i].Alerts) > maxSeverity(groups[j].Alerts)
	})

	for gi, group := range groups {
		// Group header
		m.items = append(m.items, alertCenterItem{
			isHeader: true,
			group:    gi,
			alert:    -1,
			label:    group.Label,
		})
		// Record the first selectable item index for this section
		m.sectionStarts = append(m.sectionStarts, len(m.items))
		// Individual alerts
		for ai, a := range group.Alerts {
			m.items = append(m.items, alertCenterItem{
				isHeader:  false,
				group:     gi,
				alert:     ai,
				label:     a.Title,
				details:   a.Details,
				dismissed: m.showDismissed && m.dismissedSet[a.DismissalKey()],
			})
		}
	}
}

// rebuildForShowDismissed switches to showing the raw snapshot and rebuilds items.
func (m *NotificationCenterModel) rebuildForShowDismissed() {
	m.buildItems()
	m.clampCursor()
}

func maxSeverity(as []alerts.Alert) int {
	max := 0
	for _, a := range as {
		if a.Severity > max {
			max = a.Severity
		}
	}
	return max
}

func (m *NotificationCenterModel) Init() tea.Cmd {
	return nil
}

func (m *NotificationCenterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return CloseNotificationCenterMsg{} }

		case "j", "down":
			m.moveDown()

		case "k", "up":
			m.moveUp()

		case "g":
			// Jump to first selectable
			for i, item := range m.items {
				if !item.isHeader {
					m.cursor = i
					break
				}
			}

		case "G":
			// Jump to last selectable
			for i := len(m.items) - 1; i >= 0; i-- {
				if !m.items[i].isHeader {
					m.cursor = i
					break
				}
			}

		case "n":
			return m, m.SectionGoToNext(m.sectionStarts)

		case "N":
			return m, m.SectionGoToPrev(m.sectionStarts)

		case "tab":
			if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].isHeader {
				if m.expandedItem == m.cursor {
					m.expandedItem = -1
				} else {
					m.expandedItem = m.cursor
				}
			}

		case "d":
			if a := m.selectedAlert(); a != nil {
				key := a.DismissalKey()
				cat := string(a.Category)
				// If the selected alert is dismissed, un-dismiss it
				if m.selectedItem().dismissed {
					return m, func() tea.Msg {
						return UndismissAlertMsg{Key: key}
					}
				}
				return m, func() tea.Msg {
					return DismissAlertMsg{Key: key, Category: cat}
				}
			}

		case "D":
			m.showDismissed = !m.showDismissed
			m.buildItems()
			m.clampCursor()
			m.expandedItem = -1

		case "enter":
			if a := m.selectedAlert(); a != nil {
				return m, m.navigateToAlert(*a)
			}
		}
	}

	return m, nil
}

// moveDown moves cursor to the next selectable item.
func (m *NotificationCenterModel) moveDown() {
	for i := m.cursor + 1; i < len(m.items); i++ {
		if !m.items[i].isHeader {
			m.cursor = i
			m.expandedItem = -1
			return
		}
	}
}

// moveUp moves cursor to the previous selectable item.
func (m *NotificationCenterModel) moveUp() {
	for i := m.cursor - 1; i >= 0; i-- {
		if !m.items[i].isHeader {
			m.cursor = i
			m.expandedItem = -1
			return
		}
	}
}

// selectedItem returns the currently selected item, or nil if out of bounds.
func (m *NotificationCenterModel) selectedItem() *alertCenterItem {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.items[m.cursor]
}

// clampCursor ensures the cursor is on a valid selectable item.
func (m *NotificationCenterModel) clampCursor() {
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}
	// If current position is valid and selectable, keep it
	if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].isHeader {
		return
	}
	// Find nearest selectable item (prefer moving up, then down)
	start := m.cursor - 1
	if start >= len(m.items) {
		start = len(m.items) - 1
	}
	for i := start; i >= 0; i-- {
		if !m.items[i].isHeader {
			m.cursor = i
			return
		}
	}
	for i := 0; i < len(m.items); i++ {
		if !m.items[i].isHeader {
			m.cursor = i
			return
		}
	}
	m.cursor = 0
}

// sortedGroups returns a copy of the active snapshot's groups sorted by severity.
func (m *NotificationCenterModel) sortedGroups() []alerts.AlertGroup {
	snap := m.activeSnapshot()
	if snap == nil {
		return nil
	}
	groups := make([]alerts.AlertGroup, len(snap.Groups))
	copy(groups, snap.Groups)
	sort.SliceStable(groups, func(i, j int) bool {
		return maxSeverity(groups[i].Alerts) > maxSeverity(groups[j].Alerts)
	})
	return groups
}

// selectedAlert returns the currently selected alert, or nil if on a header.
func (m *NotificationCenterModel) selectedAlert() *alerts.Alert {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	if item.isHeader {
		return nil
	}
	groups := m.sortedGroups()
	if item.group >= len(groups) {
		return nil
	}
	g := groups[item.group]
	if item.alert >= len(g.Alerts) {
		return nil
	}
	a := g.Alerts[item.alert]
	return &a
}

// navigateToAlert returns a command that dispatches navigation to the alert's task/project.
func (m *NotificationCenterModel) navigateToAlert(a alerts.Alert) tea.Cmd {
	return func() tea.Msg {
		return AlertCenterNavigateMsg{
			TaskID:    a.TaskID,
			ProjectID: a.ProjectID,
			Category:  a.Category,
		}
	}
}

// ShouldCaptureEsc returns true so the view handles its own esc/q.
func (m *NotificationCenterModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc()
}

func (m *NotificationCenterModel) View() string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Alert Center", ""))
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		if m.showDismissed {
			b.WriteString(MutedStyle.Render("  No alerts — everything looks good!"))
		} else {
			msg := "  No alerts — everything looks good!"
			if m.rawSnapshot != nil && m.rawSnapshot.TotalCount > 0 {
				msg = fmt.Sprintf("  All alerts dismissed (%d hidden). Press D to show.", m.rawSnapshot.TotalCount)
			}
			b.WriteString(MutedStyle.Render(msg))
		}
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("  Press Esc to close"))
		return b.String()
	}

	// Calculate available content height (title=1, blank=1, footer=1, padding=1)
	contentHeight := max(m.height-4, 3)

	// Prefix overhead: cursor/space (2) + severity dot (2) = 4 visible chars
	const prefixWidth = 4
	maxLabelWidth := max(m.width-prefixWidth, 20)

	// Build all lines first
	var lines []string
	for i, item := range m.items {
		if item.isHeader {
			lines = append(lines, "")
			header := SubtitleStyle.Render("  " + item.label)
			lines = append(lines, header)
		} else {
			prefix := "  "
			isSelected := i == m.cursor
			if isSelected {
				prefix = BoldStyle.Foreground(ColorPrimary).Render("▸ ")
			}
			sev := m.severityPrefix(item)
			title := item.label
			details := item.details

			flash := isSelected && m.cursorFlash

			if i == m.expandedItem {
				// Wrap the full combined text across multiple lines
				full := title
				if details != "" {
					full += "  " + details
				}
				wrapped := wrapText(full, maxLabelWidth)
				for li, wl := range wrapped {
					styled := m.styleAlertLine(wl, title, details)
					if item.dismissed {
						styled = MutedStyle.Render(wl)
					}
					var line string
					if li == 0 {
						line = prefix + sev + styled
					} else {
						line = strings.Repeat(" ", prefixWidth) + styled
					}
					if flash {
						line = CursorFlashRowStyle.Width(m.width).Render(line)
					}
					lines = append(lines, line)
				}
			} else {
				full := title
				if details != "" {
					full += "  " + details
				}
				truncated := truncateToWidth(full, maxLabelWidth)
				styled := m.styleAlertLine(truncated, title, details)
				if item.dismissed {
					styled = MutedStyle.Render(truncated)
				}
				line := prefix + sev + styled
				if flash {
					line = CursorFlashRowStyle.Width(m.width).Render(line)
				}
				lines = append(lines, line)
			}
		}
	}

	// Auto-scroll to keep cursor visible
	scrollOffset := 0
	cursorLine := 0
	lineIdx := 0
	for i, item := range m.items {
		if item.isHeader {
			lineIdx++ // blank line
			lineIdx++ // header line
		} else {
			if i == m.cursor {
				cursorLine = lineIdx
			}
			lineIdx++
		}
	}
	if cursorLine >= scrollOffset+contentHeight {
		scrollOffset = cursorLine - contentHeight + 1
	}
	if cursorLine < scrollOffset {
		scrollOffset = cursorLine
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	maxScroll := max(len(lines)-contentHeight, 0)
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}

	end := min(scrollOffset+contentHeight, len(lines))
	visible := lines[scrollOffset:end]
	b.WriteString(strings.Join(visible, "\n"))
	b.WriteString("\n")

	// Footer
	dismissLabel := "d: dismiss"
	if m.showDismissed {
		if item := m.selectedItem(); item != nil && item.dismissed {
			dismissLabel = "d: restore"
		}
	}
	toggleLabel := "D: show all"
	if m.showDismissed {
		toggleLabel = "D: hide dismissed"
	}
	left := HelpStyle.Render("j/k/n/N: navigate · Tab: expand · Enter: open · " + dismissLabel + " · " + toggleLabel + " · Esc: close")
	snap := m.activeSnapshot()
	count := 0
	if snap != nil {
		count = snap.TotalCount
	}
	right := MutedStyle.Render(fmt.Sprintf("%d alerts", count))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// severityPrefix returns a colored severity indicator for an alert item.
func (m *NotificationCenterModel) severityPrefix(item alertCenterItem) string {
	groups := m.sortedGroups()
	if item.group >= len(groups) || item.alert >= len(groups[item.group].Alerts) {
		return ""
	}
	a := groups[item.group].Alerts[item.alert]
	switch a.Severity {
	case alerts.SeverityUrgent:
		return alertUrgentDot
	case alerts.SeverityWarning:
		return alertWarningDot
	default:
		return alertInfoDot
	}
}

// truncateToWidth truncates a plain-text string to maxWidth runes, adding "…" if truncated.
func truncateToWidth(s string, maxWidth int) string {
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth < 2 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-1]) + "…"
}

var alertDetailsStyle = lipgloss.NewStyle().Foreground(ColorPrimary)

// styleAlertLine renders a line with the details portion styled in blue.
// The line is already truncated/wrapped plain text combining title and details.
func (m *NotificationCenterModel) styleAlertLine(line, title, details string) string {
	if details == "" {
		return line
	}
	// The separator between title and details is "  " (two spaces)
	sep := "  "
	titleLen := len([]rune(title))
	lineRunes := []rune(line)

	// If the line is shorter than or equal to the title, no details are visible
	if len(lineRunes) <= titleLen {
		return line
	}

	// Check if the separator is present right after the title
	sepStart := titleLen
	sepEnd := sepStart + len([]rune(sep))
	if sepEnd <= len(lineRunes) && string(lineRunes[sepStart:sepEnd]) == sep {
		// Title + sep + details portion
		titlePart := string(lineRunes[:sepStart])
		detailsPart := string(lineRunes[sepEnd:])
		return titlePart + sep + alertDetailsStyle.Render(detailsPart)
	}

	// For wrapped continuation lines that are entirely within details
	return alertDetailsStyle.Render(line)
}

// AlertCenterNavigateMsg requests navigation from the alert center to a task/project.
type AlertCenterNavigateMsg struct {
	TaskID    string
	ProjectID string
	Category  alerts.AlertCategory
}
