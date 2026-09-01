package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
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
	if themeBgColor != "" {
		s = s.Background(lipgloss.Color(themeBgColor))
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
func menuBarLayout(menus []resolvedMenu, width int) []menuLabelBox {
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
func renderMenuBarRow(menus []resolvedMenu, width int, openIdx int) string {
	boxes := menuBarLayout(menus, width)
	open := activeTabStyle(menuBarLabelPadding, false)
	closed := inactiveTabStyle(menuBarLabelPadding, false)

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

// menuBarVisible reads the machine-local flag `M` writes. RuntimeState.Machine
// is nil-safe and returns defaults, so an Outliner built without a runtime — the
// dimmed preview backgrounds among them — simply draws no bar.
func (m *OutlinerModel) menuBarVisible() bool {
	return m.runtime.Machine().MenuBarVisible
}

// menuBarRows is what the bar costs the row budget. It is deliberately a
// constant one row rather than a measurement of the drawn bar: contentHeight is
// called from key handlers that must not build a menu resolution, and a bar that
// resolved to no menus at all still owes its row or the reservation and the
// paint would disagree.
func (m *OutlinerModel) menuBarRows() int {
	if m.menuBarVisible() {
		return 1
	}
	return 0
}

// resolvedMenuBar projects the specs against the live catalog for this render.
// Nothing is cached: a menu whose every entry fell away is omitted, so which
// labels the bar draws follows the cursor the same way the entries do.
func (m *OutlinerModel) resolvedMenuBar() []resolvedMenu {
	if m.operationRegistry == nil {
		return nil
	}
	strategy, ok := m.operationRegistry.InputStrategy(InputStrategySimplified)
	if !ok {
		return nil
	}
	menus, err := resolveMenuEntries(
		m.operationRegistry,
		strategy,
		m.operationCommandContext(),
		outlinerMenuSpecs(m.primaryMode, m.menuBarVisible()),
	)
	if err != nil {
		// Startup already refused a spec naming an operation the registry does not
		// hold, so there is nothing here a render could report that validation did
		// not. Drawing an empty band beats painting an error into the chrome.
		return nil
	}
	return menus
}

// renderOutlinerMenuBar is the one line the Outliner writes above its title when
// the bar is shown.
func (m *OutlinerModel) renderOutlinerMenuBar(width int) string {
	return renderMenuBarRow(m.resolvedMenuBar(), width, m.openMenuBarIndex())
}
