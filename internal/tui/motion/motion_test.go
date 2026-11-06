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

func TestCollapseRowsShrinksTheGapAndRoundsUp(t *testing.T) {
	for _, tc := range []struct {
		rows int
		want []int
	}{
		{rows: 6, want: []int{4, 2, 0}},
		{rows: 1, want: []int{1, 1, 0}},
		{rows: 0, want: []int{0, 0, 0}},
	} {
		var got []int
		for phase := 0; phase <= Phases(Collapse); phase++ {
			got = append(got, CollapseRows(tc.rows, phase))
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("CollapseRows(%d) = %v, want %v", tc.rows, got, tc.want)
		}
	}
}

func TestDockRowsRollsTheBodyOutTopFirst(t *testing.T) {
	var got []int
	for phase := 0; phase <= Phases(Dock); phase++ {
		got = append(got, DockRows(9, phase))
	}
	if want := []int{3, 6, 9}; !slices.Equal(got, want) {
		t.Fatalf("DockRows(9) = %v, want %v", got, want)
	}
	if DockRows(1, 0) != 1 {
		t.Fatal("a one-row body is always drawn")
	}
}

func TestCollapseAndDockRunsRetireAndDropWithInput(t *testing.T) {
	var runs Runs
	if runs.StartCollapse([]Gap{{After: 0, Rows: 3}}) != nil || runs.StartDock() != nil {
		t.Fatal("an unarmed runs started a collapse or a dock")
	}
	runs.SetArmed(true)
	if runs.StartCollapse(nil) != nil {
		t.Fatal("a collapse with no gap started")
	}
	runs.StartCollapse([]Gap{{After: 1, Rows: 3}})
	if got := runs.CollapseGaps(4); len(got) != 1 || got[0] != (Gap{After: 1, Rows: 2}) {
		t.Fatalf("phase 0 gaps = %v, want one gap of two rows under row 1", got)
	}
	if runs.CollapseGaps(1) != nil {
		t.Fatal("a gap past the rebuilt rows still draws")
	}
	runs.Advance(TickMsg{Kind: Collapse, Seq: runs.seq})
	if got := runs.CollapseGaps(4); len(got) != 1 || got[0].Rows != 1 {
		t.Fatalf("phase 1 gaps = %v, want one row", got)
	}
	runs.Advance(TickMsg{Kind: Collapse, Seq: runs.seq})
	if runs.CollapseGaps(4) != nil {
		t.Fatal("a spent collapse still draws a gap")
	}
	runs.StartDock()
	if phase, ok := runs.DockPhase(); !ok || phase != 0 {
		t.Fatalf("dock phase = %d (live %t), want a live phase 0", phase, ok)
	}
	runs.Drop()
	if _, ok := runs.DockPhase(); ok {
		t.Fatal("input left the dock rolling")
	}
}
