package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// helpIntentOrder defines the display order of intent buckets in first-level
// (per-view) help. Buckets with no matching items in the current view are
// skipped during rendering.
var helpIntentOrder = []string{
	"Capture",
	"Triage",
	"Plan",
	"Work",
	"Review",
	"Route & Schedule",
	"Resolve",
	"Refine",
	"Task & Blockers",
	"View Controls",
	"Navigate",
	"Jump to View",
	"Other",
}

// helpCategoryIntent maps raw registry Category names to intent buckets.
// Categories absent from this map fall through to per-key classification in
// intentForItem.
var helpCategoryIntent = map[string]string{
	"Status":              "Resolve",
	"Priority":            "Resolve",
	"Recurrence":          "Resolve",
	"Move (m prefix)":     "Route & Schedule",
	"Due Date":            "Route & Schedule",
	"Snooze":              "Route & Schedule",
	"Linked Tasks":        "Refine",
	"Folds":               "View Controls",
	"Filters":             "View Controls",
	"Task (t prefix)":     "Task & Blockers",
	"Blockers (b prefix)": "Task & Blockers",
	"Task Operations":     "Task & Blockers",
	"Clipboard":           "Other",
	"Notes":               "Other",
}

// helpCrossViewJumpKeys lists key strings that jump from any current view to
// another top-level view. They are bucketed into "Jump to View" so they do not
// crowd the primary actions in first-level help.
var helpCrossViewJumpKeys = map[string]bool{
	":":        true,
	"|":        true,
	"#":        true,
	"ctrl+x":   true,
	"ctrl+esc": true,
}

// helpPrimaryTriageKeys lists direct keys promoted into the Triage intent
// regardless of their raw Category. Mirrors keymap.TriagePrimaryGroup plus the
// help-toggle key.
var helpPrimaryTriageKeys = map[string]bool{
	"n": true,
	"e": true,
	"D": true,
	"T": true,
	"u": true,
	"?": true,
}

// helpNavigateKeys lists direct keys that always belong in the Navigate
// intent regardless of Category. Covers list movement, viewport paging, and
// app-shell controls (esc/quit/explorer toggle).
var helpNavigateKeys = map[string]bool{
	"j":         true,
	"k":         true,
	"down":      true,
	"up":        true,
	"g":         true,
	"G":         true,
	"gg":        true,
	"ctrl+u":    true,
	"ctrl+d":    true,
	"pgup":      true,
	"pgdown":    true,
	"esc":       true,
	"ctrl+c":    true,
	"enter":     true,
	"backspace": true,
	"space":     true,
	"q":         true,
	"w":         true,
}

// intentForItem classifies a HelpItem into one of helpIntentOrder buckets.
// Key matches take precedence over Category mapping. Promotion of the primary
// triage vocabulary is source-view-aware: helpPrimaryTriageKeys are only
// elevated to "Triage" when the help originates from an inbox processor view,
// because those keys describe the triage action vocabulary there. On other
// surfaces (notably the Dashboard) the same keys mean different things and
// must not be advertised as triage. Dashboard's `O` is the lone exception —
// it opens inbox triage, so it deserves the Triage label even though its raw
// Category is generic.
func intentForItem(item HelpItem, sourceView PresentationID) string {
	if sourceView == ViewFocusBoard {
		if intent, ok := focusBoardPipelineIntentForItem(item.Key); ok {
			return intent
		}
	}
	if isInboxProcessorSourceView(sourceView) && helpPrimaryTriageKeys[item.Key] {
		return "Triage"
	}
	if sourceView == ViewDashboard && item.Key == "O" {
		return "Triage"
	}
	switch item.Key {
	case "1", "2", "3", "4", "5":
		return "Resolve"
	case "ctrl+w", "ctrl+s":
		return "Route & Schedule"
	}
	if helpNavigateKeys[item.Key] {
		return "Navigate"
	}
	if helpCrossViewJumpKeys[item.Key] {
		return "Jump to View"
	}
	if intent, ok := helpCategoryIntent[item.Category]; ok {
		return intent
	}
	switch item.Category {
	case "Navigation":
		return "Jump to View"
	}
	return "Other"
}

func focusBoardPipelineIntentForItem(key string) (string, bool) {
	switch key {
	case "a":
		return "Capture", true
	case "v", "/":
		return "Triage", true
	case "p", "J", "K":
		return "Plan", true
	case "enter", "x", "u":
		return "Work", true
	case "shift+enter":
		return "Review", true
	default:
		return "", false
	}
}

// isInboxProcessorSourceView reports whether the help originates from one of
// the inbox processor views, which are the only surfaces where the primary
// triage key vocabulary should be promoted into the "Triage" intent bucket.
func isInboxProcessorSourceView(v PresentationID) bool {
	return v == ViewInboxProcessor || v == ViewGlobalInboxProcessor
}

