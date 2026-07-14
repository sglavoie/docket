package config

// Theme groups palette colors together with background and cursor overrides for
// a single named theme (dark or light). Empty strings mean "use ANSI default".
type Theme struct {
	Palette         PaletteColors `json:"palette,omitzero"`
	BackgroundColor string        `json:"background_color,omitempty"`
	CursorColor     string        `json:"cursor_color,omitempty"`
	CursorTextColor string        `json:"cursor_text_color,omitempty"`
}

// PaletteColors holds user-configurable TUI palette overrides.
// Empty strings mean "use ANSI default".
type PaletteColors struct {
	NormalText     string `json:"normal_text,omitempty"`
	Muted          string `json:"muted,omitempty"`
	Dimmed         string `json:"dimmed,omitempty"`
	DimmedBg       string `json:"dimmed_bg,omitempty"`
	DimmedFg       string `json:"dimmed_fg,omitempty"`
	Primary        string `json:"primary,omitempty"`
	Secondary      string `json:"secondary,omitempty"`
	Accent         string `json:"accent,omitempty"`
	Success        string `json:"success,omitempty"`
	Warning        string `json:"warning,omitempty"`
	Error          string `json:"error,omitempty"`
	Border         string `json:"border,omitempty"`
	Priority1Title string `json:"priority1_title,omitempty"`
	Priority2Title string `json:"priority2_title,omitempty"`
	Priority3Title string `json:"priority3_title,omitempty"`
	Priority4Title string `json:"priority4_title,omitempty"`
	Priority5Title string `json:"priority5_title,omitempty"`
}

// isThemeZero reports whether a Theme has no non-empty values.
func isThemeZero(t Theme) bool {
	return t.BackgroundColor == "" && t.CursorColor == "" && t.CursorTextColor == "" &&
		t.Palette == (PaletteColors{})
}

// MigratePaletteToThemes migrates legacy flat palette/bg/cursor fields into the
// new ThemeDark/ThemeLight slots. It follows the same pattern as MigrateLegacyFilters.
// Returns true if any change was made (caller should save the config).
func (s *Settings) MigratePaletteToThemes() bool {
	changed := false

	legacyHasData := s.TUIBackgroundColor != "" || s.TUICursorColor != "" ||
		s.TUICursorTextColor != "" || s.Palette != (PaletteColors{})
	if isThemeZero(s.ThemeDark) && legacyHasData {
		s.ThemeDark = Theme{
			Palette:         s.Palette,
			BackgroundColor: s.TUIBackgroundColor,
			CursorColor:     s.TUICursorColor,
			CursorTextColor: s.TUICursorTextColor,
		}
		s.Palette = PaletteColors{}
		s.TUIBackgroundColor = ""
		s.TUICursorColor = ""
		s.TUICursorTextColor = ""
		s.ThemeDarkRev = DarkThemeDefaultsRev
		changed = true
	} else if legacyHasData {
		s.Palette = PaletteColors{}
		s.TUIBackgroundColor = ""
		s.TUICursorColor = ""
		s.TUICursorTextColor = ""
		changed = true
	}

	if isThemeZero(s.ThemeDark) {
		s.ThemeDark = DefaultDarkTheme()
		s.ThemeDarkRev = DarkThemeDefaultsRev
		changed = true
	} else if s.ThemeDarkRev < DarkThemeDefaultsRev {
		if s.ThemeDark == defaultDarkThemeRev1() {
			s.ThemeDark = DefaultDarkTheme()
			s.ThemeDarkName = ""
		}
		s.ThemeDarkRev = DarkThemeDefaultsRev
		changed = true
	}

	if isThemeZero(s.ThemeLight) {
		s.ThemeLight = DefaultLightTheme()
		s.ThemeLightRev = LightThemeDefaultsRev
		changed = true
	} else if s.ThemeLightRev < LightThemeDefaultsRev {
		s.ThemeLight = DefaultLightTheme()
		s.ThemeLightName = ""
		s.ThemeLightRev = LightThemeDefaultsRev
		changed = true
	}

	if s.ActiveTheme == "" || (s.ActiveTheme != "dark" && s.ActiveTheme != "light") {
		s.ActiveTheme = "dark"
		changed = true
	}

	return changed
}
