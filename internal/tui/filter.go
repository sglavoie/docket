package tui

import (
	"cmp"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

// term is one word of a filter. Every term must hold for a row to match; a
// leading minus turns a term into its opposite. A word may offer several
// readings with a bar between them, and holds when any one does.
type term struct {
	neg    bool
	above  bool // tested on the row's ancestors
	below  bool // tested on the rows under it
	parent bool // tested on the row's parent only
	child  bool // tested on the rows directly under it
	any    []cond
}

// query is a filter as read: its terms in the order written, and how they
// combine. With no group every term must hold, which is the plain filter.
type query struct {
	terms []term
	expr  *group
}

// group is a part of a filter set off by brackets or split by a bar standing
// alone: alternatives, each a run of parts that must all hold. A group with
// no alternatives is one term.
type group struct {
	neg  bool
	leaf int
	alts [][]group
}

// holds reads a group, asking leaf for each term it names.
func (g *group) holds(leaf func(int) bool) bool {
	if g.alts == nil {
		return leaf(g.leaf)
	}
	for _, run := range g.alts {
		all := true
		for i := range run {
			if !run[i].holds(leaf) {
				all = false
				break
			}
		}
		if all {
			return !g.neg
		}
	}
	return g.neg
}

// cond is one reading of a term.
type cond struct {
	kind  byte // t text, ! flag, # tag, c context, l g tag value range, $ state, S any state, n next action, f first sibling, T in Today, P project, @ deadline, < > deadline range, s start, * repeat, ~ changed, & link
	value string
	arg   string          // the value a tag range compares with
	rows  map[string]bool // the rows a link term asks for, read by terms
	loose bool            // text that starts like a sigil the filter cannot read
	line  bool            // text looked for on the row's line only, its body left out
}

// sigilStarts are the characters a filter word's sigil begins with.
const sigilStarts = "!#$@*~&"

// query reads a filter for this outline: parseFilter, and then for each link
// term the rows it names, since only the tree knows them. here is the row
// &. stands for.
func (m *Model) query(q string, here string, today time.Time) query {
	out := parseFilter(q, today)
	for _, t := range out.terms {
		for i, c := range t.any {
			if c.kind != '&' || c.value == "" {
				continue
			}
			rows := map[string]bool{}
			if c.value == "." {
				rows[here] = true
			} else {
				var walk func(id string)
				walk = func(id string) {
					for _, kid := range m.tree.Children(id) {
						if strings.Contains(strings.ToLower(m.tree.Row(kid).Text()), c.value) {
							rows[kid] = true
						}
						walk(kid)
					}
				}
				walk("")
			}
			t.any[i].rows = rows
		}
	}
	return out
}

// parseQuery reads the terms of a filter, in the order written.
func parseQuery(q string, today time.Time) []term { return parseFilter(q, today).terms }

// filterParts splits on blanks, or on alternative bars, outside phrases.
// Quotes open only at a term's start (after its optional prefixes), so an
// inch mark in 5" stays literal. An unfinished phrase runs to the end.
func filterParts(s string, bars bool) []string {
	var parts []string
	start, side := 0, 0
	quoted := false
	for i, r := range s {
		if r == '"' {
			if quoted {
				quoted = false
			} else {
				prefix := strings.TrimSuffix(strings.TrimPrefix(s[side:i], "-"), "=")
				if prefix == "" || prefix == "^" || prefix == "_" || prefix == "<" || prefix == ">" {
					quoted = true
				}
			}
		}
		if quoted {
			continue
		}
		if (bars && r == '|') || (!bars && unicode.IsSpace(r)) {
			if bars || i > start {
				parts = append(parts, s[start:i])
			}
			start = i + len(string(r))
			side = start
		} else if r == '|' {
			side = i + 1
		}
	}
	if bars || start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

// parseFilter reads a filter in the grammar rows are written in. Terms side
// by side must all hold. A bar standing alone splits what is around it into
// alternatives, and brackets standing alone set a part off as one term, with
// -( for its opposite. A part left empty or open while typing is passed over.
func parseFilter(q string, today time.Time) query {
	var out query
	words := filterParts(q, false)
	i := 0
	var read func(depth int) [][]group
	read = func(depth int) [][]group {
		runs := [][]group{nil}
		for i < len(words) {
			w := words[i]
			i++
			switch w {
			case ")":
				if depth > 0 {
					return runs
				}
			case "|":
				runs = append(runs, nil)
			case "(", "-(":
				inner := slices.DeleteFunc(read(depth+1), func(run []group) bool { return len(run) == 0 })
				if len(inner) > 0 {
					runs[len(runs)-1] = append(runs[len(runs)-1], group{neg: w == "-(", alts: inner})
				}
			default:
				if t, ok := parseTerm(w, today); ok {
					out.terms = append(out.terms, t)
					runs[len(runs)-1] = append(runs[len(runs)-1], group{leaf: len(out.terms) - 1})
				}
			}
		}
		return runs
	}
	runs := slices.DeleteFunc(read(0), func(run []group) bool { return len(run) == 0 })
	plain := len(runs) == 1 && !slices.ContainsFunc(runs[0], func(g group) bool { return g.alts != nil })
	if len(runs) > 0 && !plain {
		out.expr = &group{alts: runs}
	}
	return out
}

// parseTerm reads one word of a filter. A word that is not a readable sigil
// matches the row's text; a mark still being typed is no term yet.
func parseTerm(w string, today time.Time) (term, bool) {
	var t term
	if len(w) > 1 && w[0] == '-' {
		t.neg, w = true, w[1:]
	}
	switch w {
	case "^", "_", "<", ">":
		return t, false // still being typed
	}
	for _, twice := range []string{"^^", "__", "<<", ">>"} {
		if strings.HasPrefix(w, twice) {
			t.any = []cond{parseCond(w, today)}
			return t, true
		}
	}
	switch w[0] {
	case '^':
		t.above, w = true, w[1:]
	case '_':
		t.below, w = true, w[1:]
	case '<':
		t.parent, w = true, w[1:]
	case '>':
		t.child, w = true, w[1:]
	}
	// = holds a word to the row's own line; == starts literal text.
	line := false
	switch {
	case w == "=":
		return t, false // still being typed
	case strings.HasPrefix(w, "=="):
		t.any = []cond{parseCond(w, today)}
		return t, true
	case strings.HasPrefix(w, "="):
		line, w = true, w[1:]
	}

	// A side without its own sigil inherits the first side's:
	// $open|doing reads as $open or $doing, but #home|! keeps both sigils.
	sig := w[:len(w)-len(strings.TrimLeft(w, sigilStarts))]
	if parts := filterParts(w, true); len(parts) > 1 && !slices.Contains(parts, "") {
		for _, part := range parts {
			if !strings.ContainsRune(sigilStarts, rune(part[0])) {
				part = sig + part
			}
			c := parseCond(part, today)
			if strings.ContainsRune(sigilStarts, rune(part[0])) && c.kind == 't' {
				t.any = nil // one alternative cannot be read: the word is text
				break
			}
			t.any = append(t.any, c)
		}
	}
	if t.any == nil {
		t.any = []cond{parseCond(w, today)}
	}
	// A bare quote or an empty phrase is still waiting for text, including
	// in alternatives. It must not match everything (or exclude everything).
	t.any = slices.DeleteFunc(t.any, func(c cond) bool { return c.kind == 't' && c.value == "" })
	for i := range t.any {
		t.any[i].line = line && t.any[i].kind == 't'
	}
	return t, len(t.any) > 0
}

// parseCond reads one word, without its minus, as a sigil or else as text.
func parseCond(w string, today time.Time) cond {
	var c cond
	switch w[0] {
	case '"':
		c.kind, c.value = 't', strings.ToLower(strings.TrimSuffix(w[1:], "\""))
	case '!':
		if w == "!" {
			c.kind = '!'
		}
	case '#':
		if rest, ctx := strings.CutPrefix(w, "##"); ctx {
			if name, ok := sigil.ParseContext(rest); ok || rest == "" {
				c.kind, c.value = 'c', name
			}
			break
		}
		if tag, ok := sigil.ParseTag(w[1:]); ok || w == "#" {
			c.kind, c.value = '#', tag
		} else if i := strings.IndexAny(w, "<>"); i > 1 && i < len(w)-1 {
			// A range over a tag's value: at most, or at least.
			if name, ok := sigil.ParseTag(w[1:i]); ok && !strings.Contains(name, "=") {
				c.kind, c.value, c.arg = 'l', name, w[i+1:]
				if w[i] == '>' {
					c.kind = 'g'
				}
			}
		}
	case '$':
		// These computed filter words are read before the states.
		if w == "$" {
			c.kind = 'S'
		} else if strings.ToLower(w) == "$next" {
			c.kind = 'n'
		} else if strings.ToLower(w) == "$first" {
			c.kind = 'f'
		} else if strings.ToLower(w) == "$today" {
			c.kind = 'T'
		} else if strings.ToLower(w) == "$project" {
			c.kind = 'P'
		} else if strings.ToLower(w) == "$missed" {
			c.kind = 'M'
		} else if strings.ToLower(w) == "$pinned" {
			c.kind = '^'
		} else if state, ok := sigil.ParseState(w[1:]); ok {
			c.kind, c.value = '$', state
		}
	case '*':
		if every, ok := sigil.ParseRepeat(w[1:]); ok || w == "*" {
			c.kind, c.value = '*', every
		}
	case '~':
		if day, ok := sinceDay(strings.ToLower(w[1:]), today); ok {
			c.kind, c.value = '~', day
		}
	case '&':
		// A link: any, to the cursor's row (&.), or to a row holding the text.
		c.kind, c.value = '&', strings.ToLower(w[1:])
	case '@':
		if rest, start := strings.CutPrefix(strings.ToLower(w), "@@"); start {
			if day, ok := sigil.ParseDate(rest, today); ok || rest == "" {
				c.kind, c.value = 's', day
			} else if rest == "later" {
				c.kind, c.value = 's', rest
			}
			break
		}
		switch name := strings.ToLower(w[1:]); {
		case name == "" || name == "overdue" || name == "week":
			c.kind, c.value = '@', name
		case name[0] == '<' || name[0] == '>':
			// A range: due on or before, or on or after, the day.
			if day, ok := sigil.ParseDate(name[1:], today); ok {
				c.kind, c.value = name[0], day
			}
		default:
			if day, ok := sigil.ParseDate(name, today); ok {
				c.kind, c.value = '@', day
			}
		}
	}
	if c.kind == 0 {
		c.kind, c.value = 't', strings.ToLower(w)
		c.loose = len(w) > 1 && strings.ContainsRune(sigilStarts, rune(w[0]))
	}
	return c
}

// sinceDay reads how far back a ~ term looks, as the first day it takes in:
// today, or a count of days, weeks, months or years before today.
func sinceDay(s string, today time.Time) (string, bool) {
	if s == "today" {
		return today.Format("2006-01-02"), true
	}
	if len(s) < 2 {
		return "", false
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 0 || n > 9999 {
		return "", false
	}
	var from time.Time
	switch s[len(s)-1] {
	case 'd':
		from = today.AddDate(0, 0, -n)
	case 'w':
		from = today.AddDate(0, 0, -7*n)
	case 'm':
		from = today.AddDate(0, -n, 0)
	case 'y':
		from = today.AddDate(-n, 0, 0)
	default:
		return "", false
	}
	return from.Format("2006-01-02"), true
}

// matches reads one row against a query. above and under are the answers
// to its ancestor and descendant terms, worked out by the caller's walk.
func (m *Model) matches(r *outline.Row, q query, today time.Time, above []int, under map[string]uint64) bool {
	l := sigil.FromFields(r.Fields)
	leaf := func(i int) bool {
		t := q.terms[i]
		var ok bool
		switch {
		case t.above:
			ok = above[i] != 0
		case t.below:
			ok = under[r.ID]&(1<<min(i, 63)) != 0
		case t.parent:
			if p := m.tree.Row(r.Parent); p != nil {
				ok = m.holdsAny(p, sigil.FromFields(p.Fields), t, today)
			}
		case t.child:
			for _, kid := range m.tree.Children(r.ID) {
				if k := m.tree.Row(kid); m.holdsAny(k, sigil.FromFields(k.Fields), t, today) {
					ok = true
					break
				}
			}
		default:
			ok = m.holdsAny(r, l, t, today)
		}
		return ok != t.neg
	}
	if q.expr != nil {
		return q.expr.holds(leaf)
	}
	for i := range q.terms {
		if !leaf(i) {
			return false
		}
	}
	return true
}

func (m *Model) holdsAny(r *outline.Row, l sigil.Line, t term, today time.Time) bool {
	for _, c := range t.any {
		if m.holds(r, l, c, today) {
			return true
		}
	}
	return false
}

// holds reads one row against one reading of a term. The next actions and
// the day's picks are as the last scan found them.
func (m *Model) holds(r *outline.Row, l sigil.Line, c cond, today time.Time) bool {
	switch c.kind {
	case 't':
		if strings.Contains(strings.ToLower(l.Text), c.value) {
			return true
		}
		body := r.Fields[fieldBody]
		return !c.line && body != "" && strings.Contains(strings.ToLower(body), c.value)
	case '!':
		return l.Flag
	case '#':
		// A bare name holds whatever value the tag carries.
		return len(l.Tags) > 0 && (c.value == "" || slices.ContainsFunc(l.Tags, func(tag string) bool {
			return tag == c.value || sigil.TagName(tag) == c.value
		}))
	case 'c':
		return l.Context != "" && (c.value == "" || l.Context == c.value)
	case 'l', 'g':
		for _, tag := range l.Tags {
			if v := sigil.TagValue(tag); v != "" && sigil.TagName(tag) == c.value {
				order := compareValues(v, c.arg)
				return (c.kind == 'l' && order <= 0) || (c.kind == 'g' && order >= 0)
			}
		}
		return false
	case '$':
		return l.State == c.value
	case 'S':
		return l.State != ""
	case '^':
		return l.Pin
	case 'n':
		return m.next[r.ID]
	case 'f':
		return m.first[r.ID]
	case 'T':
		return m.inToday[r.ID]
	case 'M':
		return m.missed[r.ID]
	case 'P':
		// A project is a top-level row: neither the Inbox nor the row
		// that holds the saved filters is one.
		return r.Parent == "" && r.ID != inboxID && r.ID != filtersID
	case '@':
		_, days := sigil.DueLabel(l.Due, today)
		switch c.value {
		case "":
			return l.Due != ""
		case "overdue":
			return l.Due != "" && days < 0 && !l.Closed()
		case "week":
			return l.Due != "" && days <= dueSoonDays && !l.Closed()
		}
		return l.Due == c.value
	case '<':
		return l.Due != "" && l.Due <= c.value && !l.Closed()
	case '>':
		return l.Due != "" && l.Due >= c.value && !l.Closed()
	case 's':
		switch c.value {
		case "":
			return l.Start != ""
		case "later":
			return l.Later(today)
		}
		return l.Start == c.value
	case '*':
		return l.Every != "" && (c.value == "" || l.Every == c.value)
	case '&':
		to := r.Fields[fieldLink]
		return to != "" && (c.value == "" || c.rows[to])
	case '~':
		// When the state last changed, or for a row that never had one, when
		// it was made.
		at := r.Created
		if n, err := strconv.ParseInt(r.Fields[sigil.FieldChanged], 10, 64); err == nil {
			at = n
		}
		return time.UnixMilli(at).In(today.Location()).Format("2006-01-02") >= c.value
	}
	return false
}

// compareValues orders two tag values: as numbers when both are, else as
// text ignoring case.
func compareValues(a, b string) int {
	x, errA := strconv.ParseFloat(a, 64)
	y, errB := strconv.ParseFloat(b, 64)
	if errA == nil && errB == nil {
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// underSets answers a query's below terms for every row at once: for each
// row, which of them some row under it holds. It is one walk of the outline,
// made only when the query has such a term.
func (m *Model) underSets(terms []term, today time.Time) map[string]uint64 {
	if !slices.ContainsFunc(terms, func(t term) bool { return t.below }) {
		return nil
	}
	under := map[string]uint64{}
	var walk func(id string) uint64
	walk = func(id string) uint64 {
		var below uint64
		for _, kid := range m.tree.Children(id) {
			below |= walk(kid)
		}
		if id == "" {
			return 0
		}
		if below != 0 {
			under[id] = below
		}
		r := m.tree.Row(id)
		l := sigil.FromFields(r.Fields)
		for i, t := range terms {
			if t.below && m.holdsAny(r, l, t, today) {
				below |= 1 << min(i, 63)
			}
		}
		return below
	}
	walk("")
	return under
}

// refilter reads the place's query against its rows. In an outline a match
// keeps its ancestors, so it is read where it lives; flat drops them.
func (m *Model) refilter() {
	m.at.hits, m.at.matched = nil, map[string]bool{}
	if m.at.foldsFor != m.at.query {
		m.at.folds, m.at.foldsFor = nil, ""
	}
	today := m.today()
	q := m.query(m.at.query, m.at.here, today)
	terms := q.terms
	m.at.marks = q.marks()
	m.at.since = asks(terms, "~")
	if len(terms) == 0 {
		return
	}
	m.scan() // computed filter words read the scan's answers
	// Each slot records the first matching ancestor's walk level. Restore
	// slots at that level on return, so siblings never inherit one another.
	above := make([]int, len(terms))
	under := m.underSets(terms, today)
	hasAbove := slices.ContainsFunc(terms, func(t term) bool { return t.above })
	include := func(id string, level int) {
		if !hasAbove || id == "" {
			return
		}
		r := m.tree.Row(id)
		l := sigil.FromFields(r.Fields)
		for i, t := range terms {
			if t.above && above[i] == 0 && m.holdsAny(r, l, t, today) {
				above[i] = level
			}
		}
	}
	parents := func(id string) {
		if hasAbove {
			for id != "" {
				include(id, 1)
				id = m.tree.Row(id).Parent
			}
		}
	}
	if m.at.list != "" {
		for _, id := range m.at.rows {
			if !m.live(id) {
				continue
			}
			clear(above)
			parents(m.tree.Row(id).Parent)
			if m.matches(m.tree.Row(id), q, today, above, under) {
				m.at.hits = append(m.at.hits, visRow{id, 0})
				m.at.matched[id] = true
			}
		}
		return
	}
	if m.at.root != "" {
		parents(m.tree.Row(m.at.root).Parent)
	}
	var walk func(parent string, depth, level int) bool
	walk = func(parent string, depth, level int) bool {
		include(parent, level)
		any := false
		for _, id := range m.tree.Children(parent) {
			if hiddenTop(id) {
				continue
			}
			at := len(m.at.hits)
			ok := m.matches(m.tree.Row(id), q, today, above, under)
			if ok {
				m.at.matched[id] = true
			}
			if m.at.flat {
				if ok {
					m.at.hits = append(m.at.hits, visRow{id, 0})
				}
				any = walk(id, 0, level+1) || ok || any
				continue
			}
			m.at.hits = append(m.at.hits, visRow{id, depth})
			below := walk(id, depth+1, level+1)
			if !ok && !below {
				m.at.hits = m.at.hits[:at]
			}
			any = any || ok || below
		}
		for i := range above {
			if above[i] == level {
				above[i] = 0
			}
		}
		return any
	}
	walk(m.at.root, 0, 2)
	if m.at.flat && m.at.since && !byDeadline(terms) {
		// Newest change first, ties in the order found.
		slices.SortStableFunc(m.at.hits, func(a, b visRow) int {
			return cmp.Compare(m.changedAt(b.id), m.changedAt(a.id))
		})
	}
	if m.at.flat && byDeadline(terms) {
		// Soonest first, ties and rows with no deadline in outline order.
		slices.SortStableFunc(m.at.hits, func(a, b visRow) int {
			da, db := m.tree.Row(a.id).Fields[sigil.FieldDue], m.tree.Row(b.id).Fields[sigil.FieldDue]
			switch {
			case da == db:
				return 0
			case da == "":
				return 1
			case db == "":
				return -1
			}
			return strings.Compare(da, db)
		})
	}
}

// byDeadline reports whether a query asks about deadlines, so a flat list of
// its matches reads soonest first.
func byDeadline(terms []term) bool { return asks(terms, "@<>") }

// asks reports whether a query has a term of one of these kinds that the
// row itself must hold.
func asks(terms []term, kinds string) bool {
	for _, t := range terms {
		if t.neg || t.above || t.below || t.parent || t.child {
			continue
		}
		for _, c := range t.any {
			if strings.IndexByte(kinds, c.kind) >= 0 {
				return true
			}
		}
	}
	return false
}

// change is what a stamp asks of each row it lands on.
type change struct {
	set        map[string]string
	add, strip []string         // tags
	unctx      []string         // contexts -##name takes off; -## takes any
	shift      map[string]shift // days moved on from where each row has them
}

// shift is how far a stamp moves a day a row already has.
type shift struct {
	n    int
	unit byte
}

// parseStamp reads a line of sigils. It returns the first word it cannot
// read, in which case nothing is applied.
func parseStamp(line string, today time.Time) (change, string) {
	c := change{set: map[string]string{}, shift: map[string]shift{}}
	for _, w := range strings.Fields(line) {
		neg := len(w) > 1 && w[0] == '-'
		body := strings.TrimPrefix(w, "-")
		ok := false
		// @++1w and @--3d move the day the row has, not one counted from today.
		field, rest := sigil.FieldDue, strings.TrimPrefix(body, "@")
		if strings.HasPrefix(body, "@@") {
			field, rest = sigil.FieldStart, body[2:]
		}
		if n, unit, moves := sigil.ParseShift(rest); moves && !neg && rest != body {
			c.shift[field] = shift{n, unit}
			delete(c.set, field)
			continue
		}
		switch {
		case body == "!":
			c.set[sigil.FieldFlag], ok = "1", true
			if neg {
				c.set[sigil.FieldFlag] = ""
			}
		case body == "^":
			c.set[sigil.FieldPin], ok = "1", true
			if neg {
				c.set[sigil.FieldPin] = ""
			}
		case body == "@" && neg:
			c.set[sigil.FieldDue], ok = "", true
		case body == "@@" && neg:
			c.set[sigil.FieldStart], ok = "", true
		case body == "*" && neg:
			c.set[sigil.FieldRepeat], ok = "", true
		case body == "~" && neg:
			c.set[sigil.FieldColor], ok = "", true
		case body == "##" && neg:
			c.set[sigil.FieldContext], ok = "", true
		case len(body) < 2:
		case strings.HasPrefix(body, "##"):
			var name string
			if name, ok = sigil.ParseContext(body[2:]); ok && neg {
				c.unctx = append(c.unctx, name)
			} else if ok {
				c.set[sigil.FieldContext] = name
			}
		case body[0] == '#':
			var tag string
			if tag, ok = sigil.ParseTag(body[1:]); ok && neg {
				c.strip = append(c.strip, tag)
			} else if ok {
				c.add = append(c.add, tag)
			}
		case strings.HasPrefix(body, "@@") && !neg:
			c.set[sigil.FieldStart], ok = sigil.ParseDate(body[2:], today)
		case body[0] == '@' && !neg:
			c.set[sigil.FieldDue], ok = sigil.ParseDate(body[1:], today)
		case body[0] == '*' && !neg:
			c.set[sigil.FieldRepeat], ok = sigil.ParseRepeat(body[1:])
		case body[0] == '~' && !neg:
			c.set[sigil.FieldColor], ok = sigil.ParseColor(body[1:])
		case body[0] == '$' && !neg:
			c.set[sigil.FieldState], ok = sigil.ParseState(body[1:])
		}
		if !ok {
			return change{}, w
		}
	}
	return c, ""
}

// stampOps is the change applied to one row.
func (m *Model) stampOps(id string, c change) []outline.Op {
	want := map[string]string{}
	for k, v := range c.set {
		want[k] = v
	}
	// A row with no such day has nothing to move and is left as it is.
	for field, by := range c.shift {
		if day := m.tree.Row(id).Fields[field]; day != "" {
			want[field] = sigil.ShiftDate(day, by.n, by.unit)
		}
	}
	// -##name takes the context off only the rows that have that one.
	if ctx := m.tree.Row(id).Fields[sigil.FieldContext]; slices.Contains(c.unctx, ctx) {
		if _, set := want[sigil.FieldContext]; !set {
			want[sigil.FieldContext] = ""
		}
	}
	if len(c.add)+len(c.strip) > 0 {
		var tags []string
		// -#name takes the tag off whatever its value; -#name=value only
		// that one.
		for _, tag := range strings.Fields(m.tree.Row(id).Fields[sigil.FieldTags]) {
			if !slices.Contains(c.strip, tag) && !slices.Contains(c.strip, sigil.TagName(tag)) {
				tags = append(tags, tag)
			}
		}
		for _, tag := range c.add {
			tags = sigil.PutTag(tags, tag)
		}
		want[sigil.FieldTags] = strings.Join(tags, " ")
	}
	return m.setFields(id, want)
}

// shownRange returns the inclusive visible range, in screen order, leaving
// out the dimmed ancestors of contextual filter matches. It never walks the tree.
func (m *Model) shownRange(first, last int) []string {
	if first > last {
		first, last = last, first
	}
	var ids []string
	for i := max(first, 0); i <= last && i < len(m.vis); i++ {
		id := m.vis[i].id
		if m.at.query == "" || m.at.matched[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

// A sweep owns only the span it adds; earlier picks survive shrinking it.
func (m *Model) startSweep() {
	if m.current() == "" {
		return
	}
	m.sweep = m.current()
	m.sweepBase = maps.Clone(m.picked)
	m.updateSweep()
}

func (m *Model) endSweep() {
	m.sweep, m.sweepBase = "", nil
}

func (m *Model) clearSelection() {
	if len(m.picked) > 0 {
		m.lastPicked = m.picked
	}
	m.endSweep()
	m.picked = map[string]bool{}
}

func (m *Model) updateSweep() {
	if m.sweep == "" {
		return
	}
	for i, row := range m.vis {
		if row.id == m.sweep {
			m.picked = maps.Clone(m.sweepBase)
			for _, id := range m.shownRange(i, m.cursor) {
				m.picked[id] = true
			}
			return
		}
	}
	// A refresh can remove the anchor while the sweep is live.
	m.endSweep()
}

// countedRows takes matches from the cursor down without changing selection.
// A zero count preserves the ordinary cursor/selection targeting rules.
func (m *Model) countedRows(count int) []string {
	if count == 0 {
		return m.acting()
	}
	last, taken := m.cursor-1, 0
	for last+1 < len(m.vis) && taken < count {
		last++
		if m.at.query == "" || m.at.matched[m.vis[last].id] {
			taken++
		}
	}
	if taken == 0 {
		return nil
	}
	return m.shownRange(m.cursor, last)
}

// acting is the rows an act lands on: the selection, else the cursor's row.
func (m *Model) acting() []string {
	if len(m.picked) == 0 {
		if cur := m.current(); cur != "" {
			return []string{cur}
		}
		return nil
	}
	var out []string
	for _, v := range m.vis {
		if m.picked[v.id] {
			out = append(out, v.id)
		}
	}
	// Selected rows a fold now hides still count.
	var hidden []string
	for id := range m.picked {
		if !slices.Contains(out, id) {
			hidden = append(hidden, id)
		}
	}
	slices.Sort(hidden)
	return append(out, hidden...)
}

// pick puts every acting row in Today, or takes them all out when all are
// picked. A pick carried from an earlier day is a pick: it is kept as it
// is when others join it, and t takes it out.
func (m *Model) pick(ids []string) {
	day := m.today().Format("2006-01-02")
	all := true
	for _, id := range ids {
		r := m.tree.Row(id)
		all = all && isPicked(r.Fields, sigil.FromFields(r.Fields), day)
	}
	if all {
		m.toggle(ids, sigil.FieldToday, "")
		return
	}
	var fresh []string
	for _, id := range ids {
		if r := m.tree.Row(id); !isPicked(r.Fields, sigil.FromFields(r.Fields), day) {
			fresh = append(fresh, id)
			// Picked for today, a row in Today joins the day's own rows.
			delete(m.at.carried, id)
		}
	}
	m.toggle(fresh, sigil.FieldToday, day)
}

// toggle sets a field on every acting row, or clears it when all hold it.
func (m *Model) toggle(ids []string, field, value string) {
	all := true
	for _, id := range ids {
		all = all && m.tree.Row(id).Fields[field] == value
	}
	if all {
		value = ""
	}
	var last string
	if field == sigil.FieldToday && value != "" {
		m.scan()
		for _, id := range m.lists[kindToday] {
			last = max(last, m.tree.Row(id).Fields[fieldTodayPos])
		}
	}
	var ops []outline.Op
	for _, id := range ids {
		if m.tree.Row(id).Fields[field] != value {
			ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: field, Value: value})
			if field == sigil.FieldToday && value != "" {
				last = outline.Between(last, "")
				ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: fieldTodayPos, Value: last})
			}
		}
	}
	if len(ops) > 0 {
		m.do(ops...)
	}
}

// export writes the whole outline as indented lines in the row grammar, so
// the data is never only inside the database.
func (m *Model) export(path string) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			m.msg = err.Error()
			return
		}
		path = filepath.Join(home, "docket-"+m.today().Format("2006-01-02")+".txt")
	} else if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, rest)
		}
	}
	text, n := m.writeRows(m.tree.Children(""))
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		m.msg = err.Error()
		return
	}
	m.msg = "exported " + rowCount(n) + " to " + path
}

