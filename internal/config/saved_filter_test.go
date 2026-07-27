package config

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetSavedFilters_Empty(t *testing.T) {
	c := NewPlanningState()
	filters := c.GetSavedFilters()
	if len(filters) != 0 {
		t.Fatalf("expected 0 filters, got %d", len(filters))
	}
}

func TestSaveFilter_Add(t *testing.T) {
	c := NewPlanningState()
	c.SaveFilter(SavedFilter{ID: "id1", Name: "Bug Triage", Mode: "snapshot", FilterType: "bug"})

	filters := c.GetSavedFilters()
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}
	if filters[0].Name != "Bug Triage" {
		t.Errorf("expected name 'Bug Triage', got %q", filters[0].Name)
	}
	if filters[0].Mode != "snapshot" {
		t.Errorf("expected mode 'snapshot', got %q", filters[0].Mode)
	}
}

func TestSaveFilter_Upsert(t *testing.T) {
	c := NewPlanningState()
	c.SaveFilter(SavedFilter{ID: "id1", Name: "Bug Triage", Mode: "snapshot", FilterType: "bug"})
	c.SaveFilter(SavedFilter{ID: "id1", Name: "Bug Triage v2", Mode: "snapshot", FilterType: "task"})

	filters := c.GetSavedFilters()
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter after upsert, got %d", len(filters))
	}
	if filters[0].FilterType != "task" {
		t.Errorf("expected filter_type 'task', got %q", filters[0].FilterType)
	}
	if filters[0].Name != "Bug Triage v2" {
		t.Errorf("expected name 'Bug Triage v2', got %q", filters[0].Name)
	}
}

func TestGetSavedFilter_ByID(t *testing.T) {
	c := NewPlanningState()
	c.SaveFilter(SavedFilter{ID: "abc", Name: "My Filter", Mode: "criteria"})

	f := c.GetSavedFilter("abc")
	if f == nil {
		t.Fatal("expected to find filter")
	}
	if f.Name != "My Filter" {
		t.Errorf("expected 'My Filter', got %q", f.Name)
	}
}

func TestGetSavedFilter_NotFound(t *testing.T) {
	c := NewPlanningState()
	if f := c.GetSavedFilter("missing"); f != nil {
		t.Error("expected nil for missing filter")
	}
}

func TestGetSavedFilterByName(t *testing.T) {
	c := NewPlanningState()
	c.SaveFilter(SavedFilter{ID: "1", Name: "Bug Triage", Mode: "snapshot"})

	f := c.GetSavedFilterByName("bug triage")
	if f == nil {
		t.Fatal("expected case-insensitive match")
	}
	if f.ID != "1" {
		t.Errorf("expected ID '1', got %q", f.ID)
	}
}

func TestGetSavedFilterByName_NotFound(t *testing.T) {
	c := NewPlanningState()
	if f := c.GetSavedFilterByName("nope"); f != nil {
		t.Error("expected nil")
	}
}

func TestDeleteSavedFilter(t *testing.T) {
	c := NewPlanningState()
	c.SaveFilter(SavedFilter{ID: "a", Name: "A"})
	c.SaveFilter(SavedFilter{ID: "b", Name: "B"})

	if !c.DeleteSavedFilter("a") {
		t.Error("expected delete to return true")
	}
	if len(c.GetSavedFilters()) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(c.GetSavedFilters()))
	}
	if c.GetSavedFilters()[0].ID != "b" {
		t.Errorf("expected 'b' to remain")
	}
}

func TestDeleteSavedFilter_NotFound(t *testing.T) {
	c := NewPlanningState()
	if c.DeleteSavedFilter("missing") {
		t.Error("expected false for missing filter")
	}
}

func TestEffectiveLogic(t *testing.T) {
	f := SavedFilter{Logic: "or"}
	if f.EffectiveLogic() != "or" {
		t.Error("expected 'or'")
	}
	f.Logic = ""
	if f.EffectiveLogic() != "and" {
		t.Error("expected default 'and'")
	}
	f.Logic = "and"
	if f.EffectiveLogic() != "and" {
		t.Error("expected 'and'")
	}
}

// --- Migration tests ---

