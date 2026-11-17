package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

// fieldWords names each stored field as the status line speaks of it.
// A field not named here is bookkeeping and is not mentioned.
var fieldWords = map[string]string{
	sigil.FieldText:   "text",
	sigil.FieldState:  "state",
	sigil.FieldFlag:   "flag",
	sigil.FieldDue:    "deadline",
	sigil.FieldStart:  "start day",
	sigil.FieldTags:   "tags",
	sigil.FieldRepeat: "repeat",
	sigil.FieldToday:  "Today pick",
	fieldTodayPos:     "Today order",
	fieldLink:         "link",
	fieldQuery:        "saved filter",
}

// fieldOrder is the order fields are named in, most telling first.
var fieldOrder = []string{
	sigil.FieldText, sigil.FieldState, sigil.FieldFlag, sigil.FieldDue,
	sigil.FieldStart, sigil.FieldTags, sigil.FieldRepeat, sigil.FieldToday,
	fieldTodayPos, fieldLink, fieldQuery,
}

func rowCount(n int) string {
	if n == 1 {
		return "1 row"
	}
	return fmt.Sprintf("%d rows", n)
}

// matchCount says how many rows a filter holds, as a sentence would.
func matchCount(n int) string {
	switch n {
	case 0:
		return "no match"
	case 1:
		return "1 match"
	}
	return fmt.Sprintf("%d matches", n)
}

// failing reports a message saying that something was not written: a save,
// a backup or a sync pass. The status line draws it in the alert colour.
func failing(msg string) bool {
	for _, start := range []string{"not saved", "no backup", "sync failed", "sync stuck"} {
		if strings.HasPrefix(msg, start) {
			return true
		}
	}
	return false
}

// describe says what a group of operations did to rows, for the status
// line after undo and redo: rows that came back or went, rows moved, and
// which fields changed on how many rows. A row that came back or went is
// not also counted as moved or changed, and takes the rows under it along,
// as the count a delete gives does.
func describe(group []outline.Op, under func(id string) []string) string {
	restored, removed := map[string]bool{}, map[string]bool{}
	moved := map[string]bool{}
	changed := map[string]map[string]bool{}
	for _, op := range group {
		switch {
		case op.Kind == outline.Insert:
			restored[op.Row] = true
		case op.Kind == outline.Move:
			moved[op.Row] = true
		case op.Field == outline.FieldDeleted && op.Value == "":
			restored[op.Row] = true
		case op.Field == outline.FieldDeleted:
			removed[op.Row] = true
		default:
			if changed[op.Field] == nil {
				changed[op.Field] = map[string]bool{}
			}
			changed[op.Field][op.Row] = true
		}
	}
	for _, set := range []map[string]bool{restored, removed} {
		for _, id := range slices.Collect(maps.Keys(set)) {
			for _, kid := range under(id) {
				set[kid] = true
			}
		}
	}
	gone := func(id string) bool { return restored[id] || removed[id] }
	var parts []string
	if n := len(restored); n > 0 {
		parts = append(parts, rowCount(n)+" back")
	}
	if n := len(removed); n > 0 {
		parts = append(parts, rowCount(n)+" gone")
	}
	n := 0
	for id := range moved {
		if !gone(id) {
			n++
		}
	}
	if n > 0 {
		parts = append(parts, rowCount(n)+" moved")
	}
	// Fields changed on the same rows are named together: "text and tags
	// changed on 2 rows" reads better than two counts of the same rows.
	var names []string
	rows := map[string]bool{}
	for _, f := range fieldOrder {
		got := false
		for id := range changed[f] {
			if !gone(id) {
				rows[id], got = true, true
			}
		}
		if got {
			names = append(names, fieldWords[f])
		}
	}
	if len(names) > 0 {
		list := names[0]
		if len(names) > 1 {
			list = strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
		}
		parts = append(parts, list+" changed on "+rowCount(len(rows)))
	}
	if len(parts) == 0 {
		return "a change to the view"
	}
	return strings.Join(parts, ", ")
}

