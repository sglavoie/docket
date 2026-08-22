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
	defaultColorPriority1Title      = "#f7768e"
	defaultColorPriority2Title      = "#e0af68"
	defaultColorPriority3Title      = "#7aa2f7"
	defaultColorTaskContext         = "#73daca"
	defaultColorTaskContextDeferred = "#4f948f"

	defaultColorSemanticBugTitle             = "#ff4f58"
	defaultColorSemanticEpicTitle            = "#e17bff"
	defaultColorSemanticStoryTitle           = "#3fc5ff"
	defaultColorSemanticFeatureTitle         = "#35d67f"
	defaultColorSemanticIdeaTitle            = "#ffd23f"
	defaultColorSemanticBugSelectedTitle     = "#ff9ea3"
	defaultColorSemanticEpicSelectedTitle    = "#eeb6ff"
	defaultColorSemanticStorySelectedTitle   = "#95dfff"
	defaultColorSemanticFeatureSelectedTitle = "#90e8b9"
	defaultColorSemanticIdeaSelectedTitle    = "#ffe695"
	defaultColorSemanticBugDeferredTitle     = "#c47074"
	defaultColorSemanticEpicDeferredTitle    = "#b16fc4"
	defaultColorSemanticStoryDeferredTitle   = "#4891b0"
	defaultColorSemanticFeatureDeferredTitle = "#549572"
	defaultColorSemanticIdeaDeferredTitle    = "#9c8740"
)

// Priority title foreground colors, user-overridable via PaletteColors.
var (
	ColorPriority1Title      = lipgloss.Color(defaultColorPriority1Title)
	ColorPriority2Title      = lipgloss.Color(defaultColorPriority2Title)
	ColorPriority3Title      = lipgloss.Color(defaultColorPriority3Title)
	ColorTaskContext         = lipgloss.Color(defaultColorTaskContext)
	ColorTaskContextDeferred = lipgloss.Color(defaultColorTaskContextDeferred)

	ColorSemanticBugTitle             = lipgloss.Color(defaultColorSemanticBugTitle)
	ColorSemanticEpicTitle            = lipgloss.Color(defaultColorSemanticEpicTitle)
	ColorSemanticStoryTitle           = lipgloss.Color(defaultColorSemanticStoryTitle)
	ColorSemanticFeatureTitle         = lipgloss.Color(defaultColorSemanticFeatureTitle)
	ColorSemanticIdeaTitle            = lipgloss.Color(defaultColorSemanticIdeaTitle)
	ColorSemanticBugSelectedTitle     = lipgloss.Color(defaultColorSemanticBugSelectedTitle)
	ColorSemanticEpicSelectedTitle    = lipgloss.Color(defaultColorSemanticEpicSelectedTitle)
	ColorSemanticStorySelectedTitle   = lipgloss.Color(defaultColorSemanticStorySelectedTitle)
	ColorSemanticFeatureSelectedTitle = lipgloss.Color(defaultColorSemanticFeatureSelectedTitle)
	ColorSemanticIdeaSelectedTitle    = lipgloss.Color(defaultColorSemanticIdeaSelectedTitle)
	ColorSemanticBugDeferredTitle     = lipgloss.Color(defaultColorSemanticBugDeferredTitle)
	ColorSemanticEpicDeferredTitle    = lipgloss.Color(defaultColorSemanticEpicDeferredTitle)
	ColorSemanticStoryDeferredTitle   = lipgloss.Color(defaultColorSemanticStoryDeferredTitle)
	ColorSemanticFeatureDeferredTitle = lipgloss.Color(defaultColorSemanticFeatureDeferredTitle)
	ColorSemanticIdeaDeferredTitle    = lipgloss.Color(defaultColorSemanticIdeaDeferredTitle)
)

// tagChipColors is the active theme's tag chip ramp, indexed by slot. The slot
// index — not the hex — is what a tag owns: tagChipStyle hashes a tag onto a
// slot and reads this slice at render time, so toggling the theme repaints a
// tag without moving it to a different hue family. Declared dark and rebuilt
// per theme like every other derived color set.
var tagChipColors = tagChipRamp("dark")

