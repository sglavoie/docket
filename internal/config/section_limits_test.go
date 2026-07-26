package config

import (
	"encoding/json"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetSectionLimitClampsNegativeToDisabled(t *testing.T) {
	cfg := NewSettings()
	cfg.StateGroupLimits = map[models.WorkflowStateGroup]int{
		models.StateGroupNext: -1,
	}

	if got := cfg.GetSectionLimit("Next"); got != 0 {
		t.Fatalf("expected negative section limit clamped to 0, got %d", got)
	}
}

func TestStateGroupLimitDefaultsCoverAllModeGroups(t *testing.T) {
	settings := NewSettings()
	for _, group := range models.AllModeWorkflowStateGroups() {
		if got := settings.GetStateGroupLimit(group); got < 0 {
			t.Errorf("default for %q = %d", group, got)
		}
	}
	if got := settings.GetStateGroupLimit(models.StateGroupToday); got != 0 {
		t.Fatalf("Today limit = %d, want disabled", got)
	}
}

func TestLegacySectionLimitsMigrateDeterministically(t *testing.T) {
	var settings Settings
	err := json.Unmarshal([]byte(`{
		"tui_state_version": 3,
		"section_limit_active": 7,
		"section_limit_next": 2,
		"section_limit_standby": 4
	}`), &settings)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := settings.MigrateTUIState()
	if err != nil || !changed {
		t.Fatalf("MigrateTUIState changed=%v err=%v", changed, err)
	}
	if got := settings.GetStateGroupLimit(models.StateGroupBacklog); got != 7 {
		t.Errorf("Backlog limit = %d, want 7", got)
	}
	if got := settings.GetStateGroupLimit(models.StateGroupInProgress); got != 7 {
		t.Errorf("In Progress limit = %d, want 7", got)
	}
	if got := settings.GetStateGroupLimit(models.StateGroupNext); got != 2 {
		t.Errorf("Next limit = %d, want 2", got)
	}
	if got := settings.GetStateGroupLimit(models.StateGroupPaused); got != 4 {
		t.Errorf("Paused limit = %d, want 4", got)
	}

	out, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(out, &object); err != nil {
		t.Fatal(err)
	}
	if object["state_group_limits"] == nil {
		t.Fatalf("canonical limits missing from %s", out)
	}
	for _, key := range []string{"section_limit_active", "section_limit_next", "section_limit_standby"} {
		if object[key] != nil {
			t.Errorf("retired key %q emitted in %s", key, out)
		}
	}

	var reloaded Settings
	if err := json.Unmarshal(out, &reloaded); err != nil {
		t.Fatal(err)
	}
	if changed, err := reloaded.MigrateTUIState(); err != nil || changed {
		t.Errorf("canonical limits migrated again: changed=%v err=%v", changed, err)
	}
}

func TestCanonicalStateGroupLimitWinsOverLegacyValue(t *testing.T) {
	var settings Settings
	err := json.Unmarshal([]byte(`{
		"state_group_limits": {"backlog": 9},
		"section_limit_active": 7
	}`), &settings)
	if err != nil {
		t.Fatal(err)
	}
	if got := settings.GetStateGroupLimit(models.StateGroupBacklog); got != 9 {
		t.Errorf("Backlog limit = %d, want canonical 9", got)
	}
	if got := settings.GetStateGroupLimit(models.StateGroupInProgress); got != 7 {
		t.Errorf("In Progress limit = %d, want migrated 7", got)
	}
}
