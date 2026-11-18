package tui

import (
	"strconv"
	"strings"
	"time"
)

// Farewell is the line left in the shell on quitting: what the day finished
// and what Today still holds, as `4 done today · 2 still in Today`, or
// nothing when the day has neither. The screen is gone by then, so it costs
// the chrome nothing (rule 6).
func (m *Model) Farewell() string {
	m.scan()
	done := m.doneToday
	var parts []string
	if done > 0 {
		parts = append(parts, strconv.Itoa(done)+" done today")
	}
	if left := m.todayLeft(); left > 0 {
		parts = append(parts, strconv.Itoa(left)+" still in Today")
	}
	return strings.Join(parts, " · ")
}

// todayLeft counts the picks not yet closed, as the sidebar does beside Today.
func (m *Model) todayLeft() int {
	for _, e := range m.side {
		if e.kind == kindToday {
			return e.count
		}
	}
	return 0
}

// dayOf is the local calendar day of a moment in unix milliseconds.
func (m *Model) dayOf(at int64) string {
	return time.UnixMilli(at).In(m.today().Location()).Format("2006-01-02")
}
