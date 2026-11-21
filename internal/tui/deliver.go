package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

// The cues that keep work in blocks, in order and picked up where it was
// left. Each is derived when it is said: nothing here is stored but the
// check note, and nothing asks.
const (
	nearDays   = 3   // a delivery due within this many days, or overdue, is near
	bigMinutes = 120 // a task estimated past this with no task under it is too big
	nameCells  = 24  // how much of a row's text a cue names it by
)

// todo reports whether a row is a task still to do.
func todo(l sigil.Line) bool {
	return l.State == sigil.Open || l.State == sigil.Doing || l.State == sigil.Waiting
}

// delivery reports whether a row is a task still to do with a deadline.
func delivery(l sigil.Line) bool { return todo(l) && l.Due != "" }

// isNear reports whether a row is a delivery due within nearDays, or overdue.
func (m *Model) isNear(l sigil.Line) bool {
	return delivery(l) && l.Due <= m.today().AddDate(0, 0, nearDays).Format("2006-01-02")
}

// named is a row's text as a cue names it, cut to a few words' width.
func (m *Model) named(id string) string {
	r := m.tree.Row(id)
	if r == nil {
		return ""
	}
	return ansi.Truncate(sigil.FromFields(r.Fields).Text, nameCells, "…")
}

// byDay is a deadline as a cue says it: today, tomorrow, Fri or Nov 23, and
// today for one already past, since that is when it is now due.
func byDay(due string, today time.Time) string {
	if due <= today.Format("2006-01-02") {
		return "today"
	}
	s := dayWords(due, today)
	if len(s) == 3 {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

// nearBy is the day the last of the near deliveries is due by.
func (m *Model) nearBy() string {
	last := ""
	for _, id := range m.near {
		last = max(last, m.tree.Row(id).Fields[sigil.FieldDue])
	}
	return byDay(last, m.today())
}

// colliding is what Today's header says while two or more deliveries are
// near: how many, by when, and the soonest of them.
func (m *Model) colliding() string {
	if m.at.list != kindToday || len(m.near) < 2 {
		return ""
	}
	return strconv.Itoa(len(m.near)) + " due by " + m.nearBy() + " · first: " + m.named(m.near[0])
}

// rowSays is what the status line says of the cursor's row when nothing
// else is to be said: where its work was left off, what to check before it
// is delivered, and that its first step is too big or not written.
func (m *Model) rowSays() string {
	if m.cursor < 0 || m.cursor >= len(m.vis) {
		return ""
	}
	id := m.vis[m.cursor].id
	r := m.tree.Row(id)
	if r == nil {
		return ""
	}
	l := sigil.FromFields(r.Fields)
	var said []string
	if note := r.Fields[fieldResume]; note != "" {
		said = append(said, "left off: "+note)
	}
	near, due := m.isNear(l), ""
	if near {
		// The day is said once, before the first thing the delivery lacks.
		due = "due " + byDay(l.Due, m.today()) + ", "
		if check := r.Fields[fieldCheck]; check != "" {
			said = append(said, "check: "+check)
		} else {
			said = append(said, due+"no check written: C writes one")
			due = ""
		}
	}
	est, sized := estimateOf(l)
	whole := todo(l) && !m.taskUnder(id)
	switch {
	case whole && sized && est > bigMinutes && (l.State == sigil.Doing || m.inToday[id]):
		said = append(said, "est "+duration(est)+" and no step under it: Enter, then o writes a smaller first step")
	case whole && near && l.State != sigil.Doing:
		said = append(said, due+"first step not written: Enter, then o")
	case m.stuckPick() == 1:
		said = append(said, "picked yesterday: Enter, then o writes a smaller first step")
	case m.stuckPick() > 1:
		said = append(said, "picked "+strconv.Itoa(m.stuckPick())+" days ago: Enter, then o writes a smaller first step")
	}
	return strings.Join(said, " · ")
}

// writeCheck keeps, changes or takes off the note of what to verify before
// a row is delivered, as one undo step.
func (m *Model) writeCheck(id, note string) {
	if !m.live(id) {
		return
	}
	note = strings.Join(strings.Fields(note), " ")
	had := m.tree.Row(id).Fields[fieldCheck]
	if note == had {
		return
	}
	if m.do(outline.Op{Kind: outline.Set, Row: id, Field: fieldCheck, Value: note}) {
		switch {
		case note == "":
			m.msg = "check taken off"
		case had == "":
			m.msg = "noted what to check before it is delivered"
		default:
			m.msg = "check changed"
		}
	}
}

// bareBut is the doing rows with no left-off note, leaving out those a key
// just set doing: the work that was put down for them.
func (m *Model) bareBut(started map[string]bool) []string {
	var ids []string
	for _, id := range m.bare {
		if !started[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

// sayPutDown says that a change took rows off doing, to wait or for later,
// with no note of where their work stands.
func (m *Model) sayPutDown() {
	if m.msg != "" {
		return
	}
	n := 0
	for id := range m.closing.put {
		if r := m.tree.Row(id); r != nil && r.Fields[fieldResume] == "" {
			n++
		}
	}
	switch {
	case n == 1:
		m.msg = "put down with no left-off note: R writes one"
	case n > 1:
		m.msg = strconv.Itoa(n) + " rows put down with no left-off note: R writes one"
	}
}

// sayDue says that a deadline a change wrote lands among other near
// deliveries, at the moment the conflict is made.
func (m *Model) sayDue() {
	if m.msg != "" || len(m.closing.dued) == 0 {
		return
	}
	mine := 0
	for _, id := range m.near {
		if m.closing.dued[id] {
			mine++
		}
	}
	others := len(m.near) - mine
	if mine == 0 || others == 0 {
		return
	}
	m.msg = strconv.Itoa(others) + " other"
	if others > 1 {
		m.msg += "s"
	}
	m.msg += " due by " + m.nearBy() + ": /deliveries lists them"
}
