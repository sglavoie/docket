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
		for _, tag := range l.Tags {
			if sigil.TagName(tag) == "est" {
				t.minutes, t.estimated = estimateMinutes(sigil.TagValue(tag))
				break
			}
		}
	}
	return t
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
		minutes := math.Round(t.minutes)
		hours, remainder := math.Floor(minutes/60), math.Mod(minutes, 60)
		duration := fmt.Sprintf("%.0fm", remainder)
		if hours > 0 {
			duration = fmt.Sprintf("%.0fh", hours)
			if remainder > 0 {
				duration += fmt.Sprintf("%.0fm", remainder)
			}
		}
		s += " · " + duration
	}
	return s
}
