package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// PrefixHint represents a single key hint in a prefix popup.
type PrefixHint struct {
	Key         string
	Description string
	Detail      string
}

// PrefixHints maps prefix characters to their available key hints.
type PrefixHints map[string][]PrefixHint

// PrefixPopupInfo holds everything needed to render a prefix popup overlay.
type PrefixPopupInfo struct {
	Prefix string
	Hints  PrefixHints
}

// PrefixPopupProvider is implemented by view models that support prefix key popups.
type PrefixPopupProvider interface {
	GetPrefixPopupInfo() *PrefixPopupInfo
}

// prefixCategoryNames maps prefix characters to human-readable category names.
var prefixCategoryNames = map[string]string{
	",": "status",
	"z": "fold",
	"g": "go-to",
	"!": "priority",
	"d": "due-date",
	"s": "snooze",
	"c": "create",
}

// RenderPrefixPopup renders a which-key style popup as a bordered overlay box.
// Returns empty string if prefix is empty or not found in hints.
func RenderPrefixPopup(prefix string, hints PrefixHints, width, height int) string {
	if prefix == "" {
		return ""
	}

	keyHints, ok := hints[prefix]
	if !ok || len(keyHints) == 0 {
		return ""
	}

	// Determine category name for title
	category := prefixCategoryNames[prefix]
	if category == "" {
		category = prefix
	}

	// Build title: "prefix → category" with first letter of category highlighted
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrefixPopup)
	titleKeyStyle := lipgloss.NewStyle().Foreground(ColorPrefixPopup).Bold(true)
	titleCatFirstStyle := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	titleCatRestStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

	var categoryTitle string
	if strings.EqualFold(prefix, string(category[0])) {
		categoryTitle = titleCatFirstStyle.Render(string(category[0])) + titleCatRestStyle.Render(category[1:])
	} else {
		categoryTitle = titleCatRestStyle.Render(category)
	}
	title := titleKeyStyle.Render(prefix) + borderStyle.Render(" → ") + categoryTitle

	// Determine layout mode
	maxAvailableHeight := height - 4 // account for border, footer line, some margin
	if maxAvailableHeight > 20 {
		maxAvailableHeight = 20
	}
	if maxAvailableHeight < 3 {
		maxAvailableHeight = 3
	}

	onePerLine := len(keyHints) <= maxAvailableHeight

	// Calculate content width
	minWidth := 25
	maxWidth := width / 2
	if maxWidth < minWidth {
		maxWidth = width - 4
	}
	if maxWidth < minWidth {
		maxWidth = minWidth
	}

	var contentLines []string

	if onePerLine {
		// One hint per line: "key  description  detail"
		// Find max key and description widths for alignment
		maxKeyWidth := 0
		maxDescWidth := 0
		hasDetail := false
		for _, h := range keyHints {
			if len(h.Key) > maxKeyWidth {
				maxKeyWidth = len(h.Key)
			}
			if len(h.Description) > maxDescWidth {
				maxDescWidth = len(h.Description)
			}
			if h.Detail != "" {
				hasDetail = true
			}
		}

		contentWidth := 0
		for _, h := range keyHints {
			var line string
			var rendered string
			if hasDetail {
				line = fmt.Sprintf("%-*s  %-*s  %s", maxKeyWidth, h.Key, maxDescWidth, h.Description, h.Detail)
				rendered = PrefixKeyStyle.Render(fmt.Sprintf("%-*s", maxKeyWidth, h.Key)) + "  " +
					PrefixDescStyle.Render(fmt.Sprintf("%-*s", maxDescWidth, h.Description)) + "  " +
					MutedStyle.Render(h.Detail)
			} else {
				line = fmt.Sprintf("%-*s  %s", maxKeyWidth, h.Key, h.Description)
				rendered = PrefixKeyStyle.Render(fmt.Sprintf("%-*s", maxKeyWidth, h.Key)) + "  " + PrefixDescStyle.Render(h.Description)
			}
			contentLines = append(contentLines, rendered)
			if len(line) > contentWidth {
				contentWidth = len(line)
			}
		}

		// Set box width based on content
		boxInnerWidth := contentWidth + 2 // 1 padding each side
		if boxInnerWidth < minWidth {
			boxInnerWidth = minWidth
		}
		if boxInnerWidth > maxWidth {
			boxInnerWidth = maxWidth
		}

		return renderPrefixBox(title, contentLines, boxInnerWidth, borderStyle)
	}

	// Compact mode: hints joined on a line, wrapping
	boxInnerWidth := maxWidth
	availLineWidth := boxInnerWidth - 2 // padding

	var parts []string
	for _, h := range keyHints {
		parts = append(parts, PrefixDescStyle.Render(h.Description)+PrefixKeyStyle.Render("("+h.Key+")"))
	}

	var currentLine string
	currentWidth := 0
	for i, part := range parts {
		partWidth := lipgloss.Width(part)
		sep := ""
		sepWidth := 0
		if i > 0 {
			sep = " "
			sepWidth = 1
		}

		if currentWidth+sepWidth+partWidth > availLineWidth && currentLine != "" {
			contentLines = append(contentLines, currentLine)
			currentLine = part
			currentWidth = partWidth
		} else {
			if sep != "" {
				currentLine += sep
				currentWidth += sepWidth
			}
			currentLine += part
			currentWidth += partWidth
		}
	}
	if currentLine != "" {
		contentLines = append(contentLines, currentLine)
	}

	return renderPrefixBox(title, contentLines, boxInnerWidth, borderStyle)
}

