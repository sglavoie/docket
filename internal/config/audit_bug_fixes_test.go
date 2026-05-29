package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// TestRemoveScoringProfile_CleansProjectAssignments guarantees that removing a
// scoring profile also drops every ProjectScoringProfile entry that pointed at
// it. Without the cleanup, GetProjectScoringProfile silently returns nil for
// the orphaned reference while the stale assignment quietly survives every
// Save round-trip — and resurrects if a profile with the same ID is later
// re-added under different definition.
func TestRemoveScoringProfile_CleansProjectAssignments(t *testing.T) {
	cfg := NewConfig()
	if err := cfg.AddScoringProfile(scoring.NewICEProfile()); err != nil {
		t.Fatalf("AddScoringProfile failed: %v", err)
	}
	cfg.SetProjectScoringProfile("proj-1", "ice")
	cfg.SetProjectScoringProfile("proj-2", "ice")
	cfg.SetProjectScoringProfile("proj-3", "wsjf") // unrelated profile assignment

	if !cfg.RemoveScoringProfile("ice") {
		t.Fatal("RemoveScoringProfile should return true")
	}

	if _, ok := cfg.ProjectScoringProfile["proj-1"]; ok {
		t.Error("expected proj-1 assignment dropped after profile removal")
	}
	if _, ok := cfg.ProjectScoringProfile["proj-2"]; ok {
		t.Error("expected proj-2 assignment dropped after profile removal")
	}
	if got, ok := cfg.ProjectScoringProfile["proj-3"]; !ok || got != "wsjf" {
		t.Errorf("expected unrelated proj-3 assignment preserved, got ok=%v value=%q", ok, got)
	}
}

// TestRemoveScoringProfile_NilProjectMapDoesNotPanic guarantees that removing
// a profile with no ProjectScoringProfile map present (nil) is safe.
func TestRemoveScoringProfile_NilProjectMapDoesNotPanic(t *testing.T) {
	cfg := NewConfig()
	cfg.ProjectScoringProfile = nil
	if err := cfg.AddScoringProfile(scoring.NewICEProfile()); err != nil {
		t.Fatalf("AddScoringProfile failed: %v", err)
	}
	if !cfg.RemoveScoringProfile("ice") {
		t.Fatal("RemoveScoringProfile should return true")
	}
}

// TestMigrateLegacyFilters_RewritesCollidingSmartListIDs guarantees that a
// legacy smartList ID colliding with an existing SavedFilter (or another
// smartList) receives a fresh UUID, preserving the ID uniqueness invariant.
func TestMigrateLegacyFilters_RewritesCollidingSmartListIDs(t *testing.T) {
	cfg := NewConfig()
	cfg.SavedFilters = []SavedFilter{
		{ID: "shared", Name: "Existing"},
	}
	cfg.SmartLists = []smartList{
		{ID: "shared", Name: "Colliding"},
		{ID: "unique", Name: "Unique"},
		{ID: "shared", Name: "AnotherCollision"},
	}

	if !cfg.MigrateLegacyFilters() {
		t.Fatal("expected migration to run")
	}

	ids := make(map[string]int)
	for _, f := range cfg.SavedFilters {
		ids[f.ID]++
	}
	for id, count := range ids {
		if count > 1 {
			t.Errorf("duplicate ID %q after migration (count=%d)", id, count)
		}
	}
	if len(cfg.SavedFilters) != 4 {
		t.Fatalf("expected 4 filters after migration, got %d", len(cfg.SavedFilters))
	}
	// The pre-existing "Existing" entry must keep its "shared" ID.
	if got := cfg.GetSavedFilter("shared"); got == nil || got.Name != "Existing" {
		t.Errorf("expected pre-existing 'shared' entry preserved, got %+v", got)
	}
}

// TestConfigManager_LoadClampsDashboardBottomLayoutOutOfRange guarantees that
// an out-of-range stored DashboardBottomLayout (e.g., from a downgrade or a
// hand-edited config) is clamped to a valid value at Load time so the bogus
// value doesn't keep round-tripping through every Save.
func TestConfigManager_LoadClampsDashboardBottomLayoutOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                 1,
		"data_dir":                "/tmp/tt-test",
		"projects":                []any{},
		"dashboard_bottom_layout": 99,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DashboardBottomLayout == nil {
		t.Fatal("DashboardBottomLayout should not be nil after clamp")
	}
	if *cfg.DashboardBottomLayout != int(DashboardLayoutHorizontal) {
		t.Errorf("expected DashboardBottomLayout clamped to %d, got %d",
			int(DashboardLayoutHorizontal), *cfg.DashboardBottomLayout)
	}
}

