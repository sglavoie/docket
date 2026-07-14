package config

import "testing"

func TestMigratePaletteToThemes_FromLegacy(t *testing.T) {
	cfg := &Settings{
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
	cfg := &Settings{
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

	cfg := &Settings{
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
	cfg := &Settings{}

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
	cfg := &Settings{
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

func TestMigratePaletteToThemes_StaleRev1DarkDefaults_Refreshes(t *testing.T) {
	cfg := &Settings{
		ThemeDark:     defaultDarkThemeRev1(),
		ThemeDarkName: "tokyo-night",
		ThemeDarkRev:  1,
		ThemeLight:    DefaultLightTheme(),
		ThemeLightRev: LightThemeDefaultsRev,
		ActiveTheme:   "dark",
	}

	changed := cfg.MigratePaletteToThemes()
	if !changed {
		t.Fatal("expected changed=true when stale dark defaults match shipped rev 1")
	}

	dark := DefaultDarkTheme()
	if cfg.ThemeDark != dark {
		t.Errorf("ThemeDark = %+v, want refreshed defaults %+v", cfg.ThemeDark, dark)
	}
	if cfg.ThemeDarkRev != DarkThemeDefaultsRev {
		t.Errorf("ThemeDarkRev = %d, want %d", cfg.ThemeDarkRev, DarkThemeDefaultsRev)
	}
	if cfg.ThemeDarkName != "" {
		t.Errorf("ThemeDarkName = %q, want empty after refreshing shipped defaults", cfg.ThemeDarkName)
	}
}

func TestMigratePaletteToThemes_StaleCustomDarkTheme_PreservesAndMarksCurrent(t *testing.T) {
	customTheme := defaultDarkThemeRev1()
	customTheme.Palette.Muted = "#99aabb"
	cfg := &Settings{
		ThemeDark:     customTheme,
		ThemeDarkName: "custom-dark",
		ThemeDarkRev:  1,
		ThemeLight:    DefaultLightTheme(),
		ThemeLightRev: LightThemeDefaultsRev,
		ActiveTheme:   "dark",
	}

	changed := cfg.MigratePaletteToThemes()
	if !changed {
		t.Fatal("expected changed=true so the stale custom dark revision is saved as current")
	}

	if cfg.ThemeDark != customTheme {
		t.Errorf("ThemeDark was modified: got %+v, want %+v", cfg.ThemeDark, customTheme)
	}
	if cfg.ThemeDarkRev != DarkThemeDefaultsRev {
		t.Errorf("ThemeDarkRev = %d, want %d", cfg.ThemeDarkRev, DarkThemeDefaultsRev)
	}
	if cfg.ThemeDarkName != "custom-dark" {
		t.Errorf("ThemeDarkName = %q, want custom-dark", cfg.ThemeDarkName)
	}
	if cfg.MigratePaletteToThemes() {
		t.Error("expected second call to be a no-op")
	}
}

func TestMigratePaletteToThemes_CurrentRev_NoOp(t *testing.T) {
	light := DefaultLightTheme()
	cfg := &Settings{
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

func TestMigratePaletteToThemes_InvalidActiveThemeDefaultsDark(t *testing.T) {
	cfg := &Settings{
		ThemeDark:     DefaultDarkTheme(),
		ThemeDarkRev:  DarkThemeDefaultsRev,
		ThemeLight:    DefaultLightTheme(),
		ThemeLightRev: LightThemeDefaultsRev,
		ActiveTheme:   "sepia",
	}

	changed := cfg.MigratePaletteToThemes()
	if !changed {
		t.Fatal("expected changed=true when ActiveTheme is invalid")
	}
	if cfg.ActiveTheme != "dark" {
		t.Errorf("ActiveTheme = %q, want dark", cfg.ActiveTheme)
	}
}

func TestMigratePaletteToThemes_ClearsStaleLegacyFields(t *testing.T) {
	// Legacy palette fields linger alongside a populated ThemeDark (e.g. after
	// a hand edit or partial migration). The migration must drop the stale
	// legacy data so it stops round-tripping through Save.
	existingDark := Theme{BackgroundColor: "#222222", Palette: PaletteColors{Primary: "#abcdef"}}
	cfg := &Settings{
		ThemeDark:          existingDark,
		ThemeDarkRev:       DarkThemeDefaultsRev,
		ThemeLight:         DefaultLightTheme(),
		ThemeLightRev:      LightThemeDefaultsRev,
		ActiveTheme:        "dark",
		TUIBackgroundColor: "#999999",
		TUICursorColor:     "#888888",
		TUICursorTextColor: "#777777",
		Palette:            PaletteColors{Primary: "#101010"},
	}

	changed := cfg.MigratePaletteToThemes()
	if !changed {
		t.Fatal("expected changed=true when stale legacy fields are cleared")
	}

	if cfg.TUIBackgroundColor != "" {
		t.Errorf("TUIBackgroundColor not cleared, got %q", cfg.TUIBackgroundColor)
	}
	if cfg.TUICursorColor != "" {
		t.Errorf("TUICursorColor not cleared, got %q", cfg.TUICursorColor)
	}
	if cfg.TUICursorTextColor != "" {
		t.Errorf("TUICursorTextColor not cleared, got %q", cfg.TUICursorTextColor)
	}
	if cfg.Palette != (PaletteColors{}) {
		t.Errorf("Palette not cleared, got %+v", cfg.Palette)
	}

	// Existing ThemeDark must remain untouched.
	if cfg.ThemeDark != existingDark {
		t.Errorf("ThemeDark was modified: got %+v, want %+v", cfg.ThemeDark, existingDark)
	}

	// Second call must be a no-op (idempotent).
	if cfg.MigratePaletteToThemes() {
		t.Error("expected second call to be a no-op")
	}
}
