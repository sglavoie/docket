package tui

import (
	"errors"
	"maps"
	"math"
	"os/exec"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

// The Inbox is an ordinary row with a fixed identity, so every machine agrees
// on it. Its position sorts before any key Between can mint, which keeps it
// first among the top-level rows, where it is never drawn.
const (
	inboxID       = "inbox"
	filtersID     = "filters" // holds the saved filters, one row each
	fixedPos      = "0"
	fieldTodayPos = "todaypos"
)

// hiddenTop reports a top-level row that is never drawn among the projects.
func hiddenTop(id string) bool { return id == inboxID || id == filtersID }

const (
	kindInbox    = "inbox"
	kindToday    = "today"
	kindWaiting  = "waiting"
	kindStale    = "stale"
	kindProjects = "projects"
	kindProject  = "project"
	kindHeading  = "heading"
	kindReview   = "review" // active projects, longest untouched first
	kindFilter   = "filter" // a saved filter
	kindHere     = "here"   // a move's destination: where o would open a row
	kindRow      = "row"    // any other row, shown where it lives
	kindNone     = "none"   // a link's destination: no row, which takes the link off
	fieldLink    = "link"   // the row this one points at
	trailDepth   = 100      // how many places Ctrl+O can go back through
)

const (
	staleAfter   = 14 * 24 * time.Hour
	dueSoonDays  = 7
	sidebarCols  = 24
	sidebarFrom  = 72 // the terminal width from which the sidebar is always drawn
	sidebarAlone = 40 // the widest the sidebar is drawn when it takes a narrow pane
	// sidebarFootFrom is the height from which a scrolled sidebar keeps the
	// sync line at its foot.
	sidebarFootFrom = 8
)

// place is what the outline pane shows: the rows under root, or a flat list
// whose rows were fixed when it was opened. A list never drops a row because
// its state changed under the cursor; it is read again when reopened.
type place struct {
	root      string
	list      string
	rows      []string
	kept      map[string]bool // closed rows still shown because they closed here
	carried   map[string]bool // in Today, the picks carried from an earlier day when it was opened
	returnRow string          // row to seek in the previous place when Esc leaves this zoom

	// A filter narrows the place to the rows answering query. Like a list,
	// it holds the rows it found until the query changes.
	query   string
	flat    bool
	here    string // the row &. stands for: the cursor's when the filter was opened
	hits    []visRow
	matched map[string]bool
	marks   []cond // the text and tags the query looks for, drawn on its matches
	since   bool   // the query asks when rows changed, so each match says when
}

// visit keeps a departure independent of later list sorting, filtering and
// cursor bookkeeping (different filters can share the same placeKey).
type visit struct {
	at   place
	zoom []place
	sel  entry
	row  string
}

func clonePlace(p place) place {
	p.rows = slices.Clone(p.rows)
	p.hits = slices.Clone(p.hits)
	p.kept = maps.Clone(p.kept)
	p.carried = maps.Clone(p.carried)
	p.matched = maps.Clone(p.matched)
	return p
}

func (m *Model) departure() *visit {
	v := &visit{at: clonePlace(m.at), sel: m.sel, row: m.current()}
	for _, p := range m.zoom {
		v.zoom = append(v.zoom, clonePlace(p))
	}
	return v
}

// leaveTrail remembers a place a jump left, for Ctrl+O. A new jump drops
// the places Ctrl+I would have gone on to.
func (m *Model) leaveTrail(v *visit) {
	m.older = append(m.older, v)
	if n := len(m.older); n > trailDepth {
		m.older = m.older[n-trailDepth:]
	}
	m.newer = nil
}

// travel goes one place back along the trail of jumps, or forward again. It
// passes over a place whose row is gone, and reports whether it moved.
func (m *Model) travel(back bool) bool {
	from, to := &m.older, &m.newer
	if !back {
		from, to = to, from
	}
	for len(*from) > 0 {
		v := (*from)[len(*from)-1]
		*from = (*from)[:len(*from)-1]
		if !m.live(v.at.root) {
			continue
		}
		*to = append(*to, m.departure())
		m.leave()
		m.at, m.zoom, m.sel = v.at, v.zoom, v.sel
		m.clearSelection()
		m.cursor, m.top = 0, 0
		m.rebuild()
		m.seek(v.row)
		m.land()
		return true
	}
	m.msg = "no newer place: Ctrl+O goes back"
	if back {
		m.msg = "no older place: a jump with f, the sidebar or a link leaves one"
	}
	return false
}

func (m *Model) leaveSidebar() {
	if v := m.sideFrom; v != nil {
		if placeKey(v.at) != placeKey(m.at) || v.at.query != m.at.query || v.at.flat != m.at.flat {
			m.leaveTrail(v)
		}
	}
	m.sideFrom = nil
	m.sideFocus = false
	m.prefix = ""
}

func (m *Model) jump(e entry) {
	from := m.sideFrom
	if from == nil {
		from = m.departure()
	}
	m.sideFrom = nil
	m.sideFocus = false
	m.open(e)
	m.leaveTrail(from)
}

// entry is one line of the sidebar, and one destination.
type entry struct {
	kind, id, label string
	hint            string // rows: where the row lives
	count           int    // lists: rows that still need something
	open            int    // projects: tasks to do
	age             int    // projects: whole days since anything inside changed
	due             int    // projects: 1 a deadline within a week, 2 one overdue
	idle            bool   // projects: no task left to do
	done            bool   // Today: every pick is closed
}

func (e entry) same(o entry) bool { return e.kind == o.kind && e.id == o.id }

// ensureFixed creates a row every machine agrees on, on first launch. It is
// not an undo step.
func (m *Model) ensureFixed(id, text string) {
	if m.tree.Row(id) != nil {
		return
	}
	ops := []outline.Op{
		{Kind: outline.Insert, Row: id, Pos: fixedPos},
		{Kind: outline.Set, Row: id, Field: sigil.FieldText, Value: text},
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

// scanKey is what a scan's answer depends on: the tree as it stood, and the
// minute it was read in.
type scanKey struct {
	tree   *outline.Tree
	gen    uint64
	minute int64
}

// scan reads the whole tree once and derives everything the sidebar states:
// the computed lists, project lines, tags and headings. Nothing here is stored.
func (m *Model) scan() {
	now, today := m.now(), m.today()
	// The pass is skipped while its answer cannot have changed, which is
	// what keeps a keystroke that changes no row, a typed letter above all,
	// from costing a walk of the whole outline. Ages are whole days and the
	// screen is read again each minute, so the minute is as fine as it gets.
	key := scanKey{m.tree, m.tree.Gen(), now / time.Minute.Milliseconds()}
	if key == m.scanned {
		return
	}
	m.scanned = key
	day := today.Format("2006-01-02")
	lists := map[string][]string{}
	tagUse := map[string]int{}
	picked := 0
	m.parked = 0
	m.headings = m.headings[:0]
	m.rowNames = m.rowNames[:0]
	m.outlineOrder = map[string]int{}
	m.next = map[string]bool{}
	m.first = map[string]bool{}
	m.missed = map[string]bool{}
	m.pointers = map[string][]string{}
	firstFound := map[string]bool{}
	m.totals = map[string]rowTotals{}
	found := false // the project being visited has its next action

	var visit func(id string, stat *entry, touch *int64, hidden bool, path string) taskTotals
	visit = func(id string, stat *entry, touch *int64, hidden bool, path string) taskTotals {
		m.outlineOrder[id] = len(m.outlineOrder)
		r := m.tree.Row(id)
		l := sigil.FromFields(r.Fields)
		label := strings.TrimSuffix(l.Text, ":")
		if r.Parent == "" {
			path = label
		} else {
			if strings.HasSuffix(l.Text, ":") {
				m.headings = append(m.headings, entry{kind: kindHeading, id: id, label: path + " › " + label})
			} else if l.Text != "" {
				m.rowNames = append(m.rowNames, entry{kind: kindRow, id: id, label: l.Text, hint: path})
			}
			path += " › " + label
		}
		*touch = max(*touch, r.Modified)
		for _, tag := range l.Tags {
			tagUse[tag]++
		}
		// Today holds what was picked, and what was set to start today.
		if isPicked(r.Fields, l, day) || (l.Start == day && l.State != "") {
			lists[kindToday] = append(lists[kindToday], id)
			if !l.Closed() {
				picked++
			}
			if carried(r.Fields, l, day) {
				m.missed[id] = true
			}
		}
		if to := r.Fields[fieldLink]; to != "" {
			m.pointers[to] = append(m.pointers[to], id)
		}
		// Parked work, and everything under it, is out of every computed
		// list. Today is a pick, so it is the one list that still shows it.
		hidden = hidden || l.State == sigil.Parked
		if !hidden {
			switch l.State {
			case sigil.Waiting:
				lists[kindWaiting] = append(lists[kindWaiting], id)
			case sigil.Open, sigil.Doing:
				// Work not due to start yet has not been neglected.
				if now-r.Modified > staleAfter.Milliseconds() && !l.Later(today) {
					lists[kindStale] = append(lists[kindStale], id)
				}
			}
			// A project's next action is its first row, in outline order,
			// that can be worked on now: so the order of its rows is its
			// priority.
			if (l.State == sigil.Open || l.State == sigil.Doing) && !l.Later(today) {
				if stat != nil && !found {
					m.next[id], found = true, true
				}
				if !firstFound[r.Parent] {
					m.first[id], firstFound[r.Parent] = true, true
				}
			}
			if stat != nil && (l.State == sigil.Open || l.State == sigil.Doing || l.State == sigil.Waiting) {
				stat.idle = false
				stat.open++
				if l.Due != "" {
					if _, days := sigil.DueLabel(l.Due, today); days < 0 {
						stat.due = 2
					} else if days <= dueSoonDays {
						stat.due = max(stat.due, 1)
					}
				}
			}
		}
		totals := rowTotals{own: countTask(l)}
		for _, kid := range m.tree.Children(id) {
			totals.under.add(visit(kid, stat, touch, hidden, path))
		}
		m.totals[id] = totals
		all := totals.under
		all.add(totals.own)
		return all
	}

	var projects []entry
	var all taskTotals
	for _, top := range m.tree.Children("") {
		var touch int64
		if top == inboxID {
			visit(top, nil, &touch, false, "")
			continue
		}
		if top == filtersID {
			continue
		}
		l := sigil.FromFields(m.tree.Row(top).Fields)
		stat := entry{kind: kindProject, id: top, label: l.Text, idle: true}
		found = false
		all.add(visit(top, &stat, &touch, l.Closed(), ""))
		switch {
		case l.State == sigil.Parked:
			m.parked++
		case !l.Closed():
			stat.age = int((now - touch) / (24 * time.Hour).Milliseconds())
			projects = append(projects, stat)
		}
	}

	m.totals[""] = rowTotals{under: all}
	m.sortToday(lists[kindToday], m.missed)
	m.inToday = make(map[string]bool, len(lists[kindToday]))
	for _, id := range lists[kindToday] {
		m.inToday[id] = true
	}

	// Longest waiting and longest untouched first: the top is what to chase.
	sort.SliceStable(lists[kindWaiting], func(i, j int) bool {
		return m.changedAt(lists[kindWaiting][i]) < m.changedAt(lists[kindWaiting][j])
	})
	sort.SliceStable(lists[kindStale], func(i, j int) bool {
		return m.tree.Row(lists[kindStale][i]).Modified < m.tree.Row(lists[kindStale][j]).Modified
	})

	// The review list: every active project, the longest untouched first.
	m.stats = map[string]entry{}
	review := make([]entry, len(projects))
	copy(review, projects)
	sort.SliceStable(review, func(i, j int) bool { return review[i].age > review[j].age })
	m.oldestProject = 0
	for _, p := range review {
		m.stats[p.id] = p
		m.oldestProject = max(m.oldestProject, p.age)
		lists[kindReview] = append(lists[kindReview], p.id)
	}

	m.filters = m.filters[:0]
	for _, id := range m.tree.Children(filtersID) {
		f := m.tree.Row(id).Fields
		m.filters = append(m.filters, savedFilter{id, f[sigil.FieldText], f[fieldRoot], f[fieldQuery], f[fieldFlat] != ""})
	}

	m.lists = lists
	m.tags, m.tagUse = tagsByUse(tagUse), tagUse
	// The Inbox counts what is left to file or finish, so it empties as the
	// header's count does.
	inbox := 0
	for _, id := range m.tree.Children(inboxID) {
		if !sigil.FromFields(m.tree.Row(id).Fields).Closed() {
			inbox++
		}
	}
	m.side = append([]entry{
		{kind: kindInbox, label: "Inbox", count: inbox},
		{kind: kindToday, label: "Today", count: picked, done: picked == 0 && len(lists[kindToday]) > 0},
		{kind: kindWaiting, label: "Waiting", count: len(lists[kindWaiting])},
		{kind: kindStale, label: "Stale", count: len(lists[kindStale])},
	}, nil...)
	// The saved filters follow the four lists, in the order they were saved.
	for _, f := range m.filters {
		m.side = append(m.side, entry{kind: kindFilter, id: f.Name, label: "/" + f.Name})
	}
	m.side = append(append(m.side, entry{kind: kindProjects, label: "Projects"}), projects...)
}

// open shows a sidebar destination in the outline pane.
func (m *Model) open(e entry) {
	here := m.current()
	m.leave()
	m.sel = e
	m.zoom = nil
	m.clearSelection()
	m.cursor, m.top = 0, 0
	switch e.kind {
	case kindInbox:
		m.at = place{root: inboxID}
	case kindProjects:
		m.at = place{}
	case kindProject:
		m.at = place{root: e.id}
	case kindHeading:
		// A heading opens inside its project, so Esc backs out to it.
		m.at = place{root: e.id}
		if top := m.projectOf(e.id); top != nil {
			m.zoom = []place{{root: top.ID}}
			m.sel = entry{kind: kindProject, id: top.ID}
			if top.ID == inboxID {
				m.sel = entry{kind: kindInbox}
			}
		}
	case kindRow:
		// Any other row is shown among its siblings, inside its project.
		r := m.tree.Row(e.id)
		m.at = place{root: r.Parent}
		m.sel = entry{kind: kindProjects}
		if top := m.projectOf(e.id); top != nil && r.Parent != "" {
			if top.ID != r.Parent {
				m.zoom = []place{{root: top.ID}}
			}
			m.sel = entry{kind: kindProject, id: top.ID}
			if top.ID == inboxID {
				m.sel = entry{kind: kindInbox}
			}
		}
		if m.hideClosed && sigil.FromFields(r.Fields).Closed() {
			m.at.kept = map[string]bool{e.id: true}
		}
	case kindFilter:
		m.at = place{}
		for _, f := range m.filters {
			if f.Name == e.id {
				m.at = place{query: f.Query, flat: f.Flat, here: here}
				if m.live(f.Root) {
					m.at.root = f.Root
				}
			}
		}
		m.refilter()
	default:
		m.at = place{list: e.kind, rows: slices.Clone(m.lists[e.kind])}
		if e.kind == kindToday {
			m.at.carried = maps.Clone(m.missed)
		}
	}
	m.arrive()
	if e.kind == kindRow {
		m.seek(e.id)
	}
}

// isPicked reports whether a row is in Today by its pick: picked on the
// day, or picked on an earlier day and still to do, since a pick left
// undone stays until it is closed or unpicked.
func isPicked(fields map[string]string, l sigil.Line, day string) bool {
	return fields[sigil.FieldToday] == day || carried(fields, l, day)
}

// carried reports a pick from an earlier day that is still to do.
func carried(fields map[string]string, l sigil.Line, day string) bool {
	pick := fields[sigil.FieldToday]
	return pick != "" && pick < day &&
		(l.State == sigil.Open || l.State == sigil.Doing || l.State == sigil.Waiting)
}

// sortToday puts the day's own rows first and the picks carried from an
// earlier day after them. Within each, placed rows come first, with outline
// order for unplaced rows and equal keys (which independent picks on two
// machines can produce). Which rows are carried is read when Today is
// opened, so a row unpicked or closed there keeps its section.
func (m *Model) sortToday(rows []string, carried map[string]bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := m.tree.Row(rows[i]), m.tree.Row(rows[j])
		if a == nil || b == nil {
			return a != nil
		}
		if carried[a.ID] != carried[b.ID] {
			return !carried[a.ID]
		}
		ap, bp := a.Fields[fieldTodayPos], b.Fields[fieldTodayPos]
		if ap == bp {
			return m.outlineOrder[a.ID] < m.outlineOrder[b.ID]
		}
		return ap != "" && (bp == "" || ap < bp)
	})
}

func (m *Model) reorderToday(id string, down bool) {
	if m.at.query != "" {
		m.flat()
		return
	}
	rows := make([]string, len(m.vis))
	positions := make([]string, len(m.vis))
	initialize := false
	for i, v := range m.vis {
		rows[i] = v.id
		positions[i] = m.tree.Row(v.id).Fields[fieldTodayPos]
		// Equal keys are possible after sync; make room before using Between.
		if positions[i] == "" || (i > 0 && positions[i] <= positions[i-1]) {
			initialize = true
		}
	}
	at := slices.Index(rows, id)
	next := at - 1
	if down {
		next = at + 1
	}
	if at < 0 || next < 0 || next >= len(rows) {
		return
	}
	// The line between the day's own rows and the carried picks holds: a
	// row is carried by the day it was picked, not by where it is put.
	if m.at.carried[rows[at]] != m.at.carried[rows[next]] {
		if m.at.carried[id] {
			m.msg = "carried picks stay below the line: t t picks it for today"
		} else {
			m.msg = "today's rows stay above the line: J K order them there"
		}
		return
	}
	var ops []outline.Op
	if initialize {
		pos := ""
		for i, row := range rows {
			pos = outline.Between(pos, "")
			positions[i] = pos
			ops = append(ops, outline.Op{Kind: outline.Set, Row: row, Field: fieldTodayPos, Value: pos})
		}
	}
	var before, after string
	if down {
		before = positions[next]
		if next+1 < len(rows) {
			after = positions[next+1]
		}
	} else {
		after = positions[next]
		if next > 0 {
			before = positions[next-1]
		}
	}
	ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: fieldTodayPos, Value: outline.Between(before, after)})
	if m.do(ops...) {
		m.rebuild()
		m.seek(id)
	}
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

