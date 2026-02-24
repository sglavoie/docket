package config

import "testing"

func TestRecentTaskVisitHelpers(t *testing.T) {
	t.Run("SetRecentTaskVisits replaces entire list", func(t *testing.T) {
		cfg := NewConfig()
		cfg.SetRecentTaskVisits([]RecentTaskVisit{
			{TaskID: "t1", ProjectID: "p1"},
			{TaskID: "t2", ProjectID: "p1"},
		})

		v := cfg.GetRecentTaskVisits()
		if len(v) != 2 {
			t.Fatalf("expected 2 items, got %d", len(v))
		}
		if v[0].TaskID != "t1" || v[1].TaskID != "t2" {
			t.Errorf("unexpected visits: %v", v)
		}
	})

	t.Run("SetRecentTaskVisits overwrites previous list", func(t *testing.T) {
		cfg := NewConfig()
		cfg.SetRecentTaskVisits([]RecentTaskVisit{
			{TaskID: "t1", ProjectID: "p1"},
		})
		cfg.SetRecentTaskVisits([]RecentTaskVisit{
			{TaskID: "t3", ProjectID: "p2"},
		})

		v := cfg.GetRecentTaskVisits()
		if len(v) != 1 || v[0].TaskID != "t3" {
			t.Errorf("expected replaced list with t3, got %v", v)
		}
	})
}