func TestMigrateLegacyFilters_savedViews(t *testing.T) {
	c := NewPlanningState()
	c.SavedViews = []savedView{
		{Name: "Bugs", FilterType: "bug", FilterStatus: "todo", FilterSections: []string{string(models.StateNext)}, FilterGitHubIssue: true, FilterParked: true, SortField: 1, PinnedTagTab: "backend"},
		{Name: "High Pri", FilterPriority: 1, FilterActionable: true},
	}

	if !c.MigrateLegacyFilters() {
		t.Fatal("expected migration to return true")
	}
	if len(c.SavedViews) != 0 {
		t.Error("expected savedViews to be cleared")
	}
	if len(c.SavedFilters) != 2 {
		t.Fatalf("expected 2 saved filters, got %d", len(c.SavedFilters))
	}
	for _, f := range c.SavedFilters {
		if f.Mode != "snapshot" {
			t.Errorf("expected mode 'snapshot', got %q", f.Mode)
		}
		if f.ID == "" {
			t.Error("expected non-empty UUID")
		}
	}
	// Check field mapping
	if c.SavedFilters[0].FilterType != "bug" {
		t.Errorf("expected filter_type 'bug', got %q", c.SavedFilters[0].FilterType)
	}
	if !c.SavedFilters[0].FilterGitHubIssue {
		t.Error("expected filter_github_issue to migrate")
	}
	if !c.SavedFilters[0].FilterParked {
		t.Error("expected filter_parked to migrate")
	}
	if got, want := c.SavedFilters[0].FilterStates.Values(), []models.WorkflowState{models.StateNext}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy todo AND Next states = %v, want %v", got, want)
	}
	if !c.SavedFilters[0].HasStateFilter() {
		t.Fatal("migrated lifecycle predicates should remain an active state filter")
	}
}

func TestMigrateLegacyFilters_smartLists(t *testing.T) {
	c := NewPlanningState()
	c.SmartLists = []smartList{
		{
			ID:   "sl-1",
			Name: "Overdue",
			Criteria: []SmartListCriterion{
				{Field: "status", Operation: "equals", Value: "todo"},
				{Field: "section", Operation: "equals", Value: string(models.StateNext)},
				{Field: "due_date", Operation: "less_than", Value: "today"},
				{Field: "parked", Operation: "equals", Value: "true"},
			},
			Logic:            "and",
			IncludeCompleted: false,
		},
	}

	if !c.MigrateLegacyFilters() {
		t.Fatal("expected migration to return true")
	}
	if len(c.SmartLists) != 0 {
		t.Error("expected smartLists to be cleared")
	}
	if len(c.SavedFilters) != 1 {
		t.Fatalf("expected 1 saved filter, got %d", len(c.SavedFilters))
	}
	f := c.SavedFilters[0]
	if f.ID != "sl-1" {
		t.Errorf("expected ID 'sl-1', got %q", f.ID)
	}
	if f.Mode != "criteria" {
		t.Errorf("expected mode 'criteria', got %q", f.Mode)
	}
	if len(f.Criteria) != 4 {
		t.Fatalf("expected 4 criteria, got %d", len(f.Criteria))
	}
	if f.Criteria[0].Field != "state" || f.Criteria[0].Operation != "in" {
		t.Fatalf("status criterion = %+v, want canonical state set", f.Criteria[0])
	}
	if got, want := f.Criteria[0].States.Values(), []models.WorkflowState{models.StateInbox, models.StateReady, models.StateNext}; !reflect.DeepEqual(got, want) {
		t.Fatalf("todo criterion states = %v, want %v", got, want)
	}
	if f.Criteria[1].Field != "state" || !f.Criteria[1].States.Contains(models.StateNext) {
		t.Fatalf("section criterion = %+v, want state includes Next", f.Criteria[1])
	}
	if f.Criteria[3].Field != "parked" || f.Criteria[3].Value != "true" {
		t.Fatalf("parked criterion = %+v, want parked equals true", f.Criteria[3])
	}
}

