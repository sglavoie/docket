package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// makePresetTestModel creates a SettingsModel with a temp config dir and
// a preset dark theme with known colors so assertions are deterministic.
func makePresetTestModel(t *testing.T) (*SettingsModel, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "tt.json")

	dark := config.Theme{
		BackgroundColor: "#111111",
		CursorColor:     "#9900ff",
		CursorTextColor: "#ffffff",
		Palette: config.PaletteColors{
			NormalText: "#eeeeee",
			Primary:    "#0000cc",
			Accent:     "#dd6600",
		},
	}
	cfg := &config.Config{
		ActiveTheme: "dark",
		ThemeDark:   dark,
		ThemeLight:  config.DefaultLightTheme(),
	}
	mgr := config.NewManager(cfgPath)
	return NewSettingsModel(cfg, mgr), dir
}

// TestPresetSave_WritesFile verifies that openPresetSave + execPresetSave creates
// the theme file in the themes directory.
func TestPresetSave_WritesFile(t *testing.T) {
	m, dir := makePresetTestModel(t)
	m.tabs.Active = paletteTabIndex

	// Open save mode.
	m.openPresetSave()
	if m.themePresetMode != presetModeSave {
		t.Fatalf("mode = %d, want presetModeSave", m.themePresetMode)
	}

	// Execute save with a name.
	cmd := m.execPresetSave("my-preset")
	if cmd == nil {
		t.Fatal("execPresetSave returned nil cmd")
	}

	// Run the cmd and check for success message.
	msg := cmd()
	if _, ok := msg.(presetSavedMsg); !ok {
		t.Fatalf("cmd() = %T, want presetSavedMsg", msg)
	}

	// Verify file exists.
	themePath := filepath.Join(dir, "themes", "my-preset.json")
	if _, err := os.Stat(themePath); err != nil {
		t.Fatalf("theme file not found at %s: %v", themePath, err)
	}
}

// TestPresetLoad_NoPresets verifies openPresetLoad sets themePresetErr and stays
// in presetModeNone when no preset files exist.
func TestPresetLoad_NoPresets(t *testing.T) {
	m, _ := makePresetTestModel(t)
	m.tabs.Active = paletteTabIndex

	m.openPresetLoad()

	if m.themePresetMode != presetModeNone {
		t.Fatalf("mode = %d, want presetModeNone (no presets)", m.themePresetMode)
	}
	if m.themePresetErr == "" {
		t.Fatal("expected themePresetErr to be set when no presets exist")
	}
}

// TestPresetLoad_UpdatesInputs verifies that executing a load repopulates
// the palette inputs from the loaded ThemeFile.
func TestPresetLoad_UpdatesInputs(t *testing.T) {
	m, _ := makePresetTestModel(t)
	m.tabs.Active = paletteTabIndex

	// Write a preset manually.
	presetTheme := config.Theme{
		BackgroundColor: "#aabbcc",
		CursorColor:     "#112233",
		CursorTextColor: "#ffffff",
		Palette: config.PaletteColors{
			NormalText: "#123456",
			Primary:    "#abcdef",
		},
	}
	tf := config.NewThemeFile("test-preset", "dark", "dev", presetTheme)
	if err := m.configManager.WriteTheme(tf); err != nil {
		t.Fatalf("WriteTheme: %v", err)
	}

	// Open load mode — should find the preset.
	m.openPresetLoad()
	if m.themePresetMode != presetModeLoad {
		t.Fatalf("mode = %d, want presetModeLoad", m.themePresetMode)
	}
	if len(m.themePresetList) != 1 {
		t.Fatalf("themePresetList len = %d, want 1", len(m.themePresetList))
	}

	// Execute load.
	cmd := m.execPresetLoad()
	if cmd == nil {
		t.Fatal("execPresetLoad returned nil cmd")
	}

	// Verify inputs updated.
	if got := m.bgColorInput.Value(); got != "#aabbcc" {
		t.Errorf("bgColorInput = %q, want #aabbcc", got)
	}
	if got := m.normalTextInput.Value(); got != "#123456" {
		t.Errorf("normalTextInput = %q, want #123456", got)
	}
	if got := m.primaryInput.Value(); got != "#abcdef" {
		t.Errorf("primaryInput = %q, want #abcdef", got)
	}

	// Run the cmd and check for success message.
	msg := cmd()
	if _, ok := msg.(presetLoadedMsg); !ok {
		t.Fatalf("cmd() = %T, want presetLoadedMsg", msg)
	}
}

