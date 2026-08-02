package config

import (
	"encoding/json"
	"testing"
)

func baselineJSONFields(t *testing.T, value any) map[string]json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal baseline fixture: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("decode baseline fixture: %v", err)
	}
	return fields
}

func requireBaselineFields(t *testing.T, fields map[string]json.RawMessage, present, absent []string) {
	t.Helper()
	for _, key := range present {
		if _, ok := fields[key]; !ok {
			t.Errorf("current JSON shape is missing %q", key)
		}
	}
	for _, key := range absent {
		if _, ok := fields[key]; ok {
			t.Errorf("current JSON shape unexpectedly contains future field %q", key)
		}
	}
}

// TestDailyLoopCurrentPlanningJSONShape records the canonical planning
// persistence shape.
func TestDailyLoopCurrentPlanningJSONShape(t *testing.T) {
	t.Run("planning memberships", func(t *testing.T) {
		fields := baselineJSONFields(t, PlanningState{
			Next:  []ProjectTaskLocator{{TaskID: "next-task", ProjectID: "project"}},
			Today: []ProjectTaskLocator{{TaskID: "today-task", ProjectID: "project"}},
		})
		requireBaselineFields(
			t,
			fields,
			[]string{"today", "next"},
			[]string{"focus_queue", "daily_plan", "streak_count", "alert_dismissals", "saved_views", "smart_lists"},
		)
	})

}
