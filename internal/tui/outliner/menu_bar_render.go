package outliner

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// The bar is a view-local gated sub-state of the Outliner, not a presentation:
// it mints no PresentationID, opens no overlay, claims no InputStrategy, and
// takes no manifest entry, exactly as `gc` does. It costs one row of the list
// while it is shown, and contentHeight is the single place that subtracts it.

// menuBarLabelPadding is the horizontal padding around one bar label. It is the
// tab bar's own spelling of the same measurement, so a menu label and a tab
// label occupy the same shape.
const menuBarLabelPadding = 1

// menuLabelBox is one drawn menu label: its wording, the column its rendered
// cell starts at, and the width that cell occupies. The columns are what a
// dropdown is padded to, which is why the layout is a value rather than a
// string — RenderTabBar returns only a string and so can never say where a
// label landed.
type menuLabelBox struct {
	Title    string
	StartCol int
	Width    int
}

// menuBarMeasureStyle is the plain style the layout measures a label with, the
// same one RenderColoredTabBar sizes a tab by. Padding is the only thing that
// decides a cell's width, so this style and the drawing ones must agree on it or
// the measured layout and the painted row would drift apart.
func menuBarMeasureStyle() lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, menuBarLabelPadding)
}

// menuBarGap is the blank run between two cells and after the last one. It
// carries the theme background for the same reason a tab filler does: the bar is
// one band, and an unpainted gap would cut it into words.
func menuBarGap(width int) string {
	if width <= 0 {
		return ""
	}
	s := lipgloss.NewStyle().Width(width)
	if theme.ThemeBgColor != "" {
		s = s.Background(lipgloss.Color(theme.ThemeBgColor))
	}
	return s.Render("")
}

// menuBarLayout places the menus left to right and reports where each one
// landed. When the labels do not all fit it drops whole menus from the right —
// Help first, then Find, then Go — rather than truncating one: a dropped menu
// is still reachable by the accelerators its entries carry, while a truncated
// label is a lie about what the bar offers.
//
// The first menu always survives. At a width no single label fits, the row is
// clipped by the same full-width fit every other Outliner line goes through,
// which is the honest failure for a terminal that narrow.
func menuBarLayout(menus []ResolvedMenu, width int) []menuLabelBox {
	if len(menus) == 0 || width <= 0 {
		return nil
	}
	style := menuBarMeasureStyle()
	boxes := make([]menuLabelBox, 0, len(menus))
	col := 0
	for _, menu := range menus {
		cell := lipgloss.Width(style.Render(menu.Title))
		boxes = append(boxes, menuLabelBox{Title: menu.Title, StartCol: col, Width: cell})
		col += cell
	}
	for len(boxes) > 1 && col > width {
		col -= boxes[len(boxes)-1].Width
		boxes = boxes[:len(boxes)-1]
	}
	return boxes
}

// renderMenuBarRow draws the bar from its layout. It writes each label at the
// column the layout assigned it, so the row a user sees and the offsets a
// dropdown is padded to come from one computation rather than two.
//
// openIdx names the menu whose dropdown is open, or -1 when none is. The open
// label is the only thing on this row that ever changes appearance, and
// deciding that here keeps the highlight beside the placement the dropdown
// under it is padded to.
func renderMenuBarRow(menus []ResolvedMenu, width int, openIdx int) string {
	boxes := menuBarLayout(menus, width)
	open := chrome.ActiveTabStyle(menuBarLabelPadding, false)
	closed := chrome.InactiveTabStyle(menuBarLabelPadding, false)

	var b strings.Builder
	col := 0
	for i, box := range boxes {
		b.WriteString(menuBarGap(box.StartCol - col))
		if i == openIdx {
			b.WriteString(open.Render(box.Title))
		} else {
			b.WriteString(closed.Render(box.Title))
		}
		col = box.StartCol + box.Width
	}
	// The row is exactly m.width like every other Outliner line, so the bar reads
	// as a band rather than as a ragged string of words.
	b.WriteString(menuBarGap(width - col))
	return b.String()
}

