package tui

import (
	"image/color"
	"testing"
)

// TestNextActionStripGlyphColorsMirrorUrgencyTier locks the session 13
// scan-quality contract: the strip's glyph is recolored per kind so the
// urgency tier reads pre-attentively (red for blocked, yellow for triage,
// green for caught-up, accent for default action / first-run / plan). The
// rest of the strip layout, copy, and ANSI distinctness rules are unchanged.
func TestNextActionStripGlyphColorsMirrorUrgencyTier(t *testing.T) {
	cases := []struct {
		name string
		copy nextActionStripCopy
		want color.Color
	}{
		{
			name: "WorkOnTask uses accent glyph",
			copy: nextActionStripCopy{kind: RecommendationKindWorkOnTask},
			want: ColorAccent,
		},
		{
			name: "PlanToday uses accent glyph",
			copy: nextActionStripCopy{kind: RecommendationKindPlanToday},
			want: ColorAccent,
		},
		{
			name: "ReviewBlocked uses error red glyph",
			copy: nextActionStripCopy{kind: RecommendationKindReviewBlocked},
			want: ColorError,
		},
		{
			name: "TriageInbox uses warning yellow glyph",
			copy: nextActionStripCopy{kind: RecommendationKindTriageInbox},
			want: ColorWarning,
		},
		{
			name: "AllDone caught-up uses success green glyph",
			copy: nextActionStripCopy{kind: RecommendationKindAllDone, variant: stripVariantCaughtUp},
			want: ColorSuccess,
		},
		{
			name: "AllDone first-run uses accent glyph",
			copy: nextActionStripCopy{kind: RecommendationKindAllDone, variant: stripVariantFirstRun},
			want: ColorAccent,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripGlyphColorFor(tc.copy)
			if got != tc.want {
				t.Fatalf("stripGlyphColorFor(%v) = %v, want %v", tc.copy, got, tc.want)
			}
		})
	}
}

// TestNextActionStripGlyphStyleAcrossThemes verifies the glyph color contract
// survives a theme swap. ApplyTheme rebuilds the palette, so the color
// constants change identity between light/dark — but the glyph-style selector
// must continue to route each kind to its semantic token rather than caching
// a stale dark-theme hex.
func TestNextActionStripGlyphStyleAcrossThemes(t *testing.T) {
	for _, themeName := range []string{"", "light"} {
		t.Run("theme="+themeOrDefault(themeName), func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: themeName})
			t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

			cases := []struct {
				kind    RecommendationKind
				variant string
				want    color.Color
			}{
				{RecommendationKindReviewBlocked, stripVariantDefault, ColorError},
				{RecommendationKindTriageInbox, stripVariantDefault, ColorWarning},
				{RecommendationKindAllDone, stripVariantCaughtUp, ColorSuccess},
				{RecommendationKindWorkOnTask, stripVariantDefault, ColorAccent},
			}
			for _, c := range cases {
				got := stripGlyphColorFor(nextActionStripCopy{kind: c.kind, variant: c.variant})
				if got != c.want {
					t.Fatalf("kind=%v variant=%q: got %v, want %v", c.kind, c.variant, got, c.want)
				}
			}
		})
	}
}

func themeOrDefault(name string) string {
	if name == "" {
		return "dark"
	}
	return name
}
