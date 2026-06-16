package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetSavedFilters_Empty(t *testing.T) {
	c := NewConfig()
	filters := c.GetSavedFilters()
	if len(filters) != 0 {
		t.Fatalf("expected 0 filters, got %d", len(filters))
	}
}

func TestSaveFilter_Add(t *testing.T) {
	c := NewConfig()
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
	c := NewConfig()
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
	c := NewConfig()
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
	c := NewConfig()
	if f := c.GetSavedFilter("missing"); f != nil {
		t.Error("expected nil for missing filter")
	}
}

func TestGetSavedFilterByName(t *testing.T) {
	c := NewConfig()
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
	c := NewConfig()
	if f := c.GetSavedFilterByName("nope"); f != nil {
		t.Error("expected nil")
	}
}

func TestDeleteSavedFilter(t *testing.T) {
	c := NewConfig()
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
	c := NewConfig()
	if c.DeleteSavedFilter("missing") {
		t.Error("expected false for missing filter")
	}
}

func TestDeleteSavedFilter_CleansTabOrder(t *testing.T) {
	c := NewConfig()
	c.SaveFilter(SavedFilter{ID: "sl1", Name: "SL", ShowAsTab: true})
	c.AllTasksTabOrder = []string{"All", "sl1", "Due"}

	c.DeleteSavedFilter("sl1")
	if len(c.AllTasksTabOrder) != 2 {
		t.Fatalf("expected 2 tab entries, got %d", len(c.AllTasksTabOrder))
	}
	for _, e := range c.AllTasksTabOrder {
		if e == "sl1" {
			t.Error("sl1 should have been removed from tab order")
		}
	}
}

func TestGetTabFilters(t *testing.T) {
	c := NewConfig()
	c.SaveFilter(SavedFilter{ID: "1", Name: "Tab", ShowAsTab: true})
	c.SaveFilter(SavedFilter{ID: "2", Name: "NoTab", ShowAsTab: false})
	c.SaveFilter(SavedFilter{ID: "3", Name: "Tab2", ShowAsTab: true})

	tabs := c.GetTabFilters()
	if len(tabs) != 2 {
		t.Fatalf("expected 2 tab filters, got %d", len(tabs))
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
	c := NewConfig()
	c.SavedViews = []savedView{
		{Name: "Bugs", FilterType: "bug", SortField: 1, PinnedTagTab: "backend"},
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
	if c.SavedFilters[0].PinnedTagTab != "backend" {
		t.Errorf("expected pinned_tag_tab to migrate, got %q", c.SavedFilters[0].PinnedTagTab)
	}
}

func TestMigrateLegacyFilters_smartLists(t *testing.T) {
	c := NewConfig()
	c.SmartLists = []smartList{
		{
			ID:   "sl-1",
			Name: "Overdue",
			Criteria: []SmartListCriterion{
				{Field: "due_date", Operation: "less_than", Value: "today"},
			},
			Logic:            "and",
			IncludeCompleted: false,
		},
	}
	c.AllTasksTabOrder = []string{"All", "sl-1"}

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
	if !f.ShowAsTab {
		t.Error("expected ShowAsTab=true")
	}
	if len(f.Criteria) != 1 {
		t.Fatalf("expected 1 criterion, got %d", len(f.Criteria))
	}
	// AllTasksTabOrder should be unchanged (IDs match).
	if len(c.AllTasksTabOrder) != 2 || c.AllTasksTabOrder[1] != "sl-1" {
		t.Error("AllTasksTabOrder should be preserved")
	}
}

func TestMigrateLegacyFilters_Both(t *testing.T) {
	c := NewConfig()
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
	c := NewConfig()
	if c.MigrateLegacyFilters() {
		t.Error("expected no migration when both empty")
	}
}

func TestMigrateLegacyFilters_Idempotent(t *testing.T) {
	c := NewConfig()
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
		ID:             "test-id",
		Name:           "Complex",
		Mode:           "snapshot",
		ViewMode:       2,
		FilterType:     "bug",
		FilterTags:     []string{"a", "b"},
		FilterAttrs:    map[string]string{"x": "y"},
		FilterScoreMin: 42.5,
		SortField:      3,
		SortAscending:  true,
		PinnedTagTab:   "backend",
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
	if len(decoded.FilterTags) != 2 {
		t.Errorf("tags: got %d, want 2", len(decoded.FilterTags))
	}
	if decoded.PinnedTagTab != original.PinnedTagTab {
		t.Errorf("pinned tag tab: got %q, want %q", decoded.PinnedTagTab, original.PinnedTagTab)
	}
}

// --- Manager migration integration test ---

func TestManager_MigrationOnLoad(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".tt.json")

	// Write a config with legacy fields.
	legacy := map[string]any{
		"data_dir": dir,
		"editor":   "vim",
		"saved_views": []map[string]any{
			{"name": "My Bugs", "filter_type": "bug"},
		},
		"smart_lists": []map[string]any{
			{
				"id":   "sl-abc",
				"name": "Overdue",
				"criteria": []map[string]string{
					{"field": "due_date", "operation": "less_than", "value": "today"},
				},
			},
		},
	}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m := NewManager(configPath)
	cfg, err := m.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Legacy fields should be cleared.
	if len(cfg.SavedViews) != 0 {
		t.Error("savedViews should be empty after migration")
	}
	if len(cfg.SmartLists) != 0 {
		t.Error("smartLists should be empty after migration")
	}

	// SavedFilters should have 2 entries.
	if len(cfg.SavedFilters) != 2 {
		t.Fatalf("expected 2 saved filters, got %d", len(cfg.SavedFilters))
	}

	// Verify snapshot conversion.
	var snapshot *SavedFilter
	for i := range cfg.SavedFilters {
		if cfg.SavedFilters[i].Mode == "snapshot" {
			snapshot = &cfg.SavedFilters[i]
		}
	}
	if snapshot == nil {
		t.Fatal("no snapshot filter found")
	}
	if snapshot.FilterType != "bug" {
		t.Errorf("snapshot filter_type: got %q, want 'bug'", snapshot.FilterType)
	}

	// Verify criteria conversion.
	criteria := cfg.GetSavedFilter("sl-abc")
	if criteria == nil {
		t.Fatal("criteria filter sl-abc not found")
	}
	if criteria.Mode != "criteria" {
		t.Errorf("criteria mode: got %q", criteria.Mode)
	}
	if !criteria.ShowAsTab {
		t.Error("criteria filter should have ShowAsTab=true")
	}

	// Config file on disk should now have saved_filters and no legacy fields.
	reloaded, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(reloaded, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := raw["saved_filters"]; !ok {
		t.Error("expected saved_filters key in persisted config")
	}

	// Second load should not re-migrate.
	cfg2, err := m.Load()
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if len(cfg2.SavedFilters) != 2 {
		t.Errorf("expected 2 filters on second load, got %d", len(cfg2.SavedFilters))
	}
}