// TestConfigManager_LoadClampsNegativeDashboardBottomLayout mirrors the
// out-of-range test for negative values.
func TestConfigManager_LoadClampsNegativeDashboardBottomLayout(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                 1,
		"data_dir":                "/tmp/tt-test",
		"projects":                []any{},
		"dashboard_bottom_layout": -3,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DashboardBottomLayout == nil || *cfg.DashboardBottomLayout != 0 {
		t.Errorf("expected DashboardBottomLayout clamped to 0, got %v", cfg.DashboardBottomLayout)
	}
}

// TestConfigManager_LoadKeepsValidDashboardBottomLayout ensures the clamp
// does not perturb a legitimate stored value.
func TestConfigManager_LoadKeepsValidDashboardBottomLayout(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                 1,
		"data_dir":                "/tmp/tt-test",
		"projects":                []any{},
		"dashboard_bottom_layout": int(DashboardLayoutVertical),
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DashboardBottomLayout == nil || *cfg.DashboardBottomLayout != int(DashboardLayoutVertical) {
		t.Errorf("expected DashboardBottomLayout preserved as %d, got %v",
			int(DashboardLayoutVertical), cfg.DashboardBottomLayout)
	}
}

// TestConfigManager_LoadClampsInfoPanelModeOutOfRange guarantees that a bogus
// stored InfoPanelMode is clamped at Load time.
func TestConfigManager_LoadClampsInfoPanelModeOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":         1,
		"data_dir":        "/tmp/tt-test",
		"projects":        []any{},
		"info_panel_mode": 77,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.InfoPanelMode == nil || *cfg.InfoPanelMode != int(InfoPanelInfoCards) {
		t.Errorf("expected InfoPanelMode clamped to %d, got %v",
			int(InfoPanelInfoCards), cfg.InfoPanelMode)
	}
}

// TestConfigManager_LoadClampedValueRoundTrips guarantees the clamped value
// actually persists through a Save round-trip so the bogus value no longer
// resurrects on subsequent reads.
func TestConfigManager_LoadClampedValueRoundTrips(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                 1,
		"data_dir":                "/tmp/tt-test",
		"projects":                []any{},
		"dashboard_bottom_layout": 42,
		"info_panel_mode":         42,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if saveErr := manager.Save(cfg); saveErr != nil {
		t.Fatalf("Save failed: %v", saveErr)
	}
	reloaded, err := manager.Load()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if reloaded.DashboardBottomLayout == nil || *reloaded.DashboardBottomLayout != 0 {
		t.Errorf("expected DashboardBottomLayout = 0 after round-trip, got %v",
			reloaded.DashboardBottomLayout)
	}
	if reloaded.InfoPanelMode == nil || *reloaded.InfoPanelMode != int(InfoPanelInfoCards) {
		t.Errorf("expected InfoPanelMode = %d after round-trip, got %v",
			int(InfoPanelInfoCards), reloaded.InfoPanelMode)
	}
}

// TestProjectAccessors_NilEntriesAreSafe guarantees that every public project
// accessor/mutator on Config tolerates a stray nil project entry without
// panicking. Load() filters nil entries on disk, but in-memory mutations
// (tests, future code paths) can still introduce them — surviving callers
// must treat the nil as "skip" rather than crashing the process.
func TestProjectAccessors_NilEntriesAreSafe(t *testing.T) {
	cases := []struct {
		name string
		fn   func(c *Config)
	}{
		{"RemoveProject existing", func(c *Config) { c.RemoveProject("p1") }},
		{"RemoveProject empty id", func(c *Config) { c.RemoveProject("") }},
		{"GetSandboxProject", func(c *Config) { _ = c.GetSandboxProject() }},
		{"GetActionInboxProject", func(c *Config) { _ = c.GetActionInboxProject() }},
		{"EnsureSandboxProject", func(c *Config) { _ = c.EnsureSandboxProject() }},
		{"EnsureActionInboxProject", func(c *Config) { _, _ = c.EnsureActionInboxProject() }},
		{"GetActiveProjects", func(c *Config) { _ = c.GetActiveProjects() }},
		{"GetArchivedProjects", func(c *Config) { _ = c.GetArchivedProjects() }},
		{"GetAllLoadableProjects", func(c *Config) { _ = c.GetAllLoadableProjects() }},
		{"GetAllContexts", func(c *Config) { _ = c.GetAllContexts() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.Projects = []*models.Project{
				nil,
				{ID: "p1", Slug: "alpha", Name: "Alpha", Context: "work"},
				nil,
			}
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s panicked on nil project entry: %v", tc.name, r)
				}
			}()
			tc.fn(cfg)
		})
	}
}

