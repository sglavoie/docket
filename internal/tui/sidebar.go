package tui

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

// The Inbox is an ordinary row with a fixed identity, so every machine agrees
// on it. Its position sorts before any key Between can mint, which keeps it
// first among the top-level rows, where it is never drawn.
const (
	inboxID  = "inbox"
	inboxPos = "0"
)

const (
	kindInbox    = "inbox"
	kindToday    = "today"
	kindWaiting  = "waiting"
	kindStale    = "stale"
	kindProjects = "projects"
	kindProject  = "project"
)

const (
	staleAfter  = 14 * 24 * time.Hour
	dueSoonDays = 7
	sidebarCols = 24
	sidebarFrom = 72 // the terminal width from which the sidebar is always drawn
)

// place is what the outline pane shows: the rows under root, or a flat list
// whose rows were fixed when it was opened. A list never drops a row because
// its state changed under the cursor; it is read again when reopened.
type place struct {
	root string
	list string
	rows []string

	// A filter narrows the place to the rows answering query. Like a list,
	// it holds the rows it found until the query changes.
	query   string
	flat    bool
	hits    []visRow
	matched map[string]bool
}

// entry is one line of the sidebar, and one destination.
type entry struct {
	kind, id, label string
	count           int  // lists: rows that still need something
	age             int  // projects: whole days since anything inside changed
	due             int  // projects: 1 a deadline within a week, 2 one overdue
	idle            bool // projects: no task left to do
}

func (e entry) same(o entry) bool { return e.kind == o.kind && e.id == o.id }

// ensureInbox creates the Inbox row on first launch. It is not an undo step.
func (m *Model) ensureInbox() {
	if m.tree.Row(inboxID) != nil {
		return
	}
	ops := []outline.Op{
		{Kind: outline.Insert, Row: inboxID, Pos: inboxPos},
		{Kind: outline.Set, Row: inboxID, Field: sigil.FieldText, Value: "Inbox"},
	}
	if _, err := m.apply(ops); err != nil {
		m.msg = err.Error()
		return
	}
	if err := m.backend.Commit(ops, m.now()); err != nil {
		m.msg = "not saved: " + err.Error()
	}
}

// live reports a row that exists with no deleted row above it.
func (m *Model) live(id string) bool {
	for id != "" {
		r := m.tree.Row(id)
		if r == nil || r.Deleted() {
			return false
		}
		id = r.Parent
	}
	return true
}

// projectOf is the top-level row a row belongs to.
func (m *Model) projectOf(id string) *outline.Row {
	r := m.tree.Row(id)
	for r != nil && r.Parent != "" {
		r = m.tree.Row(r.Parent)
	}
	return r
}

func (m *Model) changedAt(id string) int64 {
	r := m.tree.Row(id)
	if n, err := strconv.ParseInt(r.Fields[sigil.FieldChanged], 10, 64); err == nil {
		return n
	}
	return r.Created
}

