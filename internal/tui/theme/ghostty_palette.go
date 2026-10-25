package theme

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sglavoie/docket/internal/config"
)

// GhosttyPalette is the effective terminal palette sent by the macOS host.
// It is session-only; the user's saved TUI choice remains in machine state.
type GhosttyPalette struct {
	Name                string   `json:"name"`
	Background          string   `json:"background"`
	Foreground          string   `json:"foreground"`
	Cursor              string   `json:"cursor"`
	CursorText          string   `json:"cursor_text"`
	SelectionBackground string   `json:"selection_background"`
	SelectionForeground string   `json:"selection_foreground"`
	Palette             []string `json:"palette"`
}

var activeGhosttyPalette *GhosttyPalette

func (p GhosttyPalette) Validate() error {
	if p.Name == "" || len(p.Name) > 128 || strings.ContainsAny(p.Name, "\r\n\x00") {
		return fmt.Errorf("invalid theme name")
	}
	if len(p.Palette) != 16 {
		return fmt.Errorf("theme palette must contain 16 colors")
	}
	for _, hex := range append([]string{p.Background, p.Foreground, p.Cursor, p.CursorText,
		p.SelectionBackground, p.SelectionForeground}, p.Palette...) {
		if !validHex(hex) {
			return fmt.Errorf("invalid theme color %q", hex)
		}
	}
	return nil
}

// ApplyGhosttyPalette maps Ghostty's ANSI colors to tt's semantic roles and
// repaints the whole TUI. Ghostty's background and foreground are used exactly.
func ApplyGhosttyPalette(p GhosttyPalette) error {
	if err := p.Validate(); err != nil {
		return err
	}
	variant := "dark"
	if contrastHex(p.Background, "#000000") > contrastHex(p.Background, "#ffffff") {
		variant = "light"
	}
	bg := p.Background
	visible := func(index int) string { return legibleHex(p.Palette[index], bg) }
	selected := func(index int) string { return legibleHex(p.Palette[index+8], p.SelectionBackground) }
	deferred := func(index int) string { return blendHex(bg, visible(index), 0.72) }
	roles := config.PaletteColors{
		NormalText: p.Foreground,
		Muted:      legibleHex(blendHex(bg, p.Foreground, 0.7), bg),
		Dimmed:     blendHex(bg, p.Foreground, 0.45),
		DimmedBg:   blendHex(bg, p.Foreground, 0.12),
		DimmedFg:   blendHex(bg, p.Foreground, 0.76),
		Primary:    visible(4), Secondary: visible(6), Accent: visible(5),
		Success: visible(2), Warning: visible(3), Error: visible(1),
		Border:         blendHex(bg, p.Foreground, 0.48),
		Priority1Title: visible(1), Priority2Title: visible(3), Priority3Title: visible(4),
		TaskContext: visible(6), TaskContextDeferred: deferred(6),
		SemanticBugTitle: visible(1), SemanticQuestionTitle: visible(3),
		SemanticEpicTitle: visible(5), SemanticStoryTitle: visible(4),
		SemanticFeatureTitle: visible(2), SemanticIdeaTitle: visible(3),
		SemanticBugSelectedTitle: selected(1), SemanticQuestionSelectedTitle: selected(3),
		SemanticEpicSelectedTitle: selected(5), SemanticStorySelectedTitle: selected(4),
		SemanticFeatureSelectedTitle: selected(2), SemanticIdeaSelectedTitle: selected(3),
		SemanticBugDeferredTitle: deferred(1), SemanticQuestionDeferredTitle: deferred(3),
		SemanticEpicDeferredTitle: deferred(5), SemanticStoryDeferredTitle: deferred(4),
		SemanticFeatureDeferredTitle: deferred(2), SemanticIdeaDeferredTitle: deferred(3),
	}
	palette := config.Theme{
		Palette: roles, BackgroundColor: bg,
		CursorColor:     p.SelectionBackground,
		CursorTextColor: legibleHex(p.SelectionForeground, p.SelectionBackground),
	}
	ApplyTheme(ThemeConfig{Name: variant, Override: &palette, Ghostty: &p})
	return nil
}

func validHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(s[1:], 16, 24)
	return err == nil
}

func rgbHex(s string) (float64, float64, float64) {
	r, _ := strconv.ParseUint(s[1:3], 16, 8)
	g, _ := strconv.ParseUint(s[3:5], 16, 8)
	b, _ := strconv.ParseUint(s[5:7], 16, 8)
	return float64(r), float64(g), float64(b)
}

func blendHex(a, b string, weight float64) string {
	ar, ag, ab := rgbHex(a)
	br, bg, bb := rgbHex(b)
	return fmt.Sprintf("#%02x%02x%02x", int(math.Round(ar+(br-ar)*weight)),
		int(math.Round(ag+(bg-ag)*weight)), int(math.Round(ab+(bb-ab)*weight)))
}

func luminanceHex(s string) float64 {
	r, g, b := rgbHex(s)
	linear := func(v float64) float64 {
		v /= 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func contrastHex(a, b string) float64 {
	x, y := luminanceHex(a), luminanceHex(b)
	if x < y {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}

// legibleContrast is the WCAG AA ratio every derived foreground must reach.
const legibleContrast = 4.5

func legibleHex(fg, bg string) string {
	if contrastHex(fg, bg) >= legibleContrast {
		return fg
	}
	target := "#ffffff"
	if contrastHex(bg, "#000000") > contrastHex(bg, "#ffffff") {
		target = "#000000"
	}
	for step := 1; step <= 20; step++ {
		candidate := blendHex(fg, target, float64(step)/20)
		if contrastHex(candidate, bg) >= legibleContrast {
			return candidate
		}
	}
	return target
}