// TestRemoveProject_NilEntriesDoNotBlockRemoval guarantees RemoveProject still
// finds and removes a real project even when nil placeholders sit before it
// in the slice.
func TestRemoveProject_NilEntriesDoNotBlockRemoval(t *testing.T) {
	cfg := NewConfig()
	cfg.Projects = []*models.Project{
		nil,
		{ID: "p1", Slug: "alpha", Name: "Alpha"},
	}
	if !cfg.RemoveProject("p1") {
		t.Fatal("RemoveProject should return true for p1 past nil entry")
	}
	// The nil entry should still be present (RemoveProject only drops the match).
	if len(cfg.Projects) != 1 || cfg.Projects[0] != nil {
		t.Errorf("expected single surviving nil entry, got %+v", cfg.Projects)
	}
}

// TestEnsureSandboxProject_BackToBackUniqueIDs guarantees that creating,
// deleting, and recreating a sandbox project within the same wall-clock
// second yields a distinct project ID. The old timestamp-only format
// produced colliding IDs.
func TestEnsureSandboxProject_BackToBackUniqueIDs(t *testing.T) {
	cfg := NewConfig()
	if !cfg.EnsureSandboxProject() {
		t.Fatal("first EnsureSandboxProject should create a sandbox")
	}
	first := cfg.GetSandboxProject()
	if first == nil {
		t.Fatal("expected sandbox project after first ensure")
	}
	id1 := first.ID
	if !cfg.RemoveProject(id1) {
		t.Fatalf("RemoveProject(%q) returned false", id1)
	}
	if !cfg.EnsureSandboxProject() {
		t.Fatal("second EnsureSandboxProject should create a fresh sandbox")
	}
	second := cfg.GetSandboxProject()
	if second == nil {
		t.Fatal("expected sandbox project after second ensure")
	}
	if second.ID == id1 {
		t.Errorf("expected distinct sandbox IDs across back-to-back recreations, got %q twice", id1)
	}
}

// TestEnsureActionInboxProject_BackToBackUniqueIDs mirrors the sandbox test
// for the action-inbox internal project.
func TestEnsureActionInboxProject_BackToBackUniqueIDs(t *testing.T) {
	cfg := NewConfig()
	first, created := cfg.EnsureActionInboxProject()
	if !created || first == nil {
		t.Fatal("first EnsureActionInboxProject should create the project")
	}
	id1 := first.ID
	if !cfg.RemoveProject(id1) {
		t.Fatalf("RemoveProject(%q) returned false", id1)
	}
	second, created2 := cfg.EnsureActionInboxProject()
	if !created2 || second == nil {
		t.Fatal("second EnsureActionInboxProject should create a fresh project")
	}
	if second.ID == id1 {
		t.Errorf("expected distinct action-inbox IDs across back-to-back recreations, got %q twice", id1)
	}
}

// TestGetProject_NilEntryAndEmptyQueryAreSafe guarantees that GetProject does
// not panic when called with an empty key or when a stray nil project entry
// is present in c.Projects (e.g. an in-memory mutation before Load scrubs).
func TestGetProject_NilEntryAndEmptyQueryAreSafe(t *testing.T) {
	cfg := NewConfig()
	cfg.Projects = []*models.Project{nil, {ID: "p1", Slug: "alpha", Name: "Alpha"}}

	if got := cfg.GetProject(""); got != nil {
		t.Errorf("expected nil for empty query, got %+v", got)
	}
	if got := cfg.GetProject("p1"); got == nil || got.Slug != "alpha" {
		t.Errorf("expected to resolve p1 past nil entry, got %+v", got)
	}
	if got := cfg.GetProject("alpha"); got == nil || got.ID != "p1" {
		t.Errorf("expected slug lookup to resolve past nil entry, got %+v", got)
	}
}

