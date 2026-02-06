package config

import "testing"

func TestGetSavedViews_Empty(t *testing.T) {
	c := NewConfig()
	views := c.GetSavedViews()
	if len(views) != 0 {
		t.Fatalf("expected 0 views, got %d", len(views))
	}
}

func TestSaveView_Add(t *testing.T) {
	c := NewConfig()
	c.SaveView(SavedView{Name: "Bug Triage", FilterType: "bug", SortField: 1})

	views := c.GetSavedViews()
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	if views[0].Name != "Bug Triage" {
		t.Errorf("expected name 'Bug Triage', got %q", views[0].Name)
	}
	if views[0].FilterType != "bug" {
		t.Errorf("expected filter_type 'bug', got %q", views[0].FilterType)
	}
}

func TestSaveView_Upsert(t *testing.T) {
	c := NewConfig()
	c.SaveView(SavedView{Name: "Bug Triage", FilterType: "bug"})
	c.SaveView(SavedView{Name: "bug triage", FilterType: "task", SortField: 2})

	views := c.GetSavedViews()
	if len(views) != 1 {
		t.Fatalf("expected 1 view after upsert, got %d", len(views))
	}
	if views[0].FilterType != "task" {
		t.Errorf("expected filter_type 'task' after upsert, got %q", views[0].FilterType)
	}
	if views[0].SortField != 2 {
		t.Errorf("expected sort_field 2 after upsert, got %d", views[0].SortField)
	}
}

func TestGetSavedView_Found(t *testing.T) {
	c := NewConfig()
	c.SaveView(SavedView{Name: "My View", FilterPriority: 1, FilterActionable: true})

	v := c.GetSavedView("my view")
	if v == nil {
		t.Fatal("expected to find view")
	}
	if v.FilterPriority != 1 {
		t.Errorf("expected priority 1, got %d", v.FilterPriority)
	}
	if !v.FilterActionable {
		t.Error("expected actionable to be true")
	}
}

func TestGetSavedView_NotFound(t *testing.T) {
	c := NewConfig()
	if v := c.GetSavedView("nonexistent"); v != nil {
		t.Error("expected nil for nonexistent view")
	}
}

func TestDeleteSavedView(t *testing.T) {
	c := NewConfig()
	c.SaveView(SavedView{Name: "View A"})
	c.SaveView(SavedView{Name: "View B"})

	if !c.DeleteSavedView("view a") {
		t.Error("expected delete to return true")
	}
	if len(c.GetSavedViews()) != 1 {
		t.Fatalf("expected 1 view after delete, got %d", len(c.GetSavedViews()))
	}
	if c.GetSavedViews()[0].Name != "View B" {
		t.Errorf("expected 'View B' to remain, got %q", c.GetSavedViews()[0].Name)
	}
}

func TestDeleteSavedView_NotFound(t *testing.T) {
	c := NewConfig()
	if c.DeleteSavedView("nonexistent") {
		t.Error("expected delete to return false for nonexistent view")
	}
}

func TestSaveView_WithComplexFilters(t *testing.T) {
	c := NewConfig()
	c.SaveView(SavedView{
		Name:             "Complex",
		ViewMode:         3,
		FilterType:       "bug",
		FilterStatus:     "in_progress",
		FilterPriority:   1,
		FilterTags:       []string{"backend", "urgent"},
		FilterSections:   []string{"Next"},
		FilterDueDate:    2,
		FilterActionable: true,
		FilterAttrs:      map[string]string{"bug.severity": "critical"},
		FilterScoreMin:   50.0,
		SortField:        1,
		SortAscending:    true,
		CustomSectionTab: "Sprint 1",
	})

	v := c.GetSavedView("complex")
	if v == nil {
		t.Fatal("expected to find view")
	}
	if v.ViewMode != 3 {
		t.Errorf("expected view_mode 3, got %d", v.ViewMode)
	}
	if len(v.FilterTags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(v.FilterTags))
	}
	if v.FilterAttrs["bug.severity"] != "critical" {
		t.Errorf("expected bug.severity=critical, got %q", v.FilterAttrs["bug.severity"])
	}
	if v.FilterScoreMin != 50.0 {
		t.Errorf("expected score_min 50, got %f", v.FilterScoreMin)
	}
	if v.CustomSectionTab != "Sprint 1" {
		t.Errorf("expected custom_section_tab 'Sprint 1', got %q", v.CustomSectionTab)
	}
}