// marks are the conditions a match is drawn with: the words and phrases a
// query looks for in a row's own text, and the tags it asks the row for.
// A term that excludes, or that reads other rows than the match, marks
// nothing, nor does one inside a part that must not hold.
func (q query) marks() []cond {
	use := make([]bool, len(q.terms))
	if q.expr == nil {
		for i := range use {
			use[i] = true
		}
	} else {
		var walk func(g *group, neg bool)
		walk = func(g *group, neg bool) {
			neg = neg != g.neg
			if g.alts == nil {
				use[g.leaf] = !neg
				return
			}
			for _, run := range g.alts {
				for i := range run {
					walk(&run[i], neg)
				}
			}
		}
		walk(q.expr, false)
	}
	var out []cond
	for i, t := range q.terms {
		if !use[i] || t.neg || t.above || t.below || t.parent || t.child {
			continue
		}
		for _, c := range t.any {
			if (c.kind == 't' || c.kind == '#') && c.value != "" {
				out = append(out, c)
			}
		}
	}
	return out
}

// markWords says which of a row's text words a match is drawn on: every
// word a looked-for word or phrase overlaps, ignoring case.
func markWords(text string, marks []cond) map[int]bool {
	lower := strings.ToLower(text)
	type span struct{ from, to int }
	var spans []span
	for _, c := range marks {
		if c.kind != 't' {
			continue
		}
		for at := 0; at < len(lower); {
			i := strings.Index(lower[at:], c.value)
			if i < 0 {
				break
			}
			spans = append(spans, span{at + i, at + i + len(c.value)})
			at += i + max(len(c.value), 1)
		}
	}
	if len(spans) == 0 {
		return nil
	}
	out := map[int]bool{}
	n, start := 0, -1
	end := func(i int) {
		for _, s := range spans {
			if s.from < i && s.to > start {
				out[n] = true
			}
		}
		n++
		start = -1
	}
	for i, r := range lower {
		blank := unicode.IsSpace(r)
		switch {
		case blank && start >= 0:
			end(i)
		case !blank && start < 0:
			start = i
		}
	}
	if start >= 0 {
		end(len(lower))
	}
	return out
}

// markTag reports a tag the query asks the row for.
func markTag(tag string, marks []cond) bool {
	for _, c := range marks {
		if c.kind == '#' && (tag == c.value || sigil.TagName(tag) == c.value) {
			return true
		}
	}
	return false
}

// nextKeys says what may follow the outline keys waiting on the status
// line, so a prefix or an operator teaches its own grammar.
func (m *Model) nextKeys() string {
	if m.help || m.ask != nil || m.edit != nil || m.sideFocus {
		return ""
	}
	op := m.operator
	switch {
	case op == "c":
		return "cc retypes the text, keeping its fields"
	case op != "" && m.prefix == "g":
		return op + "gg takes through the first row"
	case op != "" && m.prefix == "i":
		return op + "ic the rows under it · " + op + "is the row and its siblings"
	case op != "":
		return op + op + " the row · " + op + "j } ) H ] n G… through a motion · " + op + "ic " + op + "is objects"
	case m.prefix == "g":
		return "gg top · gl follow link · gn gN into the match · gp gP put the copy · g; g, last changes"
	case m.prefix == "z":
		return "zc zo shut, open the branch · zt zz zb draw at top, middle, bottom"
	}
	return ""
}

// commandUses are the commands as they are written, each with what it does,
// in the order the command line names them.
var commandUses = []struct{ use, does string }{
	{"q", "quit"},
	{"help", "the keys, in eight sections"},
	{"tutor", "a fresh Welcome tutorial"},
	{"closed", "hide or show done and canceled rows"},
	{"numbers", "show or hide each row's count from the cursor"},
	{"all", "select every row shown"},
	{"s/old/new", "change text in the row or the selection"},
	{"sort [keys]", "order siblings once: @ deadline, ! flag, $ state, else text"},
	{"fold [level]", "fold all, or keep that many levels open"},
	{"unfold", "open every fold here"},
	{"archive", "move closed rows under Archive:"},
	{"review", "active projects, longest untouched first"},
	{"save name", "keep this filter as /name"},
	{"forget name", "drop a saved filter"},
	{"export [file]", "write the outline as text"},
	{"import file", "read rows from a file"},
	{"sync [folder|off]", "share through a folder, or stop"},
}