// renderOutlinerMenuBar is the one line the Outliner writes above its title when
// the bar is shown.
func (m *OutlinerModel) renderOutlinerMenuBar(width int) string {
	menus := m.resolvedMenuBar()
	m.modalFrame.menuLabels = menuBarLayout(menus, width)
	return renderMenuBarRow(menus, width, m.openMenuBarIndex())
}

// The dropdown is a popup composited over the view that opened it, anchored to
// the column the layout gave the open label. This file returns the block and the
// column alone; the compositing, the dimming, and the rows the block may claim
// are the render's, so a menu and every other popup in the app agree about
// geometry by calling one seam rather than by resembling each other.

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
	if m == nil || !m.MenuBar.Open {
		return -1
	}
	return m.MenuBar.MenuIndex
}

// renderOutlinerMenuDropdown draws the open menu and the column it hangs at, or
// the empty string when none is open. It re-resolves the stored item index
// against the entries this frame actually holds and writes the resolved index
// back, which is what stops the highlight a user sees and the entry Enter runs
// from disagreeing after a row left the menu underneath them.
//
// maxBoxRows is the compositor's, not this function's: the block is sized to
// what the host frame leaves under its header rather than to a budget measured
// twice.
func (m *OutlinerModel) renderOutlinerMenuDropdown(width, maxBoxRows int) (string, int) {
	menus, ok := m.OpenMenuBarMenus()
	if !ok || width <= 0 {
		return "", 0
	}
	entries := menus[m.MenuBar.MenuIndex].Entries
	cursor := resolveMenuBarItem(entries, m.MenuBar.ItemIndex)
	if cursor < 0 {
		return "", 0
	}
	m.MenuBar.selectItem(cursor)

	innerWidth := menuBarDropdownWidth(entries, width)
	// The box's own rows come off the budget before the list is sized.
	maxBoxRows = max(maxBoxRows, menuBarDropdownMinRows+menuBarDropdownListRows)
	reason := ""
	if entries[cursor].DisabledReason != "" {
		reason = strings.Join(textfit.WrapText(entries[cursor].DisabledReason, innerWidth-4), "\n")
	}
	reasonRows := 0
	if reason != "" {
		reasonRows = lipgloss.Height(reason) + 1
	}
	listHeight := max(min(len(entries), maxBoxRows-menuBarDropdownListRows-reasonRows), 1)

	surface, err := newMenuBarDropdownSurface(entries, innerWidth-4)
	if err != nil {
		// The surface refuses only a malformed item set, which is this function's
		// own construction rather than anything a user did. Drawing no dropdown is
		// the honest failure: the bar above still says which menu is open.
		return "", 0
	}
	surface.SetCursorToID(menuBarDropdownItemID(cursor))
	content := listsurface.RenderCompactListSurface(surface, innerWidth-4, listHeight)
	m.modalFrame.dropdownSurface = surface
	if reason != "" {
		content += "\n" + strings.Repeat("─", innerWidth-4) + "\n" + theme.MutedStyle.Render(reason)
	}
	box := chrome.RenderOverlayBox(content, innerWidth, "", "",
		lipgloss.NewStyle().Foreground(theme.ColorPrimary))
	return box, menuBarDropdownStartCol(menus, width, m.MenuBar.MenuIndex, box)
}

// menuBarDropdownStartCol is the column the block is composited at: the open
// label's own StartCol. The compositor clamps the right edge itself, so this
// reports the anchor and nothing about the frame.
//
// A menu the drop ladder shed at a narrow width has no drawn label to align to,
// so its dropdown starts at the left margin instead of under a column nothing is
// painted at.
func menuBarDropdownStartCol(menus []ResolvedMenu, width, openIdx int, box string) int {
	startCol := 0
	if boxes := menuBarLayout(menus, width); openIdx < len(boxes) {
		startCol = boxes[openIdx].StartCol
	}
	return min(max(startCol, 0), max(width-chrome.PopupWidth(box), 0))
}

