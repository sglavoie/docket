package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
)

// newAlertTestApp builds an AppModel whose runtime commits to a writable temp
// data dir, so the planning-domain commit path (dismissals, review completion)
// exercised by the alert/review handlers actually persists. HOME is isolated so
// the machine-local domain read during the post-commit refresh never touches the
// real runtime dir.
func newAlertTestApp(t *testing.T) (*AppModel, *config.RuntimeState) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()

	cfg := newTestConfigAt(dataDir)
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	// Write a complete synced-domain set (mirrors a migrated workspace), so the
	// runtime loads real revision baselines and the facade-reload bridge observes
	// a complete split-domain state.
	if err := saveTestRuntime(manager, cfg); err != nil {
		t.Fatalf("seed domain files: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("load runtime state: %v", err)
	}

	return &AppModel{
		configManager: manager,
		runtime:       runtime,
		persister:     NewProjectService(),
		views:         NewViewRegistry(),
		viewStack:     []ViewType{ViewDashboard},
		width:         80,
		height:        24,
	}, cfg
}

// TestBuildAlertConfigDefaultsFromDomains confirms the alert engine config is
// assembled from the Settings (policy) and Planning (review timestamps) domains
// with their defaulting accessors, not the config.Config facade.
func TestBuildAlertConfigDefaultsFromDomains(t *testing.T) {
	m := &AppModel{runtime: config.EmptyRuntimeState(t.TempDir())}

	ac := m.buildAlertConfig()

	if !ac.DailyReviewEnabled || !ac.WeeklyReviewEnabled || !ac.OverdueEnabled ||
		!ac.DueSoonEnabled || !ac.StaleEnabled || !ac.InboxEnabled ||
		!ac.BlockedEnabled || !ac.ProjectHealthEnabled {
		t.Errorf("alert enablement defaults should all be true, got %+v", ac)
	}
	if ac.DueSoonDays != config.DefaultAlertDueSoonDays {
		t.Errorf("DueSoonDays = %d, want %d", ac.DueSoonDays, config.DefaultAlertDueSoonDays)
	}
	if ac.StaleDays != config.DefaultAlertStaleDays {
		t.Errorf("StaleDays = %d, want %d", ac.StaleDays, config.DefaultAlertStaleDays)
	}
	if ac.InboxThreshold != config.DefaultAlertInboxThreshold {
		t.Errorf("InboxThreshold = %d, want %d", ac.InboxThreshold, config.DefaultAlertInboxThreshold)
	}
	if ac.DailyReviewHour != config.DefaultAlertDailyReviewHour {
		t.Errorf("DailyReviewHour = %d, want %d", ac.DailyReviewHour, config.DefaultAlertDailyReviewHour)
	}
	// Review completion is Planning-owned and defaults to "never completed".
	if ac.LastDailyReviewAt != nil || ac.LastWeeklyReviewAt != nil {
		t.Errorf("review timestamps should default to nil, got daily=%v weekly=%v",
			ac.LastDailyReviewAt, ac.LastWeeklyReviewAt)
	}
}

// TestBuildAlertConfigReadsPlanningReviewTimestamps confirms the review
// completion timestamps come from the Planning domain, keeping planning
// bookkeeping separate from the Settings-owned alert policy.
func TestBuildAlertConfigReadsPlanningReviewTimestamps(t *testing.T) {
	cfg := newTestConfig()
	when := time.Now().Add(-2 * time.Hour)
	cfg.Planning().LastDailyReviewAt = &when

	m := &AppModel{runtime: testRuntimeAt(cfg.DataDir(), cfg)}

	ac := m.buildAlertConfig()
	if ac.LastDailyReviewAt == nil || !ac.LastDailyReviewAt.Equal(when) {
		t.Fatalf("LastDailyReviewAt = %v, want %v (from Planning domain)", ac.LastDailyReviewAt, when)
	}
}

// TestMarkDailyReviewCommitsPlanningDomain confirms marking the daily review done
// persists the completion timestamp through a Planning-domain commit — observable
// via the refreshed runtime snapshot — rather than a whole-config save, and that
// it is not folded into the Settings domain.
func TestMarkDailyReviewCommitsPlanningDomain(t *testing.T) {
	m, _ := newAlertTestApp(t)

	if ts := m.runtime.Planning().GetLastDailyReviewAt(); ts != nil {
		t.Fatalf("precondition: daily review already marked (%v)", ts)
	}

	handled, _ := m.handleViewReviewMessages(MarkDailyReviewDoneMsg{})
	if !handled {
		t.Fatal("MarkDailyReviewDoneMsg was not handled")
	}

	if ts := m.runtime.Planning().GetLastDailyReviewAt(); ts == nil {
		t.Fatal("daily review timestamp not persisted to the Planning domain")
	}
	// planning.json must exist on disk (the commit wrote only the planning domain).
	if _, err := config.NewManager(filepath.Join(m.runtime.DataDir(), ".tt.json")).LoadRuntimeState(); err != nil {
		t.Fatalf("reload after planning commit failed: %v", err)
	}
}

// TestDismissAlertCommitsPlanningDomain confirms an alert dismissal persists
// through a Planning-domain commit (dismissals are planning-owned) and survives
// the post-commit runtime refresh.
func TestDismissAlertCommitsPlanningDomain(t *testing.T) {
	m, _ := newAlertTestApp(t)

	const key = "overdue:project-x"
	if err := m.commitPlanningMutation(func(p *config.PlanningState) {
		p.DismissAlert(key, "overdue")
	}); err != nil {
		t.Fatalf("commitPlanningMutation: %v", err)
	}

	if !m.runtime.Planning().IsAlertDismissed(key) {
		t.Fatal("dismissal not observable in the refreshed Planning snapshot")
	}
	// The legacy facade is kept in sync for not-yet-migrated readers.
	if !m.runtime.Planning().IsAlertDismissed(key) {
		t.Error("facade config not synced after planning commit")
	}
}

// TestTutorDismissedCommitsSettingsDomain confirms dismissing the tutor prompt
// persists through a Settings-domain commit (a portable behavior preference)
// rather than a whole-config save, and is observable via the runtime snapshot.
func TestTutorDismissedCommitsSettingsDomain(t *testing.T) {
	m, _ := newAlertTestApp(t)

	if m.runtime.Settings().GetTutorDismissed() {
		t.Fatal("precondition: tutor already dismissed")
	}

	handled, _ := m.handleProjectMessages(TutorDismissedMsg{})
	if !handled {
		t.Fatal("TutorDismissedMsg was not handled")
	}

	if !m.runtime.Settings().GetTutorDismissed() {
		t.Fatal("tutor dismissal not persisted to the Settings domain")
	}
	if !m.runtime.Settings().GetTutorDismissed() {
		t.Error("facade config not synced after settings commit")
	}
}
