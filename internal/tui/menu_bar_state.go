package tui

// menuBarState is the open menu's whole sub-state: the dropdown gate and where
// the highlight sits inside it. It is a named field on OutlinerModel rather
// than three promoted ones because the cluster is genuinely separable — the
// bar's own input and render are the only readers, and every other caller
// reaches it through the one close below.
//
// It holds no menus. The bar and its entries are resolved off the live catalog
// every frame, so the two indices are the only thing that could go stale, and
// both are re-resolved against that resolution before they are read. That is
// why the clamp and the walk take a count or an entry set as an argument rather
// than reaching for a model this type deliberately cannot see: the state is the
// cursor, never the population it points into.
type menuBarState struct {
	// open is the dropdown gate. The bar itself is machine-local visibility that
	// `M` writes and this sub-state never touches — closing a menu must not hide
	// the band it dropped out of.
	open bool
	// menuIndex is which bar label is open and itemIndex where the highlight sits
	// inside it. Both are stored positions into a resolution that may have dropped
	// rows out from under them, never trusted identities.
	menuIndex int
	itemIndex int
}

// openFirstMenu drops the first menu out of the bar. It always opens on the
// leftmost menu rather than on the last one used: the bar is read left to right
// and an open menu that remembered where it was would put the highlight
// somewhere the user did not leave it, one whole session ago.
func (s *menuBarState) openFirstMenu() {
	if s == nil {
		return
	}
	s.open = true
	s.menuIndex = 0
	s.itemIndex = 0
}

// close puts the menu away without asking what it was showing. It leaves both
// indices alone because the next open resets them anyway, and a close that also
// cleared them would make the two writes look like separate facts.
func (s *menuBarState) close() {
	if s == nil {
		return
	}
	s.open = false
}

// clampMenu pulls the stored menu index back inside a resolution that may hold
// fewer menus than it did when the index was written. A menu leaves the bar
// whenever the drop ladder sheds its label or the catalog stops offering it, so
// the index is clamped every time it is read rather than on the frame the menu
// left.
func (s *menuBarState) clampMenu(count int) {
	if s == nil || count <= 0 {
		return
	}
	s.menuIndex = min(max(s.menuIndex, 0), count-1)
}

// cycleMenu moves along the bar and starts the new menu at its top. It wraps,
// unlike the vertical walk: the bar is a ring of five labels a user crosses
// repeatedly, and stopping at Help would make the leftmost menu three presses
// away from the rightmost.
func (s *menuBarState) cycleMenu(delta, count int) {
	if s == nil || count <= 0 {
		return
	}
	s.menuIndex = ((s.menuIndex+delta)%count + count) % count
	s.itemIndex = 0
}

// selectItem writes the highlight back. Both the keymap's walk and the render's
// re-resolution land here, which is what stops the row a user sees and the entry
// Enter runs from ever disagreeing.
func (s *menuBarState) selectItem(index int) {
	if s == nil {
		return
	}
	s.itemIndex = index
}
