package config

import "testing"

// TestRuntimeState_CommitMachinePersistsAndRefreshes proves the machine-local
// commit seam the migrated TUI layout consumers use: a project-specific view
// state and cross-outliner state commit only the machine domain, advance the
// in-memory snapshot so accessors observe the mutation with no extra disk load,
// leave the domains the commit does not own untouched, and survive a fresh reload
// from disk.
func TestRuntimeState_CommitMachinePersistsAndRefreshes(t *testing.T) {
	manager, _ := seededManager(t)

	rs, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}

	// Seed a value in a domain the machine commit must NOT touch.
	planning := rs.Planning()
	planning.StreakCount = 5
	if commitErr := rs.CommitPlanning(planning); commitErr != nil {
		t.Fatalf("CommitPlanning seed: %v", commitErr)
	}

	machine := rs.Machine()
	machine.SetProjectViewState("proj-1", ProjectViewState{OutlinerFilterParked: true, PinnedTagTab: "backend"})
	machine.CrossOutlinerColumnToggles = map[string]bool{"score": false}
	machine.CrossOutlinerPinnedTagTab = "ops"
	machine.SetInfoPanelMode(InfoPanelProjectSummary)
	machine.SetDashboardHorizontalSplitPct(42)
	if commitErr := rs.CommitMachine(machine); commitErr != nil {
		t.Fatalf("CommitMachine: %v", commitErr)
	}

	// The snapshot advanced in-lock-step: accessors return committed values
	// without any render-time disk load.
	if vs, ok := rs.Machine().GetProjectViewState("proj-1"); !ok || !vs.OutlinerFilterParked || vs.PinnedTagTab != "backend" {
		t.Fatalf("snapshot project view state = %+v (ok=%v), want parked+backend", vs, ok)
	}
	if got := rs.Machine().CrossOutlinerPinnedTagTab; got != "ops" {
		t.Errorf("snapshot cross-outliner pinned tab = %q, want ops", got)
	}
	if got := rs.Machine().GetInfoPanelMode(); got != InfoPanelProjectSummary {
		t.Errorf("snapshot info panel mode = %d, want project summary", got)
	}
	if got := rs.Machine().GetDashboardHorizontalSplitPct(); got != 42 {
		t.Errorf("snapshot horizontal split = %d, want 42", got)
	}
	// The planning domain the machine commit did not own survived untouched.
	if got := rs.Planning().StreakCount; got != 5 {
		t.Errorf("planning clobbered by machine commit: StreakCount = %d, want 5", got)
	}

	// A fresh reload from disk agrees, proving the commit persisted rather than
	// only mutating the in-memory snapshot.
	reloaded, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if vs, ok := reloaded.Machine().GetProjectViewState("proj-1"); !ok || !vs.OutlinerFilterParked || vs.PinnedTagTab != "backend" {
		t.Fatalf("reloaded project view state = %+v (ok=%v), want parked+backend", vs, ok)
	}
	if got := reloaded.Machine().CrossOutlinerColumnToggles["score"]; got != false {
		t.Errorf("reloaded cross-outliner toggle = %v, want false", got)
	}
	if got := reloaded.Machine().GetDashboardHorizontalSplitPct(); got != 42 {
		t.Errorf("reloaded horizontal split = %d, want 42", got)
	}
	if got := reloaded.Planning().StreakCount; got != 5 {
		t.Errorf("reloaded StreakCount = %d, want 5", got)
	}
}
