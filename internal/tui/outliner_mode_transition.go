package tui

// OutlinerModeSwitch carries the cross-mode state that must survive a mode
// change. Project scope, search, common filters, cursor identity,
// and compatible column state are preserved verbatim; only the sort is
// reconciled against the target mode's ordering policy.
//
// The switch keeps two sorts: TreeSort is the user's durable manual sort used by
// tree-order modes, and ActiveSort is the sort applied to the current mode.
// Entering a fixed-order mode deactivates manual sort without destroying
// TreeSort, so returning to a tree-order mode restores it.
type OutlinerModeSwitch struct {
	Mode         OutlinerModeID
	ProjectScope string
	Search       string
	CursorTaskID string
	// Columns is the compatible column state, preserved verbatim across switches.
	Columns []string

	TreeSort         SortState
	ActiveSort       SortState
	ManualSortActive bool
}

// ApplyOutlinerModeTransition returns the state after switching from prev to
// target. Scope, search, cursor identity, and column state are
// carried over unchanged. The sort is reconciled: the current manual sort is
// stashed into TreeSort when leaving a tree-order mode, and restored (or
// deactivated) according to the target mode's ordering policy.
func ApplyOutlinerModeTransition(state OutlinerModeSwitch, prev, target OutlinerMode) OutlinerModeSwitch {
	next := state // preserves ProjectScope, Search, CursorTaskID, Columns

	// Persist the durable tree-mode sort before leaving a tree-order mode so a
	// later tree-order mode can restore it.
	if prev.SupportsManualSort() {
		next.TreeSort = state.ActiveSort
	}

	next.Mode = target.ID
	if target.SupportsManualSort() {
		next.ActiveSort = next.TreeSort
		next.ManualSortActive = next.TreeSort.Field != SortByDefault
	} else {
		// Fixed-order modes own their ordering: manual sort is inactive, but the
		// stored tree-mode sort is left intact.
		next.ActiveSort = NewSortState()
		next.ManualSortActive = false
	}
	return next
}

// applyOutlinerModeTransition is the model-level seam for reconciling the
// active render sort with the durable tree sort. Callers must invoke it before
// mutating the legacy fields consumed by activeOutlinerMode, so prev reflects
// the mode the user is actually leaving.
func (m *OutlinerModel) applyOutlinerModeTransition(target OutlinerMode) {
	prev := m.activeOutlinerModeEntry()
	next := ApplyOutlinerModeTransition(OutlinerModeSwitch{
		Mode:             prev.ID,
		TreeSort:         m.treeSort,
		ActiveSort:       m.sort,
		ManualSortActive: prev.SupportsManualSort() && m.sort.Field != SortByDefault,
	}, prev, target)
	m.sort = next.ActiveSort
	m.treeSort = next.TreeSort
}
