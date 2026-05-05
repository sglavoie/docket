package tui

import (
	"fmt"
	"image/color"
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
)

// Default hex colors for priority title styling (Tokyo Night palette).
const (
	defaultColorPriority1Title = "#f7768e"
	defaultColorPriority2Title = "#e0af68"
	defaultColorPriority3Title = "#7aa2f7"
	defaultColorPriority4Title = "#9aa5ce"
	defaultColorPriority5Title = "#565f89"
)

// Priority title foreground colors, user-overridable via PaletteColors.
var (
	ColorPriority1Title = lipgloss.Color(defaultColorPriority1Title)
	ColorPriority2Title = lipgloss.Color(defaultColorPriority2Title)
	ColorPriority3Title = lipgloss.Color(defaultColorPriority3Title)
	ColorPriority4Title = lipgloss.Color(defaultColorPriority4Title)
	ColorPriority5Title = lipgloss.Color(defaultColorPriority5Title)
)

// darkenHex takes a 6-digit hex color (#rrggbb) and darkens each channel
// toward 0 by factor (0.0 = unchanged, 1.0 = black). Returns #rrggbb.
func darkenHex(hex string, factor float64) string {
	if len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	r, err1 := strconv.ParseInt(hex[1:3], 16, 64)
	g, err2 := strconv.ParseInt(hex[3:5], 16, 64)
	b, err3 := strconv.ParseInt(hex[5:7], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return hex
	}
	darken := func(channel int64) int64 {
		return channel - int64(float64(channel)*factor)
	}
	return fmt.Sprintf("#%02x%02x%02x", darken(r), darken(g), darken(b))
}

// lightenHex takes a 6-digit hex color (#rrggbb) and lightens each channel
// toward 255 by factor (0.0 = unchanged, 1.0 = white). Returns #rrggbb.
func lightenHex(hex string, factor float64) string {
	if len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	r, err1 := strconv.ParseInt(hex[1:3], 16, 64)
	g, err2 := strconv.ParseInt(hex[3:5], 16, 64)
	b, err3 := strconv.ParseInt(hex[5:7], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return hex
	}
	lighten := func(channel int64) int64 {
		return channel + int64(float64(255-channel)*factor)
	}
	return fmt.Sprintf("#%02x%02x%02x", lighten(r), lighten(g), lighten(b))
}

// rebuildPalette resets the palette color variables from user overrides.
// Empty fields fall back to the canonical defaults from config.DefaultTheme.
func rebuildPalette(p config.PaletteColors) {
	defaults := config.DefaultTheme(activeThemeName)
	d := defaults.Palette

	fallback := func(v, def string) color.Color {
		if v != "" {
			return lipgloss.Color(v)
		}
		return lipgloss.Color(def)
	}

	ColorPrimary = fallback(p.Primary, d.Primary)
	ColorSecondary = fallback(p.Secondary, d.Secondary)
	ColorSuccess = fallback(p.Success, d.Success)
	ColorWarning = fallback(p.Warning, d.Warning)
	ColorError = fallback(p.Error, d.Error)
	ColorMuted = fallback(p.Muted, d.Muted)
	ColorDimmed = fallback(p.Dimmed, d.Dimmed)
	ColorDimmedBg = fallback(p.DimmedBg, d.DimmedBg)
	ColorDimmedFg = fallback(p.DimmedFg, d.DimmedFg)
	ColorBorder = fallback(p.Border, d.Border)

	accent := fallback(p.Accent, d.Accent)
	ColorHighlight = accent
	ColorAccent = accent

	ColorBadgeText = ColorMuted
	ColorLightGray = ColorMuted
	ColorDiffContext = ColorMuted
	ColorSubtleBorder = ColorBorder

	ColorPriority1Title = fallback(p.Priority1Title, d.Priority1Title)
	ColorPriority2Title = fallback(p.Priority2Title, d.Priority2Title)
	ColorPriority3Title = fallback(p.Priority3Title, d.Priority3Title)
	ColorPriority4Title = fallback(p.Priority4Title, d.Priority4Title)
	ColorPriority5Title = fallback(p.Priority5Title, d.Priority5Title)

	if p.NormalText != "" {
		colorNormalText = p.NormalText
	} else {
		colorNormalText = d.NormalText
	}

	if activeThemeName == "light" {
		ColorProgressEmpty = lipgloss.Color(d.Dimmed)
	} else {
		ColorProgressEmpty = ColorMuted
	}
}
