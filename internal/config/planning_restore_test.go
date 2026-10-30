package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const planningRestoreWorkspace = "00000000-0000-4000-8000-000000000001"

func planningRestoreState(today, next string) *PlanningState {
	refs := func(names string) []ProjectTaskLocator {
		var result []ProjectTaskLocator
		for _, name := range strings.Fields(names) {
			project, task := "project", name
			if parts := strings.SplitN(name, "/", 2); len(parts) == 2 {
				project, task = parts[0], parts[1]
			}
			result = append(result, ProjectTaskLocator{ProjectID: project, TaskID: task})
		}
		return result
	}
	return &PlanningState{Today: refs(today), Next: refs(next)}
}

func planningRestoreBytes(t *testing.T, today, next string) []byte {
	t.Helper()
	data, err := encodeDomainEnvelope(domainPlanning, planningRestoreWorkspace, planningRestoreState(today, next))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPlanningRestorePreservesUnrelatedMembershipAndOrder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		expected [2]string
		desired  [2]string
		current  [2]string
		want     [2]string
	}{
		{"undo_today", [2]string{"a", ""}, [2]string{}, [2]string{"a", "b"}, [2]string{"", "b"}},
		{"redo_today", [2]string{}, [2]string{"a", ""}, [2]string{"c", "b"}, [2]string{"c a", "b"}},
		{"move_between_horizons", [2]string{"a c", "d"}, [2]string{"c", "a d"}, [2]string{"b a c", "e d"}, [2]string{"b c", "e a d"}},
		{"multiple_members", [2]string{"a c", ""}, [2]string{"x a y c z", ""}, [2]string{"a b c d", "e"}, [2]string{"x a b y c d z", "e"}},
		{"missing_anchor", [2]string{"a b c", ""}, [2]string{"a x b c", ""}, [2]string{"a d c", ""}, [2]string{"a d x c", ""}},
		{"unrelated_reorder", [2]string{"a b c", ""}, [2]string{"b c", ""}, [2]string{"a c b d", "e"}, [2]string{"c b d", "e"}},
		{"unrelated_removal_and_move", [2]string{"a b c", "d"}, [2]string{"b c", "d"}, [2]string{"a c", "b"}, [2]string{"c", "b"}},
		{"project_qualified", [2]string{"one/a", ""}, [2]string{}, [2]string{"one/a two/a", "three/a"}, [2]string{"two/a", "three/a"}},
		{"unchanged_guard_image", [2]string{"a", "b"}, [2]string{"a", "b"}, [2]string{"c a", "b d"}, [2]string{"c a", "b d"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := planningRestoreBytes(t, tc.expected[0], tc.expected[1])
			desired := planningRestoreBytes(t, tc.desired[0], tc.desired[1])
			current := planningRestoreBytes(t, tc.current[0], tc.current[1])
			original := bytes.Clone(current)
			got, ok := RebasePlanningRestore(expected, desired, current)
			want := planningRestoreBytes(t, tc.want[0], tc.want[1])
			if !ok || !bytes.Equal(got, want) {
				t.Fatalf("restore ok=%v\ngot  %s\nwant %s", ok, got, want)
			}
			if !bytes.Equal(current, original) {
				t.Fatal("restore modified its input")
			}
		})
	}
}

func TestPlanningRestoreRefusesOverlappingChanges(t *testing.T) {
	for _, tc := range []struct {
		name     string
		expected [2]string
		desired  [2]string
		current  [2]string
	}{
		{"target_moved_to_next", [2]string{"a", ""}, [2]string{}, [2]string{"b", "a"}},
		{"target_removed", [2]string{"a", ""}, [2]string{}, [2]string{"b", ""}},
		{"target_added_while_undone", [2]string{}, [2]string{"a", ""}, [2]string{"b", "a"}},
		{"target_reordered", [2]string{"a b c", ""}, [2]string{"b c", ""}, [2]string{"b a c", ""}},
		{"one_bulk_member_changed", [2]string{"a b", ""}, [2]string{}, [2]string{"a c", "b"}},
		{"ordering_intent", [2]string{"a b c", ""}, [2]string{"b a c", ""}, [2]string{"a b c d", ""}},
		{"mixed_order_and_membership_intent", [2]string{"a b c", ""}, [2]string{"c b", ""}, [2]string{"a b c d", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := RebasePlanningRestore(
				planningRestoreBytes(t, tc.expected[0], tc.expected[1]),
				planningRestoreBytes(t, tc.desired[0], tc.desired[1]),
				planningRestoreBytes(t, tc.current[0], tc.current[1]),
			); ok || got != nil {
				t.Fatalf("overlapping restore accepted: %s", got)
			}
		})
	}
}

func TestPlanningRestoreRefusesInvalidImages(t *testing.T) {
	for name, corrupt := range map[string]func([]byte) []byte{
		"corrupt_json": func(_ []byte) []byte { return []byte("{") },
		"future_schema": func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"schema_version": 1`), []byte(`"schema_version": 2`), 1)
		},
		"wrong_kind": func(data []byte) []byte { return bytes.ReplaceAll(data, []byte(`"planning"`), []byte(`"workspace"`)) },
		"wrong_workspace": func(data []byte) []byte {
			return bytes.ReplaceAll(data, []byte(planningRestoreWorkspace), []byte("00000000-0000-4000-8000-000000000002"))
		},
		"invalid_workspace": func(data []byte) []byte {
			return bytes.ReplaceAll(data, []byte(planningRestoreWorkspace), []byte("invalid"))
		},
		"unknown_envelope_field": func(data []byte) []byte { return bytes.Replace(data, []byte("{"), []byte(`{"future": true,`), 1) },
		"unknown_payload_field": func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"data": {`), []byte(`"data": {"future": true,`), 1)
		},
		"null_payload": func(data []byte) []byte { return mutatePlanningRestorePayload(t, data, nil) },
		"incomplete_identity": func(data []byte) []byte {
			return mutatePlanningRestorePayload(t, data, map[string]any{"today": []map[string]string{{"task_id": "a"}}})
		},
		"duplicate_identity": func(data []byte) []byte {
			return mutatePlanningRestorePayload(t, data, planningRestoreState("a a", ""))
		},
		"both_horizons": func(data []byte) []byte { return mutatePlanningRestorePayload(t, data, planningRestoreState("a", "a")) },
	} {
		for position := range 3 {
			t.Run(name+[]string{"/expected", "/desired", "/current"}[position], func(t *testing.T) {
				images := [3][]byte{planningRestoreBytes(t, "a", ""), planningRestoreBytes(t, "", ""), planningRestoreBytes(t, "a", "b")}
				images[position] = corrupt(images[position])
				if got, ok := RebasePlanningRestore(images[0], images[1], images[2]); ok || got != nil {
					t.Fatalf("invalid image accepted: %s", got)
				}
			})
		}
	}
}

func mutatePlanningRestorePayload(t *testing.T, data []byte, payload any) []byte {
	t.Helper()
	var env domainEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	var err error
	env.Data, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
