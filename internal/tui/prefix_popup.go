package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
)

// PrefixHint represents a single key hint in a prefix popup.
type PrefixHint struct {
	Key         string
	Description string
	Detail      string
	Active      bool // When true, description and detail are rendered in orange
	Disabled    bool // When true, all columns are rendered dimmed (muted)
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
	",":   "status",
	"z":   "fold",
	"g":   "go-to",
	"m":   "move",
	"!":   "priority",
	"d":   "due-date",
	"s":   "snooze",
	"c":   "create",
	"t":   "task",
	"b":   "blockers",
	"P":   "perspectives",
	"F":   "filter",
	"Ft":  "filter-type",
	"Fs":  "filter-status",
	"Fd":  "filter-due",
	"Fa":  "filter-attr",
	"Fas": "severity",
	"Far": "reproducible",
	"Fae": "effort",
	"Fac": "category",
	"Fav": "value",
	"Faw": "worktype",
	"Fao": "scope",
	"Fap": "points",
	"FS":  "filter-score",
	"F.":  "filter-stage",
	"Fv":  "saved-filters",
	".":   "stage",
	";":   "tt",
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
	maxAvailableHeight := max(
		// account for border, footer line, some margin
		min(

			height-4, 20), 3)

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

		activeDescStyle := lipgloss.NewStyle().Foreground(ColorWarning)
		activeDetailStyle := lipgloss.NewStyle().Foreground(ColorWarning)

		contentWidth := 0
		for _, h := range keyHints {
			keyStyle := PrefixKeyStyle
			descStyle := PrefixDescStyle
			detailStyle := MutedStyle
			if h.Disabled {
				keyStyle = MutedStyle
				descStyle = MutedStyle
				detailStyle = MutedStyle
			} else if h.Active {
				descStyle = activeDescStyle
				detailStyle = activeDetailStyle
			}

			var line string
			var rendered string
			if hasDetail {
				line = fmt.Sprintf("%-*s  %-*s  %s", maxKeyWidth, h.Key, maxDescWidth, h.Description, h.Detail)
				rendered = keyStyle.Render(fmt.Sprintf("%-*s", maxKeyWidth, h.Key)) + "  " +
					descStyle.Render(fmt.Sprintf("%-*s", maxDescWidth, h.Description)) + "  " +
					detailStyle.Render(h.Detail)
			} else {
				line = fmt.Sprintf("%-*s  %s", maxKeyWidth, h.Key, h.Description)
				rendered = keyStyle.Render(fmt.Sprintf("%-*s", maxKeyWidth, h.Key)) + "  " + descStyle.Render(h.Description)
			}
			contentLines = append(contentLines, rendered)
			if len(line) > contentWidth {
				contentWidth = len(line)
			}
		}

		// Set box width based on content
		boxInnerWidth := min(
			// 1 padding each side
			max(

				contentWidth+2, minWidth), maxWidth)

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
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)

	// Replace bottom border with "Esc to cancel" footer
	escText := MutedStyle.Render("Esc to cancel")
	lines[len(lines)-1] = renderTitledBottomBorder(escText, actualWidth, borderStyle)

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
	startRow := max(baseHeight-popupHeight-1, 0)
	startCol := max((baseWidth-popupWidth)/2, 0)

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