// TestPresetReset_ClearsInputs verifies that execPresetReset blanks all palette inputs.
func TestPresetReset_ClearsInputs(t *testing.T) {
	m, _ := makePresetTestModel(t)
	m.tabs.Active = paletteTabIndex

	// Confirm there's something to reset.
	if m.bgColorInput.Value() == "" && m.normalTextInput.Value() == "" {
		t.Skip("model already blank — nothing to verify cleared")
	}

	// Execute reset.
	cmd := m.execPresetReset()
	if cmd == nil {
		t.Fatal("execPresetReset returned nil cmd")
	}

	// All palette inputs must be blank.
	blanks := map[string]string{
		"bgColor":     m.bgColorInput.Value(),
		"cursorColor": m.cursorColorInput.Value(),
		"cursorText":  m.cursorTextColorInput.Value(),
		"normalText":  m.normalTextInput.Value(),
		"muted":       m.mutedInput.Value(),
		"primary":     m.primaryInput.Value(),
		"secondary":   m.secondaryInput.Value(),
		"accent":      m.accentInput.Value(),
		"success":     m.successInput.Value(),
		"warning":     m.warningInput.Value(),
		"error":       m.errorColorInput.Value(),
		"border":      m.borderInput.Value(),
	}
	for field, val := range blanks {
		if val != "" {
			t.Errorf("%s = %q after reset, want empty", field, val)
		}
	}

	// Run cmd and check for success.
	msg := cmd()
	if _, ok := msg.(presetResetMsg); !ok {
		t.Fatalf("cmd() = %T, want presetResetMsg", msg)
	}
}

// TestPresetSaveKey_EnterSubmits verifies that pressing enter with a non-empty
// name in save mode closes the overlay and returns a save cmd.
func TestPresetSaveKey_EnterSubmits(t *testing.T) {
	m, _ := makePresetTestModel(t)
	m.tabs.Active = paletteTabIndex
	m.openPresetSave()
	m.themePresetInput.SetValue("demo")

	handled, cmd := m.handlePresetSaveKey(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if !handled {
		t.Fatal("expected handled=true for enter in save mode")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd on enter submit")
	}
	if m.themePresetMode != presetModeNone {
		t.Fatalf("mode = %d after save submit, want presetModeNone", m.themePresetMode)
	}
}

// TestPresetSaveKey_EmptyNameRejects verifies that an empty name shows an error
// and keeps save mode active.
func TestPresetSaveKey_EmptyNameRejects(t *testing.T) {
	m, _ := makePresetTestModel(t)
	m.tabs.Active = paletteTabIndex
	m.openPresetSave()
	m.themePresetInput.SetValue("")

	handled, cmd := m.handlePresetSaveKey(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if !handled {
		t.Fatal("expected handled=true")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd for empty name")
	}
	if m.themePresetErr == "" {
		t.Fatal("expected error for empty name")
	}
	if m.themePresetMode != presetModeSave {
		t.Fatalf("mode = %d, want presetModeSave", m.themePresetMode)
	}
}

// TestPresetResetKey_Confirm verifies y/Y confirms the reset.
func TestPresetResetKey_Confirm(t *testing.T) {
	m, _ := makePresetTestModel(t)
	m.tabs.Active = paletteTabIndex
	m.themePresetMode = presetModeResetConfirm

	handled, cmd := m.handlePresetResetKey("y")
	if !handled {
		t.Fatal("expected handled=true for y in reset confirm")
	}
	if cmd == nil {
		t.Fatal("expected cmd on confirm")
	}
	if m.themePresetMode != presetModeNone {
		t.Fatalf("mode = %d after confirm, want presetModeNone", m.themePresetMode)
	}
}

// TestPresetResetKey_Cancel verifies n/N/esc cancels the reset.
func TestPresetResetKey_Cancel(t *testing.T) {
	for _, key := range []string{"n", "N", "esc"} {
		m, _ := makePresetTestModel(t)
		m.themePresetMode = presetModeResetConfirm

		handled, cmd := m.handlePresetResetKey(key)
		if !handled {
			t.Errorf("key %q: expected handled=true", key)
		}
		if cmd != nil {
			t.Errorf("key %q: expected nil cmd on cancel", key)
		}
		if m.themePresetMode != presetModeNone {
			t.Errorf("key %q: mode = %d, want presetModeNone", key, m.themePresetMode)
		}
	}
}

// TestPresetRoundTrip_SaveExportReimport verifies the full save→load cycle
// produces a model with matching input values.
func TestPresetRoundTrip_SaveExportReimport(t *testing.T) {
	m, _ := makePresetTestModel(t)
	m.tabs.Active = paletteTabIndex

	// Save current inputs as "roundtrip" preset.
	originalBg := m.bgColorInput.Value()
	originalNormal := m.normalTextInput.Value()

	cmd := m.execPresetSave("roundtrip")
	if saveMsg := cmd(); saveMsg == nil {
		t.Fatal("save cmd returned nil msg")
	} else if _, ok := saveMsg.(presetSavedMsg); !ok {
		t.Fatalf("save cmd returned %T, want presetSavedMsg", saveMsg)
	}

	// Modify inputs to verify they get overwritten by load.
	m.bgColorInput.SetValue("#ffffff")
	m.normalTextInput.SetValue("#000000")

	// Load the preset back.
	m.openPresetLoad()
	if m.themePresetMode != presetModeLoad {
		t.Fatalf("mode = %d, want presetModeLoad", m.themePresetMode)
	}

	cmd = m.execPresetLoad()
	if cmd == nil {
		t.Fatal("execPresetLoad returned nil cmd")
	}

	// Inputs must be back to original values.
	if got := m.bgColorInput.Value(); got != originalBg {
		t.Errorf("bgColorInput after load = %q, want %q", got, originalBg)
	}
	if got := m.normalTextInput.Value(); got != originalNormal {
		t.Errorf("normalTextInput after load = %q, want %q", got, originalNormal)
	}
}
