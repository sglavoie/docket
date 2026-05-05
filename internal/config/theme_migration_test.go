package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigratePaletteToThemes_FromLegacy(t *testing.T) {
	cfg := &Config{
		TUIBackgroundColor: "#111111",
		TUICursorColor:     "#222222",
		TUICursorTextColor: "#333333",
		Palette: PaletteColors{
			Primary: "#0000ff",
			Accent:  "#ff0000",
		},
	}

	changed := cfg.MigratePaletteToThemes()
	if !changed {
		t.Fatal("expected MigratePaletteToThemes to return true")
	}

	// Legacy fields must be zeroed.
	if cfg.TUIBackgroundColor != "" {
		t.Errorf("TUIBackgroundColor not zeroed, got %q", cfg.TUIBackgroundColor)
	}
	if cfg.TUICursorColor != "" {
		t.Errorf("TUICursorColor not zeroed, got %q", cfg.TUICursorColor)
	}
	if cfg.TUICursorTextColor != "" {
		t.Errorf("TUICursorTextColor not zeroed, got %q", cfg.TUICursorTextColor)
	}
	if cfg.Palette != (PaletteColors{}) {
		t.Errorf("Palette not zeroed, got %+v", cfg.Palette)
	}

	// ThemeDark must have the legacy values.
	if cfg.ThemeDark.BackgroundColor != "#111111" {
		t.Errorf("ThemeDark.BackgroundColor = %q, want #111111", cfg.ThemeDark.BackgroundColor)
	}
	if cfg.ThemeDark.CursorColor != "#222222" {
		t.Errorf("ThemeDark.CursorColor = %q, want #222222", cfg.ThemeDark.CursorColor)
	}
	if cfg.ThemeDark.CursorTextColor != "#333333" {
		t.Errorf("ThemeDark.CursorTextColor = %q, want #333333", cfg.ThemeDark.CursorTextColor)
	}
	if cfg.ThemeDark.Palette.Primary != "#0000ff" {
		t.Errorf("ThemeDark.Palette.Primary = %q, want #0000ff", cfg.ThemeDark.Palette.Primary)
	}

	// ThemeLight must be curated defaults.
	light := DefaultLightTheme()
	if cfg.ThemeLight.BackgroundColor != light.BackgroundColor {
		t.Errorf("ThemeLight.BackgroundColor = %q, want %q", cfg.ThemeLight.BackgroundColor, light.BackgroundColor)
	}
	if cfg.ThemeLight.Palette.Primary != light.Palette.Primary {
		t.Errorf("ThemeLight.Palette.Primary = %q, want %q", cfg.ThemeLight.Palette.Primary, light.Palette.Primary)
	}

	// ActiveTheme must be "dark".
	if cfg.ActiveTheme != "dark" {
		t.Errorf("ActiveTheme = %q, want dark", cfg.ActiveTheme)
	}
}

func TestMigratePaletteToThemes_Idempotent(t *testing.T) {
	cfg := &Config{
		TUIBackgroundColor: "#111111",
		Palette:            PaletteColors{Primary: "#0000ff"},
	}

	first := cfg.MigratePaletteToThemes()
	if !first {
		t.Fatal("first call should return true")
	}
	snapshot := *cfg

	second := cfg.MigratePaletteToThemes()
	if second {
		t.Error("second call should return false (idempotent)")
	}

	// State must be unchanged after second call.
	if cfg.ThemeDark != snapshot.ThemeDark {
		t.Error("ThemeDark changed on second call")
	}
	if cfg.ThemeLight != snapshot.ThemeLight {
		t.Error("ThemeLight changed on second call")
	}
	if cfg.ActiveTheme != snapshot.ActiveTheme {
		t.Error("ActiveTheme changed on second call")
	}
}

func TestActiveThemeConfig(t *testing.T) {
	dark := Theme{BackgroundColor: "#000000", Palette: PaletteColors{Primary: "#aaaaaa"}}
	light := DefaultLightTheme()

	cfg := &Config{
		ThemeDark:  dark,
		ThemeLight: light,
	}

	// Empty ActiveTheme → dark.
	got := cfg.ActiveThemeConfig()
	if got.BackgroundColor != dark.BackgroundColor {
		t.Errorf("empty ActiveTheme: got bg %q, want %q", got.BackgroundColor, dark.BackgroundColor)
	}

	// Explicit "dark".
	cfg.ActiveTheme = "dark"
	got = cfg.ActiveThemeConfig()
	if got != dark {
		t.Errorf("dark: got %+v, want %+v", got, dark)
	}

	// Explicit "light".
	cfg.ActiveTheme = "light"
	got = cfg.ActiveThemeConfig()
	if got.BackgroundColor != light.BackgroundColor {
		t.Errorf("light: got bg %q, want %q", got.BackgroundColor, light.BackgroundColor)
	}
}

