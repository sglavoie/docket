package workflow

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCanonicalWorkflowAuthoringMetadata(t *testing.T) {
	fields := ValidTriggerFields()
	for _, want := range []TriggerField{
		FieldState,
		FieldStateChanged,
		FieldParentState,
		FieldAllTasksState,
		FieldMajorityTaskState,
		FieldHasTasksWithState,
	} {
		if !slices.Contains(fields, want) {
			t.Errorf("ValidTriggerFields() missing %q", want)
		}
	}
	for _, field := range fields {
		if field == TriggerField("status") || field == TriggerField("section") {
			t.Errorf("ValidTriggerFields() exposes retired field %q", field)
		}
	}

	actions := ValidActionTypes()
	if !slices.Contains(actions, ActionSetState) {
		t.Error("ValidActionTypes() missing set_state")
	}
	for _, action := range actions {
		if action == ActionType("set_status") || action == ActionType("set_section") {
			t.Errorf("ValidActionTypes() exposes retired action %q", action)
		}
	}
}

func TestCanonicalStateTriggerValidation(t *testing.T) {
	tests := []struct {
		name    string
		trigger RuleTrigger
		wantErr bool
	}{
		{"state equals", RuleTrigger{Field: FieldState, Operation: OpEquals, Value: "ready"}, false},
		{"state in set", RuleTrigger{Field: FieldState, Operation: OpIn, Value: "ready,next"}, false},
		{"parent state not in", RuleTrigger{Field: FieldParentState, Operation: OpNotIn, Value: "done,canceled"}, false},
		{"project aggregate state in", RuleTrigger{Field: FieldHasTasksWithState, Operation: OpIn, Value: "waiting,paused"}, false},
		{"equals rejects set", RuleTrigger{Field: FieldState, Operation: OpEquals, Value: "ready,next"}, true},
		{"set rejects unknown", RuleTrigger{Field: FieldState, Operation: OpIn, Value: "ready,unknown"}, true},
		{"set rejects empty member", RuleTrigger{Field: FieldState, Operation: OpIn, Value: "ready,"}, true},
		{"in rejects non-state field", RuleTrigger{Field: FieldTags, Operation: OpIn, Value: "one,two"}, true},
		{"state rejects contains", RuleTrigger{Field: FieldState, Operation: OpContains, Value: "ready"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.trigger.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStateTransitionPatternValidation(t *testing.T) {
	tests := []struct {
		value   string
		want    string
		wantErr bool
	}{
		{"*->done", "*->done", false},
		{"waiting,paused->ready,next", "waiting,paused->ready,next", false},
		{" ready -> done,canceled ", "ready->done,canceled", false},
		{"ready->*", "ready->*", false},
		{"ready", "", true},
		{"ready->unknown", "", true},
		{"*,ready->done", "", true},
		{"->done", "", true},
		{"ready->done->canceled", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			pattern, err := ParseStateTransitionPattern(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseStateTransitionPattern() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && pattern.String() != tt.want {
				t.Errorf("pattern.String() = %q, want %q", pattern.String(), tt.want)
			}
		})
	}
}

func TestCanonicalStateActionAndScopeValidation(t *testing.T) {
	if err := (RuleAction{ActionType: ActionSetState, Value: "in_progress"}).Validate(); err != nil {
		t.Fatalf("canonical set_state rejected: %v", err)
	}
	for _, value := range []string{"", "todo", "ready,next"} {
		if err := (RuleAction{ActionType: ActionSetState, Value: value}).Validate(); err == nil {
			t.Errorf("set_state value %q should be rejected", value)
		}
	}

	rule := WorkflowRule{
		Name:     "Scoped",
		Enabled:  true,
		Triggers: []RuleTrigger{{Field: FieldState, Operation: OpEquals, Value: "ready"}},
		Actions:  []RuleAction{{ActionType: ActionSetState, Value: "next"}},
		Scope:    &RuleScope{States: []models.WorkflowState{models.StateReady, models.StateWaiting}},
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("canonical state scope rejected: %v", err)
	}
	task := &models.Task{WorkflowState: models.StateReady}
	if !rule.MatchesScope(task, "", "") {
		t.Error("ready task should match ready/waiting state scope")
	}
	task.WorkflowState = models.StateDone
	if rule.MatchesScope(task, "", "") {
		t.Error("done task should not match ready/waiting state scope")
	}
}

func TestCanonicalRuleStringRendering(t *testing.T) {
	trigger := RuleTrigger{Field: FieldState, Operation: OpNotIn, Value: "done,canceled"}
	if got := trigger.String(); got != "state not in done,canceled" {
		t.Errorf("trigger.String() = %q", got)
	}
	action := RuleAction{ActionType: ActionSetState, Value: "in_progress"}
	if got := action.String(); got != "set state to in_progress" {
		t.Errorf("action.String() = %q", got)
	}
}

func TestWorkflowRuleJSONRetainsLegacyDecodeAndWritesCanonicalRules(t *testing.T) {
	legacyJSON := `{
		"id":"legacy",
		"name":"Legacy",
		"enabled":true,
		"trigger":{"field":"status","operation":"equals","value":"todo"},
		"actions":[{"action_type":"set_section","value":"Next"}],
		"scope":{"sections":["Inbox"]}
	}`
	var legacy WorkflowRule
	if err := json.Unmarshal([]byte(legacyJSON), &legacy); err != nil {
		t.Fatalf("decode legacy rule: %v", err)
	}
	migrated, changed, err := MigrateLegacyLifecycleRules([]WorkflowRule{legacy})
	if err != nil || !changed || len(migrated) != 1 {
		t.Fatalf("legacy decode migration = %#v, changed=%v, err=%v", migrated, changed, err)
	}
	if migrated[0].Triggers[0].Field != FieldState ||
		migrated[0].Actions[0].ActionType != ActionSetState {
		t.Fatalf("legacy lifecycle schema was not canonicalized: %#v", migrated[0])
	}

	canonical := NewRule(
		"Canonical",
		RuleTrigger{Field: FieldStateChanged, Operation: OpEquals, Value: "ready,next->in_progress"},
		[]RuleAction{{ActionType: ActionSetState, Value: "done"}},
	)
	canonical.Scope = &RuleScope{States: []models.WorkflowState{models.StateNext}}
	data, err := json.Marshal(canonical)
	if err != nil {
		t.Fatalf("marshal canonical rule: %v", err)
	}
	if strings.Contains(string(data), `"trigger":`) ||
		strings.Contains(string(data), `"status"`) ||
		strings.Contains(string(data), `"section"`) {
		t.Errorf("canonical rule JSON contains legacy schema: %s", data)
	}

	var roundTrip WorkflowRule
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("round-trip canonical rule: %v", err)
	}
	if err := roundTrip.Validate(); err != nil {
		t.Fatalf("round-trip canonical rule invalid: %v", err)
	}
}

func TestDefaultRulesUseCanonicalLifecycleVocabulary(t *testing.T) {
	for _, rule := range GetDefaultRules() {
		if err := rule.Validate(); err != nil {
			t.Errorf("default rule %q invalid: %v", rule.ID, err)
		}
	}
}