// renderByIntentContent renders first-level (per-view) help grouped by user
// intent. Sections appear in helpIntentOrder; empty buckets are skipped.
func (m *HelpViewModel) renderByIntentContent() string {
	if len(m.filteredItems) == 0 {
		return MutedStyle.Render("No matching help items.")
	}

	buckets := make(map[string][]filteredHelpItem)
	for _, item := range m.filteredItems {
		bucket := intentForItem(item.HelpItem, m.sourceView)
		buckets[bucket] = append(buckets[bucket], item)
	}

	contentWidth := max(m.viewport.Width(), 1)
	keyWidth := 16
	columnGap := 4
	columnOverhead := keyWidth + 2
	descWidth := max((contentWidth-2*columnOverhead-columnGap)/2, 1)
	columnWidth := keyWidth + 2 + descWidth

	keyStyle := lipgloss.NewStyle().
		Width(keyWidth).
		Align(lipgloss.Right).
		Foreground(ColorSecondary)
	columnStyle := lipgloss.NewStyle().Width(columnWidth)

	renderItem := func(item filteredHelpItem) string {
		var renderedKey string
		if len(item.keyPositions) > 0 {
			renderedKey = keyStyle.Render(HighlightText(item.Key, item.keyPositions, TagInlineStyle))
		} else {
			renderedKey = keyStyle.Render(item.Key)
		}
		wrappedDesc := wrapDescriptionWithIndent(item.Description, descWidth, keyWidth+2)
		descLines := strings.Split(wrappedDesc, "\n")
		if len(item.descPositions) > 0 {
			descLines[0] = HighlightText(descLines[0], item.descPositions, NormalStyle)
		}
		var result strings.Builder
		for i, line := range descLines {
			if i == 0 {
				result.WriteString(renderedKey)
				result.WriteString("  ")
				result.WriteString(line)
			} else {
				result.WriteString("\n")
				result.WriteString(line)
			}
		}
		return result.String()
	}

	var b strings.Builder
	first := true
	for _, intent := range helpIntentOrder {
		items := buckets[intent]
		if len(items) == 0 {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		first = false
		b.WriteString(HelpCategoryStyle.Render(intent))
		b.WriteString("\n")
		for j := 0; j < len(items); j += 2 {
			col1 := columnStyle.Render(renderItem(items[j]))
			var col2 string
			if j+1 < len(items) {
				col2 = columnStyle.Render(renderItem(items[j+1]))
			}
			row := lipgloss.JoinHorizontal(lipgloss.Top, col1, strings.Repeat(" ", columnGap), col2)
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *HelpViewModel) renderContent() string {
	return m.renderByIntentContent()
}

// tabBarLines returns the number of visual lines the tab bar occupies.
func (m *HelpViewModel) tabBarLines() int {
	return lipgloss.Height(m.renderHelpTabBar())
}

func (m *HelpViewModel) renderHelpTabBar() string {
	return m.renderFullHelpTabBar()
}

func (m *HelpViewModel) renderFullHelpTabBar() string {
	tabNames := append([]string{"All"}, m.categories...)
	return renderWrappingTabBar(tabNames, m.activeTab, m.innerWidth())
}

func (m *HelpViewModel) renderHelpHeader(tabBar string) string {
	pad := " "
	var b strings.Builder

	b.WriteString(pad)
	b.WriteString(tabBar)
	b.WriteString("\n")

	if m.tabSearchMode {
		b.WriteString(pad)
		b.WriteString(SubtitleStyle.Render("Tab: "))
		b.WriteString(m.tabSearchInput.View())
		b.WriteString("\n\n")
	}

	if m.searchMode {
		b.WriteString(pad)
		b.WriteString(SubtitleStyle.Render("Filter: "))
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
	} else if m.searchQuery != "" {
		filterInfo := fmt.Sprintf("Filter: \"%s\" ", m.searchQuery)
		b.WriteString(pad)
		b.WriteString(HelpFilterActiveStyle.Render(filterInfo))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n\n")
	}

	return b.String()
}

func (m *HelpViewModel) renderHelpFooter() string {
	title := fmt.Sprintf("Help - %s", m.viewName)
	closeKey := "? close"
	left := StatusBarStyle.Render(title)

	scrollPercent := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height() {
		scrollPercent = fmt.Sprintf(" %3.0f%%", m.viewport.ScrollPercent()*100)
	}
	right := HelpStyle.Render(fmt.Sprintf("shift+←/→: tabs · \\ find tab · / filter · bksp clear · j/k scroll · %s%s", closeKey, scrollPercent))

	return renderStatusBarFooter(left, right, m.width)
}

func (m *HelpViewModel) View() tea.View {
	var b strings.Builder

	b.WriteString(m.renderHelpHeader(m.renderHelpTabBar()))

	// Viewport with content
	if m.ready {
		for i, line := range strings.Split(m.viewport.View(), "\n") {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(" ")
			b.WriteString(line)
		}
	}

	// Status bar footer
	b.WriteString("\n")
	b.WriteString(m.renderHelpFooter())

	return tea.NewView(b.String())
}

func (m *HelpViewModel) StatusBarLabel() string {
	return "Help"
}

// HandledKeys returns the keys handled by Help.
func (m *HelpViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// wrapDescriptionWithIndent wraps text to fit within width, indenting continuation
// lines to align under the first line of text (after key + separator).
func wrapDescriptionWithIndent(text string, width, indentWidth int) string {
	if width <= 0 {
		return text
	}

	indent := strings.Repeat(" ", indentWidth)
	var lines []string
	remaining := text

	for len(remaining) > 0 {
		// All lines use full width - indent is added as prefix, not subtracted from width
		if len(remaining) <= width {
			if len(lines) > 0 {
				lines = append(lines, indent+remaining)
			} else {
				lines = append(lines, remaining)
			}
			break
		}

		// Find break point (last space within width)
		breakPoint := width
		for i := width; i > 0; i-- {
			if remaining[i-1] == ' ' {
				breakPoint = i - 1
				break
			}
		}
		// If no space found, force break at width
		if breakPoint == width {
			breakPoint = width
		}

		line := remaining[:breakPoint]
		if len(lines) > 0 {
			lines = append(lines, indent+line)
		} else {
			lines = append(lines, line)
		}

		// Skip past the break point and any leading spaces
		remaining = remaining[breakPoint:]
		remaining = strings.TrimLeft(remaining, " ")
	}

	return strings.Join(lines, "\n")
}