// sideAt is where the selection sits among the entries, or -1.
func (m *Model) sideAt() int {
	for i, e := range m.side {
		if e.same(m.sel) {
			return i
		}
	}
	return -1
}

// sideKeep opens the entry now at a position whose own entry has gone, so
// the selection never rests on a project that is no longer listed.
func (m *Model) sideKeep(at int) {
	if m.sideAt() < 0 && len(m.side) > 0 {
		m.open(m.side[max(0, min(at, len(m.side)-1))])
	}
}

// sideDelete is dd on the chosen entry. A project's entry is its top-level
// row, so the row goes as it does under the cursor in Projects, as one undo
// step; any other entry is a list or a filter and says what removes it.
func (m *Model) sideDelete() {
	e := m.sel
	switch e.kind {
	case kindProject:
	case kindFilter:
		m.msg = "a saved filter is not deleted here: :forget " + e.id + " drops it"
		return
	default:
		m.msg = "only a project is deleted here: Enter, then dd takes rows"
		return
	}
	at := m.sideAt()
	m.takeRows("d", []string{e.id})
	if m.live(e.id) {
		return
	}
	msg := m.msg
	m.sideKeep(at)
	m.msg = msg
}

// sideTyped reports whether the chosen entry's name can be typed in, and
// says what works when it cannot. A project's name is its top-level row.
func (m *Model) sideTyped() bool {
	switch m.sel.kind {
	case kindProject:
		return true
	case kindFilter:
		m.msg = "a saved filter keeps its name: :save it as another, then :forget " + m.sel.id
	default:
		m.msg = "only a project is renamed here: Enter, then A or I types in rows"
	}
	return false
}

