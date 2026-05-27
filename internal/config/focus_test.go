package config

import (
	"fmt"
	"testing"
)

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

func TestReassignTaskProject(t *testing.T) {
	t.Run("updates FocusQueue, DailyPlan, and RecentTaskVisits", func(t *testing.T) {
		cfg := NewConfig()
		cfg.FocusQueue = []FocusQueueItem{
			{TaskID: "t1", ProjectID: "old-proj"},
			{TaskID: "t2", ProjectID: "other-proj"},
			{TaskID: "t3", ProjectID: "old-proj"},
		}
		cfg.DailyPlan = []DailyPlanItem{
			{TaskID: "t1", ProjectID: "old-proj", PlannedDate: "2026-04-13"},
			{TaskID: "t4", ProjectID: "old-proj", PlannedDate: "2026-04-13"},
		}
		cfg.RecentTaskVisits = []RecentTaskVisit{
			{TaskID: "t1", ProjectID: "old-proj"},
			{TaskID: "t5", ProjectID: "other-proj"},
		}

		movedIDs := map[string]bool{"t1": true, "t3": true}
		updated := cfg.ReassignTaskProject(movedIDs, "new-proj")

		// t1 in FocusQueue+DailyPlan+RecentTaskVisits (3), t3 in FocusQueue (1) → 4 updates
		if updated != 4 {
			t.Fatalf("expected 4 updates, got %d", updated)
		}
		// FocusQueue: t1→new-proj, t2→other-proj (unchanged), t3→new-proj
		if cfg.FocusQueue[0].ProjectID != "new-proj" {
			t.Errorf("FocusQueue[0] expected new-proj, got %s", cfg.FocusQueue[0].ProjectID)
		}
		if cfg.FocusQueue[1].ProjectID != "other-proj" {
			t.Errorf("FocusQueue[1] should be unchanged, got %s", cfg.FocusQueue[1].ProjectID)
		}
		if cfg.FocusQueue[2].ProjectID != "new-proj" {
			t.Errorf("FocusQueue[2] expected new-proj, got %s", cfg.FocusQueue[2].ProjectID)
		}
		// DailyPlan: t1→new-proj, t4→old-proj (not in moved set)
		if cfg.DailyPlan[0].ProjectID != "new-proj" {
			t.Errorf("DailyPlan[0] expected new-proj, got %s", cfg.DailyPlan[0].ProjectID)
		}
		if cfg.DailyPlan[1].ProjectID != "old-proj" {
			t.Errorf("DailyPlan[1] should be unchanged, got %s", cfg.DailyPlan[1].ProjectID)
		}
		// RecentTaskVisits: t1→new-proj, t5→other-proj (unchanged)
		if cfg.RecentTaskVisits[0].ProjectID != "new-proj" {
			t.Errorf("RecentTaskVisits[0] expected new-proj, got %s", cfg.RecentTaskVisits[0].ProjectID)
		}
		if cfg.RecentTaskVisits[1].ProjectID != "other-proj" {
			t.Errorf("RecentTaskVisits[1] should be unchanged, got %s", cfg.RecentTaskVisits[1].ProjectID)
		}
	})

	t.Run("no-op when no tasks match", func(t *testing.T) {
		cfg := NewConfig()
		cfg.FocusQueue = []FocusQueueItem{
			{TaskID: "t1", ProjectID: "proj-a"},
		}
		updated := cfg.ReassignTaskProject(map[string]bool{"t99": true}, "proj-b")
		if updated != 0 {
			t.Fatalf("expected 0 updates, got %d", updated)
		}
		if cfg.FocusQueue[0].ProjectID != "proj-a" {
			t.Errorf("should be unchanged, got %s", cfg.FocusQueue[0].ProjectID)
		}
	})

	t.Run("no-op when already correct project", func(t *testing.T) {
		cfg := NewConfig()
		cfg.FocusQueue = []FocusQueueItem{
			{TaskID: "t1", ProjectID: "new-proj"},
		}
		updated := cfg.ReassignTaskProject(map[string]bool{"t1": true}, "new-proj")
		if updated != 0 {
			t.Fatalf("expected 0 updates (already correct), got %d", updated)
		}
	})
}

