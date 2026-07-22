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
	if !jsonObjectHasKey(data, "tui_state_version") {
		m.TUIStateVersion = 0
	}
	return nil
}

func jsonObjectHasKey(data []byte, key string) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(data, &object) == nil && object[key] != nil
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

	changed := p.MigrateLegacyFilters()
	if p.TUIStateVersion != CurrentTUIStateVersion {
		p.TUIStateVersion = CurrentTUIStateVersion
		changed = true
	}
	return changed, nil
}

// MigrateTUIState advances the portable settings marker. Retired settings are
// ignored while decoding and disappear on the next write.
func (s *Settings) MigrateTUIState() (bool, error) {
	if s == nil {
		return false, nil
	}
	if s.TUIStateVersion > CurrentTUIStateVersion {
		return false, fmt.Errorf("settings: TUI state version %d not supported, please upgrade tt", s.TUIStateVersion)
	}
	changed := false
	if s.TUIStateVersion != CurrentTUIStateVersion {
		s.TUIStateVersion = CurrentTUIStateVersion
		changed = true
	}
	return changed, nil
}

// MigrateTUIState makes the six-destination startup route durable and drops
// machine-only state owned by retired presentations and command histories.
func (m *MachineState) MigrateTUIState() (bool, error) {
	if m == nil {
		return false, nil
	}
	if m.TUIStateVersion > CurrentTUIStateVersion {
		return false, fmt.Errorf("machine: TUI state version %d not supported, please upgrade tt", m.TUIStateVersion)
	}

	before := *m
	destination, route := migrateStartRoute(m.StartView, m.StartRoute)
	m.StartView = destination
	m.StartRoute = route

	if len(m.RecentPaletteItems) > MaxRecentPaletteItems {
		m.RecentPaletteItems = append([]RecentPaletteItem(nil), m.RecentPaletteItems[:MaxRecentPaletteItems]...)
	}
	m.TUIStateVersion = CurrentTUIStateVersion
	return !reflect.DeepEqual(before, *m), nil
}

// SetStartView stores a canonical destination. Changing destinations resets
// route details so a migrated Backlog/Due Dates preference does not cling to a
// later explicit Settings choice.
func (m *MachineState) SetStartView(destination string) {
	canonical, route := migrateStartRoute(destination, StartRouteState{})
	m.StartView = canonical
	m.StartRoute = route
}

// GetStartRoute returns normalized route semantics for the effective start
// destination without mutating the snapshot.
func (m *MachineState) GetStartRoute() StartRouteState {
	_, route := migrateStartRoute(m.GetStartView(), m.StartRoute)
	return route
}

func migrateStartRoute(value string, existing StartRouteState) (string, StartRouteState) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dashboard", StartViewHome:
		return StartViewHome, StartRouteState{}
	case "focus-board":
		return StartViewPlanner, StartRouteState{PlannerTab: StartPlannerToday}
	case "backlog":
		return StartViewPlanner, StartRouteState{PlannerTab: StartPlannerBacklog}
	case "cross-project-outliner":
		return StartViewTasks, StartRouteState{TaskFocus: StartTaskActive}
	case "all-tasks":
		return StartViewTasks, StartRouteState{TaskFocus: StartTaskAll}
	case "priority":
		return StartViewTasks, StartRouteState{
			TaskFocus: StartTaskHighPriority, TaskSort: StartTaskSortPriority,
		}
	case "due-dates":
		return StartViewTasks, StartRouteState{
			TaskFocus: StartTaskAll, TaskSort: StartTaskSortDueDate,
		}
	case "daily-review":
		return StartViewReview, StartRouteState{ReviewModule: StartReviewDaily}
	case StartViewPlanner:
		return StartViewPlanner, normalizeStartRoute(StartViewPlanner, existing)
	case StartViewTasks:
		return StartViewTasks, normalizeStartRoute(StartViewTasks, existing)
	case StartViewProject:
		return StartViewProject, normalizeStartRoute(StartViewProject, existing)
	case StartViewReview:
		return StartViewReview, normalizeStartRoute(StartViewReview, existing)
	case StartViewManage:
		return StartViewManage, StartRouteState{}
	default:
		return DefaultStartView, normalizeStartRoute(DefaultStartView, StartRouteState{})
	}
}

func normalizeStartRoute(destination string, route StartRouteState) StartRouteState {
	switch destination {
	case StartViewPlanner:
		if route.PlannerTab != StartPlannerBacklog {
			route.PlannerTab = StartPlannerToday
		}
		return StartRouteState{PlannerTab: route.PlannerTab}
	case StartViewTasks, StartViewProject:
		focus := route.TaskFocus
		if route.TaskLayout == StartTaskList && focus == StartTaskActive {
			focus = StartTaskAll
		}
		if focus != StartTaskAll && focus != StartTaskHighPriority {
			focus = StartTaskActive
		}
		sort := route.TaskSort
		if route.TaskGroup == StartTaskGroupDueDate {
			sort = StartTaskSortDueDate
		}
		if sort != StartTaskSortPriority && sort != StartTaskSortDueDate {
			sort = ""
		}
		return StartRouteState{
			TaskFocus: focus,
			TaskSort:  sort,
		}
	case StartViewReview:
		if route.ReviewModule == StartReviewDaily {
			return StartRouteState{ReviewModule: StartReviewDaily}
		}
	}
	return StartRouteState{}
}
