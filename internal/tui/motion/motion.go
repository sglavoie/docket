// Package motion holds the Outliner's paint-only animation runs: the smooth
// scroll, the fold roll-out, and the move trail. It owns their schedules, their
// generations, and the arithmetic each frame paints from; it owns no row, no
// cursor, and no window. The Outliner records what an operation is about to do
// as an intent, turns an intent into a run once the update that carried it has
// settled, and asks a run for its frame while it renders.
//
// Every run counts its frames through status.TickCmd and is voided by
// generation: a run started over a live one of the same kind takes a new
// sequence number, so the superseded run's queued ticks are inert. A Runs that
// is not armed records nothing and starts nothing, which is how a session
// without motion — the harness, every test, a user who turned it off — draws
// exactly the frames it would have drawn without this package.
package motion

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/status"
)

// Kind names one animation.
type Kind uint8

const (
	// Scroll paints intermediate windows between the one a page key left and
	// the one it arrived at.
	Scroll Kind = iota + 1
	// Reveal rolls the rows a fold opened out over two frames.
	Reveal
	// Trail lights the row a moved task landed on.
	Trail
)

const kinds = 3

// The schedules are the delay before each phase after the first ends, so a run
// of n phases paints its first frame on the update that started it and retires
// after the sum of its delays.
//
// Scroll and Reveal step at 20 and 24 ms. Bubble Tea's renderer coalesces
// frames at 60 fps, one every 16.7 ms, so a step any shorter could be drawn
// over before the terminal shows it; two intermediate frames at that cadence
// land the window in 40 to 48 ms, under the ~100 ms at which a key starts to
// feel late. The trail is read rather than tracked, so it takes the pulse's own
// cadence: 260 ms in the accent, then 340 ms settling, 600 ms in all.
var schedules = [kinds][]time.Duration{
	Scroll - 1: {20 * time.Millisecond, 20 * time.Millisecond},
	Reveal - 1: {24 * time.Millisecond, 24 * time.Millisecond},
	Trail - 1:  {260 * time.Millisecond, 340 * time.Millisecond},
}

// Phases is the number of frames a kind paints before it settles.
func Phases(kind Kind) int { return len(schedules[kind-1]) }

// Duration is a kind's whole run, first frame to retirement.
func Duration(kind Kind) time.Duration {
	var total time.Duration
	for _, step := range schedules[kind-1] {
		total += step
	}
	return total
}

// TickMsg advances the run of Kind that Seq names; a stale one is inert.
type TickMsg struct {
	Kind Kind
	Seq  int
}

type run struct {
	live  bool
	phase int
	seq   int
}

// Runs is every run one view holds, and every intent waiting to become one.
type Runs struct {
	armed bool
	seq   int
	runs  [kinds]run

	scrollFrom int
	revealed   []bool
	trailTask  string

	scrollIntent     bool
	scrollIntentFrom int
	revealIntent     map[string]struct{}
	trailIntent      string
	trailIntentPlace string
}

// SetArmed arms or disarms every run. Disarming retires every run and intent.
func (b *Runs) SetArmed(armed bool) {
	b.armed = armed
	if !armed {
		b.Drop()
	}
}

// Armed reports whether the runs animate.
func (b *Runs) Armed() bool { return b.armed }

// Drop retires every run and every intent at once: input means the user has
// moved on, so the next frame paints the settled state.
func (b *Runs) Drop() {
	for index := range b.runs {
		b.runs[index] = run{}
	}
	b.revealed = nil
	b.scrollIntent = false
	b.revealIntent = nil
	b.trailIntent, b.trailIntentPlace = "", ""
}

// Live reports whether a run of kind is painting, and its phase.
func (b *Runs) Live(kind Kind) (phase int, ok bool) {
	r := b.runs[kind-1]
	return r.phase, r.live
}

func (b *Runs) start(kind Kind) tea.Cmd {
	b.seq++
	b.runs[kind-1] = run{live: true, seq: b.seq}
	return tick(kind, 0, b.seq)
}