func TestRemoveTaskRefs(t *testing.T) {
	t.Run("removes matching tasks from all three lists", func(t *testing.T) {
		cfg := NewConfig()
		cfg.FocusQueue = []FocusQueueItem{
			{TaskID: "t1", ProjectID: "p1"},
			{TaskID: "t2", ProjectID: "p1"},
			{TaskID: "t3", ProjectID: "p2"},
		}
		cfg.DailyPlan = []DailyPlanItem{
			{TaskID: "t1", ProjectID: "p1", PlannedDate: "2026-04-13"},
			{TaskID: "t4", ProjectID: "p1", PlannedDate: "2026-04-13"},
		}
		cfg.RecentTaskVisits = []RecentTaskVisit{
			{TaskID: "t1", ProjectID: "p1"},
			{TaskID: "t5", ProjectID: "p2"},
			{TaskID: "t3", ProjectID: "p2"},
		}

		removed := cfg.RemoveTaskRefs(map[string]bool{"t1": true, "t3": true})

		// t1 in FocusQueue(1) + DailyPlan(1) + RecentTaskVisits(1) = 3
		// t3 in FocusQueue(1) + RecentTaskVisits(1) = 2
		// Total = 5
		if removed != 5 {
			t.Fatalf("expected 5 removals, got %d", removed)
		}
		// FocusQueue: only t2 remains
		if len(cfg.FocusQueue) != 1 || cfg.FocusQueue[0].TaskID != "t2" {
			t.Errorf("expected FocusQueue=[t2], got %v", cfg.FocusQueue)
		}
		// DailyPlan: only t4 remains
		if len(cfg.DailyPlan) != 1 || cfg.DailyPlan[0].TaskID != "t4" {
			t.Errorf("expected DailyPlan=[t4], got %v", cfg.DailyPlan)
		}
		// RecentTaskVisits: only t5 remains
		if len(cfg.RecentTaskVisits) != 1 || cfg.RecentTaskVisits[0].TaskID != "t5" {
			t.Errorf("expected RecentTaskVisits=[t5], got %v", cfg.RecentTaskVisits)
		}
	})

	t.Run("no-op when no tasks match", func(t *testing.T) {
		cfg := NewConfig()
		cfg.FocusQueue = []FocusQueueItem{{TaskID: "t1", ProjectID: "p1"}}
		cfg.DailyPlan = []DailyPlanItem{{TaskID: "t2", ProjectID: "p1", PlannedDate: "2026-04-13"}}
		cfg.RecentTaskVisits = []RecentTaskVisit{{TaskID: "t3", ProjectID: "p1"}}

		removed := cfg.RemoveTaskRefs(map[string]bool{"t99": true})

		if removed != 0 {
			t.Fatalf("expected 0 removals, got %d", removed)
		}
		if len(cfg.FocusQueue) != 1 || len(cfg.DailyPlan) != 1 || len(cfg.RecentTaskVisits) != 1 {
			t.Error("lists should be unchanged")
		}
	})

	t.Run("works with empty lists", func(t *testing.T) {
		cfg := NewConfig()
		removed := cfg.RemoveTaskRefs(map[string]bool{"t1": true})
		if removed != 0 {
			t.Fatalf("expected 0 removals on empty config, got %d", removed)
		}
	})

	t.Run("removes all entries when all match", func(t *testing.T) {
		cfg := NewConfig()
		cfg.FocusQueue = []FocusQueueItem{
			{TaskID: "t1", ProjectID: "p1"},
			{TaskID: "t2", ProjectID: "p1"},
		}
		removed := cfg.RemoveTaskRefs(map[string]bool{"t1": true, "t2": true})
		if removed != 2 {
			t.Fatalf("expected 2 removals, got %d", removed)
		}
		if len(cfg.FocusQueue) != 0 {
			t.Errorf("expected empty FocusQueue, got %v", cfg.FocusQueue)
		}
	})

	t.Run("removes matching IDs from FocusDismissedIDs", func(t *testing.T) {
		cfg := NewConfig()
		cfg.FocusDismissedIDs = []string{"t1", "t2", "t3", "t4"}

		removed := cfg.RemoveTaskRefs(map[string]bool{"t2": true, "t4": true})

		if removed != 2 {
			t.Fatalf("expected 2 removals, got %d", removed)
		}
		if len(cfg.FocusDismissedIDs) != 2 {
			t.Fatalf("expected 2 IDs remaining, got %d (%v)", len(cfg.FocusDismissedIDs), cfg.FocusDismissedIDs)
		}
		if cfg.FocusDismissedIDs[0] != "t1" || cfg.FocusDismissedIDs[1] != "t3" {
			t.Errorf("expected [t1 t3], got %v", cfg.FocusDismissedIDs)
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

	t.Run("PruneFocusDismissedIDs trims excess entries", func(t *testing.T) {
		cfg := NewConfig()
		// Create a list exceeding maxFocusDismissedIDs
		ids := make([]string, maxFocusDismissedIDs+50)
		for i := range ids {
			ids[i] = fmt.Sprintf("task-%d", i)
		}
		cfg.SetFocusDismissedIDs(ids)

		pruned := cfg.PruneFocusDismissedIDs()
		if !pruned {
			t.Error("expected pruning to return true")
		}
		if len(cfg.FocusDismissedIDs) != maxFocusDismissedIDs {
			t.Errorf("expected %d IDs after prune, got %d", maxFocusDismissedIDs, len(cfg.FocusDismissedIDs))
		}
		// First entries should be preserved (most recent are at the front)
		if cfg.FocusDismissedIDs[0] != "task-0" {
			t.Errorf("expected first entry to be task-0, got %s", cfg.FocusDismissedIDs[0])
		}
	})

	t.Run("PruneFocusDismissedIDs noop when under limit", func(t *testing.T) {
		cfg := NewConfig()
		cfg.SetFocusDismissedIDs([]string{"d1", "d2", "d3"})

		pruned := cfg.PruneFocusDismissedIDs()
		if pruned {
			t.Error("expected pruning to return false when under limit")
		}
		if len(cfg.FocusDismissedIDs) != 3 {
			t.Errorf("expected 3 IDs unchanged, got %d", len(cfg.FocusDismissedIDs))
		}
	})
}
