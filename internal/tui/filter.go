package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

// term is one word of a filter. Every term must hold for a row to match; a
// leading minus turns a term into its opposite.
type term struct {
	neg   bool
	kind  byte // t text, ! flag, # tag, $ state, @ deadline
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
		switch w[0] {
		case '!':
			if w == "!" {
				t.kind = '!'
			}
		case '#':
			if tag, ok := sigil.ParseTag(w[1:]); ok {
				t.kind, t.value = '#', tag
			}
		case '$':
			if state, ok := sigil.ParseState(w[1:]); ok {
				t.kind, t.value = '$', state
			}
		case '*':
			if every, ok := sigil.ParseRepeat(w[1:]); ok || w == "*" {
				t.kind, t.value = '*', every
			}
		case '@':
			if rest, start := strings.CutPrefix(strings.ToLower(w), "@@"); start {
				if day, ok := sigil.ParseDate(rest, today); ok || rest == "" {
					t.kind, t.value = 's', day
				} else if rest == "later" {
					t.kind, t.value = 's', rest
				}
				break
			}
			switch name := strings.ToLower(w[1:]); name {
			case "", "overdue", "week":
				t.kind, t.value = '@', name
			default:
				if day, ok := sigil.ParseDate(name, today); ok {
					t.kind, t.value = '@', day
				}
			}
		}
		if t.kind == 0 {
			t.kind, t.value = 't', strings.ToLower(w)
		}
		out = append(out, t)
	}
	return out
}

func matches(r *outline.Row, terms []term, today time.Time) bool {
	l := sigil.FromFields(r.Fields)
	for _, t := range terms {
		var ok bool
		switch t.kind {
		case 't':
			ok = strings.Contains(strings.ToLower(l.Text), t.value)
		case '!':
			ok = l.Flag
		case '#':
			ok = slices.Contains(l.Tags, t.value)
		case '$':
			ok = l.State == t.value
		case '@':
			_, days := sigil.DueLabel(l.Due, today)
			switch t.value {
			case "":
				ok = l.Due != ""
			case "overdue":
				ok = l.Due != "" && days < 0 && !l.Closed()
			case "week":
				ok = l.Due != "" && days <= dueSoonDays && !l.Closed()
			default:
				ok = l.Due == t.value
			}
		case 's':
			switch t.value {
			case "":
				ok = l.Start != ""
			case "later":
				ok = l.Later(today)
			default:
				ok = l.Start == t.value
			}
		case '*':
			ok = l.Every != "" && (t.value == "" || l.Every == t.value)
		}
		if ok == t.neg {
			return false
		}
	}
	return true
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
	if m.at.list != "" {
		for _, id := range m.at.rows {
			if m.live(id) && matches(m.tree.Row(id), terms, today) {
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
			if id == inboxID && parent == "" {
				continue
			}
			at := len(m.at.hits)
			ok := matches(m.tree.Row(id), terms, today)
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
	var ops []outline.Op
	for _, id := range ids {
		if m.tree.Row(id).Fields[field] != value {
			ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: field, Value: value})
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
	var b strings.Builder
	n, today := 0, m.today()
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		for _, id := range m.tree.Children(parent) {
			line := sigil.FromFields(m.tree.Row(id).Fields)
			b.WriteString(strings.Repeat("\t", depth) + sigil.Format(line, parent == "", today) + "\n")
			n++
			walk(id, depth+1)
		}
	}
	walk("", 0)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		m.msg = err.Error()
		return
	}
	m.msg = fmt.Sprintf("exported %d rows to %s", n, path)
}

// savedFilter is a query kept under a name, with the place it was made in.
// It is a setting of this machine.
type savedFilter struct {
	Name, Root, Query string
	Flat              bool
}

// keepFilter saves the pane's filter under a name, or forgets a saved one.
func (m *Model) keepFilter(save bool, name string) {
	at := -1
	for i, f := range m.filters {
		if f.Name == name {
			at = i
		}
	}
	switch {
	case name == "":
		m.msg = "name it: save name, or forget name"
		return
	case save && (m.at.query == "" || m.at.list != ""):
		m.msg = "filter an outline first (/), then save it"
		return
	case save && at >= 0:
		m.filters[at] = savedFilter{name, m.at.root, m.at.query, m.at.flat}
	case save:
		m.filters = append(m.filters, savedFilter{name, m.at.root, m.at.query, m.at.flat})
	case at < 0:
		m.msg = "no saved filter is called " + name
		return
	default:
		m.filters = append(m.filters[:at], m.filters[at+1:]...)
	}
	b, err := json.Marshal(m.filters)
	if err == nil {
		err = m.backend.SetMeta(metaFilters, string(b))
	}
	switch {
	case err != nil:
		m.msg = "not saved: " + err.Error()
	case save:
		m.msg = "saved: p then /" + name + " opens it"
	default:
		m.msg = "forgot /" + name
	}
}