// renderPrefixBox renders the bordered box with title and bottom "Esc" hint.
func renderPrefixBox(title string, contentLines []string, innerWidth int, borderStyle lipgloss.Style) string {
	// Render content area inside the box
	content := strings.Join(contentLines, "\n")

	// Build box using lipgloss border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrefixPopup).
		Padding(0, 1).
		Width(innerWidth)

	rendered := boxStyle.Render(content)

	// Split into lines to replace top and bottom borders
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}

	// Replace top border with titled version
	actualWidth := lipgloss.Width(lines[0])
	titleVisualWidth := lipgloss.Width(title)
	// Format: ╭─ title ─────╮
	usedByTitle := 5 + titleVisualWidth // "╭─ " + title + " " + "╮"
	remainingDashes := actualWidth - usedByTitle
	if remainingDashes < 1 {
		remainingDashes = 1
	}
	lines[0] = borderStyle.Render("╭─ ") + title + borderStyle.Render(" "+strings.Repeat("─", remainingDashes)+"╮")

	// Replace bottom border with "Esc to cancel" footer
	escText := MutedStyle.Render("Esc to cancel")
	escVisualWidth := lipgloss.Width(escText)
	usedByEsc := 5 + escVisualWidth
	remainingDashesBottom := actualWidth - usedByEsc
	if remainingDashesBottom < 1 {
		remainingDashesBottom = 1
	}
	lines[len(lines)-1] = borderStyle.Render("╰─ ") + escText + borderStyle.Render(" "+strings.Repeat("─", remainingDashesBottom)+"╯")

	return strings.Join(lines, "\n")
}

// OverlayPopup places a popup string on top of base content at bottom-center.
// Both base and popup are rendered strings. baseWidth and baseHeight define the
// dimensions of the base content area.
func OverlayPopup(base, popup string, baseWidth, baseHeight int) string {
	if popup == "" {
		return base
	}

	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")

	popupHeight := len(popupLines)
	popupWidth := 0
	for _, line := range popupLines {
		w := lipgloss.Width(line)
		if w > popupWidth {
			popupWidth = w
		}
	}

	// Pad base to baseHeight lines
	for len(baseLines) < baseHeight {
		baseLines = append(baseLines, "")
	}

	// Position: bottom-center, 1 line above the bottom
	startRow := baseHeight - popupHeight - 1
	if startRow < 0 {
		startRow = 0
	}
	startCol := (baseWidth - popupWidth) / 2
	if startCol < 0 {
		startCol = 0
	}

	// Overlay popup lines onto base lines.
	// Pad each popup line to popupWidth so shorter lines fully cover the background.
	for i, popupLine := range popupLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		lineWidth := lipgloss.Width(popupLine)
		if lineWidth < popupWidth {
			popupLine += strings.Repeat(" ", popupWidth-lineWidth)
		}
		baseLines[row] = overlayLineAt(baseLines[row], popupLine, startCol, baseWidth)
	}

	return strings.Join(baseLines, "\n")
}

