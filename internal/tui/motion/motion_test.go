package motion

import (
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestScrollStartEasesBetweenWindowsAndEndsOnTheSettledOne(t *testing.T) {
	for _, tc := range []struct {
		from, to int
		want     []int
	}{
		{from: 0, to: 10, want: []int{5, 8, 10}},
		{from: 10, to: 0, want: []int{5, 2, 0}},
		{from: 4, to: 6, want: []int{5, 5, 6}},
	} {
		var got []int
		for phase := 0; phase <= Phases(Scroll); phase++ {
			got = append(got, ScrollStart(tc.from, tc.to, phase))
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("ScrollStart(%d→%d) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestRevealHiddenWithholdsEachRunsTail(t *testing.T) {
	revealed := []bool{false, true, true, true, true, true, true, false, true}
	first := RevealHidden(revealed, 0)
	// A run of six shows two, then four; a run of one is always drawn.
	if want := []bool{false, false, false, true, true, true, true, false, false}; !slices.Equal(first, want) {
		t.Fatalf("phase 0 = %v, want %v", first, want)
	}
	second := RevealHidden(revealed, 1)
	if want := []bool{false, false, false, false, false, true, true, false, false}; !slices.Equal(second, want) {
		t.Fatalf("phase 1 = %v, want %v", second, want)
	}
	if RevealHidden(revealed, Phases(Reveal)) != nil {
		t.Fatal("a spent reveal still withholds rows")
	}
}

func TestBoardRecordsNothingUnarmed(t *testing.T) {
	var runs Runs
	runs.NoteScroll(3)
	runs.NoteReveal(map[string]struct{}{"a": {}})
	runs.NoteTrail("task", "place")
	if _, ok := runs.TakeScroll(); ok {
		t.Fatal("an unarmed runs recorded a scroll")
	}
	if _, ok := runs.TakeReveal(); ok {
		t.Fatal("an unarmed runs recorded a reveal")
	}
	if _, _, ok := runs.TrailIntent(); ok {
		t.Fatal("an unarmed runs recorded a trail")
	}
	for _, cmd := range []tea.Cmd{runs.StartScroll(1), runs.StartReveal([]bool{true}), runs.StartTrail("t", "p")} {
		if cmd != nil {
			t.Fatal("an unarmed runs started a run")
		}
	}
}

func TestBoardRunsRetireAfterTheirPhasesAndVoidSupersededTicks(t *testing.T) {
	var runs Runs
	runs.SetArmed(true)
	runs.NoteTrail("a", "p0")
	if runs.StartTrail("a", "p0") == nil {
		t.Fatal("no tick for the first trail")
	}
	stale := TickMsg{Kind: Trail, Seq: runs.seq}
	runs.StartTrail("a", "p1")
	if runs.Advance(stale) != nil {
		t.Fatal("a superseded run's tick advanced the live one")
	}
	if _, phase, ok := runs.TrailTask(); !ok || phase != 0 {
		t.Fatalf("stale tick moved the live trail to phase %d (live %t)", phase, ok)
	}
	live := TickMsg{Kind: Trail, Seq: runs.seq}
	if runs.Advance(live) == nil {
		t.Fatal("the first phase scheduled no second")
	}
	if _, phase, _ := runs.TrailTask(); phase != 1 {
		t.Fatalf("phase = %d, want 1", phase)
	}
	if runs.Advance(live) != nil {
		t.Fatal("a spent run scheduled another tick")
	}
	if _, _, ok := runs.TrailTask(); ok {
		t.Fatal("the trail outlived its phases")
	}
	if _, place, ok := runs.TrailIntent(); !ok || place != "p1" {
		t.Fatalf("intent = %q %t, want it standing at the landed place", place, ok)
	}
}

func TestBoardDropAndDisarmRetireEverything(t *testing.T) {
	var runs Runs
	runs.SetArmed(true)
	runs.StartScroll(2)
	runs.StartReveal([]bool{true, true, true})
	runs.NoteTrail("a", "p")
	runs.StartTrail("a", "p")
	runs.Drop()
	for _, kind := range []Kind{Scroll, Reveal, Trail} {
		if _, ok := runs.Live(kind); ok {
			t.Fatalf("kind %d survived Drop", kind)
		}
	}
	if _, _, ok := runs.TrailIntent(); ok {
		t.Fatal("the trail intent survived Drop")
	}
	runs.StartScroll(2)
	runs.SetArmed(false)
	if _, ok := runs.Live(Scroll); ok {
		t.Fatal("a scroll survived disarming")
	}
}

func TestRevealMaskIgnoresAReshapedProjection(t *testing.T) {
	var runs Runs
	runs.SetArmed(true)
	runs.StartReveal([]bool{false, true, true, true})
	if runs.RevealHidden(4) == nil {
		t.Fatal("a live reveal withheld nothing")
	}
	if runs.RevealHidden(5) != nil {
		t.Fatal("a reveal withheld rows of a projection it was not computed for")
	}
}

func TestDurationsStayShortOfALateKey(t *testing.T) {
	if runDuration(Scroll) > 60*time.Millisecond || runDuration(Reveal) > 60*time.Millisecond {
		t.Fatalf("scroll %v, reveal %v: a navigation frame should settle within 60ms", runDuration(Scroll), runDuration(Reveal))
	}
	if runDuration(Trail) != 600*time.Millisecond {
		t.Fatalf("trail = %v, want 600ms", runDuration(Trail))
	}
}

// runDuration is a kind's whole run, first frame to retirement.
func runDuration(kind Kind) time.Duration {
	var total time.Duration
	for _, step := range schedules[kind-1] {
		total += step
	}
	return total
}