// commandOffers names the commands the line being typed could become, so
// they are read off the line and not remembered: all of them on a blank
// line, fewer with each letter, and the last one left with what it does.
func commandOffers(line string) string {
	letters := func(s string) string {
		for i, r := range s {
			if r < 'a' || r > 'z' {
				return s[:i]
			}
		}
		return s
	}
	line = strings.TrimLeft(line, " ")
	name := letters(line)
	whole := len(line) > len(name) // the name is finished: only it answers
	var uses []string
	does := ""
	for _, c := range commandUses {
		is := letters(c.use)
		if is == "q" && name != "" && strings.HasPrefix("quit", name) || is == "help" && name == "h" {
			is = name
		}
		if is == name || !whole && strings.HasPrefix(is, name) {
			uses, does = append(uses, c.use), c.does
		}
	}
	if line == "" {
		// Seventeen commands are scanned by name; what each takes is said
		// once a letter has narrowed them.
		for i, use := range uses {
			uses[i], _, _ = strings.Cut(use, " ")
		}
	}
	switch len(uses) {
	case 0:
		return "no such command"
	case 1:
		return uses[0] + ": " + does
	}
	return strings.Join(uses, " · ")
}

// tagWord is the tag being typed at the end of a line, without its #, or
// false when the last word is no tag.
func tagWord(e *editor, surface string) (string, bool) {
	if e.caret != len(e.buf) || len(e.buf) == 0 {
		return "", false
	}
	line := string(e.buf)
	w := line[strings.LastIndexAny(line, " \t")+1:]
	if surface == askStamp || surface == askFilter {
		w = strings.TrimPrefix(w, "-")
	}
	if surface == askFilter && w != "" && strings.ContainsRune("^_<>", rune(w[0])) {
		w = w[1:]
	}
	if !strings.HasPrefix(w, "#") {
		return "", false
	}
	if surface == askFilter {
		if i := strings.LastIndexByte(w, '|'); i >= 0 {
			w = w[i+1:]
			if strings.HasPrefix(w, "#") {
				w = w[1:]
			} else if strings.ContainsAny(w[:min(len(w), 1)], sigilStarts) {
				return "", false
			}
			w = "#" + w
		}
	}
	w = strings.ToLower(w[1:])
	if strings.ContainsAny(w, "=<>") {
		return "", false
	}
	return w, true
}

// tagOffers names the tags in use that the tag being typed could become,
// most used first, each with how many rows carry it, so the line shows
// what tags there are without a list of its own.
func (m *Model) tagOffers(e *editor, surface string) string {
	typed, ok := tagWord(e, surface)
	if !ok {
		return ""
	}
	// A tag with values is one name: #est counts every #est=….
	use := map[string]int{}
	var names []string
	for _, tag := range m.tags {
		name := sigil.TagName(tag)
		if !strings.HasPrefix(name, typed) {
			continue
		}
		if _, seen := use[name]; !seen {
			names = append(names, name)
		}
		use[name] += m.tagUse[tag]
	}
	slices.SortStableFunc(names, func(a, b string) int { return use[b] - use[a] })
	var parts []string
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("#%s %d", name, use[name]))
	}
	if len(parts) == 0 {
		return "a new tag"
	}
	return strings.Join(parts, " · ")
}

// starters are the filters a new outline is given, so the views worth
// keeping are in the f picker from the first day: what could be done
// next, what is due this week, and what was finished this week.
var starters = []savedFilter{
	{Name: "next", Query: "$next", Flat: true},
	{Name: "week", Query: "@week", Flat: true},
	{Name: "done", Query: "$done ~7d", Flat: true},
}

