package tui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// The dropdown is drawn inline, immediately under the bar row and left-padded to
// the column the layout gave the open label. It is not an overlay: it grows the
// chrome the list budget is measured against rather than floating over rows, so
// the rows it covers are rows it paid for. That is the same bargain the status
// strip and the search suggestion pane already make, and it is why the block
// this file returns is added to extraFooter by the caller.

const (
	// menuBarDropdownCheckColumn is the width of the checkmark gutter. Every
	// entry in a menu that has any state to show reserves it, so the labels stay
	// on one column instead of the checked rows stepping right.
	menuBarDropdownCheckColumn = 2
	// menuBarDropdownMinRows is the shortest list worth drawing. Below this the
	// dropdown says less than the bar above it already does.
	menuBarDropdownMinRows = 3
	// menuBarDropdownListRows is what the frame costs around its rows: two border
	// lines and the two halo lines renderOverlayBox adds outside them.
	menuBarDropdownListRows = 4
)

// openMenuBarIndex is which bar label is drawn open, or -1 when none is. It is
// read by the bar row and by the dropdown from the one resolution, so the
// highlight and the block under it can never name different menus.
func (m *OutlinerModel) openMenuBarIndex() int {
	if m == nil || !m.menuBarOpen {
		return -1
	}
	return m.menuBarMenuIndex
}

// renderOutlinerMenuDropdown draws the open menu, or the empty string when none
// is open. It re-resolves the stored item index against the entries this frame
// actually holds and writes the resolved index back, which is what stops the
// highlight a user sees and the entry Enter runs from disagreeing after a row
// left the menu underneath them.
func (m *OutlinerModel) renderOutlinerMenuDropdown(width int) string {
	menus, ok := m.openMenuBarMenus()
	if !ok || width <= 0 {
		return ""
	}
	entries := menus[m.menuBarMenuIndex].Entries
	cursor := resolveMenuBarItem(entries, m.menuBarItemIndex)
	if cursor < 0 {
		return ""
	}
	m.menuBarItemIndex = cursor

	innerWidth := menuBarDropdownWidth(entries, width)
	// The box's own rows come off the budget before the list is sized.
	maxBoxRows := m.menuBarDropdownRowBudget(width)
	listHeight := max(min(len(entries), maxBoxRows-menuBarDropdownListRows), 1)

	surface, err := newMenuBarDropdownSurface(entries, innerWidth-4)
	if err != nil {
		// The surface refuses only a malformed item set, which is this function's
		// own construction rather than anything a user did. Drawing no dropdown is
		// the honest failure: the bar above still says which menu is open.
		return ""
	}
	surface.SetCursorToID(menuBarDropdownItemID(cursor))
	content := RenderCompactListSurface(surface, innerWidth-4, listHeight)
	box := renderOverlayBox(content, innerWidth, "", "",
		lipgloss.NewStyle().Foreground(theme.ColorPrimary))
	box = fitOverlayPopupHeight(box, maxBoxRows)
	return indentMenuBarDropdown(box, menuBarDropdownStartCol(menus, width, m.menuBarMenuIndex, box), width)
}

// menuBarDropdownRowBudget is how tall the block may grow. The frame it is drawn
// on holds the bar row and this box alone, so the budget is everything under the
// bar: contentHeight subtracts both the bar row and the operational title, and
// the title is not drawn on this frame, which is the one row added back here.
//
// The status strip is subtracted because the shell pins it — AppModel.View sizes
// and pads content against the status bar's height — so a box that claimed those
// rows would be clipped from below rather than scrolled, and a menu that says
// nothing about the entries it dropped is worse than one that scrolls.
//
// The floor keeps the shortest drawable box: below it the dropdown says less
// than the bar above it already does, and at a terminal that short there is
// nothing better to do than let the frame clip it.
func (m *OutlinerModel) menuBarDropdownRowBudget(width int) int {
	available := m.contentHeight() + 1 - m.statusStripHeight(width)
	return max(available, menuBarDropdownMinRows+menuBarDropdownListRows)
}

// menuBarDropdownStartCol is the column the block is padded to: the open label's
// own StartCol, clamped so the right edge stays on screen. The clamp is
// overlayPopup's, copied rather than called because that helper composites rows
// of a finished frame and this block is written into the content stream.
//
// A menu the drop ladder shed at a narrow width has no drawn label to align to,
// so its dropdown starts at the left margin instead of under a column nothing is
// painted at.
func menuBarDropdownStartCol(menus []resolvedMenu, width, openIdx int, box string) int {
	startCol := 0
	if boxes := menuBarLayout(menus, width); openIdx < len(boxes) {
		startCol = boxes[openIdx].StartCol
	}
	return min(max(startCol, 0), max(width-popupWidth(box), 0))
}