// sideType opens the chosen project's row for typing in its own entry, as
// the key would on that row in Projects: A and I type at the end and the
// start, and a word motion steps into the text.
func (m *Model) sideType(k string) {
	if !m.sideTyped() {
		return
	}
	id := m.sel.id
	switch k {
	case "A", "I":
		m.editRow(id)
		if k == "I" {
			m.edit.caret = 0
		}
		m.edit.mark(k)
	default:
		m.enterText(id, k, 0)
	}
	m.edit.side = true
}

// sideChange completes c on the chosen project: cc retypes its name keeping
// its fields, and c with a motion or a text object changes that much of the
// name from its start, as the same keys do inside the row.
func (m *Model) sideChange(k string) {
	id := m.sel.id
	if k == "c" {
		m.retypeRow(id)
		m.edit.mark("cc")
		m.edit.side = true
		return
	}
	if len([]rune(k)) != 1 {
		return // Esc, or a key that is no motion, drops the c
	}
	m.editRow(id)
	e := m.edit
	e.side, e.sideC, e.normal, e.caret = true, true, true, 0
	for _, key := range textKeys("c" + k) {
		m.textKey(key)
	}
	switch {
	case m.edit != e:
	case !e.normal:
		e.inside = false // Esc ends the change, as it would one opened by cc
	case e.pend == "":
		m.finishEdit() // the motion went nowhere: nothing was changed
	}
}

