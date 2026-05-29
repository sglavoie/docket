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

// ActiveThemeConfig returns the Theme for the currently active theme.
// Returns ThemeLight when ActiveTheme == "light", otherwise ThemeDark.
func (c *Config) ActiveThemeConfig() Theme {
	if c.ActiveTheme == "light" {
		return c.ThemeLight
	}
	return c.ThemeDark
}

// SetActiveTheme writes a Theme to the named slot ("dark" or "light").
func (c *Config) SetActiveTheme(name string, t Theme) {
	if name == "light" {
		c.ThemeLight = t
	} else {
		c.ThemeDark = t
	}
}

// isThemeZero reports whether a Theme has no non-empty values.
func isThemeZero(t Theme) bool {
	return t.BackgroundColor == "" && t.CursorColor == "" && t.CursorTextColor == "" &&
		t.Palette == (PaletteColors{})
}

// MigratePaletteToThemes migrates legacy flat palette/bg/cursor fields into the
// new ThemeDark/ThemeLight slots. It follows the same pattern as MigrateLegacyFilters.
// Returns true if any change was made (caller should save the config).
func (c *Config) MigratePaletteToThemes() bool {
	changed := false

	legacyHasData := c.TUIBackgroundColor != "" || c.TUICursorColor != "" ||
		c.TUICursorTextColor != "" || c.Palette != (PaletteColors{})
	if isThemeZero(c.ThemeDark) && legacyHasData {
		c.ThemeDark = Theme{
			Palette:         c.Palette,
			BackgroundColor: c.TUIBackgroundColor,
			CursorColor:     c.TUICursorColor,
			CursorTextColor: c.TUICursorTextColor,
		}
		c.Palette = PaletteColors{}
		c.TUIBackgroundColor = ""
		c.TUICursorColor = ""
		c.TUICursorTextColor = ""
		c.ThemeDarkRev = DarkThemeDefaultsRev
		changed = true
	} else if legacyHasData {
		c.Palette = PaletteColors{}
		c.TUIBackgroundColor = ""
		c.TUICursorColor = ""
		c.TUICursorTextColor = ""
		changed = true
	}

	if isThemeZero(c.ThemeDark) {
		c.ThemeDark = DefaultDarkTheme()
		c.ThemeDarkRev = DarkThemeDefaultsRev
		changed = true
	} else if c.ThemeDarkRev < DarkThemeDefaultsRev {
		c.ThemeDark = DefaultDarkTheme()
		c.ThemeDarkName = ""
		c.ThemeDarkRev = DarkThemeDefaultsRev
		changed = true
	}

	if isThemeZero(c.ThemeLight) {
		c.ThemeLight = DefaultLightTheme()
		c.ThemeLightRev = LightThemeDefaultsRev
		changed = true
	} else if c.ThemeLightRev < LightThemeDefaultsRev {
		c.ThemeLight = DefaultLightTheme()
		c.ThemeLightName = ""
		c.ThemeLightRev = LightThemeDefaultsRev
		changed = true
	}

	if c.ActiveTheme == "" || (c.ActiveTheme != "dark" && c.ActiveTheme != "light") {
		c.ActiveTheme = "dark"
		changed = true
	}

	return changed
}
