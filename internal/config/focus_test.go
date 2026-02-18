package config

import "testing"

func TestFocusQueueHelpers(t *testing.T) {
	t.Run("SetFocusQueue replaces entire queue", func(t *testing.T) {
		cfg := NewConfig()
		cfg.SetFocusQueue([]FocusQueueItem{
			{TaskID: "t1", ProjectID: "p1"},
			{TaskID: "t2", ProjectID: "p1"},
		})

		q := cfg.GetFocusQueue()
		if len(q) != 2 {
			t.Fatalf("expected 2 items, got %d", len(q))
		}
		if q[0].TaskID != "t1" || q[1].TaskID != "t2" {
			t.Errorf("unexpected queue: %v", q)
		}
	})

	t.Run("SetFocusQueue overwrites previous queue", func(t *testing.T) {
		cfg := NewConfig()
		cfg.SetFocusQueue([]FocusQueueItem{
			{TaskID: "t1", ProjectID: "p1"},
		})
		cfg.SetFocusQueue([]FocusQueueItem{
			{TaskID: "t3", ProjectID: "p2"},
		})

		q := cfg.GetFocusQueue()
		if len(q) != 1 || q[0].TaskID != "t3" {
			t.Errorf("expected replaced queue with t3, got %v", q)
		}
	})
}

func TestFocusDismissedHelpers(t *testing.T) {
	t.Run("SetFocusDismissedIDs replaces list", func(t *testing.T) {
		cfg := NewConfig()
		cfg.SetFocusDismissedIDs([]string{"d1", "d2"})

		ids := cfg.GetFocusDismissedIDs()
		if len(ids) != 2 || ids[0] != "d1" || ids[1] != "d2" {
			t.Errorf("expected [d1, d2], got %v", ids)
		}
	})

	t.Run("SetFocusDismissedIDs overwrites previous list", func(t *testing.T) {
		cfg := NewConfig()
		cfg.SetFocusDismissedIDs([]string{"d1"})
		cfg.SetFocusDismissedIDs([]string{"d3", "d4"})

		ids := cfg.GetFocusDismissedIDs()
		if len(ids) != 2 || ids[0] != "d3" || ids[1] != "d4" {
			t.Errorf("expected [d3, d4], got %v", ids)
		}
	})
}
