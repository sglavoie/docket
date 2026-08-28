package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
)

func TestColorProgressEmptyLightTheme(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "light"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	want := lipgloss.Color("#9ca3af")
	if ColorProgressEmpty != want {
		t.Errorf("light theme: ColorProgressEmpty = %v, want %v", ColorProgressEmpty, want)
	}
}

func TestColorProgressEmptyDarkTheme(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	// Empty progress stays on the lower-contrast structural token.
	want := lipgloss.Color("#565f89")
	if ColorProgressEmpty != want {
		t.Errorf("dark theme: ColorProgressEmpty = %v, want %v", ColorProgressEmpty, want)
	}
}

func TestDarkThemeMutedAndStructuralStyleMappings(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	readableMuted := lipgloss.Color("#8f98c7")
	structural := lipgloss.Color("#565f89")

	if MutedStyle.GetForeground() != readableMuted {
		t.Errorf("MutedStyle foreground = %v, want %v", MutedStyle.GetForeground(), readableMuted)
	}
	if ProgressEmptyStyle.GetForeground() != structural {
		t.Errorf("ProgressEmptyStyle foreground = %v, want %v", ProgressEmptyStyle.GetForeground(), structural)
	}
	if ScrollbarTrackStyle.GetForeground() != structural {
		t.Errorf("ScrollbarTrackStyle foreground = %v, want %v", ScrollbarTrackStyle.GetForeground(), structural)
	}
}

func TestOverlayHaloTracksThemeDimmedBackground(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "light"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	if got, want := OverlayHaloStyle.GetBackground(), ColorDimmedBg; got != want {
		t.Errorf("light overlay halo background = %v, want %v", got, want)
	}
}

func TestMakeViewModeStyleLightThemeBg(t *testing.T) {
	activeThemeName = "light"
	t.Cleanup(func() { activeThemeName = "" })

	darkANSI := "34"
	lightHex := "#15803d"
	s := makeViewModeStyle(darkANSI, lightHex)

	got := s.GetBackground()
	want := lipgloss.Color(lightHex)
	if got != want {
		t.Errorf("light theme: background = %v, want %v", got, want)
	}
}

func TestMakeViewModeStyleDarkThemeBg(t *testing.T) {
	activeThemeName = ""
	t.Cleanup(func() { activeThemeName = "" })

	darkHex := "#2e4a30"
	lightHex := "#15803d"
	s := makeViewModeStyle(darkHex, lightHex)

	got := s.GetBackground()
	want := lipgloss.Color(darkHex)
	if got != want {
		t.Errorf("dark theme: background = %v, want %v", got, want)
	}
}

func TestRebuildPaletteLightEmptyUsesDefaults(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "light"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	rebuildPalette(config.PaletteColors{})

	if ColorPrimary != lipgloss.Color("#0366d6") {
		t.Errorf("light empty palette: ColorPrimary = %v, want #0366d6", ColorPrimary)
	}
	if ColorMuted != lipgloss.Color("#4b5563") {
		t.Errorf("light empty palette: ColorMuted = %v, want #4b5563", ColorMuted)
	}
	if ColorTaskContext != lipgloss.Color("#0f766e") || ColorTaskContextDeferred != lipgloss.Color("#5f8782") {
		t.Errorf("light context palette = %v/%v", ColorTaskContext, ColorTaskContextDeferred)
	}
	if colorNormalText != "#111827" {
		t.Errorf("light empty palette: colorNormalText = %q, want #111827", colorNormalText)
	}
}

func TestRebuildPaletteDarkEmptyUsesDefaults(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	rebuildPalette(config.PaletteColors{})

	if ColorPrimary != lipgloss.Color("#7aa2f7") {
		t.Errorf("dark empty palette: ColorPrimary = %v, want #7aa2f7", ColorPrimary)
	}
	if ColorMuted != lipgloss.Color("#8f98c7") {
		t.Errorf("dark empty palette: ColorMuted = %v, want #8f98c7", ColorMuted)
	}
	if ColorTaskContext != lipgloss.Color("#73daca") || ColorTaskContextDeferred != lipgloss.Color("#4f948f") {
		t.Errorf("dark context palette = %v/%v", ColorTaskContext, ColorTaskContextDeferred)
	}
	if colorNormalText != "#c0caf5" {
		t.Errorf("dark empty palette: colorNormalText = %q, want #c0caf5", colorNormalText)
	}
}

func TestApplyThemeLightEmptyCursorResolvesToDefaults(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "light"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	if ColorSelectedBg != lipgloss.Color("#c7b6f0") {
		t.Errorf("light empty cursor: ColorSelectedBg = %v, want #c7b6f0", ColorSelectedBg)
	}
	if ColorSelectedFg != lipgloss.Color("#1f1b35") {
		t.Errorf("light empty cursor: ColorSelectedFg = %v, want #1f1b35", ColorSelectedFg)
	}
	if themeBgColor != "#f6f6f0" {
		t.Errorf("light empty bg: themeBgColor = %q, want #f6f6f0", themeBgColor)
	}
}

func TestApplyThemeDarkEmptyCursorResolvesToDefaults(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	if ColorSelectedBg != lipgloss.Color("#33467c") {
		t.Errorf("dark empty cursor: ColorSelectedBg = %v, want #33467c", ColorSelectedBg)
	}
	if ColorSelectedFg != lipgloss.Color("#c0caf5") {
		t.Errorf("dark empty cursor: ColorSelectedFg = %v, want #c0caf5", ColorSelectedFg)
	}
	if themeBgColor != "#1a1b26" {
		t.Errorf("dark empty bg: themeBgColor = %q, want #1a1b26", themeBgColor)
	}
}

