package config

import "testing"

// TestMachineProjectViewStateAccessors proves the MachineState project-view-state
// accessors mirror the config.Config facade: a nil map reads as "absent", a set
// value round-trips by project id, and Set lazily allocates the backing map.
func TestMachineProjectViewStateAccessors(t *testing.T) {
	m := NewMachineState()

	if _, ok := m.GetProjectViewState("proj-1"); ok {
		t.Fatalf("unset project view state should report absent")
	}

	want := ProjectViewState{
		OutlinerFilterParked: true,
		TreePriorityMode:     "all",
		PinnedTagTab:         "backend",
	}
	m.SetProjectViewState("proj-1", want)

	got, ok := m.GetProjectViewState("proj-1")
	if !ok {
		t.Fatalf("project view state should be present after Set")
	}
	if got.OutlinerFilterParked != want.OutlinerFilterParked ||
		got.TreePriorityMode != want.TreePriorityMode ||
		got.PinnedTagTab != want.PinnedTagTab {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, want)
	}
	// A distinct project id is still absent (per-project keying preserved).
	if _, ok := m.GetProjectViewState("proj-2"); ok {
		t.Fatalf("unrelated project should not share view state")
	}
}

// TestMachineInfoPanelAccessors covers the info-panel mode migration and clamping
// exactly as the facade does: nil ShowInfoPanel defaults to info cards, an
// explicit false migrates to off, out-of-range stored values clamp, and Set
// clears the legacy bool.
func TestMachineInfoPanelAccessors(t *testing.T) {
	// nil InfoPanelMode + nil ShowInfoPanel -> info cards (default).
	m := NewMachineState()
	if got := m.GetInfoPanelMode(); got != InfoPanelInfoCards {
		t.Fatalf("default info panel mode = %d, want %d", got, InfoPanelInfoCards)
	}
	if !m.GetShowInfoPanel() {
		t.Fatalf("default GetShowInfoPanel = false, want true")
	}

	// Legacy ShowInfoPanel=false migrates to Off.
	off := false
	m2 := &MachineState{ShowInfoPanel: &off}
	if got := m2.GetInfoPanelMode(); got != InfoPanelOff {
		t.Fatalf("legacy ShowInfoPanel=false -> %d, want %d", got, InfoPanelOff)
	}

	// Out-of-range stored InfoPanelMode clamps to info cards.
	bad := 99
	m3 := &MachineState{InfoPanelMode: &bad}
	if got := m3.GetInfoPanelMode(); got != InfoPanelInfoCards {
		t.Fatalf("out-of-range info panel mode = %d, want %d", got, InfoPanelInfoCards)
	}

	// SetInfoPanelMode stores the value and clears the legacy bool.
	m.SetInfoPanelMode(InfoPanelProjectSummary)
	if got := m.GetInfoPanelMode(); got != InfoPanelProjectSummary {
		t.Fatalf("after Set, mode = %d, want %d", got, InfoPanelProjectSummary)
	}
	if m.ShowInfoPanel != nil {
		t.Fatalf("SetInfoPanelMode must clear legacy ShowInfoPanel")
	}

	// CycleInfoPanelMode wraps through the three modes.
	m.SetInfoPanelMode(InfoPanelOff)
	if got := m.CycleInfoPanelMode(); got != InfoPanelInfoCards {
		t.Fatalf("cycle off -> %d, want %d", got, InfoPanelInfoCards)
	}
	if got := m.CycleInfoPanelMode(); got != InfoPanelProjectSummary {
		t.Fatalf("cycle info cards -> %d, want %d", got, InfoPanelProjectSummary)
	}
	if got := m.CycleInfoPanelMode(); got != InfoPanelOff {
		t.Fatalf("cycle project summary -> %d, want %d", got, InfoPanelOff)
	}
}

// TestMachineDashboardLayoutBounds proves the dashboard layout/split accessors
// default and clamp to the documented bounds, matching the facade so layout
// consumers migrated onto MachineState observe identical behavior.
func TestMachineDashboardLayoutBounds(t *testing.T) {
	m := NewMachineState()

	if got := m.GetDashboardBottomLayout(); got != DashboardLayoutHorizontal {
		t.Fatalf("default layout = %d, want horizontal", got)
	}
	if got := m.GetDashboardHorizontalSplitPct(); got != DefaultDashboardHorizontalSplitPct {
		t.Fatalf("default horizontal split = %d, want %d", got, DefaultDashboardHorizontalSplitPct)
	}
	if got := m.GetDashboardVerticalSplitPct(); got != DefaultDashboardVerticalSplitPct {
		t.Fatalf("default vertical split = %d, want %d", got, DefaultDashboardVerticalSplitPct)
	}

	// Out-of-range layout clamps to horizontal.
	m.SetDashboardBottomLayout(DashboardBottomLayout(99))
	if got := m.GetDashboardBottomLayout(); got != DashboardLayoutHorizontal {
		t.Fatalf("out-of-range layout clamped to %d, want horizontal", got)
	}

	// Splits clamp to [Min, Max].
	m.SetDashboardHorizontalSplitPct(5)
	if got := m.GetDashboardHorizontalSplitPct(); got != MinDashboardSplitPct {
		t.Fatalf("low horizontal split = %d, want %d", got, MinDashboardSplitPct)
	}
	m.SetDashboardVerticalSplitPct(1000)
	if got := m.GetDashboardVerticalSplitPct(); got != MaxDashboardSplitPct {
		t.Fatalf("high vertical split = %d, want %d", got, MaxDashboardSplitPct)
	}

	// Validate rejects an out-of-range stored layout enum.
	bad := 7
	if err := (&MachineState{DashboardBottomLayout: &bad}).Validate(); err == nil {
		t.Fatalf("Validate should reject out-of-range dashboard layout")
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate on clamped state: %v", err)
	}
}
