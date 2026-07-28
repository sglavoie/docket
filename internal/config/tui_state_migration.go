package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// The domain store unmarshals into constructors seeded with current defaults.
// These hooks distinguish an absent persisted TUI marker from an explicit
// current marker so legacy payloads still run their one-time rewrite.
func (p *PlanningState) UnmarshalJSON(data []byte) error {
	type plain PlanningState
	if err := json.Unmarshal(data, (*plain)(p)); err != nil {
		return err
	}
	p.retiredSectionsDecoded = jsonObjectHasRetiredKey(data, PlanningState{})
	if !jsonObjectHasKey(data, "tui_state_version") {
		p.TUIStateVersion = 0
	}
	return nil
}

func (s *Settings) UnmarshalJSON(data []byte) error {
	type plain Settings
	if err := json.Unmarshal(data, (*plain)(s)); err != nil {
		return err
	}
	if err := s.decodeLegacySectionLimits(data); err != nil {
		return err
	}
	if err := s.decodeLegacySectionColors(data); err != nil {
		return err
	}
	if !jsonObjectHasKey(data, "tui_state_version") {
		s.TUIStateVersion = 0
	}
	return nil
}

func (m *MachineState) UnmarshalJSON(data []byte) error {
	type plain MachineState
	if err := json.Unmarshal(data, (*plain)(m)); err != nil {
		return err
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}

	if _, canonical := object["startup_route"]; canonical {
		m.StartupRoute = NormalizeOutlinerRouteState(m.StartupRoute)
	} else {
		route, found, err := decodeLegacyStartupRoute(data)
		if err != nil {
			return err
		}
		if found {
			m.StartupRoute = route
			m.legacyOutlinerRoutesDecoded = true
		} else {
			m.StartupRoute = NormalizeOutlinerRouteState(OutlinerRouteState{})
		}
	}

	if _, canonical := object["current_route"]; canonical {
		m.CurrentRoute = NormalizeOutlinerRouteState(m.CurrentRoute)
	} else {
		route, found, err := decodeLegacyCurrentRoute(data)
		if err != nil {
			return err
		}
		if found {
			m.CurrentRoute = route
			m.legacyOutlinerRoutesDecoded = true
		} else {
			m.CurrentRoute = NormalizeOutlinerRouteState(OutlinerRouteState{})
		}
	}

	if _, canonical := object["project_routes"]; canonical {
		m.ProjectRoutes = normalizeProjectRoutes(m.ProjectRoutes)
	} else {
		routes, found, err := decodeLegacyProjectRoutes(data)
		if err != nil {
			return err
		}
		if found {
			m.ProjectRoutes = routes
			m.legacyOutlinerRoutesDecoded = true
		}
	}

	if !jsonObjectHasKey(data, "tui_state_version") {
		m.TUIStateVersion = 0
	}
	return nil
}

type legacyStartRouteState struct {
	PlannerTab   string `json:"planner_tab,omitempty"`
	TaskLayout   string `json:"task_layout,omitempty"`
	TaskFocus    string `json:"task_focus,omitempty"`
	TaskSort     string `json:"task_sort,omitempty"`
	TaskGroup    string `json:"task_group,omitempty"`
	ReviewModule string `json:"review_module,omitempty"`
}

type legacyOutlinerViewState struct {
	Mode                        string   `json:"mode,omitempty"`
	OutlinerMode                string   `json:"outliner_mode,omitempty"`
	CurrentMode                 string   `json:"current_mode,omitempty"`
	OutlinerActivePerspective   *int     `json:"outliner_active_perspective"`
	FilterStatus                string   `json:"filter_status,omitempty"`
	FilterSections              []string `json:"filter_sections,omitempty"`
	FilterParked                bool     `json:"filter_parked,omitempty"`
	OutlinerFilterStatus        string   `json:"outliner_filter_status,omitempty"`
	OutlinerFilterSections      []string `json:"outliner_filter_sections,omitempty"`
	OutlinerFilterParked        bool     `json:"outliner_filter_parked,omitempty"`
	CrossOutlinerFilterStatus   string   `json:"cross_outliner_filter_status,omitempty"`
	CrossOutlinerFilterSections []string `json:"cross_outliner_filter_sections,omitempty"`
	CrossOutlinerFilterParked   bool     `json:"cross_outliner_filter_parked,omitempty"`
}

