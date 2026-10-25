package tui

import (
	"reflect"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/status"
)

// The harness gives every driver its own virtual clock. A timer scheduled
// through tickCmd reports itself to the driver instead of sleeping, so a
// scenario never pays wall-clock time for a status expiry, a flash frame or the
// periodic save, and no assertion turns on whether a timer beat the drain
// budget. A "wait" action advances the clock; timers the advance passes fire in
// deadline order.
//
// The scheduler below is installed once for the whole test binary and keeps no
// state of its own, so parallel scenarios each keep their own clock on their own
// driver.

func init() {
	status.TickScheduler = func(delay time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
		return func() tea.Msg {
			return uitestScheduledTick{delay: delay, fire: fire}
		}
	}
}

// animationCmdPackages are the imported packages whose commands exist only to
// animate: each sleeps out a frame and re-arms itself. tickCmd cannot virtualize
// them because the app never sees them — they come back from a bubbles Update —
// so the harness skips them outright and leaves the cursor and spinner in a
// fixed phase. A scenario asserts on text and layout, never on a frame, and a
// harness that animated would spend a cursor blink (530ms) per keystroke.
var animationCmdPackages = []string{
	"charm.land/bubbles/v2/cursor.",
	"charm.land/bubbles/v2/spinner.",
}

// isAnimationCmd reports whether cmd was produced by one of those packages,
// identifying it by the function behind the closure.
func isAnimationCmd(cmd tea.Cmd) bool {
	fn := runtime.FuncForPC(reflect.ValueOf(cmd).Pointer())
	if fn == nil {
		return false
	}
	for _, pkg := range animationCmdPackages {
		if strings.HasPrefix(fn.Name(), pkg) {
			return true
		}
	}
	return false
}

// uitestScheduledTick is the message a virtualized timer returns immediately in
// place of the delayed message it was scheduled to deliver.
type uitestScheduledTick struct {
	delay time.Duration
	fire  func(time.Time) tea.Msg
}

// uitestTimer is a scheduled tick waiting for its driver's clock to reach it.
// seq breaks ties so timers sharing a deadline fire in schedule order.
type uitestTimer struct {
	at   time.Time
	seq  uint64
	fire func(time.Time) tea.Msg
}

// schedule records a virtualized timer against the driver's clock.
func (d *UITestDriver) schedule(tick uitestScheduledTick) {
	d.timerSeq++
	d.timers = append(d.timers, uitestTimer{
		at:   d.now.Add(tick.delay),
		seq:  d.timerSeq,
		fire: tick.fire,
	})
}

// advanceClock moves the virtual clock forward by the given span and fires every
// timer it passes, in deadline order, draining what each one produces. A timer
// armed by a timer that just fired runs too when it also falls inside the span,
// which is how a footer flash animates to its last frame.
func (d *UITestDriver) advanceClock(by time.Duration) {
	deadline := d.now.Add(by)
	for {
		index, ok := d.nextTimerDueBy(deadline)
		if !ok {
			break
		}
		timer := d.timers[index]
		d.timers = append(d.timers[:index], d.timers[index+1:]...)
		d.now = timer.at
		d.processCmds(d.handleMsg(timer.fire(d.now)), d.drainOpts())
	}
	d.now = deadline
}

// nextTimerDueBy returns the index of the earliest timer due no later than
// deadline, or false when none is.
func (d *UITestDriver) nextTimerDueBy(deadline time.Time) (int, bool) {
	best := -1
	for i, timer := range d.timers {
		if timer.at.After(deadline) {
			continue
		}
		if best < 0 || timer.at.Before(d.timers[best].at) ||
			(timer.at.Equal(d.timers[best].at) && timer.seq < d.timers[best].seq) {
			best = i
		}
	}
	return best, best >= 0
}
