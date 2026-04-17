package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// makeToggleTestModel returns a SettingsModel initialised with explicit dark and
// light themes so toggle tests have deterministic values to assert on.
func makeToggleTestModel(t *testing.T) *SettingsModel {
	t.Helper()
	dark := config.Theme{
		BackgroundColor: "#0d0d0d",
		CursorColor:     "#9900ff",
		CursorTextColor: "#ffffff",
		Palette: config.PaletteColors{
			NormalText:     "#eeeeee",
			Muted:          "#888888",
			Primary:        "#0000cc",
			Secondary:      "#005555",
			Accent:         "#dd6600",
			Success:        "#006600",
			Warning:        "#775500",
			Error:          "#aa0000",
			Border:         "#444444",
			Priority1Title: "#111111",
			Priority2Title: "#330000",
			Priority3Title: "#001133",
			Priority4Title: "#333333",
			Priority5Title: "#555555",
		},
	}
	light := config.DefaultLightTheme()
	cfg := &config.Config{
		ActiveTheme: "dark",
		ThemeDark:   dark,
		ThemeLight:  light,
	}
	mgr := config.NewManager("/tmp/tt_toggle_test.json")
	return NewSettingsModel(cfg, mgr)
}

func TestPaletteToggle_RoundTrip(t *testing.T) {
	m := makeToggleTestModel(t)

	// Initially active = dark.
	if m.activeTheme != "dark" {
		t.Fatalf("initial activeTheme = %q, want dark", m.activeTheme)
	}
	if got := m.normalTextInput.Value(); got != "#eeeeee" {
		t.Fatalf("initial normalText = %q, want #eeeeee", got)
	}

	// Toggle → should show light values.
	m.togglePaletteTheme()
	if m.activeTheme != "light" {
		t.Fatalf("after 1st toggle activeTheme = %q, want light", m.activeTheme)
	}
	wantLightNormal := config.DefaultLightTheme().Palette.NormalText
	if got := m.normalTextInput.Value(); got != wantLightNormal {
		t.Errorf("after 1st toggle normalText = %q, want %q", got, wantLightNormal)
	}

	// Edit accent while in light.
	m.accentInput.SetValue("#ff00ff")

	// Toggle back → dark values should be restored.
	m.togglePaletteTheme()
	if m.activeTheme != "dark" {
		t.Fatalf("after 2nd toggle activeTheme = %q, want dark", m.activeTheme)
	}
	if got := m.normalTextInput.Value(); got != "#eeeeee" {
		t.Errorf("after 2nd toggle normalText = %q, want #eeeeee", got)
	}

	// Toggle to light again → light accent edit must be preserved.
	m.togglePaletteTheme()
	if got := m.accentInput.Value(); got != "#ff00ff" {
		t.Errorf("light accent after re-toggle = %q, want #ff00ff (edit not preserved)", got)
	}
}

func TestPaletteToggle_SavePersistsBoth(t *testing.T) {
	m := makeToggleTestModel(t)

	// Toggle to light and edit NormalText.
	m.togglePaletteTheme()
	m.normalTextInput.SetValue("#123456")

	// saveSettings mutates m.config synchronously before returning the cmd.
	m.saveSettings()

	if m.config.ActiveTheme != "light" {
		t.Errorf("ActiveTheme = %q, want light", m.config.ActiveTheme)
	}
	// Active (light) theme should carry the edited value.
	wantLightNormal := normalizeHexColor("#123456")
	if got := m.config.ThemeLight.Palette.NormalText; got != wantLightNormal {
		t.Errorf("ThemeLight.NormalText = %q, want %q", got, wantLightNormal)
	}
	// Inactive (dark) theme should carry original dark value.
	wantDarkNormal := normalizeHexColor("#eeeeee")
	if got := m.config.ThemeDark.Palette.NormalText; got != wantDarkNormal {
		t.Errorf("ThemeDark.NormalText = %q, want %q", got, wantDarkNormal)
	}
}

func TestPaletteToggle_RenderSelection(t *testing.T) {
	m := makeToggleTestModel(t)
	m.focusIndex = 4

	// Active = dark → Dark must be marked selected.
	m.activeTheme = "dark"
	out := m.renderPaletteColorsSection()
	if !strings.Contains(out, "◉ Dark") {
		t.Errorf("dark active: expected %q in output, got:\n%s", "◉ Dark", out)
	}
	if !strings.Contains(out, "○ Light") {
		t.Errorf("dark active: expected %q in output, got:\n%s", "○ Light", out)
	}

	// Active = light → Light must be marked selected.
	m.activeTheme = "light"
	out = m.renderPaletteColorsSection()
	if !strings.Contains(out, "○ Dark") {
		t.Errorf("light active: expected %q in output, got:\n%s", "○ Dark", out)
	}
	if !strings.Contains(out, "◉ Light") {
		t.Errorf("light active: expected %q in output, got:\n%s", "◉ Light", out)
	}
}

func TestPaletteToggle_CancelReverts(t *testing.T) {
	m := makeToggleTestModel(t)

	// The original theme is snapshotted at init (dark theme, no bg set).
	origBg := m.originalTheme.BgColor

	// Toggle to light and make edits.
	m.togglePaletteTheme()
	m.bgColorInput.SetValue("#ffffff")
	m.accentInput.SetValue("#ff0000")

	// OnExit should call ApplyTheme(m.originalTheme) — verify it doesn't panic
	// and that originalTheme is unchanged by the toggle.
	m.OnExit()

	if m.originalTheme.BgColor != origBg {
		t.Errorf("originalTheme.BgColor mutated by toggle: got %q, want %q",
			m.originalTheme.BgColor, origBg)
	}
}
