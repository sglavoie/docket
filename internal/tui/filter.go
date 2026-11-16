package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

// term is one word of a filter. Every term must hold for a row to match; a
// leading minus turns a term into its opposite. A word may offer several
// readings with a bar between them, and holds when any one does.
type term struct {
	neg bool
	any []cond
}

// cond is one reading of a term.
type cond struct {
	kind  byte // t text, ! flag, # tag, $ state, n next action, @ deadline, s start, * repeat, ~ changed
	value string
}

// parseQuery reads a filter in the grammar rows are written in. A word that
// is not a readable sigil matches the row's text.
func parseQuery(q string, today time.Time) []term {
	var out []term
	for _, w := range strings.Fields(q) {
		var t term
		if len(w) > 1 && w[0] == '-' {
			t.neg, w = true, w[1:]
		}
		if w == "#" || w == "$" {
			continue // a sigil still being typed
		}
		// The sigil is written once and stands for every alternative:
		// $open|doing reads as $open or $doing.
		sig := w[:len(w)-len(strings.TrimLeft(w, "#$@*~"))]
		if parts := strings.Split(w[len(sig):], "|"); len(parts) > 1 && !slices.Contains(parts, "") {
			for _, part := range parts {
				c := parseCond(sig+part, today)
				if sig != "" && c.kind == 't' {
					t.any = nil // one alternative cannot be read: the word is text
					break
				}
				t.any = append(t.any, c)
			}
		}
		if t.any == nil {
			t.any = []cond{parseCond(w, today)}
		}
		out = append(out, t)
	}
	return out
}

// parseCond reads one word, without its minus, as a sigil or else as text.
func parseCond(w string, today time.Time) cond {
	var c cond
	switch w[0] {
	case '!':
		if w == "!" {
			c.kind = '!'
		}
	case '#':
		if tag, ok := sigil.ParseTag(w[1:]); ok {
			c.kind, c.value = '#', tag
		}
	case '$':
		// $next is read before the states: it is a filter word, not a state.
		if strings.ToLower(w) == "$next" {
			c.kind = 'n'
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
	case '@':
		if rest, start := strings.CutPrefix(strings.ToLower(w), "@@"); start {
			if day, ok := sigil.ParseDate(rest, today); ok || rest == "" {
				c.kind, c.value = 's', day
			} else if rest == "later" {
				c.kind, c.value = 's', rest
			}
			break
		}
		switch name := strings.ToLower(w[1:]); name {
		case "", "overdue", "week":
			c.kind, c.value = '@', name
		default:
			if day, ok := sigil.ParseDate(name, today); ok {
				c.kind, c.value = '@', day
			}
		}
	}
	if c.kind == 0 {
		c.kind, c.value = 't', strings.ToLower(w)
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

// matches reads one row against a query. next is each project's next
// action, as the last scan found it.
func matches(r *outline.Row, terms []term, today time.Time, next map[string]bool) bool {
	l := sigil.FromFields(r.Fields)
	for _, t := range terms {
		ok := false
		for _, c := range t.any {
			ok = ok || holds(r, l, c, today, next)
		}
		if ok == t.neg {
			return false
		}
	}
	return true
}

func holds(r *outline.Row, l sigil.Line, c cond, today time.Time, next map[string]bool) bool {
	switch c.kind {
	case 't':
		return strings.Contains(strings.ToLower(l.Text), c.value)
	case '!':
		return l.Flag
	case '#':
		return slices.Contains(l.Tags, c.value)
	case '$':
		return l.State == c.value
	case 'n':
		return next[r.ID]
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

// refilter reads the place's query against its rows. In an outline a match
// keeps its ancestors, so it is read where it lives; flat drops them.
func (m *Model) refilter() {
	m.at.hits, m.at.matched = nil, map[string]bool{}
	today := m.today()
	terms := parseQuery(m.at.query, today)
	if len(terms) == 0 {
		return
	}
	m.scan() // $next reads the scan's answer
	if m.at.list != "" {
		for _, id := range m.at.rows {
			if m.live(id) && matches(m.tree.Row(id), terms, today, m.next) {
				m.at.hits = append(m.at.hits, visRow{id, 0})
				m.at.matched[id] = true
			}
		}
		return
	}
	var walk func(parent string, depth int) bool
	walk = func(parent string, depth int) bool {
		any := false
		for _, id := range m.tree.Children(parent) {
			if hiddenTop(id) {
				continue
			}
			at := len(m.at.hits)
			ok := matches(m.tree.Row(id), terms, today, m.next)
			if ok {
				m.at.matched[id] = true
			}
			if m.at.flat {
				if ok {
					m.at.hits = append(m.at.hits, visRow{id, 0})
				}
				any = walk(id, 0) || ok || any
				continue
			}
			m.at.hits = append(m.at.hits, visRow{id, depth})
			below := walk(id, depth+1)
			if !ok && !below {
				m.at.hits = m.at.hits[:at]
			}
			any = any || ok || below
		}
		return any
	}
	walk(m.at.root, 0)
}

// change is what a stamp asks of each row it lands on.
type change struct {
	set        map[string]string
	add, strip []string // tags
}

// parseStamp reads a line of sigils. It returns the first word it cannot
// read, in which case nothing is applied.
func parseStamp(line string, today time.Time) (change, string) {
	c := change{set: map[string]string{}}
	for _, w := range strings.Fields(line) {
		neg := len(w) > 1 && w[0] == '-'
		body := strings.TrimPrefix(w, "-")
		ok := false
		switch {
		case body == "!":
			c.set[sigil.FieldFlag], ok = "1", true
			if neg {
				c.set[sigil.FieldFlag] = ""
			}
		case body == "@" && neg:
			c.set[sigil.FieldDue], ok = "", true
		case body == "@@" && neg:
			c.set[sigil.FieldStart], ok = "", true
		case body == "*" && neg:
			c.set[sigil.FieldRepeat], ok = "", true
		case len(body) < 2:
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
	if len(c.add)+len(c.strip) > 0 {
		var tags []string
		for _, tag := range strings.Fields(m.tree.Row(id).Fields[sigil.FieldTags]) {
			if !slices.Contains(c.strip, tag) {
				tags = append(tags, tag)
			}
		}
		for _, tag := range c.add {
			if !slices.Contains(tags, tag) {
				tags = append(tags, tag)
			}
		}
		want[sigil.FieldTags] = strings.Join(tags, " ")
	}
	return m.setFields(id, want)
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
	m.msg = fmt.Sprintf("exported %d rows to %s", n, path)
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
			ops = append(ops, outline.Op{Kind: outline.Insert, Row: id, Parent: filtersID, Pos: m.tree.PosLast(filtersID)})
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
		m.msg = "saved: p then /" + name + " opens it"
	default:
		m.msg = "forgot /" + name
	}
}
