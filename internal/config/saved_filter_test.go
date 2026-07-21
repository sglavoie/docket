package config

import (
	"encoding/json"
	"testing"
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
		{Name: "Bugs", FilterType: "bug", FilterGitHubIssue: true, FilterParked: true, SortField: 1, PinnedTagTab: "backend"},
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
}

func TestMigrateLegacyFilters_smartLists(t *testing.T) {
	c := NewPlanningState()
	c.SmartLists = []smartList{
		{
			ID:   "sl-1",
			Name: "Overdue",
			Criteria: []SmartListCriterion{
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
	if len(f.Criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(f.Criteria))
	}
	if f.Criteria[1].Field != "parked" || f.Criteria[1].Value != "true" {
		t.Fatalf("parked criterion = %+v, want parked equals true", f.Criteria[1])
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