// sideWidth is how wide the sidebar is drawn.
func (m *Model) sideWidth() int {
	w, _ := m.size()
	if w >= sidebarFrom {
		return sidebarCols
	}
	return min(w, m.sideFit())
}

// sideReplay is undo or redo with sidebar focus. A project the step brought
// back is chosen again, and one it took away gives up the selection.
func (m *Model) sideReplay(from, to *[][]outline.Op, verb string) {
	n, at := len(*from), m.sideAt()
	if n == 0 {
		m.replay(from, to, verb)
		return
	}
	group := (*from)[n-1]
	row := group[len(group)-1].Row
	m.replay(from, to, verb)
	if len(*from) == n {
		return
	}
	msg := m.msg
	for _, e := range m.side {
		if e.kind == kindProject && e.id == row && !e.same(m.sel) {
			m.open(e)
		}
	}
	m.sideKeep(at)
	m.msg = msg
}

// targets are the destinations a picker offers: every top-level row, the
// Inbox and the headings, and for a jump the lists and saved filters too.
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
		if !hiddenTop(top) {
			out = append(out, entry{kind: kindProject, id: top, label: m.tree.Row(top).Text()})
		}
	}
	if jump {
		for _, f := range m.filters {
			out = append(out, entry{kind: kindFilter, id: f.Name, label: "/" + f.Name})
		}
	}
	return append(out, m.headings...)
}

