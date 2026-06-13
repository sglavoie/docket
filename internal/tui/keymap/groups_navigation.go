package keymap

// ListNavigationDeps holds action closures for standard list navigation.
type ListNavigationDeps struct {
	// CursorDown moves the cursor one item down.
	CursorDown Action
	// CursorUp moves the cursor one item up.
	CursorUp Action
	// JumpToBottom moves the cursor to the last item.
	JumpToBottom Action
	// PageUp scrolls up by a page.
	PageUp Action
	// PageDown scrolls down by a page.
	PageDown Action
}

// ListNavigationGroup returns top-level navigation bindings: j, k, down, up, G, ctrl+u, ctrl+d.
func ListNavigationGroup(deps ListNavigationDeps) []Entry {
	cat := "Navigation"
	return []Entry{
		Bind("j", "Cursor down", cat, deps.CursorDown),
		Bind("down", "Cursor down", cat, deps.CursorDown, AsHidden()),
		Bind("k", "Cursor up", cat, deps.CursorUp),
		Bind("up", "Cursor up", cat, deps.CursorUp, AsHidden()),
		Bind("G", "Jump to bottom", cat, deps.JumpToBottom),
		Bind("ctrl+u", "Page up", cat, deps.PageUp),
		Bind("ctrl+d", "Page down", cat, deps.PageDown),
	}
}
