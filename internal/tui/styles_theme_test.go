package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestColorProgressEmptyLightTheme(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: "light"})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	want := lipgloss.Color("#9ca3af")
	if theme.ColorProgressEmpty != want {
		t.Errorf("light theme: ColorProgressEmpty = %v, want %v", theme.ColorProgressEmpty, want)
	}
}

func TestColorProgressEmptyDarkTheme(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	// Empty progress stays on the lower-contrast structural token.
	want := lipgloss.Color("#565f89")
	if theme.ColorProgressEmpty != want {
		t.Errorf("dark theme: ColorProgressEmpty = %v, want %v", theme.ColorProgressEmpty, want)
	}
}

func TestDarkThemeMutedAndStructuralStyleMappings(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	readableMuted := lipgloss.Color("#8f98c7")
	structural := lipgloss.Color("#565f89")

	if theme.MutedStyle.GetForeground() != readableMuted {
		t.Errorf("MutedStyle foreground = %v, want %v", theme.MutedStyle.GetForeground(), readableMuted)
	}
	if theme.ProgressEmptyStyle.GetForeground() != structural {
		t.Errorf("ProgressEmptyStyle foreground = %v, want %v", theme.ProgressEmptyStyle.GetForeground(), structural)
	}
	if theme.ScrollbarTrackStyle.GetForeground() != structural {
		t.Errorf("ScrollbarTrackStyle foreground = %v, want %v", theme.ScrollbarTrackStyle.GetForeground(), structural)
	}
}

func TestOverlayHaloTracksThemeDimmedBackground(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: "light"})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	if got, want := theme.OverlayHaloStyle.GetBackground(), theme.ColorDimmedBg; got != want {
		t.Errorf("light overlay halo background = %v, want %v", got, want)
	}
}

func TestMakeViewModeStyleLightThemeBg(t *testing.T) {
	theme.ActiveThemeName = "light"
	t.Cleanup(func() { theme.ActiveThemeName = "" })

	darkANSI := "34"
	lightHex := "#15803d"
	s := theme.MakeViewModeStyle(darkANSI, lightHex)

	got := s.GetBackground()
	want := lipgloss.Color(lightHex)
	if got != want {
		t.Errorf("light theme: background = %v, want %v", got, want)
	}
}

func TestMakeViewModeStyleDarkThemeBg(t *testing.T) {
	theme.ActiveThemeName = ""
	t.Cleanup(func() { theme.ActiveThemeName = "" })

	darkHex := "#2e4a30"
	lightHex := "#15803d"
	s := theme.MakeViewModeStyle(darkHex, lightHex)

	got := s.GetBackground()
	want := lipgloss.Color(darkHex)
	if got != want {
		t.Errorf("dark theme: background = %v, want %v", got, want)
	}
}

func TestRebuildPaletteLightEmptyUsesDefaults(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: "light"})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	theme.RebuildPalette(config.PaletteColors{})

	if theme.ColorPrimary != lipgloss.Color("#0366d6") {
		t.Errorf("light empty palette: ColorPrimary = %v, want #0366d6", theme.ColorPrimary)
	}
	if theme.ColorMuted != lipgloss.Color("#4b5563") {
		t.Errorf("light empty palette: ColorMuted = %v, want #4b5563", theme.ColorMuted)
	}
	if theme.ColorTaskContext != lipgloss.Color("#0f766e") || theme.ColorTaskContextDeferred != lipgloss.Color("#5f8782") {
		t.Errorf("light context palette = %v/%v", theme.ColorTaskContext, theme.ColorTaskContextDeferred)
	}
	if theme.ColorNormalText != "#111827" {
		t.Errorf("light empty palette: ColorNormalText = %q, want #111827", theme.ColorNormalText)
	}
}