// offers are the destinations a picker shows for what is typed: its matches,
// with here for a blank move or none for a blank link before them.
func (m *Model) offers(a *prompt, text string) []entry {
	places := m.targets(a.kind == askJump)
	if a.kind == askLink {
		places = places[1:] // a link points at a row, and the Inbox is a place
	}
	// A jump and a link reach every row by its text once something is typed;
	// a row must hold the words, where a place may answer to its letters.
	var hits []entry
	if a.kind != askMove && text != "" {
		hits = match(text, 3, places, m.rowNames)
	} else {
		hits = match(text, 3, places)
	}
	if a.kind == askMove && text == "" && m.canMoveHere() {
		hits = append([]entry{{kind: kindHere, label: "here"}}, hits...)
	}
	if a.kind == askLink && text == "" && slices.ContainsFunc(a.rows, func(id string) bool {
		return m.tree.Row(id).Fields[fieldLink] != ""
	}) {
		hits = append([]entry{{kind: kindNone, label: "none"}}, hits...)
	}
	return hits
}

// picker reports a prompt that lists matches to choose from.
func picker(kind string) bool { return kind == askJump || kind == askMove || kind == askLink }

// linkTo points rows at a row, or at none, as one undo step.
func (m *Model) linkTo(rows []string, to entry) {
	dest := to.rowID()
	if to.kind == kindNone {
		dest = ""
	} else if !m.live(dest) {
		m.msg = "that row is no longer live: " + to.label
		return
	}
	var ops []outline.Op
	for _, id := range rows {
		if m.live(id) && id != dest && m.tree.Row(id).Fields[fieldLink] != dest {
			ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: fieldLink, Value: dest})
		}
	}
	switch {
	case len(ops) == 0 && len(rows) == 1 && rows[0] == dest:
		m.msg = "a row cannot link to itself"
	case len(ops) == 0 || !m.do(ops...):
	case dest == "":
		m.msg = "link removed"
		m.repeat = repeatAct{link: to}
	default:
		m.msg = "linked to " + to.label + ": gl follows it"
		m.repeat = repeatAct{link: to}
	}
}

