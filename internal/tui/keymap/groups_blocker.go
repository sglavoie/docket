package keymap

// BlockerDeps holds action closures for the "b" blocker prefix group.
type BlockerDeps struct {
	// ViewDependencies opens the dependency viewer.
	ViewDependencies Action
	// ManageDependencies opens the dependency manager.
	ManageDependencies Action
}

// BlockerGroup returns entries for the "b" prefix: dependency / blocker operations.
func BlockerGroup(deps BlockerDeps) []Entry {
	cat := "Blockers (b prefix)"
	children := []Entry{
		Bind("b", "View blockers/dependencies", cat, deps.ViewDependencies),
		Bind("m", "Manage dependencies", cat, deps.ManageDependencies),
	}
	return []Entry{Prefix("b", "Blockers", cat, children)}
}