// OverlayPopupBottomLeft places a popup string on top of base content at bottom-left.
// Same as OverlayPopup but left-aligned with a 1-column margin instead of centered.
func OverlayPopupBottomLeft(base, popup string, baseWidth, baseHeight int) string {
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

	// Position: bottom-left, 1 line above the bottom
	startRow := max(baseHeight-popupHeight-1, 0)
	startCol := 1

	// Overlay popup lines onto base lines.
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
		{Key: "r", Description: "review", Detail: "Send task to review"},
		{Key: "S", Description: "standby", Detail: "Put task on standby"},
		{Key: "k", Description: "skip", Detail: "Skip next recurrence"},
		{Key: "D", Description: "defer", Detail: "Defer recurrence"},
	}

	// FoldPrefixHints contains hints for the "z" (fold) prefix.
	FoldPrefixHints = []PrefixHint{
		{Key: "a", Description: "toggle", Detail: "Toggle fold on current node"},
		{Key: "o", Description: "open", Detail: "Expand current node"},
		{Key: "c", Description: "close", Detail: "Collapse current node"},
		{Key: "R", Description: "expand-all", Detail: "Expand all nodes"},
		{Key: "M", Description: "collapse-all", Detail: "Collapse all nodes"},
		{Key: "v", Description: "focus", Detail: "Focus on current subtree"},
		{Key: "p", Description: "preview", Detail: "Toggle inline preview for cursor task"},
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
		{Key: "r", Description: "review", Detail: "Go to Review section"},
		{Key: "S", Description: "standby", Detail: "Go to Standby section"},
		{Key: "z", Description: "snoozed", Detail: "Go to Snoozed section"},
		{Key: "*", Description: "all", Detail: "Go to All section"},
		{Key: "o", Description: "origin", Detail: "Jump to task origin"},
		{Key: "d", Description: "due-filter", Detail: "Toggle due date filter"},
		{Key: "v", Description: "view-cycle", Detail: "Cycle through view modes"},
		{Key: "h", Description: "history", Detail: "Show task history"},
		{Key: "X", Description: "deselect", Detail: "Clear selection"},
		{Key: "l", Description: "activity", Detail: "Show project recent activity"},
	}

	// ProjectTreeMovePrefixHints contains "m" prefix hints for project tree move operations.
	ProjectTreeMovePrefixHints = []PrefixHint{
		{Key: "a", Description: "→Active", Detail: "Send task to Active section"},
		{Key: "n", Description: "→Next", Detail: "Send task to Next section"},
		{Key: "i", Description: "→Inbox", Detail: "Send task to Inbox section"},
		{Key: "w", Description: "→Waiting", Detail: "Send task to Waiting section"},
		{Key: "r", Description: "→Review", Detail: "Send task to Review section"},
		{Key: "S", Description: "→Standby", Detail: "Send task to Standby section"},
		{Key: "s", Description: "→Someday", Detail: "Send task to Someday section"},
		{Key: "K", Description: "to-top", Detail: "Move task to top of section"},
		{Key: "J", Description: "to-bottom", Detail: "Move task to bottom of section"},
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
		{Key: "r", Description: "in review", Detail: "Focus in-review section"},
		{Key: "S", Description: "on standby", Detail: "Focus on-standby section"},
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

	// OutlinerFoldPrefixHints contains "z" prefix hints for outliner fold operations.
	OutlinerFoldPrefixHints = []PrefixHint{
		{Key: "a", Description: "toggle", Detail: "Toggle fold on current node"},
		{Key: "o", Description: "expand", Detail: "Expand current node"},
		{Key: "c", Description: "collapse", Detail: "Collapse current node"},
		{Key: "R", Description: "expand-all", Detail: "Expand all nodes"},
		{Key: "M", Description: "collapse-all", Detail: "Collapse all nodes"},
		{Key: "v", Description: "focus", Detail: "Focus on current subtree"},
		{Key: "p", Description: "preview", Detail: "Toggle inline preview for cursor task"},
	}

	// SectionFoldPrefixHints contains hints for the "z" (fold) prefix in section-based views.
	SectionFoldPrefixHints = []PrefixHint{
		{Key: "R", Description: "expand-all", Detail: "Expand all sections"},
		{Key: "M", Description: "collapse-all", Detail: "Collapse all sections"},
	}

	// FilterTypePrefixHints contains hints for the "Ft" (filter type) sub-prefix.
	FilterTypePrefixHints = []PrefixHint{
		{Key: "e", Description: "epic"},
		{Key: "s", Description: "story"},
		{Key: "t", Description: "task"},
		{Key: "b", Description: "bug"},
		{Key: "i", Description: "idea"},
		{Key: "0", Description: "clear"},
	}

	// FilterStatusPrefixHints contains hints for the "Fs" (filter status) sub-prefix.
	FilterStatusPrefixHints = []PrefixHint{
		{Key: "t", Description: "todo"},
		{Key: "s", Description: "in-progress"},
		{Key: "r", Description: "review"},
		{Key: "S", Description: "standby"},
		{Key: "b", Description: "blocked"},
		{Key: "d", Description: "done"},
		{Key: "x", Description: "canceled"},
		{Key: "0", Description: "clear"},
	}

	// FilterDuePrefixHints contains hints for the "Fd" (filter due-date) sub-prefix.
	FilterDuePrefixHints = []PrefixHint{
		{Key: "o", Description: "overdue"},
		{Key: "t", Description: "today"},
		{Key: "w", Description: "this-week"},
		{Key: "n", Description: "no-due-date"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrPrefixHints contains hints for the "Fa" (filter attribute) sub-prefix.
	FilterAttrPrefixHints = []PrefixHint{
		{Key: "s", Description: "severity", Detail: "Bug severity (critical/major/minor/cosmetic)"},
		{Key: "r", Description: "reproducible", Detail: "Bug reproducibility (yes/no/sometimes)"},
		{Key: "e", Description: "effort", Detail: "Effort estimate (small/medium/large/unknown)"},
		{Key: "c", Description: "category", Detail: "Idea category (feature/improvement/refactor/research)"},
		{Key: "v", Description: "value", Detail: "Idea value (low/medium/high)"},
		{Key: "V", Description: "validated", Detail: "Idea validated flag"},
		{Key: "w", Description: "worktype", Detail: "Task work type (development/documentation/testing/devops/design)"},
		{Key: "o", Description: "scope", Detail: "Epic scope (defined/evolving/undefined)"},
		{Key: "p", Description: "points", Detail: "Story points (1/2/3/5/8)"},
		{Key: "0", Description: "clear", Detail: "Clear all attribute filters"},
	}

	// FilterAttrSeverityPrefixHints contains hints for bug severity values.
	FilterAttrSeverityPrefixHints = []PrefixHint{
		{Key: "c", Description: "critical"},
		{Key: "j", Description: "major"},
		{Key: "m", Description: "minor"},
		{Key: "o", Description: "cosmetic"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrReproduciblePrefixHints contains hints for bug reproducible values.
	FilterAttrReproduciblePrefixHints = []PrefixHint{
		{Key: "y", Description: "yes"},
		{Key: "n", Description: "no"},
		{Key: "s", Description: "sometimes"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrEffortPrefixHints contains hints for effort estimate values.
	FilterAttrEffortPrefixHints = []PrefixHint{
		{Key: "s", Description: "small"},
		{Key: "m", Description: "medium"},
		{Key: "l", Description: "large"},
		{Key: "u", Description: "unknown"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrCategoryPrefixHints contains hints for idea category values.
	FilterAttrCategoryPrefixHints = []PrefixHint{
		{Key: "f", Description: "feature"},
		{Key: "i", Description: "improvement"},
		{Key: "r", Description: "refactor"},
		{Key: "s", Description: "research"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrValuePrefixHints contains hints for idea value values.
	FilterAttrValuePrefixHints = []PrefixHint{
		{Key: "l", Description: "low"},
		{Key: "m", Description: "medium"},
		{Key: "h", Description: "high"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrWorktypePrefixHints contains hints for task worktype values.
	FilterAttrWorktypePrefixHints = []PrefixHint{
		{Key: "d", Description: "development"},
		{Key: "o", Description: "documentation"},
		{Key: "t", Description: "testing"},
		{Key: "v", Description: "devops"},
		{Key: "e", Description: "design"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrScopePrefixHints contains hints for epic scope values.
	FilterAttrScopePrefixHints = []PrefixHint{
		{Key: "d", Description: "defined"},
		{Key: "e", Description: "evolving"},
		{Key: "u", Description: "undefined"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrPointsPrefixHints contains hints for story points values.
	FilterAttrPointsPrefixHints = []PrefixHint{
		{Key: "1", Description: "1 point"},
		{Key: "2", Description: "2 points"},
		{Key: "3", Description: "3 points"},
		{Key: "5", Description: "5 points"},
		{Key: "8", Description: "8 points"},
		{Key: "0", Description: "clear"},
	}

	// SavedFilterPrefixHints contains hints for the "Fv" (saved filters/filters) sub-prefix.
	SavedFilterPrefixHints = []PrefixHint{
		{Key: "s", Description: "save", Detail: "Save current view"},
		{Key: "l", Description: "load", Detail: "Browse/apply saved filters"},
		{Key: "d", Description: "delete", Detail: "Delete a saved filter"},
		{Key: "c", Description: "criteria", Detail: "Create criteria filter"},
	}

	// FilterScorePrefixHints contains hints for the "FS" (filter score) sub-prefix.
	// Kept for backward compatibility; prefer BuildFilterScorePrefixHints for dynamic hints.
	FilterScorePrefixHints = []PrefixHint{
		{Key: "1", Description: ">=30"},
		{Key: "2", Description: ">=60"},
		{Key: "3", Description: ">=90"},
		{Key: "4", Description: ">=120"},
		{Key: "5", Description: ">=150"},
		{Key: "6", Description: ">=180"},
		{Key: "7", Description: ">=210"},
		{Key: "8", Description: ">=240"},
		{Key: "9", Description: ">=270"},
		{Key: "c", Description: "custom"},
		{Key: "C", Description: "set value"},
		{Key: "0", Description: "clear"},
	}

	// TaskPrefixHints contains hints for the "t" (task action) prefix.
	TaskPrefixHints = []PrefixHint{
		{Key: "n", Description: "new", Detail: "Create new task"},
		{Key: "N", Description: "new-template", Detail: "Create task with template picker"},
		{Key: "r", Description: "rename", Detail: "Rename task (inline)"},
		{Key: "s", Description: "section", Detail: "Change section"},
		{Key: "o", Description: "open", Detail: "Open in external editor"},
		{Key: "p", Description: "pin", Detail: "Toggle pin/unpin"},
		{Key: "t", Description: "tags", Detail: "Manage task tags"},
		{Key: "c", Description: "comments", Detail: "View/add comments"},
		{Key: "a", Description: "attachments", Detail: "View/add attachments"},
		{Key: "l", Description: "checklist", Detail: "Edit checklist items"},
		{Key: "d", Description: "description", Detail: "Edit description"},
		{Key: "m", Description: "move", Detail: "Move to another project"},
		{Key: "D", Description: "delete", Detail: "Delete task"},
	}

	// SemicolonPrefixHints contains hints for the ";" (tt global) prefix.
	SemicolonPrefixHints = []PrefixHint{
		{Key: "/", Description: "global search", Detail: "Search tasks across all projects"},
		{Key: "a", Description: "all tasks", Detail: "View all tasks across projects"},
		{Key: "d", Description: "daily review", Detail: "Open Daily Review"},
		{Key: "f", Description: "what's next", Detail: "Cross-project task suggestions"},
		{Key: "i", Description: "global inbox", Detail: "Process inbox across all projects"},
		{Key: "l", Description: "latest activity", Detail: "Recently modified tasks"},
		{Key: "m", Description: "metrics", Detail: "Completion rates and throughput"},
		{Key: "n", Description: "notifications", Detail: "Open Notification Center"},
		{Key: "o", Description: "outliner", Detail: "Cross-project outliner"},
		{Key: "s", Description: "settings", Detail: "Open Settings"},
		{Key: "w", Description: "weekly review", Detail: "Open Weekly Review"},
		{Key: "B", Description: "global blockers", Detail: "Blockers across all projects"},
		{Key: "D", Description: "due date overview", Detail: "View tasks by due date"},
		{Key: "F", Description: "focus board", Detail: "Personal command center"},
		{Key: "N", Description: "global notes", Detail: "Open global notes"},
		{Key: "W", Description: "workflow rules", Detail: "Manage workflow automation rules"},
	}

	// BlockerPrefixHints contains hints for the "b" (blocker/dependency) prefix.
	BlockerPrefixHints = []PrefixHint{
		{Key: "b", Description: "view", Detail: "View blockers/dependencies"},
		{Key: "m", Description: "manage", Detail: "Manage dependencies"},
	}
)

// CommonTaskActionPrefixHints returns the prefix hints shared across most
// task-operating views: status (,), priority (!), stage (.), due date (d),
// snooze (s), create-linked (c), and move (m). Views extend the returned
// map with their view-specific entries (e.g. "g" navigation, "z" folds).
func CommonTaskActionPrefixHints(stages []string, currentStage string) PrefixHints {
	return PrefixHints{
		",": StatusPrefixHints,
		"!": PriorityPrefixHints,
		".": BuildStagePrefixHints(stages, currentStage),
		"d": DueDatePrefixHints,
		"s": SnoozePrefixHints,
		"c": CreateLinkedPrefixHints,
		"m": SecondaryViewMovePrefixHints,
	}
}

// BuildFilterScorePrefixHints builds dynamic filter score hints showing the
// current custom value from config. Used by project tree and outliner views.
func BuildFilterScorePrefixHints(cfg *config.Config) []PrefixHint {
	customDetail := "(not set)"
	if cfg != nil && cfg.ScoreFilterCustom != nil {
		customDetail = fmt.Sprintf(">=%.0f", *cfg.ScoreFilterCustom)
	}

	return []PrefixHint{
		{Key: "1", Description: ">=30"},
		{Key: "2", Description: ">=60"},
		{Key: "3", Description: ">=90"},
		{Key: "4", Description: ">=120"},
		{Key: "5", Description: ">=150"},
		{Key: "6", Description: ">=180"},
		{Key: "7", Description: ">=210"},
		{Key: "8", Description: ">=240"},
		{Key: "9", Description: ">=270"},
		{Key: "c", Description: "custom", Detail: customDetail},
		{Key: "C", Description: "set value", Detail: "inline input"},
		{Key: "0", Description: "clear"},
	}
}

// BuildStagePrefixHints builds dynamic stage hints for the "." (stage) prefix.
// Keys 1-9 map to the first 9 configured stages, 0 clears.
func BuildStagePrefixHints(stages []string, currentStage string) []PrefixHint {
	hints := make([]PrefixHint, 0, len(stages)+1)
	for i, stage := range stages {
		hints = append(hints, PrefixHint{
			Key:         stageKeyForIndex(i),
			Description: stage,
			Active:      stage == currentStage,
		})
	}
	hints = append(hints, PrefixHint{Key: "0", Description: "clear"})
	return hints
}

// BuildFilterStagePrefixHints builds dynamic filter stage hints for the "F." prefix.
// Keys 1-9 map to the first 9 configured stages, 0 clears.
func BuildFilterStagePrefixHints(stages []string, currentFilter string) []PrefixHint {
	hints := make([]PrefixHint, 0, len(stages)+1)
	for i, stage := range stages {
		hints = append(hints, PrefixHint{
			Key:         stageKeyForIndex(i),
			Description: stage,
			Active:      stage == currentFilter,
		})
	}
	hints = append(hints, PrefixHint{Key: "0", Description: "clear"})
	return hints
}