// writeRows is the shared copy/export writer. Each root starts at depth zero;
// folds and closed-row visibility do not affect the live subtree.
func (m *Model) writeRows(roots []string) (string, int) {
	var b strings.Builder
	n, today := 0, m.today()
	var walk func(string, int)
	walk = func(id string, depth int) {
		if id == filtersID || !m.live(id) {
			return
		}
		row := m.tree.Row(id)
		text := sigil.Format(sigil.FromFields(row.Fields), m.defaultsToNotes(row.Parent), today)
		if text == "" {
			state := row.Fields[sigil.FieldState]
			if state == "" {
				state = "note"
			}
			text = "$" + state
		}
		if pastePrefix(text) {
			text = `\` + text
		}
		b.WriteString(strings.Repeat("\t", depth) + text + "\n")
		if body := row.Fields[fieldBody]; body != "" {
			// The body follows its row, one tab deeper, each line after >.
			for _, line := range strings.Split(body, "\n") {
				b.WriteString(strings.Repeat("\t", depth+1) + strings.TrimRight("> "+line, " ") + "\n")
			}
		}
		n++
		for _, child := range m.tree.Children(id) {
			walk(child, depth+1)
		}
	}
	for _, id := range roots {
		walk(id, 0)
	}
	return b.String(), n
}

// savedFilter is a query kept under a name, with the place it was made in.
// It is stored as a row under a hidden top-level row, so it is undone,
// backed up and synced like everything else.
type savedFilter struct {
	ID, Name, Root, Query string
	Flat                  bool
}

const (
	fieldQuery = "query"
	fieldFlat  = "flat"
	fieldRoot  = "root"
)

// keepFilter saves the pane's filter under a name, or forgets a saved one.
func (m *Model) keepFilter(save bool, name string) {
	id := ""
	for _, f := range m.filters {
		if f.Name == name {
			id = f.ID
		}
	}
	var ops []outline.Op
	set := func(field, value string) {
		ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: field, Value: value})
	}
	switch {
	case name == "":
		m.msg = "name it: save name, or forget name"
		return
	case save && (m.at.query == "" || m.at.list != ""):
		m.msg = "filter an outline first (/), then save it"
		return
	case !save && id == "":
		m.msg = "no saved filter is called " + name
		return
	case !save:
		set(outline.FieldDeleted, "1")
	default:
		if id == "" {
			id = m.newID()
			ops = append(ops, outline.Op{Kind: outline.Insert, Row: id, Parent: filtersID, Pos: m.tree.PosLast(filtersID, false)})
			set(sigil.FieldText, name)
		}
		flat := ""
		if m.at.flat {
			flat = "1"
		}
		set(fieldQuery, m.at.query)
		set(fieldFlat, flat)
		set(fieldRoot, m.at.root)
	}
	switch {
	case !m.do(ops...):
	case save:
		m.msg = "saved: /" + name + " is in the sidebar, and f opens it by name"
	default:
		m.msg = "forgot /" + name
	}
}
