package keymap

// FoldDeps holds action closures for the "z" fold prefix group.
type FoldDeps struct {
	// ToggleFold toggles the fold state of the current node.
	ToggleFold Action
	// ExpandFold expands the current fold.
	ExpandFold Action
	// CollapseFold collapses the current fold.
	CollapseFold Action
	// ExpandAll expands all folds in the view.
	ExpandAll Action
	// CollapseAll collapses all folds in the view.
	CollapseAll Action
	// FocusSubtree enters a subtree-focused view.
	FocusSubtree Action
	// ToggleInlinePreview toggles the inline task preview.
	ToggleInlinePreview Action
}

// FoldGroup returns entries for the "z" prefix: fold / collapse operations.
func FoldGroup(deps FoldDeps) []Entry {
	cat := "Folds"
	children := []Entry{
		Bind("a", "Toggle fold", cat, deps.ToggleFold),
		Bind("o", "Expand fold", cat, deps.ExpandFold),
		Bind("c", "Collapse fold", cat, deps.CollapseFold),
		Bind("R", "Expand all", cat, deps.ExpandAll),
		Bind("M", "Collapse all", cat, deps.CollapseAll),
		Bind("v", "Focus on subtree", cat, deps.FocusSubtree),
		Bind("p", "Toggle inline preview", cat, deps.ToggleInlinePreview),
	}
	return []Entry{Prefix("z", "Folds", cat, children)}
}
