package keymap

// FoldDeps holds action closures for the "f" fold prefix group.
type FoldDeps struct {
	// ToggleFold toggles the fold state of the current node.
	ToggleFold Action
	// ToggleSectionFold toggles the fold state of the current section.
	ToggleSectionFold Action
	// ToggleAllSectionFolds toggles the fold state of all visible sections.
	ToggleAllSectionFolds Action
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
		Bind("v", "Focus on subtree", cat, deps.FocusSubtree),
		Bind("p", "Toggle inline preview", cat, deps.ToggleInlinePreview),
		Bind("1", "Collapse to depth 1", cat, deps.CollapseToDepth1),
		Bind("2", "Collapse to depth 2", cat, deps.CollapseToDepth2),
		Bind("3", "Collapse to depth 3", cat, deps.CollapseToDepth3),
	}
	if deps.ToggleSectionFold != nil {
		children = append(children, Bind("s", "Toggle section fold", cat, deps.ToggleSectionFold))
	}
	if deps.ToggleAllSectionFolds != nil {
		children = append(children, Bind("S", "Toggle all section folds", cat, deps.ToggleAllSectionFolds))
	}
	return []Entry{Prefix("f", "Folds", cat, children)}
}