// TestPruneOrphanedProjectRefs_DropsStaleKeysAcrossSurfaces guarantees that
// every persisted project-keyed map/slice has its dangling entries dropped so
// they don't survive a Save round-trip.
func TestPruneOrphanedProjectRefs_DropsStaleKeysAcrossSurfaces(t *testing.T) {
	cfg := NewConfig()
	cfg.Projects = []*models.Project{{ID: "live", Slug: "live", Name: "Live"}}

	cfg.ProjectScoringProfile = map[string]string{"live": "ice", "ghost": "wsjf"}
	cfg.ProjectViewStates = map[string]ProjectViewState{"live": {}, "ghost": {}}
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{"live": {}, "ghost": {}}
	cfg.FocusQueue = []FocusQueueItem{{TaskID: "t1", ProjectID: "live"}, {TaskID: "t2", ProjectID: "ghost"}}
	cfg.DailyPlan = []DailyPlanItem{{TaskID: "t1", ProjectID: "live"}, {TaskID: "t3", ProjectID: "ghost"}}
	cfg.RecentTaskVisits = []RecentTaskVisit{{TaskID: "t1", ProjectID: "live"}, {TaskID: "t4", ProjectID: "ghost"}}
	cfg.CycleProjects = []string{"live", "ghost"}
	cfg.ContextCycleProjects = map[string][]string{"work": {"live", "ghost"}}
	cfg.CustomContexts = map[string][]string{"team": {"live", "ghost"}}
	cfg.ActionInboxProjectID = "ghost"

	if !cfg.PruneOrphanedProjectRefs() {
		t.Fatal("expected PruneOrphanedProjectRefs to report changes")
	}

	if _, ok := cfg.ProjectScoringProfile["ghost"]; ok {
		t.Error("expected ghost dropped from ProjectScoringProfile")
	}
	if _, ok := cfg.ProjectViewStates["ghost"]; ok {
		t.Error("expected ghost dropped from ProjectViewStates")
	}
	if _, ok := cfg.WorkflowEvalStates["ghost"]; ok {
		t.Error("expected ghost dropped from WorkflowEvalStates")
	}
	for _, item := range cfg.FocusQueue {
		if item.ProjectID == "ghost" {
			t.Error("expected ghost dropped from FocusQueue")
		}
	}
	for _, item := range cfg.DailyPlan {
		if item.ProjectID == "ghost" {
			t.Error("expected ghost dropped from DailyPlan")
		}
	}
	for _, v := range cfg.RecentTaskVisits {
		if v.ProjectID == "ghost" {
			t.Error("expected ghost dropped from RecentTaskVisits")
		}
	}
	for _, pid := range cfg.CycleProjects {
		if pid == "ghost" {
			t.Error("expected ghost dropped from CycleProjects")
		}
	}
	for _, pid := range cfg.ContextCycleProjects["work"] {
		if pid == "ghost" {
			t.Error("expected ghost dropped from ContextCycleProjects[work]")
		}
	}
	for _, pid := range cfg.CustomContexts["team"] {
		if pid == "ghost" {
			t.Error("expected ghost dropped from CustomContexts[team]")
		}
	}
	if cfg.ActionInboxProjectID != "" {
		t.Errorf("expected ActionInboxProjectID cleared, got %q", cfg.ActionInboxProjectID)
	}
	// Live references must be preserved.
	if cfg.ProjectScoringProfile["live"] != "ice" {
		t.Error("expected live scoring profile assignment preserved")
	}
	if len(cfg.FocusQueue) != 1 || cfg.FocusQueue[0].ProjectID != "live" {
		t.Errorf("expected single live FocusQueue entry, got %+v", cfg.FocusQueue)
	}
}

// TestPruneOrphanedProjectRefs_NoopWhenNothingStale guarantees the helper
// reports false when every reference is live, so callers don't trigger
// unnecessary saves.
func TestPruneOrphanedProjectRefs_NoopWhenNothingStale(t *testing.T) {
	cfg := NewConfig()
	cfg.Projects = []*models.Project{{ID: "p1", Slug: "p1", Name: "P1"}}
	cfg.ProjectScoringProfile = map[string]string{"p1": "ice"}
	cfg.FocusQueue = []FocusQueueItem{{TaskID: "t", ProjectID: "p1"}}
	if cfg.PruneOrphanedProjectRefs() {
		t.Fatal("expected no changes when all refs are live")
	}
}

