package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// failingSettingsCommitter is a SettingsCommitter whose commit always fails, so a
// test can drive the settings editor's failed-checked-commit path without a real
// revision conflict.
type failingSettingsCommitter struct{ err error }

func (f failingSettingsCommitter) CommitSettings(_ *config.Settings, _ *config.MachineState) error {
	return f.err
}

// TestSettingsDefaultsFromDomainRuntime confirms the editor seeds its widgets
// from the settings domain's defaulting accessors — not the config.Config facade
// — when the runtime holds a freshly defaulted snapshot.
func TestSettingsDefaultsFromDomainRuntime(t *testing.T) {
	deps := &shellDeps{Runtime: config.EmptyRuntimeState(t.TempDir())}
	m := NewSettingsModel(deps)

	if got := m.undoDepthInput.Value(); got != "50" {
		t.Errorf("undo depth default = %q, want 50", got)
	}
	if got := m.dueSoonInput.Value(); got != "7" {
		t.Errorf("review due-soon default = %q, want 7", got)
	}
	if got := m.recentActivityInput.Value(); got != "20" {
		t.Errorf("recent activity default = %q, want 20", got)
	}
	if !m.recentActivityShowAll {
		t.Error("recent activity show-all default = false, want true")
	}
	if !m.autoTheme {
		t.Error("auto-theme default = false, want true")
	}
	if m.activeTheme != "dark" {
		t.Errorf("active theme default = %q, want dark", m.activeTheme)
	}
}

// TestSettingsThemeSwitchingReadsDomain confirms toggling the active palette in
// the editor swaps between the two theme slots the settings domain owns, with no
// config.Config involved.
func TestSettingsThemeSwitchingReadsDomain(t *testing.T) {
	cfg := newTestConfig()
	cfg.Settings().ActiveTheme = "dark"
	cfg.Settings().ThemeDark = config.Theme{Palette: config.PaletteColors{NormalText: "#dark11"}}
	cfg.Settings().ThemeLight = config.Theme{Palette: config.PaletteColors{NormalText: "#light1"}}

	deps := &shellDeps{Runtime: testRuntimeAt(cfg.DataDir(), cfg)}
	m := NewSettingsModel(deps)

	if got := m.normalTextInput.Value(); got != "#dark11" {
		t.Fatalf("initial normalText = %q, want #dark11 (dark slot)", got)
	}
	m.togglePaletteTheme()
	if m.activeTheme != "light" {
		t.Fatalf("after toggle activeTheme = %q, want light", m.activeTheme)
	}
	if got := m.normalTextInput.Value(); got != "#light1" {
		t.Fatalf("after toggle normalText = %q, want #light1 (light slot)", got)
	}
}

// TestSettingsSaveFailedCheckedCommit confirms a failing domain commit surfaces a
// TransactionErrorMsg and leaves the runtime snapshot unrefreshed (unchanged).
func TestSettingsSaveFailedCheckedCommit(t *testing.T) {
	cfg := newTestConfig()
	cfg.Settings().ActiveTheme = "dark"
	rs := testRuntimeAt(cfg.DataDir(), cfg)
	deps := &shellDeps{Runtime: rs}
	m := NewSettingsModel(deps)
	// Route the save through a committer that always fails.
	m.coordinator = newSettingsCoordinator(failingSettingsCommitter{err: config.ErrStaleObject})

	m.activeTheme = "light"
	cmd := m.saveSettings()
	if cmd == nil {
		t.Fatal("saveSettings returned nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(TransactionErrorMsg); !ok {
		t.Fatalf("failed commit resolved to %T, want TransactionErrorMsg", msg)
	}
	// A failed commit never refreshes the snapshot, so the committed theme is
	// still the original.
	if got := rs.Settings().ActiveTheme; got != "dark" {
		t.Errorf("runtime ActiveTheme after failed commit = %q, want unchanged dark", got)
	}
}

// TestSettingsSavePostCommitRuntimeRefresh confirms a successful save commits the
// settings + machine domains and refreshes the runtime snapshot so the committed
// value is observable through the domain accessor with no further disk load.
func TestSettingsSavePostCommitRuntimeRefresh(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate machine.json under the local runtime root
	dataDir := t.TempDir()
	cfg := newTestConfigAt(dataDir)

	rs := testRuntimeAt(dataDir, cfg)
	deps := &shellDeps{Runtime: rs}
	m := NewSettingsModel(deps)

	m.undoDepthInput.SetValue("7")
	cmd := m.saveSettings()
	if cmd == nil {
		t.Fatal("saveSettings returned nil cmd")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("saveSettings cmd resolved to nil")
	} else if _, ok := msg.(SettingsSavedMsg); !ok {
		t.Fatalf("successful save resolved to %T, want SettingsSavedMsg", msg)
	}

	got := rs.Settings().UndoHistoryDepth
	if got == nil || *got != 7 {
		t.Fatalf("runtime UndoHistoryDepth after commit = %v, want 7 (snapshot refreshed)", got)
	}
}