func TestMigrateLegacyFilters_Both(t *testing.T) {
	c := NewPlanningState()
	c.SavedViews = []savedView{{Name: "V"}}
	c.SmartLists = []smartList{{ID: "s1", Name: "S"}}

	c.MigrateLegacyFilters()
	if len(c.SavedFilters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(c.SavedFilters))
	}
	if c.SavedFilters[0].Mode != "snapshot" {
		t.Error("first should be snapshot")
	}
	if c.SavedFilters[1].Mode != "criteria" {
		t.Error("second should be criteria")
	}
}

func TestMigrateLegacyFilters_Empty(t *testing.T) {
	c := NewPlanningState()
	if c.MigrateLegacyFilters() {
		t.Error("expected no migration when both empty")
	}
}

func TestMigrateLegacyFilters_Idempotent(t *testing.T) {
	c := NewPlanningState()
	c.SavedViews = []savedView{{Name: "V"}}
	c.MigrateLegacyFilters()

	count := len(c.SavedFilters)
	if c.MigrateLegacyFilters() {
		t.Error("second call should be no-op")
	}
	if len(c.SavedFilters) != count {
		t.Error("filters should not change on second call")
	}
}

// --- Round-trip JSON test ---

func TestSavedFilter_JSONRoundTrip(t *testing.T) {
	states, err := models.NewWorkflowStateSet(models.StateWaiting, models.StateNext)
	if err != nil {
		t.Fatalf("state set: %v", err)
	}
	original := SavedFilter{
		ID:                "test-id",
		Name:              "Complex",
		Mode:              "snapshot",
		FilterType:        "bug",
		FilterTags:        []string{"a", "b"},
		FilterAttrs:       map[string]string{"x": "y"},
		FilterGitHubIssue: true,
		FilterParked:      true,
		FilterScoreMin:    42.5,
	}
	original.SetFilterStates(states, true)

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SavedFilter
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != original.ID || decoded.Name != original.Name {
		t.Error("basic fields mismatch")
	}
	if decoded.FilterScoreMin != original.FilterScoreMin {
		t.Errorf("score min: got %f, want %f", decoded.FilterScoreMin, original.FilterScoreMin)
	}
	if decoded.FilterGitHubIssue != original.FilterGitHubIssue {
		t.Errorf("filter_github_issue: got %v, want %v", decoded.FilterGitHubIssue, original.FilterGitHubIssue)
	}
	if decoded.FilterParked != original.FilterParked {
		t.Errorf("filter_parked: got %v, want %v", decoded.FilterParked, original.FilterParked)
	}
	if len(decoded.FilterTags) != 2 {
		t.Errorf("tags: got %d, want 2", len(decoded.FilterTags))
	}
	if !decoded.HasStateFilter() || !decoded.FilterStates.Equal(states) {
		t.Errorf("filter states = %v active=%v, want %v active", decoded.FilterStates.Values(), decoded.HasStateFilter(), states.Values())
	}
}

func TestSavedFilterJSONMigratesLegacyLifecycleIntersection(t *testing.T) {
	var decoded SavedFilter
	err := json.Unmarshal([]byte(`{
		"id":"legacy",
		"name":"Legacy",
		"mode":"snapshot",
		"filter_status":"todo",
		"filter_sections":["Next"]
	}`), &decoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, want := decoded.FilterStates.Values(), []models.WorkflowState{models.StateNext}; !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated states = %v, want %v", got, want)
	}

	data, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("decode fields: %v", err)
	}
	if _, ok := fields["filter_states"]; !ok {
		t.Fatal("canonical JSON missing filter_states")
	}
	if _, ok := fields["filter_status"]; ok {
		t.Fatal("canonical JSON retained filter_status")
	}
	if _, ok := fields["filter_sections"]; ok {
		t.Fatal("canonical JSON retained filter_sections")
	}
}

