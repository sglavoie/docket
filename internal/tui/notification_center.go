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
	isHeader bool   // true for group headers (not selectable)
	group    int    // index into snapshot.Groups
	alert    int    // index into group.Alerts (-1 for headers)
	label    string // pre-rendered display text (title only for alerts)
	details  string // reasoning/context appended after title
}

// NotificationCenterModel shows proactive alerts grouped by category.
type NotificationCenterModel struct {
	BaseView
	InteractiveViewState

	snapshot     *alerts.AlertSnapshot
	items        []alertCenterItem // flattened items for navigation
	expandedItem int               // index of expanded item, -1 = none
}

// NewNotificationCenterModel creates an alert center from the current snapshot.
func NewNotificationCenterModel(snapshot *alerts.AlertSnapshot) *NotificationCenterModel {
	m := &NotificationCenterModel{
		snapshot:     snapshot,
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

// buildItems flattens alert groups into navigable items, ordered by severity (urgent first).
func (m *NotificationCenterModel) buildItems() {
	m.items = nil
	if m.snapshot == nil || len(m.snapshot.Groups) == 0 {
		return
	}

	// Sort groups by max severity descending
	groups := make([]alerts.AlertGroup, len(m.snapshot.Groups))
	copy(groups, m.snapshot.Groups)
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
		// Individual alerts
		for ai, a := range group.Alerts {
			m.items = append(m.items, alertCenterItem{
				isHeader: false,
				group:    gi,
				alert:    ai,
				label:    a.Title,
				details:  a.Details,
			})
		}
	}
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
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)

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

		case "tab":
			if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].isHeader {
				if m.expandedItem == m.cursor {
					m.expandedItem = -1
				} else {
					m.expandedItem = m.cursor
				}
			}

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

// selectedAlert returns the currently selected alert, or nil if on a header.
func (m *NotificationCenterModel) selectedAlert() *alerts.Alert {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	if item.isHeader {
		return nil
	}
	// Re-sort groups the same way buildItems does
	groups := make([]alerts.AlertGroup, len(m.snapshot.Groups))
	copy(groups, m.snapshot.Groups)
	sort.SliceStable(groups, func(i, j int) bool {
		return maxSeverity(groups[i].Alerts) > maxSeverity(groups[j].Alerts)
	})
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
		b.WriteString(MutedStyle.Render("  No alerts — everything looks good!"))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("  Press Esc to close"))
		return b.String()
	}

	// Calculate available content height (title=1, blank=1, footer=1, padding=1)
	contentHeight := m.height - 4
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Prefix overhead: cursor/space (2) + severity dot (2) = 4 visible chars
	const prefixWidth = 4
	maxLabelWidth := m.width - prefixWidth
	if maxLabelWidth < 20 {
		maxLabelWidth = 20
	}

	// Build all lines first
	var lines []string
	for i, item := range m.items {
		if item.isHeader {
			lines = append(lines, "")
			header := SubtitleStyle.Render("  " + item.label)
			lines = append(lines, header)
		} else {
			prefix := "  "
			if i == m.cursor {
				prefix = BoldStyle.Foreground(ColorPrimary).Render("▸ ")
			}
			sev := m.severityPrefix(item)
			title := item.label
			details := item.details

			if i == m.expandedItem {
				// Wrap the full combined text across multiple lines
				full := title
				if details != "" {
					full += "  " + details
				}
				wrapped := wrapText(full, maxLabelWidth)
				for li, wl := range wrapped {
					styled := m.styleAlertLine(wl, title, details)
					if li == 0 {
						lines = append(lines, prefix+sev+styled)
					} else {
						lines = append(lines, strings.Repeat(" ", prefixWidth)+styled)
					}
				}
			} else {
				full := title
				if details != "" {
					full += "  " + details
				}
				truncated := truncateToWidth(full, maxLabelWidth)
				styled := m.styleAlertLine(truncated, title, details)
				lines = append(lines, prefix+sev+styled)
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
	maxScroll := len(lines) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}

	end := scrollOffset + contentHeight
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[scrollOffset:end]
	b.WriteString(strings.Join(visible, "\n"))
	b.WriteString("\n")

	// Footer
	left := HelpStyle.Render("j/k: navigate · Tab: expand · Enter: go to task · Esc: close")
	right := MutedStyle.Render(fmt.Sprintf("%d alerts", m.snapshot.TotalCount))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// severityPrefix returns a colored severity indicator for an alert item.
func (m *NotificationCenterModel) severityPrefix(item alertCenterItem) string {
	groups := make([]alerts.AlertGroup, len(m.snapshot.Groups))
	copy(groups, m.snapshot.Groups)
	sort.SliceStable(groups, func(i, j int) bool {
		return maxSeverity(groups[i].Alerts) > maxSeverity(groups[j].Alerts)
	})
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
}
