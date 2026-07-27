package tui

import "github.com/sglavoie/tt/internal/config"

// OutlinerOpenOptions applies canonical mode, sort, or persisted route state
// without changing the typed TaskWorkspace route that owns scope.
type OutlinerOpenOptions struct {
	Sort    SortField
	SetSort bool
	Mode    OutlinerModeID
	SetMode bool

	RouteState    config.OutlinerRouteState
	SetRouteState bool
}

func (m *OutlinerModel) ApplyOpenOptions(opts OutlinerOpenOptions) {
	if opts.SetRouteState {
		m.applyOutlinerRouteState(opts.RouteState)
		return
	}

	mode := m.activeOutlinerModeEntry()
	if opts.SetMode {
		if selected, ok := OutlinerModeByID(opts.Mode); ok {
			mode = selected
			m.selectOutlinerMode(selected.ID)
		}
	}
	if opts.SetSort && mode.SupportsManualSort() {
		sortState := SortState{Field: opts.Sort, Ascending: true}
		m.sort = sortState
		m.treeSort = sortState
		m.rebuildRows()
	}
}
