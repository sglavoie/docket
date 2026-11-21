package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/sigil"
)

// The outline rewards what is finished, takes the user back in after time
// away, and names one row to start on when nothing is in hand. Nothing here
// is stored in the outline, and no key is added.

// metaSeen is the last day the outline was open on this machine, so a
// launch after days away is told from one the morning after.
const metaSeen = "seen"

// backAfter is how long the window must have been away before coming back
// to it reads the work in hand back, and awayFrom how many days must have
// passed since the outline was last open for a launch to say so.
const (
	backAfter = 10 * time.Minute
	awayFrom  = 2
)

// bars are the heights a day of the week is drawn in, lowest first.
var bars = []rune("▁▂▃▄▅▆▇█")

// weekBars draws the rows done on each of the last seven days, today last,
// each against the busiest of them; a day that finished nothing is a blank,
// and nothing counts the days in a row. It is said beside the usual day,
// once there is enough history for that, dim, with today's bar in the text
// colour.
func (m *Model) weekBars() string {
	top := 0
	for _, n := range m.week {
		top = max(top, n)
	}
	if m.usualDay == 0 || top == 0 {
		return ""
	}
	c := m.colors()
	var b strings.Builder
	b.WriteString(c.dim + "7d ")
	for i, n := range m.week {
		glyph := ' '
		if n > 0 {
			glyph = bars[(n*len(bars)+top-1)/top-1]
		}
		if i == len(m.week)-1 {
			b.WriteString(sgrReset)
		}
		b.WriteRune(glyph)
	}
	return b.String() + sgrReset
}

// closeFlashFor is how long a finished row is lit. Tests set it to nothing,
// as they do a copy's flash, and turn it on where they look for it.
var closeFlashFor = 200 * time.Millisecond

// lightClosed lights the rows a change the user made has just finished, for
// as long as a copy lights what it took, so a row done is seen to be done.
// An undo, a sync pass and the day turning over light nothing.
func (m *Model) lightClosed() {
	if !m.recorded || len(m.closing.rows) == 0 || closeFlashFor == 0 {
		return
	}
	m.flashed = m.closing.rows
	m.flashGen++
	gen := m.flashGen
	m.pendingCmd = tea.Batch(m.pendingCmd, tea.Tick(closeFlashFor, func(time.Time) tea.Msg { return flashMsg{gen} }))
}

// doingSays names the work in hand, led by lead, as `doing: Draft report ·
// left off: …`, or how many rows are, as `3 rows doing: /$doing lists them`.
// It is blank while nothing is doing.
func (m *Model) doingSays(lead string) string {
	m.scan()
	if m.doing > 1 {
		return strconv.Itoa(m.doing) + " rows " + lead + ": /$doing lists them"
	}
	if m.doing == 0 {
		return ""
	}
	var find func(id string) string
	find = func(id string) string {
		for _, kid := range m.tree.Children(id) {
			if m.tree.Row(kid).Fields[sigil.FieldState] == sigil.Doing {
				return kid
			}
			if got := find(kid); got != "" {
				return got
			}
		}
		return ""
	}
	id := find("")
	if id == "" {
		return ""
	}
	said := lead + ": " + m.named(id)
	if note := m.tree.Row(id).Fields[fieldResume]; note != "" {
		said += " · left off: " + note
	}
	return said
}

// startHere names one row to begin with when nothing is in hand: the
// soonest near delivery still open, else the first pick still open in
// Today's order, else the first row /suggest finds. A row waiting on
// someone else is never the one named.
func (m *Model) startHere() string {
	m.scan()
	open := func(id string) bool {
		r := m.tree.Row(id)
		return m.live(id) && r.Fields[sigil.FieldState] == sigil.Open
	}
	for _, id := range m.near {
		if open(id) {
			return "start here: " + m.named(id) + ", due " + byDay(m.tree.Row(id).Fields[sigil.FieldDue], m.today())
		}
	}
	for _, id := range m.lists[kindToday] {
		if open(id) {
			return "start here: " + m.named(id) + ", first in Today"
		}
	}
	for _, id := range m.found(builtinQuery("suggest")) {
		if open(id) {
			return "start here: " + m.named(id) + " · /suggest lists more"
		}
	}
	return ""
}

// builtinQuery is the query a built-in filter reads.
func builtinQuery(name string) string {
	for _, f := range builtins {
		if f.Name == name {
			return f.Query
		}
	}
	return ""
}

// found is what a query finds flat over Projects, leaving the pane's place
// as it was.
func (m *Model) found(query string) []string {
	held := m.at
	defer func() { m.at = held }()
	m.at = place{query: query, flat: true}
	m.refilter()
	var ids []string
	for _, hit := range m.at.hits {
		if m.live(hit.id) {
			ids = append(ids, hit.id)
		}
	}
	return ids
}

// sayOpening is what the outline says when it is opened: after days away,
// how long and what slipped meanwhile; then what was left doing, or, with
// nothing in hand, where to start.
func (m *Model) sayOpening() {
	var said []string
	if days := m.awayDays(); days >= awayFrom {
		said = append(said, "away "+strconv.Itoa(days)+" days")
		if n := len(m.missed); n == 1 {
			said = append(said, "1 pick carried")
		} else if n > 1 {
			said = append(said, strconv.Itoa(n)+" picks carried")
		}
		if n := len(m.found(builtinQuery("slipping"))); n > 0 {
			said = append(said, strconv.Itoa(n)+" slipping: /slipping lists them")
		}
	}
	if doing := m.doingSays("left doing"); doing != "" {
		said = append(said, doing)
	} else if start := m.startHere(); start != "" {
		said = append(said, start)
	}
	m.msg = strings.Join(said, " · ")
	m.markSeen()
}

// awayDays is how many days have turned since the outline was last open on
// this machine, or 0 when that is not known.
func (m *Model) awayDays() int {
	seen, err := m.backend.Meta(metaSeen)
	if err != nil || seen == "" {
		return 0
	}
	then, err := time.ParseInLocation("2006-01-02", seen, m.today().Location())
	if err != nil {
		return 0
	}
	return int(m.today().Sub(then).Hours()/24 + 0.5)
}

// markSeen notes that the outline is open today, once a day.
func (m *Model) markSeen() {
	if day := m.today().Format("2006-01-02"); day != m.seen {
		if m.backend.SetMeta(metaSeen, day) == nil {
			m.seen = day
		}
	}
}

// welcomeBack reads the work in hand back when the window has its focus
// again after a while away, as `back after 25m · doing: Draft report · left
// off: …`, or names where to start when nothing is in hand. It says nothing
// over a message that something was not written, and nothing when there is
// nothing to take up.
func (m *Model) welcomeBack() {
	away := time.Duration(m.now()-m.blurred) * time.Millisecond
	if m.blurred == 0 || away < backAfter || failing(m.msg) {
		m.blurred = 0
		return
	}
	m.blurred = 0
	said := m.doingSays("doing")
	if said == "" {
		said = m.startHere()
	}
	if said == "" {
		return
	}
	m.msg = "back after " + awayFor(away) + " · " + said
}

// awayFor spells a time away as 25m, 2h10m or, from a day, 3d.
func awayFor(d time.Duration) string {
	if d >= 24*time.Hour {
		return strconv.Itoa(int(d/(24*time.Hour))) + "d"
	}
	return duration(d.Minutes())
}

// staleHint is what Stale says when nothing else is to be said: a list
// this long is a weight, and parking it all is one undo away.
const staleHint = ":all, then $p parks every row here · u brings them back"
