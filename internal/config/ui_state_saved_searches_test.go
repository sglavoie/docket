package config

import "testing"

func TestGetSavedSearches_Empty(t *testing.T) {
	cfg := NewConfig()

	searches := cfg.GetSavedSearches()

	if searches == nil {
		t.Error("GetSavedSearches should return empty map, not nil")
	}
	if len(searches) != 0 {
		t.Errorf("expected 0 searches, got %d", len(searches))
	}
}

func TestSaveSearch(t *testing.T) {
	cfg := NewConfig()

	cfg.SaveSearch("urgent", "priority:1 status:in_progress")

	query := cfg.GetSavedSearch("urgent")
	if query != "priority:1 status:in_progress" {
		t.Errorf("expected 'priority:1 status:in_progress', got '%s'", query)
	}
}

func TestSaveSearch_Overwrite(t *testing.T) {
	cfg := NewConfig()

	cfg.SaveSearch("my-search", "old query")
	cfg.SaveSearch("my-search", "new query")

	query := cfg.GetSavedSearch("my-search")
	if query != "new query" {
		t.Errorf("expected 'new query', got '%s'", query)
	}
}

func TestSaveSearch_NilMap(t *testing.T) {
	cfg := &Config{}
	cfg.SavedSearches = nil

	cfg.SaveSearch("test", "query")

	if cfg.SavedSearches == nil {
		t.Error("SavedSearches should be initialized")
	}
	if cfg.SavedSearches["test"] != "query" {
		t.Error("search should be saved")
	}
}

func TestDeleteSavedSearch(t *testing.T) {
	cfg := NewConfig()
	cfg.SaveSearch("to-delete", "query")
	cfg.SaveSearch("to-keep", "other query")

	deleted := cfg.DeleteSavedSearch("to-delete")

	if !deleted {
		t.Error("DeleteSavedSearch should return true for existing search")
	}
	if cfg.GetSavedSearch("to-delete") != "" {
		t.Error("deleted search should return empty string")
	}
	if cfg.GetSavedSearch("to-keep") != "other query" {
		t.Error("to-keep should still exist")
	}
}

func TestDeleteSavedSearch_NonExistent(t *testing.T) {
	cfg := NewConfig()

	deleted := cfg.DeleteSavedSearch("nonexistent")

	if deleted {
		t.Error("DeleteSavedSearch should return false for non-existent search")
	}
}

func TestDeleteSavedSearch_NilMap(t *testing.T) {
	cfg := &Config{}
	cfg.SavedSearches = nil

	deleted := cfg.DeleteSavedSearch("test")

	if deleted {
		t.Error("DeleteSavedSearch should return false with nil map")
	}
}

func TestGetSavedSearch_NotFound(t *testing.T) {
	cfg := NewConfig()
	cfg.SaveSearch("exists", "query")

	query := cfg.GetSavedSearch("nonexistent")

	if query != "" {
		t.Errorf("expected empty string for non-existent search, got '%s'", query)
	}
}

func TestGetSavedSearch_NilMap(t *testing.T) {
	cfg := &Config{}
	cfg.SavedSearches = nil

	query := cfg.GetSavedSearch("test")

	if query != "" {
		t.Errorf("expected empty string with nil map, got '%s'", query)
	}
}