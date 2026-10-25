package tui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/theme"
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
		t.Fatal("clearing the text canceled the animation it had already earned")
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

	// A sticky message is the one case where an absent expiry is the answer
	// rather than a dropped one. No Outliner path writes one any more — the
	// multi-select summary moved to the context row — so the sticky half is
	// driven straight through the setter that marks it.
	sticky := app.captureFooterFlashProbe()
	app.crossOutliner.SetStatus("Comment change already in progress")
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