// tagChipRamp lifts one variant's curated ramp into lipgloss colors. The ramp
// itself lives in internal/config, where its perceptual distance from the
// semantic and priority titles can be gated against the same palette.
func tagChipRamp(variant string) []color.Color {
	hexes := config.DefaultTagChipRamp(variant)
	ramp := make([]color.Color, len(hexes))
	for i, hex := range hexes {
		ramp[i] = lipgloss.Color(hex)
	}
	return ramp
}

// rebuildTagChipColors re-resolves the chip ramp for the active theme.
func rebuildTagChipColors() {
	tagChipColors = tagChipRamp(activeThemeName)
}

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

// rebuildAccentColors sets pipeline accent colors appropriate for the active theme.
func rebuildAccentColors() {
	if activeThemeName == "light" {
		AccentCapture = lipgloss.Color("#b91c1c")
		AccentTriage = lipgloss.Color("#0e7490")
		AccentPlan = lipgloss.Color("#15803d")
		AccentWork = lipgloss.Color("#c2410c")
		AccentExecute = lipgloss.Color("#0f766e")
		AccentSupport = lipgloss.Color("#4b5563")
		AccentSearch = lipgloss.Color("#7c3aed")
	} else {
		AccentCapture = lipgloss.Color("#f7768e")
		AccentTriage = lipgloss.Color("#7dcfff")
		AccentPlan = lipgloss.Color("#9ece6a")
		AccentWork = lipgloss.Color("#ff9e64")
		AccentExecute = lipgloss.Color("#73daca")
		AccentSupport = lipgloss.Color("#565f89")
		AccentSearch = lipgloss.Color("#bb9af7")
	}
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
	ColorSubtleBorder = ColorBorder

	ColorPriority1Title = fallback(p.Priority1Title, d.Priority1Title)
	ColorPriority2Title = fallback(p.Priority2Title, d.Priority2Title)
	ColorPriority3Title = fallback(p.Priority3Title, d.Priority3Title)
	ColorTaskContext = fallback(p.TaskContext, d.TaskContext)
	ColorTaskContextDeferred = fallback(p.TaskContextDeferred, d.TaskContextDeferred)

	ColorSemanticBugTitle = fallback(p.SemanticBugTitle, d.SemanticBugTitle)
	ColorSemanticEpicTitle = fallback(p.SemanticEpicTitle, d.SemanticEpicTitle)
	ColorSemanticStoryTitle = fallback(p.SemanticStoryTitle, d.SemanticStoryTitle)
	ColorSemanticFeatureTitle = fallback(p.SemanticFeatureTitle, d.SemanticFeatureTitle)
	ColorSemanticIdeaTitle = fallback(p.SemanticIdeaTitle, d.SemanticIdeaTitle)
	ColorSemanticBugSelectedTitle = fallback(p.SemanticBugSelectedTitle, d.SemanticBugSelectedTitle)
	ColorSemanticEpicSelectedTitle = fallback(p.SemanticEpicSelectedTitle, d.SemanticEpicSelectedTitle)
	ColorSemanticStorySelectedTitle = fallback(p.SemanticStorySelectedTitle, d.SemanticStorySelectedTitle)
	ColorSemanticFeatureSelectedTitle = fallback(p.SemanticFeatureSelectedTitle, d.SemanticFeatureSelectedTitle)
	ColorSemanticIdeaSelectedTitle = fallback(p.SemanticIdeaSelectedTitle, d.SemanticIdeaSelectedTitle)
	ColorSemanticBugDeferredTitle = fallback(p.SemanticBugDeferredTitle, d.SemanticBugDeferredTitle)
	ColorSemanticEpicDeferredTitle = fallback(p.SemanticEpicDeferredTitle, d.SemanticEpicDeferredTitle)
	ColorSemanticStoryDeferredTitle = fallback(p.SemanticStoryDeferredTitle, d.SemanticStoryDeferredTitle)
	ColorSemanticFeatureDeferredTitle = fallback(p.SemanticFeatureDeferredTitle, d.SemanticFeatureDeferredTitle)
	ColorSemanticIdeaDeferredTitle = fallback(p.SemanticIdeaDeferredTitle, d.SemanticIdeaDeferredTitle)

	if p.NormalText != "" {
		colorNormalText = p.NormalText
	} else {
		colorNormalText = d.NormalText
	}

	if activeThemeName == "light" {
		ColorProgressEmpty = lipgloss.Color(d.Dimmed)
	} else {
		ColorProgressEmpty = ColorBorder
	}
}