// indentMenuBarDropdown moves the finished block right to its column. Every line
// is padded, including the halo rows, so the frame stays rectangular.
func indentMenuBarDropdown(box string, startCol, width int) string {
	if box == "" {
		return ""
	}
	pad := strings.Repeat(" ", max(startCol, 0))
	lines := strings.Split(box, "\n")
	for i, line := range lines {
		lines[i] = textfit.TruncateToVisualWidth(pad+line, width)
	}
	return strings.Join(lines, "\n")
}

// menuBarDropdownWidth sizes the frame to its longest entry rather than to a
// fraction of the screen. A dropdown is anchored under one label and reads as
// belonging to it, so a box that claimed three fifths of the terminal would
// read as a modal that happens to be left-aligned.
func menuBarDropdownWidth(entries []resolvedMenuEntry, width int) int {
	marker := menuBarDropdownMarkerWidth(entries)
	longest := 0
	for _, entry := range entries {
		if entry.OperationID == "" {
			continue
		}
		// The gap is LayoutListText's own, charged here because the labels are
		// padded out to it: the widest row is the column every key hint is then
		// flushed to, so a menu wide enough for that row truncates nothing.
		row := marker + lipgloss.Width(entry.Label)
		if entry.Key != "" {
			row += lipgloss.Width(listTextGap) + lipgloss.Width(entry.Key)
		}
		longest = max(longest, row)
	}
	// The four columns are the frame's: one border and one padding cell each side.
	return min(max(longest+4, 20), max(width-6, 24))
}

// menuBarDropdownMarkerWidth is the chrome charged to every row before its text:
// the cursor gutter the list renderer prepends, plus the checkmark column when
// this menu has any state to show. It is measured once for the whole menu rather
// than per row, because a column that appeared only on the checked rows would
// step every label beside it out of line.
func menuBarDropdownMarkerWidth(entries []resolvedMenuEntry) int {
	if menuBarDropdownChecks(entries) {
		return 2 + menuBarDropdownCheckColumn
	}
	return 2
}

// menuBarDropdownChecks reports whether this menu has any state to read back.
// One entry carrying a checkbox or a radio is enough: the gutter is a column of
// the menu, not a property of a row.
func menuBarDropdownChecks(entries []resolvedMenuEntry) bool {
	for _, entry := range entries {
		if entry.Checked != nil {
			return true
		}
	}
	return false
}

// newMenuBarDropdownSurface builds the list the frame holds. It is rebuilt every
// frame from the live resolution for the same reason resolvedMenuBar is: a
// cached surface would keep offering a row the cursor stopped being able to run.
//
// The menu owns its own navigation, so this surface never receives a cursor key:
// separators are ordinary rows here because nothing in it can land on one.
func newMenuBarDropdownSurface(entries []resolvedMenuEntry, contentWidth int) (*ListSurfaceModel, error) {
	checked := menuBarDropdownChecks(entries)
	// A rule starts where the labels start and ends where they end, so it wears
	// the same marker they do and is measured against what the marker leaves.
	blank := ""
	if checked {
		blank = strings.Repeat(" ", menuBarDropdownCheckColumn)
	}
	textWidth := max(contentWidth-menuBarDropdownMarkerWidth(entries), 1)
	items := make([]ListItem, 0, len(entries))
	for index, entry := range entries {
		item := ListItem{ID: menuBarDropdownItemID(index), PrimaryMarker: blank}
		if entry.OperationID == "" {
			item.PrimaryText = strings.Repeat("─", textWidth)
			items = append(items, item)
			continue
		}
		item.PrimaryText = padMenuBarDropdownLabel(entry.Label, entry.Key, textWidth)
		item.SecondaryText = entry.Key
		if entry.Checked != nil && *entry.Checked {
			item.PrimaryMarker = "✓ "
		}
		items = append(items, item)
	}
	return NewListSurfaceModel(items, nil, ListSurfaceConfig{SelectionMode: ListSelectionSingle})
}

// padMenuBarDropdownLabel pads a label out so its key hint lands flush against
// the menu's right edge, the way a desktop menu reads: the eye scans one column
// of labels and one column of keys instead of a ragged pair. The padding stops
// short of LayoutListText's own gap, which the layout adds back after it.
//
// A row too wide to hold both at this width is left unpadded, so the layout
// truncates exactly as it did before. Alignment is a courtesy the menu extends
// when it has the columns to spend; a key pushed off the edge to buy it would
// cost the one thing the row is there to say.
func padMenuBarDropdownLabel(label, key string, textWidth int) string {
	if key == "" {
		return label
	}
	pad := textWidth - lipgloss.Width(label) - lipgloss.Width(listTextGap) - lipgloss.Width(key)
	if pad <= 0 {
		return label
	}
	return label + strings.Repeat(" ", pad)
}

// menuBarDropdownItemID names one row by its position in the resolution, which
// is the only identity a menu entry has: two rows can name the same operation
// with different wording, and a separator names none.
func menuBarDropdownItemID(index int) ListItemID {
	return ListItemID("menu-entry-" + strconv.Itoa(index))
}
