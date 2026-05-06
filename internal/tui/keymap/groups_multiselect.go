package keymap

// MultiSelectDeps holds action closures for multi-select mode bindings.
type MultiSelectDeps struct {
	// ToggleMultiSelect enters or exits multi-select mode.
	ToggleMultiSelect Action
	// ToggleSelection toggles the current item's selection.
	ToggleSelection Action
	// SelectAll selects all visible items.
	SelectAll Action
	// DeselectAll deselects all items.
	DeselectAll Action
}

// MultiSelectGroup returns bindings for multi-select operations: v, space, a, A.
func MultiSelectGroup(deps MultiSelectDeps) []Entry {
	cat := "Multi-select"
	return []Entry{
		Bind("v", "Toggle multi-select mode", cat, deps.ToggleMultiSelect),
		Bind("space", "Toggle selection", cat, deps.ToggleSelection),
		Bind("a", "Select all visible", cat, deps.SelectAll),
		Bind("A", "Deselect all visible", cat, deps.DeselectAll),
	}
}