func decodeLegacyStartupRoute(data []byte) (OutlinerRouteState, bool, error) {
	var legacy struct {
		StartView  string                `json:"start_view"`
		StartRoute legacyStartRouteState `json:"start_route"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return OutlinerRouteState{}, false, err
	}
	found := jsonObjectHasKey(data, "start_view") || jsonObjectHasKey(data, "start_route")
	if !found {
		return OutlinerRouteState{}, false, nil
	}

	mode := legacy.StartView
	switch normalizeOutlinerModeToken(legacy.StartView) {
	case "focus-board", "planner":
		if normalizeOutlinerModeToken(legacy.StartRoute.PlannerTab) == "backlog" {
			mode = OutlinerModeBacklog
		} else {
			mode = OutlinerModeAll
		}
	case "backlog":
		mode = OutlinerModeBacklog
	case "cross-project-outliner", "all-tasks", "tasks", "project":
		mode = legacy.StartRoute.TaskFocus
		switch {
		case normalizeOutlinerModeToken(legacy.StartRoute.TaskGroup) == "due-date",
			normalizeOutlinerModeToken(legacy.StartRoute.TaskSort) == "due-date":
			mode = OutlinerModeDue
		case normalizeOutlinerModeToken(legacy.StartRoute.TaskSort) == "priority":
			mode = OutlinerModePriority
		case strings.TrimSpace(mode) == "":
			mode = OutlinerModeAll
		}
	case "review":
		if normalizeOutlinerModeToken(legacy.StartRoute.ReviewModule) == "daily" {
			mode = OutlinerModeDailyReview
		}
	case "dashboard", "home", "manage", "":
		mode = OutlinerModeAll
	}
	return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: mode}), true, nil
}

func decodeLegacyCurrentRoute(data []byte) (OutlinerRouteState, bool, error) {
	var legacy legacyOutlinerViewState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return OutlinerRouteState{}, false, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return OutlinerRouteState{}, false, err
	}
	keys := []string{
		"current_mode",
		"outliner_mode",
		"outliner_active_perspective",
		"filter_status",
		"filter_sections",
		"filter_parked",
		"cross_outliner_filter_status",
		"cross_outliner_filter_sections",
		"cross_outliner_filter_parked",
	}
	found := false
	for _, key := range keys {
		if object[key] != nil {
			found = true
			break
		}
	}
	if !found {
		return OutlinerRouteState{}, false, nil
	}

	mode := legacy.CurrentMode
	if strings.TrimSpace(mode) == "" {
		mode = legacy.OutlinerMode
	}
	route := legacyRouteForPerspective(legacy.OutlinerActivePerspective)
	if strings.TrimSpace(mode) != "" {
		route = NormalizeOutlinerRouteState(OutlinerRouteState{Mode: mode})
	}
	status := legacy.CrossOutlinerFilterStatus
	sections := legacy.CrossOutlinerFilterSections
	if status == "" && len(sections) == 0 {
		status = legacy.FilterStatus
		sections = legacy.FilterSections
	}
	route = applyLegacyLifecycleRouteFilter(route, status, sections)
	if legacy.CrossOutlinerFilterParked || legacy.FilterParked {
		route.Mode = OutlinerModeParked
	}
	return NormalizeOutlinerRouteState(route), true, nil
}

func decodeLegacyProjectRoutes(data []byte) (map[string]OutlinerRouteState, bool, error) {
	var legacy struct {
		ProjectViewStates map[string]legacyOutlinerViewState `json:"project_view_states"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, false, err
	}
	if !jsonObjectHasKey(data, "project_view_states") {
		return nil, false, nil
	}

	routes := make(map[string]OutlinerRouteState, len(legacy.ProjectViewStates))
	for projectID, view := range legacy.ProjectViewStates {
		if strings.TrimSpace(projectID) == "" || !legacyViewHasRouteState(view) {
			continue
		}
		route := legacyRouteForPerspective(view.OutlinerActivePerspective)
		mode := view.OutlinerMode
		if strings.TrimSpace(mode) == "" {
			mode = view.Mode
		}
		if strings.TrimSpace(mode) != "" {
			route = NormalizeOutlinerRouteState(OutlinerRouteState{Mode: mode})
		}
		status := view.OutlinerFilterStatus
		sections := view.OutlinerFilterSections
		if status == "" && len(sections) == 0 {
			status = view.FilterStatus
			sections = view.FilterSections
		}
		route = applyLegacyLifecycleRouteFilter(route, status, sections)
		if view.OutlinerFilterParked || view.FilterParked {
			route.Mode = OutlinerModeParked
		}
		routes[projectID] = NormalizeOutlinerRouteState(route)
	}
	if len(routes) == 0 {
		return nil, true, nil
	}
	return routes, true, nil
}

func legacyViewHasRouteState(view legacyOutlinerViewState) bool {
	return view.OutlinerActivePerspective != nil ||
		strings.TrimSpace(view.OutlinerMode) != "" ||
		strings.TrimSpace(view.Mode) != "" ||
		view.OutlinerFilterStatus != "" ||
		len(view.OutlinerFilterSections) > 0 ||
		view.OutlinerFilterParked ||
		view.FilterStatus != "" ||
		len(view.FilterSections) > 0 ||
		view.FilterParked
}

func legacyRouteForPerspective(perspective *int) OutlinerRouteState {
	if perspective == nil {
		return NormalizeOutlinerRouteState(OutlinerRouteState{})
	}
	switch *perspective {
	case 0:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: "snoozed"})
	case 1:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: OutlinerModeAll})
	case 2:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: "active"})
	case 3:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: OutlinerModeNext})
	case 4:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: OutlinerModeInbox})
	case 5:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: "standby"})
	case 6:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: "waiting"})
	case 7:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: "someday"})
	case 8:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: "completed"})
	case 9:
		return NormalizeOutlinerRouteState(OutlinerRouteState{Mode: "review"})
	default:
		return NormalizeOutlinerRouteState(OutlinerRouteState{})
	}
}

