package tui

import (
	"github.com/sglavoie/tt/internal/config"
)

// persistActiveRouteState is intentionally run-local in the lean machine model.
func (m *OutlinerModel) persistActiveRouteState() error {
	return nil
}

// captureOutlinerRouteState persists only the active mode. Filter facets are
// run-local view refinements and must never survive relaunch through legacy
// machine-route storage.
// applyOutlinerRouteState restores only the canonical mode through the
// mode selection seam. Legacy route filter fields remain migration input,
// but live filter facets belong to the current run and are deliberately ignored.
func (m *OutlinerModel) applyOutlinerRouteState(route config.OutlinerRouteState) {
	if m == nil {
		return
	}
	route = config.NormalizeOutlinerRouteState(route)

	m.selectOutlinerMode(OutlinerModeID(route.Mode))
	m.crossForceRebuild = true
	m.rebuildRows()
}