func TestRebuildPaletteDarkEmptyUsesDefaults(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	theme.RebuildPalette(config.PaletteColors{})

	if theme.ColorPrimary != lipgloss.Color("#7aa2f7") {
		t.Errorf("dark empty palette: ColorPrimary = %v, want #7aa2f7", theme.ColorPrimary)
	}
	if theme.ColorMuted != lipgloss.Color("#8f98c7") {
		t.Errorf("dark empty palette: ColorMuted = %v, want #8f98c7", theme.ColorMuted)
	}
	if theme.ColorTaskContext != lipgloss.Color("#73daca") || theme.ColorTaskContextDeferred != lipgloss.Color("#4f948f") {
		t.Errorf("dark context palette = %v/%v", theme.ColorTaskContext, theme.ColorTaskContextDeferred)
	}
	if theme.ColorNormalText != "#c0caf5" {
		t.Errorf("dark empty palette: ColorNormalText = %q, want #c0caf5", theme.ColorNormalText)
	}
}

func TestApplyThemeLightEmptyCursorResolvesToDefaults(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: "light"})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	if theme.ColorSelectedBg != lipgloss.Color("#c7b6f0") {
		t.Errorf("light empty cursor: ColorSelectedBg = %v, want #c7b6f0", theme.ColorSelectedBg)
	}
	if theme.ColorSelectedFg != lipgloss.Color("#1f1b35") {
		t.Errorf("light empty cursor: ColorSelectedFg = %v, want #1f1b35", theme.ColorSelectedFg)
	}
	if theme.ThemeBgColor != "#f6f6f0" {
		t.Errorf("light empty bg: ThemeBgColor = %q, want #f6f6f0", theme.ThemeBgColor)
	}
}

func TestApplyThemeDarkEmptyCursorResolvesToDefaults(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	if theme.ColorSelectedBg != lipgloss.Color("#33467c") {
		t.Errorf("dark empty cursor: ColorSelectedBg = %v, want #33467c", theme.ColorSelectedBg)
	}
	if theme.ColorSelectedFg != lipgloss.Color("#c0caf5") {
		t.Errorf("dark empty cursor: ColorSelectedFg = %v, want #c0caf5", theme.ColorSelectedFg)
	}
	if theme.ThemeBgColor != "#1a1b26" {
		t.Errorf("dark empty bg: ThemeBgColor = %q, want #1a1b26", theme.ThemeBgColor)
	}
}

func TestColorSelectedTextFgLightTheme(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: "light"})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	want := lipgloss.Color("#0f172a")
	if theme.ColorSelectedTextFg != want {
		t.Errorf("light theme: ColorSelectedTextFg = %v, want %v", theme.ColorSelectedTextFg, want)
	}
}

func TestColorSelectedTextFgDarkTheme(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	want := lipgloss.Color("#c0caf5")
	if theme.ColorSelectedTextFg != want {
		t.Errorf("dark theme: ColorSelectedTextFg = %v, want %v", theme.ColorSelectedTextFg, want)
	}
}