// TestPruneOrphanedProjectRefs_NormalizesLegacySlugRefs guarantees that
// slug-based project references from older or hand-edited configs are not
// mistaken for orphans. They should be rewritten to canonical IDs so the next
// Save round-trip does not preserve the legacy slug form.
func TestPruneOrphanedProjectRefs_NormalizesLegacySlugRefs(t *testing.T) {
	cfg := NewConfig()
	cfg.Projects = []*models.Project{{ID: "p1", Slug: "alpha", Name: "Alpha"}}
	cfg.ProjectScoringProfile = map[string]string{"alpha": "ice"}
	cfg.ProjectViewStates = map[string]ProjectViewState{"alpha": {}}
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{"alpha": {}}
	cfg.FocusQueue = []FocusQueueItem{{TaskID: "t1", ProjectID: "alpha"}}
	cfg.DailyPlan = []DailyPlanItem{{TaskID: "t2", ProjectID: "alpha"}}
	cfg.RecentTaskVisits = []RecentTaskVisit{{TaskID: "t3", ProjectID: "alpha"}}
	cfg.CycleProjects = []string{"alpha"}
	cfg.ContextCycleProjects = map[string][]string{"work": {"alpha"}}
	cfg.CustomContexts = map[string][]string{"team": {"alpha"}}
	cfg.ActionInboxProjectID = "alpha"

	if !cfg.PruneOrphanedProjectRefs() {
		t.Fatal("expected slug refs to be normalized")
	}

	if got := cfg.ProjectScoringProfile["p1"]; got != "ice" {
		t.Errorf("expected scoring profile moved to p1, got %q", got)
	}
	if _, ok := cfg.ProjectScoringProfile["alpha"]; ok {
		t.Error("expected legacy slug key removed from ProjectScoringProfile")
	}
	if _, ok := cfg.ProjectViewStates["p1"]; !ok {
		t.Error("expected ProjectViewStates key normalized to p1")
	}
	if _, ok := cfg.WorkflowEvalStates["p1"]; !ok {
		t.Error("expected WorkflowEvalStates key normalized to p1")
	}
	if cfg.FocusQueue[0].ProjectID != "p1" {
		t.Errorf("expected FocusQueue project normalized to p1, got %q", cfg.FocusQueue[0].ProjectID)
	}
	if cfg.DailyPlan[0].ProjectID != "p1" {
		t.Errorf("expected DailyPlan project normalized to p1, got %q", cfg.DailyPlan[0].ProjectID)
	}
	if cfg.RecentTaskVisits[0].ProjectID != "p1" {
		t.Errorf("expected RecentTaskVisits project normalized to p1, got %q", cfg.RecentTaskVisits[0].ProjectID)
	}
	if len(cfg.CycleProjects) != 1 || cfg.CycleProjects[0] != "p1" {
		t.Errorf("expected CycleProjects normalized to [p1], got %+v", cfg.CycleProjects)
	}
	if got := cfg.ContextCycleProjects["work"]; len(got) != 1 || got[0] != "p1" {
		t.Errorf("expected ContextCycleProjects normalized to [p1], got %+v", got)
	}
	if got := cfg.CustomContexts["team"]; len(got) != 1 || got[0] != "p1" {
		t.Errorf("expected CustomContexts normalized to [p1], got %+v", got)
	}
	if cfg.ActionInboxProjectID != "p1" {
		t.Errorf("expected ActionInboxProjectID normalized to p1, got %q", cfg.ActionInboxProjectID)
	}
}