// openURL starts the system opener without holding up the next key.
func openURL(address string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	cmd := exec.Command(opener, address)
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap the process off the keys' path once it finishes.
	go func() { _ = cmd.Wait() }()
	return nil
}

// follow jumps to the linked row, or opens the first address in its text.
func (m *Model) follow() {
	cur := m.current()
	if cur == "" {
		return
	}
	switch dest := m.tree.Row(cur).Fields[fieldLink]; {
	case dest == "":
		for _, word := range strings.Fields(m.tree.Row(cur).Fields[sigil.FieldText]) {
			if address := linkOf(word); address != "" {
				if err := m.openURL(address); err != nil {
					m.msg = err.Error()
				} else {
					m.msg = "opened " + address
				}
				return
			}
		}
		// The way back: a row with no way out of its own goes to the row
		// that links to it.
		m.scan()
		switch from := m.pointers[cur]; len(from) {
		case 0:
			m.msg = "this row has no link: L makes one"
		case 1:
			m.jump(entry{kind: kindRow, id: from[0]})
		default:
			m.msg = strconv.Itoa(len(from)) + " rows link here: /&. lists them"
		}
	case !m.live(dest):
		m.msg = "the linked row is gone"
	default:
		m.jump(entry{kind: kindRow, id: dest})
	}
}