// menuBarDropdownWidth sizes the frame to its longest entry rather than to a
// fraction of the screen. A dropdown is anchored under one label and reads as
// belonging to it, so a box that claimed three fifths of the terminal would
// read as a modal that happens to be left-aligned.
func menuBarDropdownWidth(entries []ResolvedMenuEntry, width int) int {
	marker := menuBarDropdownMarkerWidth(entries)
	longest := 0
	for _, entry := range entries {
		if entry.OperationID == "" && entry.Label == "" {
			continue
		}
		// The gap is LayoutListText's own, charged here because the labels are
		// padded out to it: the widest row is the column every key hint is then
		// flushed to, so a menu wide enough for that row truncates nothing.
		row := marker + lipgloss.Width(entry.Label)
		if entry.Key != "" {
			row += lipgloss.Width(listsurface.ListTextGap) + lipgloss.Width(entry.Key)
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
func menuBarDropdownMarkerWidth(entries []ResolvedMenuEntry) int {
	if menuBarDropdownChecks(entries) {
		return 2 + menuBarDropdownCheckColumn
	}
	return 2
}

// menuBarDropdownChecks reports whether this menu has any state to read back.
// One entry carrying a checkbox or a radio is enough: the gutter is a column of
// the menu, not a property of a row.
func menuBarDropdownChecks(entries []ResolvedMenuEntry) bool {
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
func newMenuBarDropdownSurface(entries []ResolvedMenuEntry, contentWidth int) (*listsurface.ListSurfaceModel, error) {
	checked := menuBarDropdownChecks(entries)
	// A rule starts where the labels start and ends where they end, so it wears
	// the same marker they do and is measured against what the marker leaves.
	blank := ""
	if checked {
		blank = strings.Repeat(" ", menuBarDropdownCheckColumn)
	}
	textWidth := max(contentWidth-menuBarDropdownMarkerWidth(entries), 1)
	items := make([]listsurface.ListItem, 0, len(entries))
	for index, entry := range entries {
		item := listsurface.ListItem{ID: menuBarDropdownItemID(index), PrimaryMarker: blank}
		if entry.OperationID == "" {
			if entry.Label != "" {
				item.PrimaryText = theme.MutedStyle.Render(entry.Label)
			} else {
				item.PrimaryText = strings.Repeat("─", textWidth)
			}
			items = append(items, item)
			continue
		}
		item.PrimaryText = padMenuBarDropdownLabel(entry.Label, entry.Key, textWidth)
		item.SecondaryText = entry.Key
		if entry.DisabledReason != "" {
			item.PrimaryText = theme.MutedStyle.Render(item.PrimaryText)
			item.SecondaryText = theme.MutedStyle.Render(item.SecondaryText)
		}
		if entry.Checked != nil && *entry.Checked {
			item.PrimaryMarker = "✓ "
		}
		items = append(items, item)
	}
	return listsurface.NewListSurfaceModel(items, nil, listsurface.ListSurfaceConfig{SelectionMode: listsurface.ListSelectionSingle})
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
	pad := textWidth - lipgloss.Width(label) - lipgloss.Width(listsurface.ListTextGap) - lipgloss.Width(key)
	if pad <= 0 {
		return label
	}
	return label + strings.Repeat(" ", pad)
}

// menuBarDropdownItemID names one row by its position in the resolution, which
// is the only identity a menu entry has: two rows can name the same operation
// with different wording, and a separator names none.
func menuBarDropdownItemID(index int) listsurface.ListItemID {
	return listsurface.ListItemID(menuBarDropdownItemPrefix + strconv.Itoa(index))
}

const menuBarDropdownItemPrefix = "menu-entry-"

// menuBarDropdownItemIndex reads menuBarDropdownItemID back, for a click that
// names the row the render drew.
func menuBarDropdownItemIndex(id listsurface.ListItemID) (int, bool) {
	raw, ok := strings.CutPrefix(string(id), menuBarDropdownItemPrefix)
	if !ok {
		return 0, false
	}
	index, err := strconv.Atoi(raw)
	return index, err == nil && index >= 0
}