func TestColorSelectedTextFgLightTheme(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "light"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	want := lipgloss.Color("#0f172a")
	if ColorSelectedTextFg != want {
		t.Errorf("light theme: ColorSelectedTextFg = %v, want %v", ColorSelectedTextFg, want)
	}
}

func TestColorSelectedTextFgDarkTheme(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	want := lipgloss.Color("#c0caf5")
	if ColorSelectedTextFg != want {
		t.Errorf("dark theme: ColorSelectedTextFg = %v, want %v", ColorSelectedTextFg, want)
	}
}

func TestAllPaletteColorVarsSetAfterDarkTheme(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	checks := []struct {
		name string
		got  color.Color
		want string
	}{
		{"ColorPrimary", ColorPrimary, "#7aa2f7"},
		{"ColorSecondary", ColorSecondary, "#7dcfff"},
		{"ColorSuccess", ColorSuccess, "#9ece6a"},
		{"ColorWarning", ColorWarning, "#e0af68"},
		{"ColorError", ColorError, "#f7768e"},
		{"ColorMuted", ColorMuted, "#8f98c7"},
		{"ColorDimmed", ColorDimmed, "#565f89"},
		{"ColorBorder", ColorBorder, "#565f89"},
		{"ColorAccent", ColorAccent, "#e0af68"},
		{"ColorProgressEmpty", ColorProgressEmpty, "#565f89"},
		{"ColorBadgeText", ColorBadgeText, "#8f98c7"},
		{"ColorLightGray", ColorLightGray, "#8f98c7"},
		{"ColorPriority1Title", ColorPriority1Title, "#f7768e"},
		{"ColorPriority2Title", ColorPriority2Title, "#e0af68"},
		{"ColorPriority3Title", ColorPriority3Title, "#7aa2f7"},
		{"ColorTaskContext", ColorTaskContext, "#73daca"},
		{"ColorTaskContextDeferred", ColorTaskContextDeferred, "#4f948f"},
		{"ColorSemanticBugTitle", ColorSemanticBugTitle, "#ff4f58"},
		{"ColorSemanticQuestionTitle", ColorSemanticQuestionTitle, "#f57a00"},
		{"ColorSemanticEpicTitle", ColorSemanticEpicTitle, "#e17bff"},
		{"ColorSemanticStoryTitle", ColorSemanticStoryTitle, "#3fc5ff"},
		{"ColorSemanticFeatureTitle", ColorSemanticFeatureTitle, "#35d67f"},
		{"ColorSemanticIdeaTitle", ColorSemanticIdeaTitle, "#ffd23f"},
		{"ColorSemanticBugSelectedTitle", ColorSemanticBugSelectedTitle, "#ff9ea3"},
		{"ColorSemanticQuestionSelectedTitle", ColorSemanticQuestionSelectedTitle, "#ffa366"},
		{"ColorSemanticEpicSelectedTitle", ColorSemanticEpicSelectedTitle, "#eeb6ff"},
		{"ColorSemanticStorySelectedTitle", ColorSemanticStorySelectedTitle, "#95dfff"},
		{"ColorSemanticFeatureSelectedTitle", ColorSemanticFeatureSelectedTitle, "#90e8b9"},
		{"ColorSemanticIdeaSelectedTitle", ColorSemanticIdeaSelectedTitle, "#ffe695"},
		{"ColorSemanticBugDeferredTitle", ColorSemanticBugDeferredTitle, "#c47074"},
		{"ColorSemanticQuestionDeferredTitle", ColorSemanticQuestionDeferredTitle, "#d66e29"},
		{"ColorSemanticEpicDeferredTitle", ColorSemanticEpicDeferredTitle, "#b16fc4"},
		{"ColorSemanticStoryDeferredTitle", ColorSemanticStoryDeferredTitle, "#4891b0"},
		{"ColorSemanticFeatureDeferredTitle", ColorSemanticFeatureDeferredTitle, "#549572"},
		{"ColorSemanticIdeaDeferredTitle", ColorSemanticIdeaDeferredTitle, "#9c8740"},
	}
	for _, c := range checks {
		if c.got != lipgloss.Color(c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestRebuildPaletteSemanticHierarchyOverrides(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "dark"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	rebuildPalette(config.PaletteColors{
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
		{"ColorSemanticEpicTitle", ColorSemanticEpicTitle, "#111111"},
		{"ColorSemanticStoryTitle", ColorSemanticStoryTitle, "#222222"},
		{"ColorSemanticEpicSelectedTitle", ColorSemanticEpicSelectedTitle, "#333333"},
		{"ColorSemanticStorySelectedTitle", ColorSemanticStorySelectedTitle, "#444444"},
		{"ColorSemanticEpicDeferredTitle", ColorSemanticEpicDeferredTitle, "#555555"},
		{"ColorSemanticStoryDeferredTitle", ColorSemanticStoryDeferredTitle, "#666666"},
	}
	for _, check := range checks {
		if check.got != lipgloss.Color(check.want) {
			t.Errorf("%s = %v, want %s", check.name, check.got, check.want)
		}
	}
}

func TestRebuildPaletteTaskContextOverrides(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "dark"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	rebuildPalette(config.PaletteColors{
		TaskContext:         "#111111",
		TaskContextDeferred: "#222222",
	})
	if ColorTaskContext != lipgloss.Color("#111111") || ColorTaskContextDeferred != lipgloss.Color("#222222") {
		t.Fatalf("context override = %v/%v", ColorTaskContext, ColorTaskContextDeferred)
	}
}
