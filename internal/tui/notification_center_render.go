package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/alerts"
)

var (
	alertUrgentDot  = ErrorIconStyle.Render("✖ ")
	alertWarningDot = WarningIconStyle.Render("▲ ")
	alertInfoDot    = MutedStyle.Render("● ")
)

// View implements tea.Model.
func (m *NotificationCenterModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Alert Center", "", m.AccentColor()))
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		if m.showDismissed {
			b.WriteString(MutedStyle.Render("  " + EmptyStateNoAlerts))
		} else {
			msg := "  " + EmptyStateNoAlerts
			if m.rawSnapshot != nil && m.rawSnapshot.TotalCount > 0 {
				msg = fmt.Sprintf("  All alerts dismissed (%d hidden). Press D to show.", m.rawSnapshot.TotalCount)
			}
			b.WriteString(MutedStyle.Render(msg))
		}
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("  Press Esc to close"))
		return tea.NewView(b.String())
	}

	// Build the footer first so its actual height (the help text wraps to extra
	// lines at narrow widths) can be reserved when sizing the alert window.
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
	footerLeft := HelpStyle.Render("j/k/n/N: navigate · Tab: expand · Enter: open · " + dismissLabel + " · " + toggleLabel + " · Esc: close")
	snap := m.activeSnapshot()
	count := 0
	if snap != nil {
		count = snap.TotalCount
	}
	footerRight := MutedStyle.Render(fmt.Sprintf("%d alerts", count))
	footer := renderStatusBarFooter(footerLeft, footerRight, m.width, m.AccentColor())

	// NotificationCenter is app-framed: it renders its own title/footer, then
	// AppModel appends the global two-line status bar. Keep the alert window
	// inside that combined chrome budget so alert-heavy snapshots scroll instead
	// of pushing the app status bar past short terminals.
	// Chrome: title (1) + blank (1) + content boundary "\n" (1) + wrapped footer.
	const notificationCenterFixedChrome = 3
	contentHeight := max(m.height-notificationCenterFixedChrome-lipgloss.Height(footer)-appStatusBarHeight, 3)

	// Prefix overhead: cursor/space (2) + severity dot (2) = 4 visible chars
	const prefixWidth = 4
	maxLabelWidth := max(m.width-prefixWidth, 1)

	lines := m.buildAlertLines(prefixWidth, maxLabelWidth)

	// Auto-scroll to keep cursor visible.
	// Track actual rendered line counts (expanded items produce multiple lines).
	scrollOffset := 0
	cursorLine := 0
	lineIdx := 0
	for i, item := range m.items {
		if item.isHeader {
			lineIdx += 2 // blank line + header line
		} else {
			if i == m.cursor {
				cursorLine = lineIdx
			}
			if i == m.expandedItem {
				full := item.label
				if item.details != "" {
					full += "  " + item.details
				}
				lineIdx += len(wrapText(full, maxLabelWidth))
			} else {
				lineIdx++
			}
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
	b.WriteString(footer)

	return tea.NewView(b.String())
}

// buildAlertLines renders each alert item into styled lines for display.
func (m *NotificationCenterModel) buildAlertLines(prefixWidth, maxLabelWidth int) []string {
	var lines []string
	for i, item := range m.items {
		if item.isHeader {
			lines = append(lines, "")
			headerIndent := "  "
			headerWidth := m.width - len(headerIndent)
			if headerWidth <= 0 {
				headerIndent = ""
				headerWidth = max(m.width, 1)
			}
			header := SubtitleStyle.Render(headerIndent + truncateToVisualWidth(item.label, headerWidth))
			lines = append(lines, header)
			continue
		}
		prefix := "  "
		isSelected := i == m.cursor
		if isSelected {
			prefix = BoldPrimaryStyle.Render(IconCursorRight + " ")
		}
		sev := m.severityPrefix(item)
		flash := isSelected && m.cursorFlash

		full := item.label
		if item.details != "" {
			full += "  " + item.details
		}

		if i == m.expandedItem {
			wrapped := wrapText(full, maxLabelWidth)
			for li, wl := range wrapped {
				styled := m.styleAlertLine(wl, item.details)
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
					line = SelectedFlashLine(line, m.width)
				}
				lines = append(lines, line)
			}
		} else {
			truncated := truncateToVisualWidth(full, maxLabelWidth)
			styled := m.styleAlertLine(truncated, item.details)
			if item.dismissed {
				styled = MutedStyle.Render(truncated)
			}
			line := prefix + sev + styled
			if flash {
				line = SelectedFlashLine(line, m.width)
			}
			lines = append(lines, line)
		}
	}
	return lines
}

// severityPrefix returns a colored severity indicator for an alert item.
func (m *NotificationCenterModel) severityPrefix(item alertCenterItem) string {
	switch item.alert.Severity {
	case alerts.SeverityUrgent:
		return alertUrgentDot
	case alerts.SeverityWarning:
		return alertWarningDot
	default:
		return alertInfoDot
	}
}

var alertDetailsStyle = InfoStatusStyle

// styleAlertLine renders a line with the details portion styled in blue.
// The line is already truncated/wrapped plain text combining title and details.
func (m *NotificationCenterModel) styleAlertLine(line, details string) string {
	if details == "" {
		return line
	}
	// The separator between title and details is "  " (two spaces)
	sep := "  "
	parts := strings.SplitN(line, sep, 2)
	if len(parts) < 2 {
		return line
	}
	return parts[0] + sep + alertDetailsStyle.Render(parts[1])
}