// TestConfigManager_LoadPrunesOrphanedProjectRefs guarantees the Load-time
// integration drops dangling references rather than letting them resurrect
// across reloads.
func TestConfigManager_LoadPrunesOrphanedProjectRefs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":  1,
		"data_dir": tmpDir,
		"projects": []map[string]any{
			{"id": "p1", "slug": "alpha", "name": "Alpha", "path": "/tmp/alpha"},
		},
		"project_scoring_profile": map[string]string{"p1": "ice", "missing": "wsjf"},
		"focus_queue": []map[string]any{
			{"task_id": "t1", "project_id": "p1"},
			{"task_id": "t2", "project_id": "missing"},
		},
		"cycle_projects":          []string{"p1", "missing"},
		"action_inbox_project_id": "missing",
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if _, ok := cfg.ProjectScoringProfile["missing"]; ok {
		t.Error("expected missing scoring profile dropped at Load")
	}
	if len(cfg.FocusQueue) != 1 || cfg.FocusQueue[0].ProjectID != "p1" {
		t.Errorf("expected single live FocusQueue entry, got %+v", cfg.FocusQueue)
	}
	for _, pid := range cfg.CycleProjects {
		if pid == "missing" {
			t.Error("expected missing dropped from CycleProjects at Load")
		}
	}
	if cfg.ActionInboxProjectID != "" {
		t.Errorf("expected ActionInboxProjectID cleared at Load, got %q", cfg.ActionInboxProjectID)
	}
}

// TestConfigManager_LoadClampsOutOfRangeTimeoutsAndDays guarantees that
// out-of-range stored values for fzf_timeout, popup_timeout_seconds,
// alert_daily_review_hour, alert_weekly_review_day, and the dashboard split
// percentages are clamped at Load so a bogus stored value doesn't keep
// round-tripping through every Save. Each respective getter clamps on read,
// but without the Load-time clamp the bogus stored value resurrects on every
// reload.
func TestConfigManager_LoadClampsOutOfRangeTimeoutsAndDays(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                        1,
		"data_dir":                       "/tmp/tt-test",
		"projects":                       []any{},
		"fzf_timeout":                    -5,
		"popup_timeout_seconds":          -10,
		"alert_daily_review_hour":        99,
		"alert_weekly_review_day":        42,
		"dashboard_horizontal_split_pct": 5,   // below min
		"dashboard_vertical_split_pct":   200, // above max
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.FzfTimeout == nil || *cfg.FzfTimeout != 1 {
		t.Errorf("expected FzfTimeout clamped to 1, got %v", cfg.FzfTimeout)
	}
	if cfg.PopupTimeoutSeconds == nil || *cfg.PopupTimeoutSeconds != 0 {
		t.Errorf("expected PopupTimeoutSeconds clamped to 0, got %v", cfg.PopupTimeoutSeconds)
	}
	if cfg.AlertDailyReviewHour == nil || *cfg.AlertDailyReviewHour != DefaultAlertDailyReviewHour {
		t.Errorf("expected AlertDailyReviewHour clamped to %d, got %v", DefaultAlertDailyReviewHour, cfg.AlertDailyReviewHour)
	}
	if cfg.AlertWeeklyReviewDay == nil || *cfg.AlertWeeklyReviewDay != DefaultAlertWeeklyReviewDay {
		t.Errorf("expected AlertWeeklyReviewDay clamped to %d, got %v", DefaultAlertWeeklyReviewDay, cfg.AlertWeeklyReviewDay)
	}
	if cfg.DashboardHorizontalSplitPct == nil || *cfg.DashboardHorizontalSplitPct != MinDashboardSplitPct {
		t.Errorf("expected DashboardHorizontalSplitPct clamped to %d, got %v",
			MinDashboardSplitPct, cfg.DashboardHorizontalSplitPct)
	}
	if cfg.DashboardVerticalSplitPct == nil || *cfg.DashboardVerticalSplitPct != MaxDashboardSplitPct {
		t.Errorf("expected DashboardVerticalSplitPct clamped to %d, got %v",
			MaxDashboardSplitPct, cfg.DashboardVerticalSplitPct)
	}

	// Verify the clamped values persist through Save round-trip.
	if saveErr := manager.Save(cfg); saveErr != nil {
		t.Fatalf("Save failed: %v", saveErr)
	}
	reloaded, err := manager.Load()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if reloaded.FzfTimeout == nil || *reloaded.FzfTimeout != 1 {
		t.Errorf("expected FzfTimeout = 1 after round-trip, got %v", reloaded.FzfTimeout)
	}
	if reloaded.AlertDailyReviewHour == nil || *reloaded.AlertDailyReviewHour != DefaultAlertDailyReviewHour {
		t.Errorf("expected AlertDailyReviewHour persisted after round-trip, got %v", reloaded.AlertDailyReviewHour)
	}
}