func TestSavedFilterJSONCanonicalStatesWinMixedLegacyInputAndTriggerRewrite(t *testing.T) {
	var decoded SavedFilter
	err := json.Unmarshal([]byte(`{
		"id":"mixed",
		"name":"Mixed",
		"mode":"snapshot",
		"filter_states":["waiting","next"],
		"filter_status":"done",
		"filter_sections":["Completed"]
	}`), &decoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := testConfigStateSet(t, models.StateNext, models.StateWaiting)
	if !decoded.FilterStates.Equal(want) {
		t.Fatalf("mixed states = %v, want canonical %v", decoded.FilterStates.Values(), want.Values())
	}

	planning := NewPlanningState()
	planning.SavedFilters = []SavedFilter{decoded}
	if !planning.MigrateLegacyFilters() {
		t.Fatal("mixed legacy fields should request a canonical rewrite")
	}
	if planning.MigrateLegacyFilters() {
		t.Fatal("canonical rewrite marker should be idempotent")
	}
	data, err := json.Marshal(planning.SavedFilters[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if containsJSONField(data, "filter_status") || containsJSONField(data, "filter_sections") {
		t.Fatalf("rewritten JSON retained legacy snapshot fields: %s", data)
	}
}

func TestSavedFilterJSONRejectsInvalidCanonicalStates(t *testing.T) {
	for _, payload := range []string{
		`{"id":"bad","name":"Bad","mode":"snapshot","filter_states":["todo"]}`,
		`{"id":"bad","name":"Bad","mode":"criteria","criteria":[{"field":"state","operation":"in","states":["todo"]}]}`,
		`{"id":"bad","name":"Bad","mode":"criteria","criteria":[{"field":"state","operation":"equals","value":"inbox"}]}`,
	} {
		var decoded SavedFilter
		if err := json.Unmarshal([]byte(payload), &decoded); err == nil {
			t.Fatalf("json.Unmarshal accepted invalid canonical state payload %s", payload)
		}
	}
}

func TestSavedFilterJSONPreservesContradictoryLegacyLifecycleAsMatchNone(t *testing.T) {
	var decoded SavedFilter
	err := json.Unmarshal([]byte(`{
		"id":"legacy-none",
		"name":"Legacy none",
		"mode":"snapshot",
		"filter_status":"done",
		"filter_sections":["Next"]
	}`), &decoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !decoded.HasStateFilter() || !decoded.FilterStates.IsEmpty() {
		t.Fatalf("contradictory migration = states %v active=%v, want active empty set", decoded.FilterStates.Values(), decoded.HasStateFilter())
	}
	data, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !json.Valid(data) || !containsJSONField(data, "filter_states") {
		t.Fatalf("canonical match-none JSON = %s, want filter_states", data)
	}
}

func TestSavedFilterJSONMigratesLegacyCriteriaLifecycleFields(t *testing.T) {
	var decoded SavedFilter
	err := json.Unmarshal([]byte(`{
		"id":"criteria",
		"name":"Criteria",
		"mode":"criteria",
		"logic":"and",
		"criteria":[
			{"field":"status","operation":"equals","value":"todo"},
			{"field":"section","operation":"not_equals","value":"Completed"}
		]
	}`), &decoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for i, criterion := range decoded.Criteria {
		if criterion.Field != "state" {
			t.Fatalf("criterion %d field = %q, want state", i, criterion.Field)
		}
	}
	data, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) == "" || containsJSONField(data, "filter_status") || containsJSONField(data, "filter_sections") {
		t.Fatalf("canonical criteria JSON retained legacy lifecycle fields: %s", data)
	}
}

func containsJSONField(data []byte, field string) bool {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false
	}
	return containsJSONFieldValue(value, field)
}

func containsJSONFieldValue(value any, field string) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if key == field || containsJSONFieldValue(child, field) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if containsJSONFieldValue(child, field) {
				return true
			}
		}
	}
	return false
}

func testConfigStateSet(t *testing.T, states ...models.WorkflowState) models.WorkflowStateSet {
	t.Helper()
	set, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		t.Fatalf("state set: %v", err)
	}
	return set
}

func TestSavedFilter_JSONMissingGitHubIssueDefaultsFalse(t *testing.T) {
	var decoded SavedFilter
	if err := json.Unmarshal([]byte(`{"id":"test-id","name":"Legacy","mode":"snapshot"}`), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.FilterGitHubIssue {
		t.Error("missing filter_github_issue should default to false")
	}
	if decoded.FilterParked {
		t.Error("missing filter_parked should default to false")
	}
}

// --- Manager migration integration test ---