func tick(kind Kind, phase, seq int) tea.Cmd {
	return status.TickCmd(schedules[kind-1][phase], func(time.Time) tea.Msg {
		return TickMsg{Kind: kind, Seq: seq}
	})
}

// Advance moves the run msg names to its next phase, retiring it after its
// last, and returns the tick that ends the phase it moved to.
func (b *Runs) Advance(msg TickMsg) tea.Cmd {
	if msg.Kind < Scroll || msg.Kind > Trail {
		return nil
	}
	r := &b.runs[msg.Kind-1]
	if !r.live || r.seq != msg.Seq {
		return nil
	}
	r.phase++
	if r.phase >= Phases(msg.Kind) {
		*r = run{}
		if msg.Kind == Reveal {
			b.revealed = nil
		}
		return nil
	}
	return tick(msg.Kind, r.phase, msg.Seq)
}

// NoteScroll records that a page key is about to move the cursor from a window
// whose first row is from.
func (b *Runs) NoteScroll(from int) {
	if b.armed {
		b.scrollIntent, b.scrollIntentFrom = true, from
	}
}

// TakeScroll consumes the scroll intent.
func (b *Runs) TakeScroll() (from int, ok bool) {
	from, ok = b.scrollIntentFrom, b.scrollIntent
	b.scrollIntent = false
	return from, ok
}

// StartScroll paints from the window starting at from toward the settled one.
func (b *Runs) StartScroll(from int) tea.Cmd {
	if !b.armed {
		return nil
	}
	b.scrollFrom = from
	return b.start(Scroll)
}

// ScrollFrom is the first row of the window a live scroll left, and its phase.
func (b *Runs) ScrollFrom() (from, phase int, ok bool) {
	phase, ok = b.Live(Scroll)
	return b.scrollFrom, phase, ok
}

// NoteReveal records the rows drawn before a fold operation, by key.
func (b *Runs) NoteReveal(before map[string]struct{}) {
	if b.armed {
		b.revealIntent = before
	}
}

// TakeReveal consumes the reveal intent.
func (b *Runs) TakeReveal() (before map[string]struct{}, ok bool) {
	before, b.revealIntent = b.revealIntent, nil
	return before, before != nil
}

// StartReveal rolls out the rows revealed marks; revealed is indexed like the
// rows it was computed from.
func (b *Runs) StartReveal(revealed []bool) tea.Cmd {
	if !b.armed {
		return nil
	}
	b.revealed = revealed
	return b.start(Reveal)
}

// RevealHidden reports which rows a live reveal still withholds this frame,
// or nil. rows is the projection's current length: a projection rebuilt to a
// different length mid-run is not the one the mask names, so it withholds
// nothing.
func (b *Runs) RevealHidden(rows int) []bool {
	phase, ok := b.Live(Reveal)
	if !ok || len(b.revealed) != rows {
		return nil
	}
	return RevealHidden(b.revealed, phase)
}

// NoteTrail records the task a move is aimed at and the place it held.
func (b *Runs) NoteTrail(task, place string) {
	if b.armed && task != "" {
		b.trailIntent, b.trailIntentPlace = task, place
	}
}

// TrailIntent is the task a move was aimed at, and the place last seen for it.
func (b *Runs) TrailIntent() (task, place string, ok bool) {
	return b.trailIntent, b.trailIntentPlace, b.trailIntent != ""
}

// StartTrail lights task at place, which becomes the place a further move is
// measured from: the intent stays standing until input, so a burst of moves
// whose commits land after their keys re-arms once per landing.
func (b *Runs) StartTrail(task, place string) tea.Cmd {
	if !b.armed {
		return nil
	}
	b.trailIntentPlace = place
	b.trailTask = task
	return b.start(Trail)
}

// TrailTask is the task a live trail lights, and its phase.
func (b *Runs) TrailTask() (task string, phase int, ok bool) {
	phase, ok = b.Live(Trail)
	return b.trailTask, phase, ok
}