// canMoveHere reports whether the selection can land at the cursor: an
// outline is on screen, and the cursor's row is neither selected nor under a
// selected row.
func (m *Model) canMoveHere() bool {
	cur := m.current()
	if len(m.picked) == 0 || cur == "" || m.at.list != "" || m.at.query != "" {
		return false
	}
	for id := cur; id != ""; id = m.tree.Row(id).Parent {
		if m.picked[id] {
			return false
		}
	}
	return true
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
// A row among them must hold the query whole: its letters in order would
// offer half the outline.
func match(query string, worst int, lists ...[]entry) []entry {
	type scored struct {
		e entry
		s int
	}
	var hits []scored
	for _, entries := range lists {
		for _, e := range entries {
			if s, ok := rank(e.label, query); ok && s <= worst && (s < 3 || e.kind != kindRow) {
				hits = append(hits, scored{e, s})
			}
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
	fields := m.setFields(ops[0].Row, sigil.Parse(line, m.defaultsToNotes(parent), m.today()).Fields())
	more, err := m.apply(fields)
	if err != nil {
		m.tree.Forget(ops[0].Row)
		m.msg = err.Error()
		return false
	}
	if !m.record(append(ops, fields...), append(more, inv...)) {
		m.tree.Forget(ops[0].Row)
		return false
	}
	return true
}

// route reads where a captured line goes: the project or heading a %name
// word matches, else the Inbox. An unmatched %name stays in the text.
func (m *Model) route(line string) (entry, []string) {
	target := entry{kind: kindInbox, label: "Inbox"}
	var words []string
	for _, w := range strings.Fields(line) {
		if name, ok := strings.CutPrefix(w, "%"); ok && name != "" {
			if hits := match(name, 2, m.targets(false)); len(hits) > 0 {
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

// Launch opens the interactive outline, adding a tutorial only when there
// are no projects and no Inbox rows. New, Capture and List never seed it.
func Launch(tree *outline.Tree, backend Backend, folds map[string]bool) *Model {
	m := New(tree, backend, folds)
	for _, id := range tree.Children("") {
		if !hiddenTop(id) {
			return m
		}
	}
	if len(tree.Children(inboxID)) == 0 {
		m.tutor()
		m.starterFilters()
	}
	return m
}

// Capture files lines from outside the app, each as the capture line would,
// and says where each went. It is what the docket add command runs: one
// short-lived window on the database, which syncs once before it closes.
func Capture(tree *outline.Tree, backend Backend, lines []string) ([]string, error) {
	m := New(tree, backend, nil)
	var said []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m.msg = ""
		target, words := m.route(line)
		if len(words) == 0 || !m.create(target.rowID(), strings.Join(words, " ")) {
			if m.msg == "" {
				m.msg = "nothing to add"
			}
			return said, errors.New(m.msg)
		}
		said = append(said, "added to "+target.label)
	}
	m.syncQuit()
	return said, nil
}

// List answers a filter from a shell: the rows it matches, one a line, each
// spelled in the row grammar with its state, then a tab and where the row
// lives. With no filter it is the whole outline as export writes it. Nothing
// is changed.
func List(tree *outline.Tree, backend Backend, filter string) []string {
	return New(tree, backend, nil).list(filter)
}

func (m *Model) list(filter string) []string {
	if strings.TrimSpace(filter) == "" {
		text, _ := m.writeRows(m.tree.Children(""))
		return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	var out []string
	for _, root := range []string{inboxID, ""} {
		m.at = place{root: root, query: filter, flat: true}
		m.refilter()
		for _, hit := range m.at.hits {
			if !m.live(hit.id) {
				continue
			}
			r := m.tree.Row(hit.id)
			var path []string
			for p := m.tree.Row(r.Parent); p != nil; p = m.tree.Row(p.Parent) {
				path = append([]string{strings.TrimSuffix(p.Text(), ":")}, path...)
			}
			// Out of its outline a row has no default to be read against, so
			// a task always says its state.
			out = append(out, sigil.Format(sigil.FromFields(r.Fields), true, m.today())+"\t"+strings.Join(path, " › "))
		}
	}
	return out
}

// sideLines draws the sidebar at exactly w columns and h rows.
// sideInfo is what an entry says at its right edge: a count, a finished
// day, or a project's deadline mark and age.
func (m *Model) sideInfo(e entry) string {
	switch {
	case e.kind == kindProject:
		info := ""
		if e.due == 2 {
			// Bold as well, so overdue is not told by colour alone.
			info = m.colors().alert + sgrBold + "@" + sgrReset + " "
		} else if e.due == 1 {
			info = m.colors().warn + "@" + sgrReset + " "
		}
		if e.idle {
			info += m.colors().dim + "∅" + sgrReset
		} else if e.age > 0 {
			info += ageStyle(m.colors(), int64(e.age), int64(m.oldestProject)) + strconv.Itoa(e.age) + "d" + sgrReset
		}
		return strings.TrimRight(info, " ")
	case e.count > 0:
		return m.colors().dim + strconv.Itoa(e.count) + sgrReset
	case e.done:
		// A finished day is told from one with nothing picked.
		return m.colors().dim + "✓" + sgrReset
	}
	return ""
}

// ageStyle draws an age against the oldest beside it: the text colour for
// one just begun, warming smoothly through pale yellow, yellow and orange
// to red for the oldest, so the one left alone longest is the one the eye
// finds. Both are in the same unit.
func ageStyle(c palette, age, oldest int64) string {
	t := 0.0
	if oldest > 0 {
		t = min(max(float64(age)/float64(oldest), 0), 1)
	}
	at := t * float64(len(c.ages)-1)
	i := min(int(at), len(c.ages)-2)
	f := at - float64(i)
	from, to := c.ages[i], c.ages[i+1]
	ch := func(k int) string {
		return strconv.Itoa(int(math.Round(from[k] + (to[k]-from[k])*f)))
	}
	return "\x1b[38;2;" + ch(0) + ";" + ch(1) + ";" + ch(2) + "m"
}

// sideFit is the width the sidebar needs when it takes a narrow pane alone:
// its widest entry, so each count stays beside the name it counts, and no
// narrower than it is drawn beside the pane.
func (m *Model) sideFit() int {
	w := sidebarCols
	for _, e := range m.side {
		need := 2 + ansi.StringWidth(e.label)
		if info := ansi.StringWidth(m.sideInfo(e)); info > 0 {
			need += 1 + info
		}
		w = max(w, need)
	}
	if m.parked > 0 {
		w = max(w, 2+len(parkedLabel(m.parked)))
	}
	if m.syncNote != "" {
		w = max(w, 2+ansi.StringWidth(m.syncNote))
	}
	return min(w, sidebarAlone)
}

func (m *Model) sideLines(w, h int) []string {
	lines, _ := m.sideView(w, h)
	return lines
}

// sideView draws the sidebar at exactly w columns and h rows, and places
// the caret when a project's name is being typed in its entry.
func (m *Model) sideView(w, h int) ([]string, *tea.Cursor) {
	type line struct {
		text  string
		sel   bool
		entry bool // a destination, which a marker counts when it is hidden
		caret int  // the caret's column plus one, on the line that holds it
	}
	var lines []line
	for i, e := range m.side {
		// The filters the user saved are a group of their own, set apart
		// from the four lists every outline has.
		if e.kind == kindProjects || (e.kind == kindFilter && m.side[i-1].kind != kindFilter) {
			lines = append(lines, line{})
		}
		sel := e.same(m.sel)
		if ed := m.edit; ed != nil && ed.side && e.kind == kindProject && e.id == ed.row {
			// The name being typed takes the entry's place, wrapped to the
			// sidebar, with what the line makes of it so far in colour.
			room := max(w-2, 1)
			l := ed.wrapped(room, m.styler(surfaceRow))
			last := len(l.lines) - 1
			l.lines[last] += m.ghost(l.lines[last], m.suggest(ed, surfaceRow), room)
			p := l.pos[ed.caret]
			for n, text := range l.lines {
				at := 0
				if n == p.y {
					at = 2 + p.x + 1
				}
				text = m.tint(m.colors().accent+"▌"+sgrReset+" "+text, w, m.colors().cursor)
				lines = append(lines, line{text: text, sel: true, entry: n == 0, caret: at})
			}
			continue
		}
		info := m.sideInfo(e)
		gutter := " "
		if sel {
			gutter = m.colors().dim + "▌" + sgrReset
			if m.sideFocus {
				gutter = m.colors().accent + "▌" + sgrReset
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
		text := gutter + " " + label + pad + info
		if sel && m.sideFocus {
			text = m.tint(text, w, m.colors().cursor)
		}
		lines = append(lines, line{text: text, sel: sel, entry: true})
	}
	if m.parked > 0 {
		// A parked project has no entry, so the count stands for it while
		// the pane is inside one.
		gutter, here := " ", false
		if r := m.tree.Row(m.sel.id); m.sel.kind == kindProject && r != nil && r.Fields[sigil.FieldState] == sigil.Parked {
			gutter, here = m.colors().dim+"▌"+sgrReset, true
		}
		text := gutter + " " + m.colors().dim + parkedLabel(m.parked) + sgrReset
		lines = append(lines, line{}, line{text: text, sel: here})
	}
	// The last word on sync is the sidebar's last line whatever is scrolled:
	// a pass that failed is an alarm, and an alarm out of sight is none.
	var foot []line
	if m.syncNote != "" {
		style := m.colors().dim
		if m.syncFailed {
			style = m.colors().alert
		}
		foot = []line{{}, {text: "  " + style + ansi.Truncate(m.syncNote, w-2, "…") + sgrReset}}
	}
	if len(lines)+len(foot) <= h || h < sidebarFootFrom {
		// Everything fits, or the sidebar is too short to give two rows up.
		lines, foot = append(lines, foot...), nil
	}
	whole := h
	h -= len(foot)

	// Keep the selection in view when the list is taller than the pane, and
	// off the last row while more follows, which a marker takes.
	off := 0
	for i, l := range lines {
		if l.sel && len(lines) > h && i >= h-1 {
			off = max(min(i-h/2, len(lines)-h), 0)
		}
	}
	// What is scrolled out of sight is counted at the edge it left by.
	hidden := func(from, to int) string {
		n := 0
		for _, l := range lines[from:to] {
			if l.entry {
				n++
			}
		}
		return strconv.Itoa(n)
	}
	if h >= 3 && off > 0 {
		lines[off] = line{text: "  " + m.colors().dim + "↑ " + hidden(0, off+1) + sgrReset}
	}
	if last := off + h - 1; h >= 3 && last < len(lines)-1 {
		lines[last] = line{text: "  " + m.colors().dim + "↓ " + hidden(last, len(lines)) + sgrReset}
	}
	out := make([]string, whole)
	var caret *tea.Cursor
	for i := range out {
		text := ""
		if i >= h {
			text = foot[i-h].text
		} else if off+i < len(lines) {
			text = lines[off+i].text
			if at := lines[off+i].caret; at > 0 {
				caret = tea.NewCursor(min(at-1, w-1), i)
				caret.Shape = tea.CursorBar
				if m.edit.normal {
					caret.Shape = tea.CursorBlock
				}
			}
		}
		out[i] = text + strings.Repeat(" ", max(w-ansi.StringWidth(text), 0))
	}
	return out, caret
}

// parkedLabel says what the sidebar's last count counts: projects, since a
// parked task inside an active project is not in it.
func parkedLabel(n int) string {
	if n == 1 {
		return "1 parked project"
	}
	return strconv.Itoa(n) + " parked projects"
}