// starterFilters saves the starter filters as part of the undo step that
// made the tutorial, when no filter has been saved yet.
func (m *Model) starterFilters() {
	if len(m.filters) > 0 || len(m.undo) == 0 {
		return
	}
	var ops, inv []outline.Op
	for _, f := range starters {
		id := m.newID()
		flat := ""
		if f.Flat {
			flat = "1"
		}
		group := []outline.Op{
			{Kind: outline.Insert, Row: id, Parent: filtersID, Pos: m.tree.PosLast(filtersID)},
			{Kind: outline.Set, Row: id, Field: sigil.FieldText, Value: f.Name},
			{Kind: outline.Set, Row: id, Field: fieldQuery, Value: f.Query},
			{Kind: outline.Set, Row: id, Field: fieldFlat, Value: flat},
		}
		undo, err := m.apply(group)
		if err != nil {
			break
		}
		ops, inv = append(ops, group...), append(undo, inv...)
	}
	if len(ops) == 0 || !m.save(ops, inv) {
		return
	}
	last := len(m.undo) - 1
	m.undo[last] = append(inv, m.undo[last]...)
	m.keepHistory()
	m.rebuild()
}

// strayKeys are keys a Vim hand reaches for that the outline does not
// bind. Each names the outline's way to do what was meant; none acts, so
// none is a second route (rule 3).
var strayKeys = map[string]string{
	"i": "I types at the start, A at the end; w steps into the text",
	"s": "cc retypes the row, keeping its fields; w steps into the text",
	"S": "cc retypes the row, keeping its fields",
	"C": "cc retypes the row, keeping its fields; A types at the end",
	"r": "w steps into the text, where r replaces a character",
	"R": "cc retypes the row, keeping its fields",
	"X": "dd deletes the row; x marks it done",
	"D": "dd deletes the row with what is under it",
	"Y": "yy copies the row with what is under it",
	"U": "u undoes and Ctrl+R redoes, a step at a time",
	"q": ":q quits; . repeats the last change, so there are no macros",
	"~": "w steps into the text, where ~ switches case",
}

// changeCap is how many changed rows g; and g, can walk back through.
const changeCap = 100

// noteChange keeps the row a change was made on, for g; and g,. A run of
// changes to one row is one place to come back to; a saved filter is none.
func (m *Model) noteChange(ops []outline.Op) {
	if len(ops) == 0 {
		return
	}
	id := ops[len(ops)-1].Row
	for top := id; top != ""; {
		r := m.tree.Row(top)
		if r == nil {
			return
		}
		if r.Parent == "" && top == filtersID {
			return
		}
		top = r.Parent
	}
	if n := len(m.changes); n == 0 || m.changes[n-1] != id {
		m.changes = append(m.changes, id)
		if len(m.changes) > changeCap {
			m.changes = m.changes[len(m.changes)-changeCap:]
		}
	}
	m.changeAt = len(m.changes)
}

// walkChanges goes count changes back (g;) or on (g,) through the rows
// changed this session. A row since deleted stands for its nearest live
// ancestor. On screen, the cursor moves to it; elsewhere it is opened
// where it lives, as a jump, so Ctrl+O comes back.
func (m *Model) walkChanges(older bool, count int) {
	if len(m.changes) == 0 {
		m.msg = "no change made yet this session"
		return
	}
	step := 1
	if older {
		step = -1
	}
	cur := m.current()
	at, target, found := m.changeAt, "", m.changeAt
	for left := max(count, 1); left > 0; {
		next := at + step
		if next < 0 || next >= len(m.changes) {
			break
		}
		at = next
		id := m.changes[at]
		for id != "" && !m.live(id) {
			if r := m.tree.Row(id); r != nil {
				id = r.Parent
			} else {
				id = ""
			}
		}
		// A change on the row the cursor is on is no place to go.
		if id == "" || (target == "" && id == cur) {
			continue
		}
		target, found = id, at
		left--
	}
	if target == "" {
		if older {
			m.msg = "at the oldest change"
		} else {
			m.msg = "at the newest change"
		}
		return
	}
	m.changeAt = found
	m.msg = fmt.Sprintf("change %d of %d", found+1, len(m.changes))
	for i, v := range m.vis {
		if v.id == target {
			m.cursor = i
			m.land()
			return
		}
	}
	m.jump(entry{kind: kindRow, id: target})
	m.msg = fmt.Sprintf("change %d of %d", found+1, len(m.changes))
}
