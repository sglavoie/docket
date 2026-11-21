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
var closeFlashFor = 300 * time.Millisecond

// closeSweep is what a finished row's glyph is drawn as while it is lit, in
// the accent, an equal share of the light each: its circle fills and turns
// to the check. None of them is a state's own glyph, so no frame is read as
// one; the check stays, dimmed, once the light is out.
var closeSweep = []string{"◍", "◉", "✓"}

// sweepMsg draws frame of closeSweep, from 1, unless a later light has
// taken over.
type sweepMsg struct{ gen, frame int }

// lightClosed lights the rows a change the user made has just finished, for
// a moment, and sweeps their glyphs to the check, so a row done is seen to
// be done. With Bell on it rings the terminal's bell once, which Docket.app
// plays as a sound when its setting asks for one. An undo, a sync pass and
// the day turning over light nothing and ring nothing.
func (m *Model) lightClosed() {
	if !m.recorded || len(m.closing.rows) == 0 {
		return
	}
	if m.Bell {
		m.pendingCmd = tea.Batch(m.pendingCmd, tea.Raw("\a"))
	}
	if closeFlashFor == 0 {
		return
	}
	m.flashed = m.closing.rows
	m.flashGen++
	m.closeFrame = 1
	gen := m.flashGen
	step := closeFlashFor / time.Duration(len(closeSweep))
	cmds := []tea.Cmd{m.pendingCmd}
	for frame := 2; frame <= len(closeSweep); frame++ {
		cmds = append(cmds, tea.Tick(step*time.Duration(frame-1), func(time.Time) tea.Msg { return sweepMsg{gen, frame} }))
	}
	cmds = append(cmds, tea.Tick(closeFlashFor, func(time.Time) tea.Msg { return flashMsg{gen} }))
	m.pendingCmd = tea.Batch(cmds...)
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
// soonest near delivery still open, else, after time away, the first row
// /small finds, since a cold start goes best on a small step, else the
// first pick still open in Today's order, else the first row /suggest
// finds. A row waiting on someone else is never the one named.
func (m *Model) startHere(away bool) string {
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
	if away {
		for _, id := range m.found(builtinQuery("small")) {
			if open(id) {
				return "start here: " + m.named(id) + ", a small one · /small lists more"
			}
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
	if back := m.lookBack(); back != "" {
		said = append(said, back)
	}
	days := m.awayDays()
	if days >= awayFrom {
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
	} else if start := m.startHere(days >= awayFrom); start != "" {
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
	var said []string
	if back := m.lookBack(); back != "" {
		said = append(said, back)
	}
	if doing := m.doingSays("doing"); doing != "" {
		said = append(said, doing)
	} else if start := m.startHere(away >= 24*time.Hour); start != "" {
		said = append(said, start)
	}
	if len(said) == 0 {
		return
	}
	m.msg = "back after " + awayFor(away) + " · " + strings.Join(said, " · ")
}

// metaLookedBack is the Monday of the week whose look back was last said
// on this machine, so it is said once a week.
const metaLookedBack = "looked-back"

// lookBack says, once a week, what the week before finished: `last week:
// 23 done in 9 projects · busiest Tue`. It is said at the first launch of
// the week, or on first coming back to a window left open into it. A week
// that finished nothing says nothing, and nothing counts weeks in a row.
func (m *Model) lookBack() string {
	today := m.today()
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	week := monday.Format("2006-01-02")
	if m.lookedBack == week {
		return ""
	}
	m.lookedBack = week
	if said, err := m.backend.Meta(metaLookedBack); err != nil || said == week {
		return ""
	}
	if m.backend.SetMeta(metaLookedBack, week) != nil {
		return ""
	}
	from := monday.AddDate(0, 0, -7).Format("2006-01-02")
	done := 0
	var onDay [7]int
	projects := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		for _, kid := range m.tree.Children(id) {
			r := m.tree.Row(kid)
			if r.Deleted() || kid == filtersID {
				continue
			}
			if r.Fields[sigil.FieldState] == sigil.Done {
				if on := m.dayOf(m.changedAt(kid)); on >= from && on < week {
					done++
					day, _ := time.ParseInLocation("2006-01-02", on, today.Location())
					onDay[(int(day.Weekday())+6)%7]++
					if p := m.projectOf(kid); p != nil && p.ID != inboxID && p.ID != kid {
						projects[p.ID] = true
					}
				}
			}
			walk(kid)
		}
	}
	walk("")
	if done == 0 {
		return ""
	}
	said := "last week: " + strconv.Itoa(done) + " done"
	switch n := len(projects); n {
	case 0:
	case 1:
		said += " in 1 project"
	default:
		said += " in " + strconv.Itoa(n) + " projects"
	}
	busiest, days := 0, 0
	for i, n := range onDay {
		if n > 0 {
			days++
		}
		if n > onDay[busiest] {
			busiest = i
		}
	}
	if days > 1 {
		said += " · busiest " + monday.AddDate(0, 0, busiest).Format("Mon")
	}
	return said
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
