package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigManager_LoadHandlesNilProjectEntry(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// A JSON `null` entry in the projects array previously caused a nil
	// pointer panic in the validity check.
	data := `{
		"version": 1,
		"data_dir": "/tmp/tt-test",
		"projects": [
			null,
			{"id": "p1", "slug": "valid", "name": "Valid", "path": "/tmp/valid"},
			null
		]
	}`
	if err := os.WriteFile(configPath, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Projects) != 1 {
		t.Fatalf("expected 1 surviving project, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0] == nil || cfg.Projects[0].ID != "p1" {
		t.Errorf("expected surviving project p1, got %+v", cfg.Projects[0])
	}
}

func TestConfigManager_LoadDoesNotWarnOnEmptySlugDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// Two projects with empty slugs should not produce a spurious duplicate
	// slug warning (the previous code treated "" as a duplicate value).
	data := `{
		"version": 1,
		"data_dir": "/tmp/tt-test",
		"projects": [
			{"id": "p1", "name": "One", "path": "/tmp/one"},
			{"id": "p2", "name": "Two", "path": "/tmp/two"}
		]
	}`
	if err := os.WriteFile(configPath, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)

	// Capture stderr to confirm no duplicate-slug warning is emitted.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w

	if _, err := manager.Load(); err != nil {
		os.Stderr = oldStderr
		_ = w.Close()
		_ = r.Close()
		t.Fatalf("Load failed: %v", err)
	}
	_ = w.Close()
	os.Stderr = oldStderr

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	_ = r.Close()
	output := string(buf[:n])

	if contains(output, "duplicate project slug") {
		t.Errorf("unexpected duplicate slug warning for empty slugs:\n%s", output)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestConfigManager_ClampsNegativeWeeklyAndAlertFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                      1,
		"data_dir":                     "/tmp/tt-test",
		"projects":                     []any{},
		"weekly_stale_project_days":    -5,
		"weekly_stale_task_days":       -10,
		"weekly_inbox_threshold":       -1,
		"weekly_blocked_threshold":     -2,
		"weekly_in_progress_threshold": -3,
		"stale_task_days":              -4,
		"priority_escalation_days":     -6,
		"alert_due_soon_days":          -7,
		"alert_stale_days":             -8,
		"alert_inbox_threshold":        -9,
		"alert_blocked_days":           -11,
		"alert_project_stale_days":     -12,
		"recent_activity_limit":        -13,
		"max_activity_entries":         -14,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	checks := []struct {
		name string
		ptr  *int
	}{
		{"WeeklyStaleProjectDays", cfg.WeeklyStaleProjectDays},
		{"WeeklyStaleTaskDays", cfg.WeeklyStaleTaskDays},
		{"WeeklyInboxThreshold", cfg.WeeklyInboxThreshold},
		{"WeeklyBlockedThreshold", cfg.WeeklyBlockedThreshold},
		{"WeeklyInProgressThreshold", cfg.WeeklyInProgressThreshold},
		{"StaleTaskDays", cfg.StaleTaskDays},
		{"PriorityEscalationDays", cfg.PriorityEscalationDays},
		{"AlertDueSoonDays", cfg.AlertDueSoonDays},
		{"AlertStaleDays", cfg.AlertStaleDays},
		{"AlertInboxThreshold", cfg.AlertInboxThreshold},
		{"AlertBlockedDays", cfg.AlertBlockedDays},
		{"AlertProjectStaleDays", cfg.AlertProjectStaleDays},
		{"RecentActivityLimit", cfg.RecentActivityLimit},
		{"MaxActivityEntries", cfg.MaxActivityEntries},
	}
	for _, ch := range checks {
		if ch.ptr == nil {
			t.Errorf("%s should not be nil", ch.name)
			continue
		}
		if *ch.ptr != 0 {
			t.Errorf("%s expected clamped to 0, got %d", ch.name, *ch.ptr)
		}
	}
}

func TestDismissAlert_RefreshesExpiredEntry(t *testing.T) {
	cfg := NewConfig()
	// Seed an expired dismissal directly.
	cfg.AlertDismissals = append(cfg.AlertDismissals, AlertDismissal{
		Key:         "overdue:t1",
		Category:    "overdue",
		DismissedAt: time.Now().Add(-31 * 24 * time.Hour),
	})

	if cfg.IsAlertDismissed("overdue:t1") {
		t.Error("expired entry must not count as dismissed")
	}

	cfg.DismissAlert("overdue:t1", "overdue")
	if len(cfg.AlertDismissals) != 1 {
		t.Errorf("expected single refreshed entry, got %d", len(cfg.AlertDismissals))
	}
	if !cfg.IsAlertDismissed("overdue:t1") {
		t.Error("expected refreshed entry to be dismissed")
	}
}
