package tui

import (
	"fmt"
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
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
// Empty fields fall back to ANSI defaults.
func rebuildPalette(p config.PaletteColors) {
	if p.Primary != "" {
		ColorPrimary = lipgloss.Color(p.Primary)
	} else {
		ColorPrimary = lipgloss.Color("39")
	}
	if p.Secondary != "" {
		ColorSecondary = lipgloss.Color(p.Secondary)
	} else {
		ColorSecondary = lipgloss.Color("33")
	}
	if p.Success != "" {
		ColorSuccess = lipgloss.Color(p.Success)
	} else {
		ColorSuccess = lipgloss.Color("42")
	}
	if p.Warning != "" {
		ColorWarning = lipgloss.Color(p.Warning)
	} else {
		ColorWarning = lipgloss.Color("214")
	}
	if p.Error != "" {
		ColorError = lipgloss.Color(p.Error)
	} else {
		ColorError = lipgloss.Color("196")
	}
	if p.Muted != "" {
		ColorMuted = lipgloss.Color(p.Muted)
	} else {
		ColorMuted = lipgloss.Color("248")
	}
	if p.Dimmed != "" {
		ColorDimmed = lipgloss.Color(p.Dimmed)
	} else {
		ColorDimmed = lipgloss.Color("240")
	}
	if p.DimmedBg != "" {
		ColorDimmedBg = lipgloss.Color(p.DimmedBg)
	} else {
		ColorDimmedBg = lipgloss.Color("237")
	}
	if p.DimmedFg != "" {
		ColorDimmedFg = lipgloss.Color(p.DimmedFg)
	} else {
		ColorDimmedFg = lipgloss.Color("250")
	}
	if p.Border != "" {
		ColorBorder = lipgloss.Color(p.Border)
	} else {
		ColorBorder = lipgloss.Color("244")
	}
	if p.Accent != "" {
		ColorHighlight = lipgloss.Color(p.Accent)
		ColorAccent = lipgloss.Color(p.Accent)
	} else {
		ColorHighlight = lipgloss.Color("#f2eb16")
		ColorAccent = lipgloss.Color("226")
	}

	ColorBadgeText = ColorMuted
	ColorLightGray = ColorMuted
	ColorDiffContext = ColorMuted
	ColorSubtleBorder = ColorBorder

	colorNormalText = p.NormalText
}
