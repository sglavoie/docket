package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// tagFilterApplicable is the minimal interface for views that can receive a tag
// filter selection (used by applyTagFilterToView to avoid per-view type switches).
type tagFilterApplicable interface {
	getFilter() *CommonFilterState
	refreshAndClamp()
}

// commonFilterView abstracts the per-view filter wiring needed by the shared
// filter glue helpers. Views that embed CommonFilterState and support the
// standard F-prefix filter system implement this to share the dispatch logic.
type commonFilterView interface {
	getFilter() *CommonFilterState
	getFilterMachine() *config.MachineState
	setFilterPendingPrefix(string)
	refreshAndClamp()
	buildTagCmd() tea.Cmd
}

// handleFilterPrefixForView handles a first-level F-prefix key by trying
// handleCommonFilterPrefix. Returns (cmd, handled). The caller should check
// view-specific keys before calling this.
func handleFilterPrefixForView(v commonFilterView, key string) (tea.Cmd, bool) {
	result := handleCommonFilterPrefix(key, v.getFilter(), v.buildTagCmd)
	if !result.Handled {
		return nil, false
	}
	cmd := applyCommonFilterResult(result, v.setFilterPendingPrefix, v.refreshAndClamp)
	return cmd, true
}

// handleFilterSubPrefixForView handles all filter sub-prefix keys (Ft, Fs, Fd,
// Fa, Fas-Fap, FS) for views using CommonFilterState.
func handleFilterSubPrefixForView(v commonFilterView, prefix, key string) {
	if dispatchFilterSubPrefix(prefix, key, v.getFilter(), v.getFilterMachine(), v.setFilterPendingPrefix) {
		v.refreshAndClamp()
	}
}
