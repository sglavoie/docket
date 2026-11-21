package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestWaitingAndStaleCountsGiveWayPastNinetyNine(t *testing.T) {
	h := start(t).wide()
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 16})
	rows := []string{"Project", "\tPick $open"}
	for i := range 120 {
		rows = append(rows, fmt.Sprintf("\tAsked %d $waiting", i))
	}
	h.pasteText(strings.Join(rows, "\n"))
	h.press("gg", "j", "t")
	h.wantSideHas("Today 1", "Waiting 99+")
	c := h.m.colors()
	raw := h.raw()
	// What is the user's to do is in the text colour; what waits on
	// others is dim.
	if !strings.Contains(raw, c.text+"1"+sgrReset) {
		t.Fatalf("Today's count is not in the text colour:\n%q", raw)
	}
	if !strings.Contains(raw, c.dim+"99+"+sgrReset) {
		t.Fatalf("Waiting's count is not dim:\n%q", raw)
	}
	// The list itself says how many exactly.
	h.press("f", `"Waiting"`, "enter")
	if got := h.header(); !strings.HasPrefix(got, "Waiting  · 120 waiting") {
		t.Fatalf("header = %q", got)
	}
	// Ninety-nine is still said as it is.
	h.press(":", `"all"`, "enter", "esc")
	h.press("f", `"Projects"`, "enter", "l", "j", "j", "2", "1", "d", "d")
	h.wantSideHas("Waiting 99")
}

// wantSideHas fails unless each line is one of the sidebar's.
func (h *harness) wantSideHas(lines ...string) {
	h.t.Helper()
	side := "\n" + h.side() + "\n"
	for _, l := range lines {
		if !strings.Contains(side, "\n"+l+"\n") {
			h.t.Fatalf("sidebar has no %q:\n%s", l, side)
		}
	}
}

func TestSmallFindsWhatCanBeStartedCold(t *testing.T) {
	h := start(t).wide()
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 16})
	h.pasteText(strings.Join([]string{
		"Project",
		"\tBig $open #est=2h",
		"\tFive $open #est=5m",
		"\tQuarter $open #est=15m",
		"\tTwenty $open #est=20m",
		"\tCall back $open #quick",
		"\tNot yet $open #quick @@+3d",
		"\tFinished $done #est=5m",
		"\tAsked $waiting #quick",
	}, "\n"))
	h.press("F", `"small"`, "enter")
	h.want(
		"▌ ○ Five #est=5m · Project",
		"  ○ Quarter #est=15m · Project",
		"  ○ Call back #quick · Project",
	)
}

func TestATagValueWithAUnitIsALengthOfTime(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"5m", "15m", -1},
		{"1h", "15m", 1},
		{"90m", "1h", 1},
		{"2h", "3", -1},
		{"10h", "3", 1},
		{"1d", "8h", 0},
		{"3", "10", -1},
		{"m", "n", -1},
		{"bob", "15m", 1},
	} {
		if got := compareValues(tc.a, tc.b); got != tc.want {
			t.Errorf("compareValues(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestAfterTimeAwayStartHereNamesASmallOne(t *testing.T) {
	h := start(t).wide()
	h.m.Update(tea.WindowSizeMsg{Width: 180, Height: 16})
	h.pasteText("Project\n\tPicked $open #est=1h\n\tEmail Ana $open #quick\n\tTax report $open @+2d")
	h.press("gg", "j", "t")
	if got := h.m.startHere(true); got != "start here: Tax report, due Tue" {
		t.Fatalf("a delivery still comes first: %q", got)
	}
	h.press("G", "x")
	if got := h.m.startHere(false); got != "start here: Picked, first in Today" {
		t.Fatalf("a day in: %q", got)
	}
	if got := h.m.startHere(true); got != "start here: Email Ana, a small one · /small lists more" {
		t.Fatalf("after time away: %q", got)
	}

	launch := func() string {
		tree, err := h.st.Load()
		if err != nil {
			t.Fatal(err)
		}
		h.m = Launch(tree, h.st, nil, h.now)
		h.m.Update(tea.WindowSizeMsg{Width: 220, Height: 16})
		return h.status()
	}
	launch()
	h.clock = h.clock.AddDate(0, 0, 4)
	if got := launch(); !strings.HasSuffix(got, "start here: Email Ana, a small one · /small lists more") {
		t.Fatalf("a launch after days away: %q", got)
	}
	// Coming back to the window after a day reads the same way, and after
	// an hour as before.
	away := func(d time.Duration) string {
		h.m.Update(tea.BlurMsg{})
		h.clock = h.clock.Add(d)
		h.m.Update(tea.FocusMsg{})
		return h.status()
	}
	if got := away(time.Hour); got != "back after 1h · start here: Picked, first in Today" {
		t.Fatalf("an hour away: %q", got)
	}
	if got := away(25 * time.Hour); got != "back after 1d · start here: Email Ana, a small one · /small lists more" {
		t.Fatalf("a day away: %q", got)
	}
}

func TestTheFirstLaunchOfAWeekLooksBack(t *testing.T) {
	h := start(t).wide()
	sunday := h.clock // Sunday, November 15
	launch := func() string {
		tree, err := h.st.Load()
		if err != nil {
			t.Fatal(err)
		}
		h.m = Launch(tree, h.st, nil, h.now)
		h.m.Update(tea.WindowSizeMsg{Width: 220, Height: 16})
		return h.status()
	}
	on := func(day time.Time, text string) {
		h.clock = day
		h.press("G", "H")
		h.pasteText(text)
		h.clock = sunday
	}
	// Last week ran from Monday the 2nd to Sunday the 8th.
	on(sunday.AddDate(0, 0, -12), "Alpha\n\tOne $done\n\tTwo $done\n\tLeft $open")
	on(sunday.AddDate(0, 0, -10), "Beta\n\tThree $done")
	on(sunday.AddDate(0, 0, -8), "Gamma\n\tFour $done")
	on(sunday.AddDate(0, 0, -14), "Delta\n\tBefore it $done")
	on(sunday.AddDate(0, 0, -1), "Epsilon\n\tThis week $done")
	if got := launch(); !strings.HasPrefix(got, "last week: 4 done in 3 projects · busiest Tue · ") {
		t.Fatalf("the first launch of the week: %q", got)
	}
	// It is said once a week.
	if got := launch(); strings.Contains(got, "last week") {
		t.Fatalf("a second launch said %q", got)
	}

	// A window left open into the next week says it on coming back to it,
	// once.
	h.clock = sunday.Add(2 * time.Hour)
	h.m.Update(tea.BlurMsg{})
	h.clock = sunday.AddDate(0, 0, 1).Add(-2 * time.Hour)
	h.m.Update(tea.FocusMsg{})
	if got := h.status(); !strings.HasPrefix(got, "back after 20h · last week: 1 done in 1 project · ") {
		t.Fatalf("coming back into a new week: %q", got)
	}
	h.m.Update(tea.BlurMsg{})
	h.clock = h.clock.Add(time.Hour)
	h.m.Update(tea.FocusMsg{})
	if got := h.status(); strings.Contains(got, "last week") {
		t.Fatalf("said twice in one week: %q", got)
	}

	// A week that finished nothing says nothing of itself.
	h.clock = sunday.AddDate(0, 0, 15)
	if got := launch(); strings.Contains(got, "last week") {
		t.Fatalf("an empty week said %q", got)
	}
}