// scan reads the whole tree once and derives everything the sidebar states:
// the three computed lists and each project's line. Nothing here is stored.
func (m *Model) scan() {
	now, today := m.now(), m.today()
	day := today.Format("2006-01-02")
	lists := map[string][]string{}
	tagUse := map[string]int{}
	picked := 0
	m.parked = 0

	var visit func(id string, stat *entry, touch *int64, hidden bool)
	visit = func(id string, stat *entry, touch *int64, hidden bool) {
		r := m.tree.Row(id)
		l := sigil.FromFields(r.Fields)
		*touch = max(*touch, r.Modified)
		for _, tag := range l.Tags {
			tagUse[tag]++
		}
		if r.Fields[sigil.FieldToday] == day {
			lists[kindToday] = append(lists[kindToday], id)
			if !l.Closed() {
				picked++
			}
		}
		// Parked work, and everything under it, is out of every computed
		// list. Today is a pick, so it is the one list that still shows it.
		hidden = hidden || l.State == sigil.Parked
		if !hidden {
			switch l.State {
			case sigil.Waiting:
				lists[kindWaiting] = append(lists[kindWaiting], id)
			case sigil.Open, sigil.Doing:
				if now-r.Modified > staleAfter.Milliseconds() {
					lists[kindStale] = append(lists[kindStale], id)
				}
			}
			if stat != nil && (l.State == sigil.Open || l.State == sigil.Doing || l.State == sigil.Waiting) {
				stat.idle = false
				if l.Due != "" {
					if _, days := sigil.DueLabel(l.Due, today); days < 0 {
						stat.due = 2
					} else if days <= dueSoonDays {
						stat.due = max(stat.due, 1)
					}
				}
			}
		}
		for _, kid := range m.tree.Children(id) {
			visit(kid, stat, touch, hidden)
		}
	}

	var projects []entry
	for _, top := range m.tree.Children("") {
		var touch int64
		if top == inboxID {
			visit(top, nil, &touch, false)
			continue
		}
		l := sigil.FromFields(m.tree.Row(top).Fields)
		stat := entry{kind: kindProject, id: top, label: l.Text, idle: true}
		visit(top, &stat, &touch, l.Closed())
		switch {
		case l.State == sigil.Parked:
			m.parked++
		case !l.Closed():
			stat.age = int((now - touch) / (24 * time.Hour).Milliseconds())
			projects = append(projects, stat)
		}
	}

	// Longest waiting and longest untouched first: the top is what to chase.
	sort.SliceStable(lists[kindWaiting], func(i, j int) bool {
		return m.changedAt(lists[kindWaiting][i]) < m.changedAt(lists[kindWaiting][j])
	})
	sort.SliceStable(lists[kindStale], func(i, j int) bool {
		return m.tree.Row(lists[kindStale][i]).Modified < m.tree.Row(lists[kindStale][j]).Modified
	})

	m.lists = lists
	m.tags = tagsByUse(tagUse)
	m.side = append([]entry{
		{kind: kindInbox, label: "Inbox", count: len(m.tree.Children(inboxID))},
		{kind: kindToday, label: "Today", count: picked},
		{kind: kindWaiting, label: "Waiting", count: len(lists[kindWaiting])},
		{kind: kindStale, label: "Stale", count: len(lists[kindStale])},
		{kind: kindProjects, label: "Projects"},
	}, projects...)
}

// open shows a sidebar destination in the outline pane.
func (m *Model) open(e entry) {
	m.sel = e
	m.zoom = nil
	m.picked = map[string]bool{}
	m.cursor, m.top = 0, 0
	switch e.kind {
	case kindInbox:
		m.at = place{root: inboxID}
	case kindProjects:
		m.at = place{}
	case kindProject:
		m.at = place{root: e.id}
	default:
		m.at = place{list: e.kind, rows: slices.Clone(m.lists[e.kind])}
	}
	m.rebuild()
}

// sideStep moves the sidebar selection, showing each destination as it goes.
func (m *Model) sideStep(by int) {
	at := -1
	for i, e := range m.side {
		if e.same(m.sel) {
			at = i
		}
	}
	next := max(0, min(at+by, len(m.side)-1))
	if at < 0 {
		next = 0
	}
	if next != at {
		m.open(m.side[next])
	}
}

// targets are the destinations a picker offers: every top-level row, the
// Inbox, and for a jump the computed lists as well.
func (m *Model) targets(jump bool) []entry {
	out := []entry{{kind: kindInbox, label: "Inbox"}}
	if jump {
		out = append(out,
			entry{kind: kindToday, label: "Today"},
			entry{kind: kindWaiting, label: "Waiting"},
			entry{kind: kindStale, label: "Stale"},
			entry{kind: kindProjects, label: "Projects"})
	}
	for _, top := range m.tree.Children("") {
		if top != inboxID {
			out = append(out, entry{kind: kindProject, id: top, label: m.tree.Row(top).Text()})
		}
	}
	return out
}

// rank scores how well a name answers a query: its start, the start of one
// of its words, anywhere inside it, or its letters in order. Lower is better.
func rank(name, query string) (int, bool) {
	n, q := strings.ToLower(name), strings.ToLower(query)
	if q == "" || strings.HasPrefix(n, q) {
		return 0, true
	}
	if i := strings.Index(n, q); i > 0 {
		if prev := []rune(n[:i]); !unicode.IsLetter(prev[len(prev)-1]) && !unicode.IsDigit(prev[len(prev)-1]) {
			return 1, true
		}
		return 2, true
	}
	rest := []rune(q)
	for _, r := range n {
		if len(rest) > 0 && r == rest[0] {
			rest = rest[1:]
		}
	}
	return 3, len(rest) == 0
}

