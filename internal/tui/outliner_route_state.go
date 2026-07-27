package tui

import (
	"github.com/sglavoie/tt/internal/config"
)

func outlinerRouteStatesEqual(left, right config.OutlinerRouteState) bool {
	left = config.NormalizeOutlinerRouteState(left)
	right = config.NormalizeOutlinerRouteState(right)
	return left.Mode == right.Mode &&
		left.FilterStates.Equal(right.FilterStates) &&
		left.FilterSnoozed == right.FilterSnoozed &&
		left.FilterPinned == right.FilterPinned
}

// persistActiveRouteState stores the current DTO in the slot selected by the
// live scope. It performs no write when the normalized state is unchanged.
func (m *OutlinerModel) persistActiveRouteState() error {
	if m == nil || m.runtime == nil {
		return nil
	}
	route := m.captureOutlinerRouteState()
	projectID := m.scopedProjectID
	if m.project != nil {
		projectID = m.project.ID
	}
	machine := m.runtime.Machine()
	if projectID == "" {
		if outlinerRouteStatesEqual(machine.CurrentRoute, route) {
			return nil
		}
	} else if existing, ok := machine.ProjectRoutes[projectID]; ok && outlinerRouteStatesEqual(existing, route) {
		return nil
	}

	return commitMachineToRuntime(m.runtime, func(machine *config.MachineState) {
		if projectID == "" {
			machine.CurrentRoute = route
			return
		}
		if machine.ProjectRoutes == nil {
			machine.ProjectRoutes = make(map[string]config.OutlinerRouteState)
		}
		machine.ProjectRoutes[projectID] = route
	})
}

func (m *OutlinerModel) projectRouteState(projectID string) config.OutlinerRouteState {
	if m != nil && m.runtime != nil {
		if route, ok := m.runtime.Machine().ProjectRoutes[projectID]; ok {
			return config.NormalizeOutlinerRouteState(route)
		}
	}
	return config.NormalizeOutlinerRouteState(config.OutlinerRouteState{})
}

// captureOutlinerRouteState projects the live renderer adapters onto the
// canonical persisted DTO. Scope and project identity are intentionally absent:
// NavigationState and the caller-selected CurrentRoute/ProjectRoutes slot own
// those identities.
func (m *OutlinerModel) captureOutlinerRouteState() config.OutlinerRouteState {
	if m == nil {
		return config.NormalizeOutlinerRouteState(config.OutlinerRouteState{})
	}
	route := config.OutlinerRouteState{
		Mode:          string(m.activeOutlinerMode()),
		FilterStates:  cloneCommonWorkflowStateSet(m.filter.States),
		FilterSnoozed: m.filter.Snoozed,
		FilterPinned:  m.filter.Pinned,
	}

	return config.NormalizeOutlinerRouteState(route)
}

// applyOutlinerRouteState restores a canonical persisted DTO through the
// ten-mode selection seam. Canonical lifecycle and snooze predicates stay in
// CommonFilterState and compose directly with the state-native primary rows;
// no legacy presentation filter is needed to reproduce their visible behavior.
func (m *OutlinerModel) applyOutlinerRouteState(route config.OutlinerRouteState) {
	if m == nil {
		return
	}
	route = config.NormalizeOutlinerRouteState(route)
	m.filter.setStateFilter(route.FilterStates, !route.FilterStates.IsEmpty())
	m.filter.Snoozed = route.FilterSnoozed
	m.filter.Pinned = route.FilterPinned

	m.selectOutlinerMode(OutlinerModeID(route.Mode))
	m.crossForceRebuild = true
	m.rebuildRows()
}
