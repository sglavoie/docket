package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sglavoie/docket/internal/sigil"
)

// Totals are view state. scan accumulates descendants once; rebuild sums
// only the held rows for a list or filter, without counting their children.
type taskTotals struct {
	todo, done, total int
	waiting           int
	minutes           float64
	estimated         bool
}

type rowTotals struct {
	own, under taskTotals
}

func (t *taskTotals) add(other taskTotals) {
	t.todo += other.todo
	t.done += other.done
	t.total += other.total
	t.waiting += other.waiting
	t.minutes += other.minutes
	t.estimated = t.estimated || other.estimated
}

func countTask(l sigil.Line) taskTotals {
	var t taskTotals
	switch l.State {
	case sigil.Done:
		t.done, t.total = 1, 1
	case sigil.Open, sigil.Doing, sigil.Waiting:
		t.todo, t.total = 1, 1
		if l.State == sigil.Waiting {
			t.waiting = 1
		}
		t.minutes, t.estimated = estimateOf(l)
	}
	return t
}

// estimateOf reads a row's first #est, in minutes, and whether it is
// readable.
func estimateOf(l sigil.Line) (float64, bool) {
	for _, tag := range l.Tags {
		if sigil.TagName(tag) == "est" {
			return estimateMinutes(sigil.TagValue(tag))
		}
	}
	return 0, false
}

func estimateMinutes(value string) (float64, bool) {
	scale := 60.0
	if value != "" {
		switch value[len(value)-1] {
		case 'm':
			scale, value = 1, strings.TrimSuffix(value, "m")
		case 'h':
			value = strings.TrimSuffix(value, "h")
		case 'd':
			scale, value = 8*60, strings.TrimSuffix(value, "d")
		}
	}
	n, err := strconv.ParseFloat(value, 64)
	minutes := n * scale
	if err != nil || n < 0 || math.IsNaN(minutes) || math.IsInf(minutes, 0) {
		return 0, false
	}
	return minutes, true
}

func (t taskTotals) summary() string { return t.said(t.todo, "to do") }

// said counts n rows as what they are, then the estimate left to do.
func (t taskTotals) said(n int, what string) string {
	if n == 0 {
		return ""
	}
	s := fmt.Sprintf("%d %s", n, what)
	if t.estimated {
		s += " · " + duration(t.minutes)
	}
	return s
}

// duration spells minutes as hours and minutes, as 1h40m, 2h or 25m.
func duration(minutes float64) string {
	minutes = math.Round(minutes)
	hours, remainder := math.Floor(minutes/60), math.Mod(minutes, 60)
	if hours == 0 {
		return fmt.Sprintf("%.0fm", remainder)
	}
	if remainder == 0 {
		return fmt.Sprintf("%.0fh", hours)
	}
	return fmt.Sprintf("%.0fh%.0fm", hours, remainder)
}

// doingAge is what a doing row says of how long it has been doing: `· 1h40m`
// when it went doing today, from one minute up, in the warning colour once
// past a readable #est, and `· since Nov 18` when it went doing on an
// earlier day. It is read from changed and the clock when drawn.
func (m *Model) doingAge(id string, l sigil.Line) (word, bool) {
	since := m.changedAt(id)
	if day := m.dayOf(since); day != m.dayOf(m.now()) {
		return word{m.colors().dim, "· since " + dayWords(day, m.today())}, true
	}
	minutes := math.Floor(float64(m.now()-since) / 60000)
	if minutes < 1 {
		return word{}, false
	}
	style := m.colors().dim
	if est, ok := estimateOf(l); ok && minutes > est {
		style = m.colors().warn
	}
	return word{style, "· " + duration(minutes)}, true
}

// took is what closing one doing row says it took: `done in 1h40m`, with
// its #est beside it when readable, or the day it went doing when that was
// an earlier day.
func (m *Model) took(since int64, l sigil.Line) string {
	if day := m.dayOf(since); day != m.dayOf(m.now()) {
		return "done · doing since " + dayWords(day, m.today())
	}
	s := "done in " + duration(math.Floor(float64(max(m.now()-since, 0))/60000))
	if est, ok := estimateOf(l); ok {
		s += " · est " + duration(est)
	}
	return s
}
