package tui

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestFooterDividerAnimationPhases(t *testing.T) {
	app := flashTestApp(t)
	segmentWidth := footerFlashSegmentWidth(app.viewNav.width)

	glyphsAt := func(frame int) []rune {
		t.Helper()
		app.feedback.flash = footerFlashState{severity: flashError, seq: 1, frame: frame}
		glyphs := []rune(ansi.Strip(app.renderFooterDivider(app.viewNav.width)))
		if len(glyphs) != app.viewNav.width {
			t.Fatalf("frame %d divider width = %d, want %d", frame, len(glyphs), app.viewNav.width)
		}
		return glyphs
	}
	countHeavy := func(glyphs []rune) int {
		count := 0
		for _, glyph := range glyphs {
			if glyph == footerDividerFlashGlyph {
				count++
			}
		}
		return count
	}

	app.feedback.flash = footerFlashState{}
	idle := []rune(ansi.Strip(app.renderFooterDivider(app.viewNav.width)))
	if countHeavy(idle) != 0 || idle[0] != footerDividerIdleGlyph || idle[len(idle)-1] != footerDividerIdleGlyph {
		t.Fatalf("idle divider glyphs = %q, want all thin", string(idle))
	}

	impact := glyphsAt(0)
	if got := countHeavy(impact); got != app.viewNav.width {
		t.Fatalf("impact heavy cells = %d, want %d", got, app.viewNav.width)
	}
	wantImpact := chrome.RenderHorizontalDivider(
		app.viewNav.width,
		chrome.WithDividerColor(theme.ColorError),
		chrome.WithDividerGlyph(footerDividerFlashGlyph),
	)
	if got := app.renderFooterDivider(app.viewNav.width); got != wantImpact {
		t.Fatalf("impact divider = %q, want %q", got, wantImpact)
	}

	sweepStart := glyphsAt(footerFlashImpactFrames)
	if got := countHeavy(sweepStart); got != segmentWidth || sweepStart[0] != footerDividerFlashGlyph || sweepStart[len(sweepStart)-1] != footerDividerIdleGlyph {
		t.Fatalf("sweep start heavy=%d endpoints=%q/%q, want %d heavy at left", got, sweepStart[0], sweepStart[len(sweepStart)-1], segmentWidth)
	}

	sweepMid := glyphsAt(footerFlashImpactFrames + footerFlashSweepFrames/2)
	if got := countHeavy(sweepMid); got != segmentWidth || sweepMid[0] != footerDividerIdleGlyph || sweepMid[len(sweepMid)-1] != footerDividerIdleGlyph {
		t.Fatalf("sweep midpoint heavy=%d endpoints=%q/%q, want %d heavy in middle", got, sweepMid[0], sweepMid[len(sweepMid)-1], segmentWidth)
	}

	sweepEnd := glyphsAt(footerFlashImpactFrames + footerFlashSweepFrames - 1)
	if got := countHeavy(sweepEnd); got != segmentWidth || sweepEnd[0] != footerDividerIdleGlyph || sweepEnd[len(sweepEnd)-1] != footerDividerFlashGlyph {
		t.Fatalf("sweep end heavy=%d endpoints=%q/%q, want %d heavy at right", got, sweepEnd[0], sweepEnd[len(sweepEnd)-1], segmentWidth)
	}

	settleStart := glyphsAt(footerFlashImpactFrames + footerFlashSweepFrames)
	settleEnd := glyphsAt(footerFlashFrames - 1)
	if startHeavy, endHeavy := countHeavy(settleStart), countHeavy(settleEnd); startHeavy >= segmentWidth || endHeavy >= startHeavy || endHeavy < 1 {
		t.Fatalf("settle heavy cells %d -> %d, want shrinking positive trail below %d", startHeavy, endHeavy, segmentWidth)
	}
	if settleStart[len(settleStart)-1] != footerDividerFlashGlyph || settleEnd[len(settleEnd)-1] != footerDividerFlashGlyph {
		t.Fatal("settle trail must stay anchored to the right edge")
	}

	app.feedback.flash = footerFlashState{}
	cleared := []rune(ansi.Strip(app.renderFooterDivider(app.viewNav.width)))
	if countHeavy(cleared) != 0 {
		t.Fatalf("cleared divider glyphs = %q, want all thin", string(cleared))
	}
}