func TestMigratePaletteToThemes_FreshConfig_SetsRev(t *testing.T) {
	cfg := &Config{}

	changed := cfg.MigratePaletteToThemes()
	if !changed {
		t.Fatal("expected changed=true for fresh config")
	}

	light := DefaultLightTheme()
	if cfg.ThemeLight.BackgroundColor != light.BackgroundColor {
		t.Errorf("ThemeLight.BackgroundColor = %q, want %q", cfg.ThemeLight.BackgroundColor, light.BackgroundColor)
	}
	if cfg.ThemeLightRev != LightThemeDefaultsRev {
		t.Errorf("ThemeLightRev = %d, want %d", cfg.ThemeLightRev, LightThemeDefaultsRev)
	}
}

func TestMigratePaletteToThemes_StaleRev_ForceRefresh(t *testing.T) {
	customTheme := Theme{BackgroundColor: "#aabbcc", Palette: PaletteColors{Primary: "#001122"}}
	cfg := &Config{
		ThemeLight:    customTheme,
		ThemeLightRev: 0, // stale
		ActiveTheme:   "light",
	}

	changed := cfg.MigratePaletteToThemes()
	if !changed {
		t.Fatal("expected changed=true when ThemeLightRev is stale")
	}

	// ThemeLight must be overwritten with fresh defaults.
	light := DefaultLightTheme()
	if cfg.ThemeLight.BackgroundColor != light.BackgroundColor {
		t.Errorf("ThemeLight.BackgroundColor = %q, want %q", cfg.ThemeLight.BackgroundColor, light.BackgroundColor)
	}
	if cfg.ThemeLightRev != LightThemeDefaultsRev {
		t.Errorf("ThemeLightRev = %d, want %d", cfg.ThemeLightRev, LightThemeDefaultsRev)
	}
}

func TestMigratePaletteToThemes_CurrentRev_NoOp(t *testing.T) {
	light := DefaultLightTheme()
	cfg := &Config{
		ThemeLight:    light,
		ThemeLightRev: LightThemeDefaultsRev,
		ThemeDark:     Theme{BackgroundColor: "#000000"},
		ThemeDarkRev:  DarkThemeDefaultsRev,
		ActiveTheme:   "dark",
	}
	snapshot := *cfg

	changed := cfg.MigratePaletteToThemes()
	if changed {
		t.Error("expected changed=false when ThemeLightRev is current")
	}
	if cfg.ThemeLight != snapshot.ThemeLight {
		t.Error("ThemeLight must not be modified when rev is current")
	}
	if cfg.ThemeLightRev != snapshot.ThemeLightRev {
		t.Errorf("ThemeLightRev changed unexpectedly: got %d, want %d", cfg.ThemeLightRev, snapshot.ThemeLightRev)
	}
}

func TestConfigVersionBump_V1toV2(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "v1-config.json")

	// Write a v1 config with legacy palette fields.
	v1 := map[string]any{
		"version":              1,
		"data_dir":             tmpDir,
		"editor":               "vim",
		"default_context":      "work",
		"tui_background_color": "#abcdef",
		"palette": map[string]any{
			"primary": "#123456",
		},
		"projects": []any{},
	}
	data, err := json.MarshalIndent(v1, "", "  ")
	if err != nil {
		t.Fatalf("marshal v1 config: %v", err)
	}
	if wErr := os.WriteFile(configPath, data, 0o600); wErr != nil {
		t.Fatalf("write v1 config: %v", wErr)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Version must be bumped to 2.
	if cfg.Version != 2 {
		t.Errorf("Version = %d, want 2", cfg.Version)
	}

	// Legacy fields must be gone.
	if cfg.TUIBackgroundColor != "" {
		t.Errorf("TUIBackgroundColor should be empty after migration, got %q", cfg.TUIBackgroundColor)
	}
	if cfg.Palette != (PaletteColors{}) {
		t.Errorf("Palette should be empty after migration, got %+v", cfg.Palette)
	}

	// ThemeDark must have the legacy values.
	if cfg.ThemeDark.BackgroundColor != "#abcdef" {
		t.Errorf("ThemeDark.BackgroundColor = %q, want #abcdef", cfg.ThemeDark.BackgroundColor)
	}
	if cfg.ThemeDark.Palette.Primary != "#123456" {
		t.Errorf("ThemeDark.Palette.Primary = %q, want #123456", cfg.ThemeDark.Palette.Primary)
	}

	// ThemeLight must be curated.
	light := DefaultLightTheme()
	if cfg.ThemeLight.BackgroundColor != light.BackgroundColor {
		t.Errorf("ThemeLight.BackgroundColor = %q, want %q", cfg.ThemeLight.BackgroundColor, light.BackgroundColor)
	}

	// active_theme must be "dark".
	if cfg.ActiveTheme != "dark" {
		t.Errorf("ActiveTheme = %q, want dark", cfg.ActiveTheme)
	}

	// Reload — must be stable (no re-migration).
	cfg2, err := manager.Load()
	if err != nil {
		t.Fatalf("second Load failed: %v", err)
	}
	if cfg2.Version != 2 {
		t.Errorf("second load Version = %d, want 2", cfg2.Version)
	}
	if cfg2.ThemeDark.BackgroundColor != "#abcdef" {
		t.Errorf("second load ThemeDark.BackgroundColor = %q, want #abcdef", cfg2.ThemeDark.BackgroundColor)
	}
}
