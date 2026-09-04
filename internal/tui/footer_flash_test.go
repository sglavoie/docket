package tui

import (
	"errors"
	"image/color"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func flashTestApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	app.viewNav.width = 100
	app.viewNav.height = 35
	app.crossOutliner.SetSize(app.viewNav.viewSize())
	return app
}

func TestFooterFlashArmsOnShellStatusUpdate(t *testing.T) {
	app := flashTestApp(t)

	_, cmd := app.Update(status.StatusMsg("task saved"))

	if cmd == nil {
		t.Fatal("expected a command batch carrying the first flash tick")
	}
	if got := app.footerFlash.severity; got != flashSuccess {
		t.Fatalf("severity = %v, want flashSuccess", got)
	}
	if app.footerFlash.frame != 0 {
		t.Fatalf("frame = %d, want 0", app.footerFlash.frame)
	}
	if app.footerFlash.seq == 0 {
		t.Fatal("expected a non-zero flash generation")
	}
}

func TestFooterFlashArmsOnError(t *testing.T) {
	app := flashTestApp(t)

	app.Update(status.ErrorMsg{Err: errors.New("storage failed")})

	if got := app.footerFlash.severity; got != flashError {
		t.Fatalf("severity = %v, want flashError", got)
	}
}

func TestFooterFlashDoesNotArmOnStatusClear(t *testing.T) {
	app := flashTestApp(t)
	app.Update(status.StatusMsg("task saved"))
	armedSeq := app.footerFlash.seq

	app.Update(status.StatusClearMsg{})

	if app.statusMessage != "" {
		t.Fatalf("status not cleared: %q", app.statusMessage)
	}
	if app.footerFlash.seq != armedSeq {
		t.Fatalf("clearing status re-armed the flash: seq %d -> %d", armedSeq, app.footerFlash.seq)
	}
}

func TestFooterFlashAdvancesThroughFramesThenClears(t *testing.T) {
	if got := time.Duration(footerFlashFrames) * footerFlashFrameInterval; got != 500*time.Millisecond {
		t.Fatalf("flash duration = %s, want 500ms", got)
	}
	if got := footerFlashImpactFrames + footerFlashSweepFrames + footerFlashSettleFrames; got != footerFlashFrames {
		t.Fatalf("phase frames = %d, want %d", got, footerFlashFrames)
	}

	app := flashTestApp(t)
	app.Update(status.StatusMsg("task saved"))
	seq := app.footerFlash.seq

	for frame := 1; frame < footerFlashFrames; frame++ {
		_, cmd := app.Update(FooterFlashTickMsg{Seq: seq})
		if app.footerFlash.frame != frame {
			t.Fatalf("frame = %d, want %d", app.footerFlash.frame, frame)
		}
		if cmd == nil {
			t.Fatalf("frame %d returned no follow-up tick", frame)
		}
	}

	_, cmd := app.Update(FooterFlashTickMsg{Seq: seq})
	if app.footerFlash.active() {
		t.Fatalf("flash still active after %d frames: %+v", footerFlashFrames, app.footerFlash)
	}
	if cmd != nil {
		t.Fatal("expected no follow-up tick once the flash is spent")
	}
}

func TestFooterFlashIgnoresStaleTick(t *testing.T) {
	app := flashTestApp(t)
	app.Update(status.StatusMsg("task saved"))
	seq := app.footerFlash.seq

	_, cmd := app.Update(FooterFlashTickMsg{Seq: seq - 1})

	if app.footerFlash.frame != 0 {
		t.Fatalf("stale tick advanced the frame to %d", app.footerFlash.frame)
	}
	if cmd != nil {
		t.Fatal("stale tick scheduled another frame")
	}
}

func TestFooterFlashRestartsOnNewStatus(t *testing.T) {
	app := flashTestApp(t)
	app.Update(status.StatusMsg("task saved"))
	firstSeq := app.footerFlash.seq
	app.Update(FooterFlashTickMsg{Seq: firstSeq})

	app.Update(status.InfoStatusMsg("filters cleared"))

	if app.footerFlash.frame != 0 {
		t.Fatalf("frame = %d, want the restart to reset to 0", app.footerFlash.frame)
	}
	if app.footerFlash.severity != flashInfo {
		t.Fatalf("severity = %v, want flashInfo", app.footerFlash.severity)
	}
	if app.footerFlash.seq == firstSeq {
		t.Fatal("expected a new flash generation to supersede the old ticks")
	}

	app.Update(FooterFlashTickMsg{Seq: firstSeq})
	if app.footerFlash.frame != 0 {
		t.Fatalf("superseded tick advanced the restarted flash to frame %d", app.footerFlash.frame)
	}
}

func TestFooterFlashArmsOnViewStatusLine(t *testing.T) {
	app := flashTestApp(t)

	before := app.captureFooterFlashProbe()
	app.crossOutliner.SetStatus("Exit search before changing modes")
	cmd := app.armFooterFlashSince(before)

	if cmd == nil {
		t.Fatal("expected the Outliner status line to arm a flash")
	}
	if got := app.footerFlash.severity; got != flashInfo {
		t.Fatalf("severity = %v, want flashInfo", got)
	}
}

func TestSetWarningStatusRecordsWarningSeverity(t *testing.T) {
	app := flashTestApp(t)

	app.setWarningStatus("$q is not a workflow action")

	if app.statusSeverity != flashWarning {
		t.Fatalf("statusSeverity = %v, want flashWarning", app.statusSeverity)
	}
	if !app.statusIsInfo {
		t.Fatal("warning text must keep the neutral status surface")
	}
}

func TestFooterDividerAnimationPhases(t *testing.T) {
	app := flashTestApp(t)
	segmentWidth := footerFlashSegmentWidth(app.viewNav.width)

	glyphsAt := func(frame int) []rune {
		t.Helper()
		app.footerFlash = footerFlashState{severity: flashError, seq: 1, frame: frame}
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

	app.footerFlash = footerFlashState{}
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

	app.footerFlash = footerFlashState{}
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

	app.footerFlash = footerFlashState{severity: flashError, seq: 1}
	wantImpact := blendColor(background, theme.ColorError, footerFlashBackgroundMix)
	if got := app.footerFlashBackgroundColor(); !sameColor(got, wantImpact) {
		t.Fatalf("impact background = %v, want %v", got, wantImpact)
	}

	app.footerFlash.frame = footerFlashFrames / 2
	mid := app.footerFlashBackgroundColor()
	if sameColor(mid, wantImpact) || sameColor(mid, background) {
		t.Fatalf("midpoint background = %v, want a partial fade", mid)
	}

	app.footerFlash.frame = footerFlashFrames - 1
	if got := app.footerFlashBackgroundColor(); !sameColor(got, background) {
		t.Fatalf("final background = %v, want theme background %v", got, background)
	}
}

func TestFooterBackgroundFlashFillsRowsAndSurvivesChildResets(t *testing.T) {
	app := flashTestApp(t)
	app.viewNav.width = 24
	app.footerFlash = footerFlashState{severity: flashSuccess, seq: 1}

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
	app.footerFlash = footerFlashState{
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

	app.statusMessage = "task saved"
	assertDivider("status message")

	app.operations.chord = []string{"$"}
	assertDivider("pending chord")
	app.operations.chord = nil

	app.statusMessage = ""
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