func TestFooterBackgroundFlashFadesBackToTheme(t *testing.T) {
	app := flashTestApp(t)
	background := theme.ColorBackground

	if got := app.footerFlashBackgroundColor(); !sameColor(got, background) {
		t.Fatalf("idle background = %v, want theme background %v", got, background)
	}

	app.feedback.flash = footerFlashState{severity: flashError, seq: 1}
	wantImpact := blendColor(background, theme.ColorError, footerFlashBackgroundMix)
	if got := app.footerFlashBackgroundColor(); !sameColor(got, wantImpact) {
		t.Fatalf("impact background = %v, want %v", got, wantImpact)
	}

	app.feedback.flash.frame = footerFlashFrames / 2
	mid := app.footerFlashBackgroundColor()
	if sameColor(mid, wantImpact) || sameColor(mid, background) {
		t.Fatalf("midpoint background = %v, want a partial fade", mid)
	}

	app.feedback.flash.frame = footerFlashFrames - 1
	if got := app.footerFlashBackgroundColor(); !sameColor(got, background) {
		t.Fatalf("final background = %v, want theme background %v", got, background)
	}
}

func TestFooterBackgroundFlashFillsRowsAndSurvivesChildResets(t *testing.T) {
	app := flashTestApp(t)
	app.viewNav.width = 24
	// A warning, because the wash only ever paints under a severity that
	// animates; a success state here would describe a frame nothing can reach.
	app.feedback.flash = footerFlashState{severity: flashWarning, seq: 1}

	content := theme.SuccessStyle.Render("saved") + " plain\n" + theme.HelpStyle.Render("Esc close")
	got := app.renderFooterContent(content, app.viewNav.width)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("rendered rows = %d, want 2", len(lines))
	}

	prefix, _ := footerBackgroundStyleParts(app.footerFlashBackgroundColor())
	for index, line := range lines {
		if width := lipgloss.Width(line); width != app.viewNav.width {
			t.Fatalf("row %d width = %d, want %d", index, width, app.viewNav.width)
		}
		if occurrences := strings.Count(line, prefix); occurrences < 2 {
			t.Fatalf("row %d restored background %d times, want at least 2 in %q", index, occurrences, line)
		}
	}
	if plain := ansi.Strip(got); plain != "saved plain             \nEsc close               " {
		t.Fatalf("plain footer = %q", plain)
	}
}

func TestStatusBarDividerCarriesFlashAppearanceOnEveryPath(t *testing.T) {
	app := flashTestApp(t)
	app.feedback.flash = footerFlashState{
		severity: flashError,
		seq:      1,
		frame:    footerFlashImpactFrames + footerFlashSweepFrames/2,
	}
	wantDivider := app.renderFooterDivider(app.viewNav.width)

	assertDivider := func(name string) {
		t.Helper()
		got, content, _ := strings.Cut(app.renderStatusBar(), "\n")
		if got != wantDivider {
			t.Fatalf("%s divider = %q, want the flash appearance %q", name, got, wantDivider)
		}
		for index, line := range strings.Split(content, "\n") {
			if width := lipgloss.Width(line); width != app.viewNav.width {
				t.Fatalf("%s content row %d width = %d, want %d", name, index, width, app.viewNav.width)
			}
		}
	}

	app.feedback.message = "task saved"
	assertDivider("status message")

	app.operations.Chord = []string{"w"}
	assertDivider("pending chord")
	app.operations.Chord = nil

	app.feedback.message = ""
	assertDivider("contextual guidance")
}

func TestBlendColorEndpointsAndMidpoint(t *testing.T) {
	from := lipgloss.Color("#000000")
	to := lipgloss.Color("#ffffff")

	if got := blendColor(from, to, 0); !sameColor(got, from) {
		t.Fatalf("t=0 gave %v, want %v", got, from)
	}
	if got := blendColor(from, to, 1); !sameColor(got, to) {
		t.Fatalf("t=1 gave %v, want %v", got, to)
	}
	if got := blendColor(from, to, 0.5); !sameColor(got, lipgloss.Color("#808080")) {
		t.Fatalf("t=0.5 gave %v, want #808080", got)
	}
	if got := blendColor(from, to, 4); !sameColor(got, to) {
		t.Fatalf("t=4 clamped to %v, want %v", got, to)
	}
	if got := blendColor(nil, to, 0); !sameColor(got, to) {
		t.Fatalf("nil from gave %v, want %v", got, to)
	}
	if got := blendColor(from, nil, 1); !sameColor(got, from) {
		t.Fatalf("nil to gave %v, want %v", got, from)
	}
}

func TestBlendColorStepsThroughTranslucentInput(t *testing.T) {
	from := color.RGBA{R: 0x80, G: 0x00, B: 0x00, A: 0x80}
	to := lipgloss.Color("#ffffff")

	if got := blendColor(from, to, 0.4); !sameColor(got, from) {
		t.Fatalf("t=0.4 gave %v, want the translucent source", got)
	}
	if got := blendColor(from, to, 0.6); !sameColor(got, to) {
		t.Fatalf("t=0.6 gave %v, want %v", got, to)
	}
}

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