// overlayLineAt places overlayStr on top of baseLine starting at column startCol.
func overlayLineAt(baseLine, overlayStr string, startCol, totalWidth int) string {
	// Ensure base is at least totalWidth wide
	baseWidth := lipgloss.Width(baseLine)
	if baseWidth < totalWidth {
		baseLine += strings.Repeat(" ", totalWidth-baseWidth)
	}

	overlayWidth := lipgloss.Width(overlayStr)

	// Use ANSI-aware Cut to extract left and right portions of the base line.
	left := ""
	if startCol > 0 {
		left = ansi.Cut(baseLine, 0, startCol)
		// Pad left to exact startCol width in case of wide chars
		leftW := lipgloss.Width(left)
		if leftW < startCol {
			left += strings.Repeat(" ", startCol-leftW)
		}
	}

	rightStart := startCol + overlayWidth
	right := ""
	if rightStart < totalWidth {
		right = ansi.Cut(baseLine, rightStart, totalWidth)
	}

	return left + overlayStr + right
}

// Common prefix hints used across multiple views.
var (
	// StatusPrefixHints contains hints for the "," (status) prefix.
	StatusPrefixHints = []PrefixHint{
		{Key: "s", Description: "start", Detail: "Set task to in-progress"},
		{Key: "d", Description: "done", Detail: "Mark task as completed"},
		{Key: "b", Description: "block", Detail: "Mark task as blocked"},
		{Key: "x", Description: "cancel", Detail: "Cancel the task"},
		{Key: "o", Description: "reopen", Detail: "Reopen a closed task"},
	}

	// FoldPrefixHints contains hints for the "z" (fold) prefix.
	FoldPrefixHints = []PrefixHint{
		{Key: "a", Description: "toggle", Detail: "Toggle fold on current node"},
		{Key: "o", Description: "open", Detail: "Expand current node"},
		{Key: "c", Description: "close", Detail: "Collapse current node"},
		{Key: "R", Description: "expand-all", Detail: "Expand all nodes"},
		{Key: "M", Description: "collapse-all", Detail: "Collapse all nodes"},
		{Key: "v", Description: "focus", Detail: "Focus on current subtree"},
	}

	// NavigationPrefixHints contains hints for the "g" (go/navigation) prefix - basic version.
	NavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first", Detail: "Jump to first item"},
	}

	// PriorityViewNavigationPrefixHints contains "g" prefix hints for priority view.
	PriorityViewNavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first", Detail: "Jump to first item"},
		{Key: "t", Description: "tree", Detail: "Switch to tree view"},
	}

	// ProjectTreeNavigationPrefixHints contains extended "g" prefix hints for project tree.
	ProjectTreeNavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first", Detail: "Jump to first item"},
		{Key: "a", Description: "active", Detail: "Go to Active section"},
		{Key: "n", Description: "next", Detail: "Go to Next section"},
		{Key: "i", Description: "inbox", Detail: "Go to Inbox section"},
		{Key: "w", Description: "waiting", Detail: "Go to Waiting section"},
		{Key: "s", Description: "someday", Detail: "Go to Someday section"},
		{Key: "c", Description: "completed", Detail: "Go to Completed section"},
		{Key: "z", Description: "snoozed", Detail: "Go to Snoozed section"},
		{Key: "A", Description: "→Active", Detail: "Send task to Active section"},
		{Key: "N", Description: "→Next", Detail: "Send task to Next section"},
		{Key: "I", Description: "→Inbox", Detail: "Send task to Inbox section"},
		{Key: "W", Description: "→Waiting", Detail: "Send task to Waiting section"},
		{Key: "S", Description: "→Someday", Detail: "Send task to Someday section"},
		{Key: "o", Description: "origin", Detail: "Jump to task origin"},
		{Key: "d", Description: "due-filter", Detail: "Toggle due date filter"},
		{Key: "v", Description: "view-cycle", Detail: "Cycle through view modes"},
		{Key: "h", Description: "history", Detail: "Show task history"},
		{Key: "X", Description: "deselect", Detail: "Clear selection"},
	}

	// PriorityPrefixHints contains hints for the "!" (priority) prefix.
	PriorityPrefixHints = []PrefixHint{
		{Key: "1", Description: "critical", Detail: "Set priority to critical"},
		{Key: "2", Description: "high", Detail: "Set priority to high"},
		{Key: "3", Description: "normal", Detail: "Set priority to normal"},
		{Key: "4", Description: "low", Detail: "Set priority to low"},
		{Key: "5", Description: "someday", Detail: "Set priority to someday"},
	}

	// DueDatePrefixHints contains hints for the "d" (due date) prefix.
	DueDatePrefixHints = []PrefixHint{
		{Key: "t", Description: "today", Detail: "Set due date to today"},
		{Key: "m", Description: "tomorrow", Detail: "Set due date to tomorrow"},
		{Key: "w", Description: "+week", Detail: "Set due date one week out"},
		{Key: "n", Description: "+month", Detail: "Set due date one month out"},
		{Key: "0", Description: "clear", Detail: "Remove due date"},
		{Key: "d", Description: "picker", Detail: "Open date picker"},
	}

	// SnoozePrefixHints contains hints for the "s" (snooze) prefix.
	SnoozePrefixHints = []PrefixHint{
		{Key: "t", Description: "tomorrow", Detail: "Snooze until tomorrow"},
		{Key: "w", Description: "+week", Detail: "Snooze for one week"},
		{Key: "m", Description: "+month", Detail: "Snooze for one month"},
		{Key: "n", Description: "next Mon", Detail: "Snooze until next Monday"},
		{Key: "0", Description: "clear", Detail: "Remove snooze date"},
		{Key: "d", Description: "picker", Detail: "Open date picker"},
	}

	// SectionPrefixHints contains hints for the "s" (section focus) prefix in Review view.
	SectionPrefixHints = []PrefixHint{
		{Key: "d", Description: "overdue", Detail: "Jump to overdue section"},
		{Key: "t", Description: "today", Detail: "Jump to today section"},
		{Key: "s", Description: "due-soon", Detail: "Jump to due-soon section"},
		{Key: "l", Description: "due-later", Detail: "Jump to due-later section"},
		{Key: "f", Description: "future", Detail: "Jump to future section"},
		{Key: "o", Description: "old", Detail: "Jump to old section"},
	}

	// CreateLinkedPrefixHints contains hints for the "c" (create linked) prefix.
	CreateLinkedPrefixHints = []PrefixHint{
		{Key: "b", Description: "blocker", Detail: "Create a blocking task"},
		{Key: "d", Description: "dependent", Detail: "Create a dependent task"},
		{Key: "s", Description: "subtask", Detail: "Create a subtask"},
		{Key: "f", Description: "follow-up", Detail: "Create a follow-up task"},
		{Key: "p", Description: "cross-project", Detail: "Link to another project"},
		{Key: "n", Description: "navigate", Detail: "Navigate to linked task"},
		{Key: "x", Description: "unlink", Detail: "Remove a link"},
	}

	// KanbanNavigationPrefixHints contains "g" prefix hints for kanban view.
	// Supports column navigation and send-to-section operations.
	KanbanNavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first", Detail: "Jump to first item"},
		{Key: "a", Description: "active", Detail: "Go to Active column"},
		{Key: "n", Description: "next", Detail: "Go to Next column"},
		{Key: "i", Description: "inbox", Detail: "Go to Inbox column"},
		{Key: "w", Description: "waiting", Detail: "Go to Waiting column"},
		{Key: "s", Description: "someday", Detail: "Go to Someday column"},
		{Key: "A", Description: "→Active", Detail: "Send task to Active column"},
		{Key: "N", Description: "→Next", Detail: "Send task to Next column"},
		{Key: "I", Description: "→Inbox", Detail: "Send task to Inbox column"},
		{Key: "W", Description: "→Waiting", Detail: "Send task to Waiting column"},
		{Key: "S", Description: "→Someday", Detail: "Send task to Someday column"},
		{Key: "h", Description: "history", Detail: "Show task history"},
		{Key: "X", Description: "deselect", Detail: "Clear selection"},
	}

	// KanbanFoldPrefixHints contains "z" prefix hints for kanban fold operations.
	KanbanFoldPrefixHints = []PrefixHint{
		{Key: "a", Description: "toggle", Detail: "Toggle fold on current node"},
		{Key: "o", Description: "expand", Detail: "Expand current node"},
		{Key: "c", Description: "collapse", Detail: "Collapse current node"},
		{Key: "R", Description: "expand-all", Detail: "Expand all nodes"},
		{Key: "M", Description: "collapse-all", Detail: "Collapse all nodes"},
		{Key: "v", Description: "focus", Detail: "Focus on current subtree"},
	}
)
