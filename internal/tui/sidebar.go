package tui

import (
	"errors"
	"maps"
	"os/exec"
	"runtime"
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
	staleAfter  = 14 * 24 * time.Hour
	dueSoonDays = 7
	sidebarCols = 24
	sidebarFrom = 72 // the terminal width from which the sidebar is always drawn
)

// place is what the outline pane shows: the rows under root, or a flat list
// whose rows were fixed when it was opened. A list never drops a row because
// its state changed under the cursor; it is read again when reopened.
type place struct {
	root      string
	list      string
	rows      []string
	kept      map[string]bool // closed rows still shown because they closed here
	returnRow string          // row to seek in the previous place when Esc leaves this zoom

	// A filter narrows the place to the rows answering query. Like a list,
	// it holds the rows it found until the query changes.
	query   string
	flat    bool
	here    string // the row &. stands for: the cursor's when the filter was opened
	hits    []visRow
	matched map[string]bool
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
	m.msg = "no newer place"
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
	found := false // the project being visited has its next action

	var visit func(id string, stat *entry, touch *int64, hidden bool, path string)
	visit = func(id string, stat *entry, touch *int64, hidden bool, path string) {
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
		if r.Fields[sigil.FieldToday] == day || (l.Start == day && l.State != "") {
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
				// Work not due to start yet has not been neglected.
				if now-r.Modified > staleAfter.Milliseconds() && !l.Later(today) {
					lists[kindStale] = append(lists[kindStale], id)
				}
			}
			// A project's next action is its first row, in outline order,
			// that can be worked on now: so the order of its rows is its
			// priority.
			if stat != nil && !found && (l.State == sigil.Open || l.State == sigil.Doing) && !l.Later(today) {
				m.next[id], found = true, true
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
		for _, kid := range m.tree.Children(id) {
			visit(kid, stat, touch, hidden, path)
		}
	}

	var projects []entry
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
		visit(top, &stat, &touch, l.Closed(), "")
		switch {
		case l.State == sigil.Parked:
			m.parked++
		case !l.Closed():
			stat.age = int((now - touch) / (24 * time.Hour).Milliseconds())
			projects = append(projects, stat)
		}
	}

	m.sortToday(lists[kindToday])
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
	for _, p := range review {
		m.stats[p.id] = p
		lists[kindReview] = append(lists[kindReview], p.id)
	}

	m.filters = m.filters[:0]
	for _, id := range m.tree.Children(filtersID) {
		f := m.tree.Row(id).Fields
		m.filters = append(m.filters, savedFilter{id, f[sigil.FieldText], f[fieldRoot], f[fieldQuery], f[fieldFlat] != ""})
	}

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
	}
	m.arrive()
	if e.kind == kindRow {
		m.seek(e.id)
	}
}

// sortToday keeps placed rows first, with outline order for unplaced rows
// and equal keys (which independent picks on two machines can produce).
func (m *Model) sortToday(rows []string) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := m.tree.Row(rows[i]), m.tree.Row(rows[j])
		if a == nil || b == nil {
			return a != nil
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
		m.msg = "this row has no link: L makes one"
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
	if m.syncNote != "" {
		style := sgrDim
		if m.syncFailed {
			style = sgrAlert
		}
		lines = append(lines, line{}, line{text: "  " + style + ansi.Truncate(m.syncNote, w-2, "…") + sgrReset})
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