func applyLegacyLifecycleRouteFilter(route OutlinerRouteState, status string, sections []string) OutlinerRouteState {
	states, active := legacySnapshotStates(status, sections)
	if !active {
		return route
	}
	if route.FilterStates.IsEmpty() {
		route.FilterStates = states
		return route
	}
	intersection := route.FilterStates.Intersection(states)
	if !intersection.IsEmpty() {
		route.FilterStates = intersection
	}
	return route
}

func normalizeProjectRoutes(routes map[string]OutlinerRouteState) map[string]OutlinerRouteState {
	if len(routes) == 0 {
		return nil
	}
	normalized := make(map[string]OutlinerRouteState, len(routes))
	for projectID, route := range routes {
		projectID = strings.TrimSpace(projectID)
		if projectID == "" {
			continue
		}
		normalized[projectID] = NormalizeOutlinerRouteState(route)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func jsonObjectHasKey(data []byte, key string) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(data, &object) == nil && object[key] != nil
}

// jsonObjectHasRetiredKey reports whether data carries a key the canonical
// struct no longer defines. Callers must reject payloads from a newer build
// first, so an unrecognized key can only be a retired one.
func jsonObjectHasRetiredKey(data []byte, canonical any) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(data, &object) != nil {
		return false
	}
	canonicalType := reflect.TypeOf(canonical)
	known := make(map[string]struct{}, canonicalType.NumField())
	for i := range canonicalType.NumField() {
		name, _, _ := strings.Cut(canonicalType.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			known[name] = struct{}{}
		}
	}
	for key := range object {
		if _, ok := known[key]; !ok {
			return true
		}
	}
	return false
}

// MigrateTUIState rewrites portable planning UI state in place. Query fields
// remain durable; view, sort, pin, tab, and board presentation fields do not.
func (p *PlanningState) MigrateTUIState() (bool, error) {
	if p == nil {
		return false, nil
	}
	if p.TUIStateVersion > CurrentTUIStateVersion {
		return false, fmt.Errorf("planning: TUI state version %d not supported, please upgrade tt", p.TUIStateVersion)
	}

	// Retired planning sections have no canonical destination: filtering is now
	// session-scoped f-chord state, so any section the current schema no longer
	// defines is dropped on read and the domain is rewritten without it. The
	// version guard above already rejected payloads from a newer build.
	changed := p.retiredSectionsDecoded
	p.retiredSectionsDecoded = false
	if p.NormalizePlanningMemberships() {
		changed = true
	}
	if p.TUIStateVersion != CurrentTUIStateVersion {
		p.TUIStateVersion = CurrentTUIStateVersion
		changed = true
	}
	return changed, nil
}

// MigrateTUIState advances the portable settings marker. Retired settings are
// ignored while decoding and disappear on the next write. Legacy section WIP
// values are decoded into canonical state-group limits before this method runs;
// the marker below makes that rewrite durable even for an otherwise-current
// settings payload.
func (s *Settings) MigrateTUIState() (bool, error) {
	if s == nil {
		return false, nil
	}
	if s.TUIStateVersion > CurrentTUIStateVersion {
		return false, fmt.Errorf("settings: TUI state version %d not supported, please upgrade tt", s.TUIStateVersion)
	}
	changed := s.legacySectionLimitsDecoded
	s.legacySectionLimitsDecoded = false
	colorsChanged, err := s.MigrateLegacySectionColors()
	if err != nil {
		return false, err
	}
	changed = colorsChanged || changed
	if s.TUIStateVersion != CurrentTUIStateVersion {
		s.TUIStateVersion = CurrentTUIStateVersion
		changed = true
	}
	return changed, nil
}

// MigrateTUIState advances the machine TUI-state marker as a one-way cleanup.
// The retired startup destination (start view/route), Home layout sizing, and
// sidebar state are dropped when the payload decodes into the current struct and
// are omitted from the next save; only durable navigation, histories, and the
// capped palette recents survive.
func (m *MachineState) MigrateTUIState() (bool, error) {
	if m == nil {
		return false, nil
	}
	if m.TUIStateVersion > CurrentTUIStateVersion {
		return false, fmt.Errorf("machine: TUI state version %d not supported, please upgrade tt", m.TUIStateVersion)
	}

	before := *m
	if len(m.RecentPaletteItems) > MaxRecentPaletteItems {
		m.RecentPaletteItems = append([]RecentPaletteItem(nil), m.RecentPaletteItems[:MaxRecentPaletteItems]...)
	}
	m.SetRecentDestinations(m.RecentDestinations)
	m.StartupRoute = NormalizeOutlinerRouteState(m.StartupRoute)
	m.CurrentRoute = NormalizeOutlinerRouteState(m.CurrentRoute)
	m.ProjectRoutes = normalizeProjectRoutes(m.ProjectRoutes)
	m.legacyOutlinerRoutesDecoded = false
	m.TUIStateVersion = CurrentTUIStateVersion
	return !reflect.DeepEqual(before, *m), nil
}
