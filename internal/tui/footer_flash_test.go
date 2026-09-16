package tui

import (
	"errors"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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

// armWarningFlash raises a warning through the same probe pair AppModel.Update's
// deferred closure holds, and returns the generation it armed. A warning has no
// message type of its own — the shell's warnings are set, not sent — so this is
// the one route to a live animation that does not go through an error.
func armWarningFlash(t *testing.T, app *AppModel, msg string) uint64 {
	t.Helper()
	before := app.captureFooterFlashProbe()
	app.setWarningStatus(msg)
	if cmd := app.armFooterFlashSince(before); cmd == nil {
		t.Fatalf("a warning armed no flash: %q", msg)
	}
	if !app.feedback.flash.active() {
		t.Fatalf("a warning left no live flash: %+v", app.feedback.flash)
	}
	return app.feedback.flash.seq
}

// TestFooterFlashStaysQuietOnRoutineShellStatus is the rule this whole file is
// arranged around: the footer says what happened and the divider does not move
// for it. Both routine surfaces are named, because a success and an
// informational line reach the decision through different severities and only
// the severity tells them apart.
func TestFooterFlashStaysQuietOnRoutineShellStatus(t *testing.T) {
	for _, routine := range []struct {
		name string
		msg  tea.Msg
		text string
		want footerFlashSeverity
	}{
		{name: "success", msg: status.StatusMsg("task saved"), text: "task saved", want: flashSuccess},
		{name: "info", msg: status.InfoStatusMsg("filters cleared"), text: "filters cleared", want: flashInfo},
	} {
		t.Run(routine.name, func(t *testing.T) {
			app := flashTestApp(t)

			app.Update(routine.msg)

			// Delivery is untouched: the line is on the footer and carries the
			// severity that decided it was quiet in the first place.
			if app.feedback.message != routine.text {
				t.Fatalf("status = %q, want %q", app.feedback.message, routine.text)
			}
			if app.feedback.severity != routine.want {
				t.Fatalf("recorded severity = %v, want %v", app.feedback.severity, routine.want)
			}
			if app.feedback.flash.active() {
				t.Fatalf("a routine %s animated the divider: %+v", routine.name, app.feedback.flash)
			}
			if app.feedback.flash.seq != 0 {
				t.Fatalf("a routine %s spent flash generation %d", routine.name, app.feedback.flash.seq)
			}
			if got := app.footerFlashBackgroundColor(); !sameColor(got, theme.ColorBackground) {
				t.Fatalf("a routine %s washed the footer background %v", routine.name, got)
			}
		})
	}
}

func TestFooterFlashArmsOnError(t *testing.T) {
	app := flashTestApp(t)

	_, cmd := app.Update(status.ErrorMsg{Err: errors.New("storage failed")})

	if cmd == nil {
		t.Fatal("expected a command batch carrying the first flash tick")
	}
	if got := app.feedback.flash.severity; got != flashError {
		t.Fatalf("severity = %v, want flashError", got)
	}
	if app.feedback.flash.frame != 0 {
		t.Fatalf("frame = %d, want 0", app.feedback.flash.frame)
	}
}

// TestFooterFlashArmsOnWarningStatus is the other half of the emphasis that
// survives. A warning keeps the neutral status surface, so the divider is the
// only thing on screen distinguishing it from the routine line it replaced.
func TestFooterFlashArmsOnWarningStatus(t *testing.T) {
	app := flashTestApp(t)

	seq := armWarningFlash(t, app, "wq is not a workflow action")

	if got := app.feedback.flash.severity; got != flashWarning {
		t.Fatalf("severity = %v, want flashWarning", got)
	}
	if app.feedback.flash.frame != 0 {
		t.Fatalf("frame = %d, want 0", app.feedback.flash.frame)
	}
	if seq == 0 {
		t.Fatal("expected a non-zero flash generation")
	}
}

func TestFooterFlashDoesNotArmOnStatusClear(t *testing.T) {
	app := flashTestApp(t)
	armedSeq := armWarningFlash(t, app, "wq is not a workflow action")

	app.Update(status.StatusClearMsg{})

	if app.feedback.message != "" {
		t.Fatalf("status not cleared: %q", app.feedback.message)
	}
	if app.feedback.flash.seq != armedSeq {
		t.Fatalf("clearing status re-armed the flash: seq %d -> %d", armedSeq, app.feedback.flash.seq)
	}
	// Expiry is the message's clock, not the animation's: the warning's remaining
	// frames belong to the warning, and a text clear neither arms nor retires.
	if !app.feedback.flash.active() {
		t.Fatal("clearing the text cancelled the animation it had already earned")
	}
}

// TestQuietFeedbackRetiresAnAnimatedWarning is the supersession. The warning's
// glyph and wash are that warning's; once a routine line has taken the footer,
// leaving them running would say the routine line is the alert.
func TestQuietFeedbackRetiresAnAnimatedWarning(t *testing.T) {
	app := flashTestApp(t)
	warned := armWarningFlash(t, app, "wq is not a workflow action")
	app.Update(FooterFlashTickMsg{Seq: warned})
	if app.feedback.flash.frame != 1 {
		t.Fatalf("the warning is at frame %d, so it was not animating when it was superseded", app.feedback.flash.frame)
	}

	app.Update(status.StatusMsg("task saved"))

	if app.feedback.flash.active() {
		t.Fatalf("the superseded warning is still animating: %+v", app.feedback.flash)
	}
	if app.feedback.flash.seq != 0 {
		t.Fatalf("quiet feedback armed flash generation %d", app.feedback.flash.seq)
	}
	if got := app.footerFlashBackgroundColor(); !sameColor(got, theme.ColorBackground) {
		t.Fatalf("the superseded warning's wash survived it: %v", got)
	}
	// Its queued ticks are invalidated rather than merely ignored once: the
	// generation moved on, so none of the frames still in flight can restart it.
	_, cmd := app.Update(FooterFlashTickMsg{Seq: warned})
	if cmd != nil || app.feedback.flash.active() {
		t.Fatalf("a queued tick from the retired warning revived it: %+v", app.feedback.flash)
	}
	if app.feedback.flashSeq <= warned {
		t.Fatalf("flash generation = %d, want it past the retired %d", app.feedback.flashSeq, warned)
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
	seq := armWarningFlash(t, app, "wq is not a workflow action")

	for frame := 1; frame < footerFlashFrames; frame++ {
		_, cmd := app.Update(FooterFlashTickMsg{Seq: seq})
		if app.feedback.flash.frame != frame {
			t.Fatalf("frame = %d, want %d", app.feedback.flash.frame, frame)
		}
		if cmd == nil {
			t.Fatalf("frame %d returned no follow-up tick", frame)
		}
	}

	_, cmd := app.Update(FooterFlashTickMsg{Seq: seq})
	if app.feedback.flash.active() {
		t.Fatalf("flash still active after %d frames: %+v", footerFlashFrames, app.feedback.flash)
	}
	if cmd != nil {
		t.Fatal("expected no follow-up tick once the flash is spent")
	}
}

func TestFooterFlashIgnoresStaleTick(t *testing.T) {
	app := flashTestApp(t)
	seq := armWarningFlash(t, app, "wq is not a workflow action")

	_, cmd := app.Update(FooterFlashTickMsg{Seq: seq - 1})

	if app.feedback.flash.frame != 0 {
		t.Fatalf("stale tick advanced the frame to %d", app.feedback.flash.frame)
	}
	if cmd != nil {
		t.Fatal("stale tick scheduled another frame")
	}
}

// TestFooterFlashRestartsOnANewWarning keeps the repeated-message case honest:
// a second warning is a second thing to notice, so it restarts rather than
// riding the first one's remaining frames, and the first one's ticks stop
// counting the moment it does.
func TestFooterFlashRestartsOnANewWarning(t *testing.T) {
	app := flashTestApp(t)
	firstSeq := armWarningFlash(t, app, "wq is not a workflow action")
	app.Update(FooterFlashTickMsg{Seq: firstSeq})

	secondSeq := armWarningFlash(t, app, "fq is not a filter action")

	if app.feedback.flash.frame != 0 {
		t.Fatalf("frame = %d, want the restart to reset to 0", app.feedback.flash.frame)
	}
	if app.feedback.flash.severity != flashWarning {
		t.Fatalf("severity = %v, want flashWarning", app.feedback.flash.severity)
	}
	if secondSeq == firstSeq {
		t.Fatal("expected a new flash generation to supersede the old ticks")
	}

	app.Update(FooterFlashTickMsg{Seq: firstSeq})
	if app.feedback.flash.frame != 0 {
		t.Fatalf("superseded tick advanced the restarted flash to frame %d", app.feedback.flash.frame)
	}
}

// TestFooterFlashStaysQuietOnViewStatusLine covers the second status surface.
// A view owns a line the shell never writes, and it reaches the same decision
// through the view stamp rather than the shell's sequence — so it has to be
// asserted separately or a view notice could stay loud on its own path.
func TestFooterFlashStaysQuietOnViewStatusLine(t *testing.T) {
	app := flashTestApp(t)
	warned := armWarningFlash(t, app, "wq is not a workflow action")

	before := app.captureFooterFlashProbe()
	app.crossOutliner.SetStatus("Exit search before changing modes")
	cmd := app.armFooterFlashSince(before)

	if cmd != nil {
		t.Fatal("the Outliner's own status line armed a flash")
	}
	if app.feedback.flash.active() {
		t.Fatalf("a view notice left the divider animating: %+v", app.feedback.flash)
	}
	// It supersedes on the view's path too: the warning underneath is no longer
	// the newest thing said, so its animation goes with it.
	if app.feedback.flashSeq <= warned {
		t.Fatalf("flash generation = %d, want it past the retired %d", app.feedback.flashSeq, warned)
	}
	if got := app.crossOutliner.StatusMessage; got != "Exit search before changing modes" {
		t.Fatalf("the notice itself was lost: %q", got)
	}
}

// TestViewStatusExpiryStaysIndependentOfTheFlash pins the two clocks apart. They
// read one probe and answer different questions, so a notice that arms nothing
// still expires on time, and a sticky one still expires never.
func TestViewStatusExpiryStaysIndependentOfTheFlash(t *testing.T) {
	app := flashTestApp(t)

	before := app.captureFooterFlashProbe()
	app.crossOutliner.SetTransientStatus("Exit search before changing modes")
	flash := app.armFooterFlashSince(before)
	expiry := app.scheduleViewStatusExpiry(before)

	if flash != nil {
		t.Fatal("a transient view notice armed a flash")
	}
	if expiry == nil {
		t.Fatal("a quiet transient notice lost its expiry, so quietness changed how long it lasts")
	}

	// The multi-select summary is the sticky exception, and it is the one case
	// where an absent expiry is the answer rather than a dropped one.
	sticky := app.captureFooterFlashProbe()
	app.crossOutliner.SetStatus("3 selected")
	if cmd := app.scheduleViewStatusExpiry(sticky); cmd != nil {
		t.Fatal("a sticky summary scheduled its own dismissal")
	}
}

// TestFooterFlashSeverityAnimatesOnlyWarningAndError states the split as the
// closed table it is, so adding a severity is a decision about motion rather
// than an inheritance from whichever branch it was written next to.
func TestFooterFlashSeverityAnimatesOnlyWarningAndError(t *testing.T) {
	t.Parallel()

	for severity, want := range map[footerFlashSeverity]bool{
		flashNone:    false,
		flashInfo:    false,
		flashSuccess: false,
		flashWarning: true,
		flashError:   true,
	} {
		if got := severity.animates(); got != want {
			t.Errorf("severity %v animates = %v, want %v", severity, got, want)
		}
	}
}

func TestSetWarningStatusRecordsWarningSeverity(t *testing.T) {
	app := flashTestApp(t)

	app.setWarningStatus("wq is not a workflow action")

	if app.feedback.severity != flashWarning {
		t.Fatalf("feedback.severity = %v, want flashWarning", app.feedback.severity)
	}
	if !app.feedback.info {
		t.Fatal("warning text must keep the neutral status surface")
	}
}

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
