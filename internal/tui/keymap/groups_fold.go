package keymap

// FoldDeps holds action closures for the "f" fold prefix group.
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
	// CollapseToDepth1 collapses all nodes deeper than depth 1.
	CollapseToDepth1 Action
	// CollapseToDepth2 collapses all nodes deeper than depth 2.
	CollapseToDepth2 Action
	// CollapseToDepth3 collapses all nodes deeper than depth 3.
	CollapseToDepth3 Action
}

// FoldGroup returns entries for the "f" prefix: fold / collapse operations.
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
		Bind("1", "Collapse to depth 1", cat, deps.CollapseToDepth1),
		Bind("2", "Collapse to depth 2", cat, deps.CollapseToDepth2),
		Bind("3", "Collapse to depth 3", cat, deps.CollapseToDepth3),
	}
	return []Entry{Prefix("f", "Folds", cat, children)}
}
