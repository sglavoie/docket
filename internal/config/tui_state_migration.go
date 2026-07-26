package config

import (
	"encoding/json"
	"fmt"
	"reflect"
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
	if err := s.decodeLegacySectionLimits(data); err != nil {
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
	m.TUIStateVersion = CurrentTUIStateVersion
	return !reflect.DeepEqual(before, *m), nil
}