// match returns the entries answering query, best first, ties in list order.
func match(entries []entry, query string, worst int) []entry {
	type scored struct {
		e entry
		s int
	}
	var hits []scored
	for _, e := range entries {
		if s, ok := rank(e.label, query); ok && s <= worst {
			hits = append(hits, scored{e, s})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].s < hits[j].s })
	out := make([]entry, len(hits))
	for i, h := range hits {
		out[i] = h.e
	}
	return out
}

func (e entry) rowID() string {
	if e.kind == kindInbox {
		return inboxID
	}
	return e.id
}

// create adds a row from a typed line as the last child of parent, as one
// undo step.
func (m *Model) create(parent, line string) bool {
	ops := []outline.Op{{Kind: outline.Insert, Row: m.newID(), Parent: parent, Pos: m.tree.PosLast(parent)}}
	inv, err := m.apply(ops)
	if err != nil {
		m.msg = err.Error()
		return false
	}
	fields := m.setFields(ops[0].Row, sigil.Parse(line, parent == "", m.today()).Fields())
	more, err := m.apply(fields)
	if err != nil {
		m.tree.Forget(ops[0].Row)
		m.msg = err.Error()
		return false
	}
	m.record(append(ops, fields...), append(more, inv...))
	return true
}

// route reads where a captured line goes: the project a %name word matches,
// else the Inbox. A %name matching no project stays in the text, where it
// shows.
func (m *Model) route(line string) (entry, []string) {
	target := entry{kind: kindInbox, label: "Inbox"}
	var words []string
	for _, w := range strings.Fields(line) {
		if name, ok := strings.CutPrefix(w, "%"); ok && name != "" {
			if hits := match(m.targets(false), name, 2); len(hits) > 0 {
				target = hits[0]
				continue
			}
		}
		words = append(words, w)
	}
	return target, words
}

// capture files a typed line where route says.
func (m *Model) capture(line string) {
	target, words := m.route(line)
	if len(words) > 0 && m.create(target.rowID(), strings.Join(words, " ")) {
		m.msg = "added to " + target.label
	}
}

// sideLines draws the sidebar at exactly w columns and h rows.
func (m *Model) sideLines(w, h int) []string {
	type line struct {
		text string
		sel  bool
	}
	var lines []line
	for _, e := range m.side {
		if e.kind == kindProjects {
			lines = append(lines, line{})
		}
		sel := e.same(m.sel)
		info := ""
		switch {
		case e.kind == kindProject:
			if e.due == 2 {
				info = sgrAlert + "!" + sgrReset + " "
			} else if e.due == 1 {
				info = sgrWarn + "!" + sgrReset + " "
			}
			if e.idle {
				info += sgrDim + "∅" + sgrReset
			} else if e.age > 0 {
				info += sgrDim + strconv.Itoa(e.age) + "d" + sgrReset
			}
			info = strings.TrimRight(info, " ")
		case e.count > 0:
			info = sgrDim + strconv.Itoa(e.count) + sgrReset
		}
		gutter := " "
		if sel {
			gutter = sgrDim + "▌" + sgrReset
			if m.sideFocus {
				gutter = sgrAccent + "▌" + sgrReset
			}
		}
		room := w - 2 - ansi.StringWidth(info)
		if info != "" {
			room--
		}
		label := ansi.Truncate(e.label, max(room, 1), "…")
		pad := strings.Repeat(" ", max(w-2-ansi.StringWidth(label)-ansi.StringWidth(info), 0))
		if sel {
			label = sgrBold + label + sgrReset
		}
		lines = append(lines, line{gutter + " " + label + pad + info, sel})
	}
	if m.parked > 0 {
		text := "  " + sgrDim + "parked " + strconv.Itoa(m.parked) + sgrReset
		lines = append(lines, line{}, line{text: text})
	}

	// Keep the selection in view when the list is taller than the pane.
	off := 0
	for i, l := range lines {
		if l.sel && i >= h {
			off = min(i-h/2, len(lines)-h)
		}
	}
	out := make([]string, h)
	for i := range out {
		text := ""
		if off+i < len(lines) {
			text = lines[off+i].text
		}
		out[i] = text + strings.Repeat(" ", max(w-ansi.StringWidth(text), 0))
	}
	return out
}
