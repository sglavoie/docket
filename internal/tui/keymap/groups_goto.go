package keymap

// GotoDeps holds action closures for the "g" goto prefix group.
type GotoDeps struct {
	// JumpToTop scrolls to the first item.
	JumpToTop Action
	// GoToProject navigates to the current task's project. Nil if not supported.
	GoToProject Action
	// Extra holds view-specific "g" children appended after the common ones.
	Extra []Entry
}

// GotoGroup returns entries for the "g" prefix: navigation shortcuts.
// Common children: gg (top), gp (go to project).
// Views append view-specific children via deps.Extra.
func GotoGroup(deps GotoDeps) []Entry {
	cat := "Navigation"
	children := []Entry{
		Bind("g", "Jump to top", cat, deps.JumpToTop),
	}
	if deps.GoToProject != nil {
		children = append(children, Bind("p", "Go to task in project", cat, deps.GoToProject))
	}
	children = append(children, deps.Extra...)
	return []Entry{Prefix("g", "Go to", cat, children)}
}