func TestAllPaletteColorVarsSetAfterDarkTheme(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	checks := []struct {
		name string
		got  color.Color
		want string
	}{
		{"ColorPrimary", theme.ColorPrimary, "#7aa2f7"},
		{"ColorSecondary", theme.ColorSecondary, "#7dcfff"},
		{"ColorSuccess", theme.ColorSuccess, "#9ece6a"},
		{"ColorWarning", theme.ColorWarning, "#e0af68"},
		{"ColorError", theme.ColorError, "#f7768e"},
		{"ColorMuted", theme.ColorMuted, "#8f98c7"},
		{"ColorDimmed", theme.ColorDimmed, "#565f89"},
		{"ColorBorder", theme.ColorBorder, "#565f89"},
		{"ColorAccent", theme.ColorAccent, "#e0af68"},
		{"ColorProgressEmpty", theme.ColorProgressEmpty, "#565f89"},
		{"ColorBadgeText", theme.ColorBadgeText, "#8f98c7"},
		{"ColorLightGray", theme.ColorLightGray, "#8f98c7"},
		{"ColorPriority1Title", theme.ColorPriority1Title, "#f7768e"},
		{"ColorPriority2Title", theme.ColorPriority2Title, "#e0af68"},
		{"ColorPriority3Title", theme.ColorPriority3Title, "#7aa2f7"},
		{"ColorTaskContext", theme.ColorTaskContext, "#73daca"},
		{"ColorTaskContextDeferred", theme.ColorTaskContextDeferred, "#4f948f"},
		{"ColorSemanticBugTitle", theme.ColorSemanticBugTitle, "#ff4f58"},
		{"ColorSemanticQuestionTitle", theme.ColorSemanticQuestionTitle, "#f57a00"},
		{"ColorSemanticEpicTitle", theme.ColorSemanticEpicTitle, "#e17bff"},
		{"ColorSemanticStoryTitle", theme.ColorSemanticStoryTitle, "#3fc5ff"},
		{"ColorSemanticFeatureTitle", theme.ColorSemanticFeatureTitle, "#35d67f"},
		{"ColorSemanticIdeaTitle", theme.ColorSemanticIdeaTitle, "#ffd23f"},
		{"ColorSemanticBugSelectedTitle", theme.ColorSemanticBugSelectedTitle, "#ff9ea3"},
		{"ColorSemanticQuestionSelectedTitle", theme.ColorSemanticQuestionSelectedTitle, "#ffa366"},
		{"ColorSemanticEpicSelectedTitle", theme.ColorSemanticEpicSelectedTitle, "#eeb6ff"},
		{"ColorSemanticStorySelectedTitle", theme.ColorSemanticStorySelectedTitle, "#95dfff"},
		{"ColorSemanticFeatureSelectedTitle", theme.ColorSemanticFeatureSelectedTitle, "#90e8b9"},
		{"ColorSemanticIdeaSelectedTitle", theme.ColorSemanticIdeaSelectedTitle, "#ffe695"},
		{"ColorSemanticBugDeferredTitle", theme.ColorSemanticBugDeferredTitle, "#c47074"},
		{"ColorSemanticQuestionDeferredTitle", theme.ColorSemanticQuestionDeferredTitle, "#d66e29"},
		{"ColorSemanticEpicDeferredTitle", theme.ColorSemanticEpicDeferredTitle, "#b16fc4"},
		{"ColorSemanticStoryDeferredTitle", theme.ColorSemanticStoryDeferredTitle, "#4891b0"},
		{"ColorSemanticFeatureDeferredTitle", theme.ColorSemanticFeatureDeferredTitle, "#549572"},
		{"ColorSemanticIdeaDeferredTitle", theme.ColorSemanticIdeaDeferredTitle, "#9c8740"},
	}
	for _, c := range checks {
		if c.got != lipgloss.Color(c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestRebuildPaletteSemanticHierarchyOverrides(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: "dark"})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	theme.RebuildPalette(config.PaletteColors{
		SemanticEpicTitle:          "#111111",
		SemanticStoryTitle:         "#222222",
		SemanticEpicSelectedTitle:  "#333333",
		SemanticStorySelectedTitle: "#444444",
		SemanticEpicDeferredTitle:  "#555555",
		SemanticStoryDeferredTitle: "#666666",
	})

	checks := []struct {
		name string
		got  color.Color
		want string
	}{
		{"ColorSemanticEpicTitle", theme.ColorSemanticEpicTitle, "#111111"},
		{"ColorSemanticStoryTitle", theme.ColorSemanticStoryTitle, "#222222"},
		{"ColorSemanticEpicSelectedTitle", theme.ColorSemanticEpicSelectedTitle, "#333333"},
		{"ColorSemanticStorySelectedTitle", theme.ColorSemanticStorySelectedTitle, "#444444"},
		{"ColorSemanticEpicDeferredTitle", theme.ColorSemanticEpicDeferredTitle, "#555555"},
		{"ColorSemanticStoryDeferredTitle", theme.ColorSemanticStoryDeferredTitle, "#666666"},
	}
	for _, check := range checks {
		if check.got != lipgloss.Color(check.want) {
			t.Errorf("%s = %v, want %s", check.name, check.got, check.want)
		}
	}
}

func TestRebuildPaletteTaskContextOverrides(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: "dark"})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	theme.RebuildPalette(config.PaletteColors{
		TaskContext:         "#111111",
		TaskContextDeferred: "#222222",
	})
	if theme.ColorTaskContext != lipgloss.Color("#111111") || theme.ColorTaskContextDeferred != lipgloss.Color("#222222") {
		t.Fatalf("context override = %v/%v", theme.ColorTaskContext, theme.ColorTaskContextDeferred)
	}
}
