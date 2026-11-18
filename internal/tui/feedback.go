package tui

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

// fieldWords names each stored field as the status line speaks of it.
// A field not named here is bookkeeping and is not mentioned.
var fieldWords = map[string]string{
	sigil.FieldText:    "text",
	fieldBody:          "body",
	sigil.FieldState:   "state",
	sigil.FieldFlag:    "flag",
	sigil.FieldPin:     "pin",
	sigil.FieldDue:     "deadline",
	sigil.FieldStart:   "start day",
	sigil.FieldTags:    "tags",
	sigil.FieldContext: "context",
	sigil.FieldRepeat:  "repeat",
	sigil.FieldToday:   "Today pick",
	fieldTodayPos:      "Today order",
	fieldLink:          "link",
	fieldQuery:         "saved filter",
}

// fieldOrder is the order fields are named in, most telling first.
var fieldOrder = []string{
	sigil.FieldText, fieldBody, sigil.FieldState, sigil.FieldFlag, sigil.FieldPin, sigil.FieldDue,
	sigil.FieldStart, sigil.FieldTags, sigil.FieldContext, sigil.FieldRepeat, sigil.FieldToday,
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
// query looks for in a row's own text, and the tags and context it asks the
// row for.
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
			if (c.kind == 't' || c.kind == '#' || c.kind == 'c') && c.value != "" {
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

// markBody reports a looked-for word or phrase in a body.
func markBody(body string, marks []cond) bool {
	lower := strings.ToLower(body)
	for _, c := range marks {
		if c.kind == 't' && !c.line && strings.Contains(lower, c.value) {
			return true
		}
	}
	return false
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

// markContext reports a context the query asks the row for.
func markContext(ctx string, marks []cond) bool {
	for _, c := range marks {
		if c.kind == 'c' && c.value == ctx {
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
	case op == "c" && len(m.picked) > 0:
		return "cc retypes one row, not a selection"
	case op == "c":
		return "cc retypes the text, keeping its fields"
	case op != "" && len(m.picked) > 0:
		// Doubled, the key takes the selection; anything else is refused, so
		// the line does not offer it.
		rows := "the selected row"
		if n := len(m.picked); n > 1 {
			rows = fmt.Sprintf("the %d selected rows", n)
		}
		return op + op + " " + rows + " · no motion or object while rows are selected"
	case op != "" && m.prefix == "g":
		return op + "gg takes through the first row"
	case op != "" && m.prefix == "i":
		return op + "ic the rows under it · " + op + "is the row and its siblings"
	case op != "":
		return op + op + " the row · " + op + "j } ) H ] n G… through a motion · " + op + "ic " + op + "is objects"
	case m.prefix == "g":
		return "gg top · gf project · gl follow link · gn gN into the match · gp gP put the copy · g; g, last changes"
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
	uses, does := commandMatches(line)
	if strings.TrimLeft(line, " ") == "" {
		// Seventeen commands are scanned by name; what each takes is said
		// once a letter has narrowed them.
		for i, use := range uses {
			uses[i], _, _ = strings.Cut(use, " ")
		}
	}
	switch len(uses) {
	case 0:
		if hint, ok := strayCommands[strings.ToLower(strings.TrimSpace(line))]; ok {
			return hint
		}
		return "no such command"
	case 1:
		return uses[0] + ": " + does
	}
	return strings.Join(uses, " · ")
}

// commandMatches are the commands, as written, that the line could become,
// with what the last of them does.
func commandMatches(line string) (uses []string, does string) {
	line = strings.TrimLeft(line, " ")
	name := commandName(line)
	whole := len(line) > len(name) // the name is finished: only it answers
	for _, c := range commandUses {
		is := commandName(c.use)
		if is == "q" && name != "" && strings.HasPrefix("quit", name) || is == "help" && name == "h" {
			is = name
		}
		if is == name || !whole && strings.HasPrefix(is, name) {
			uses, does = append(uses, c.use), c.does
		}
	}
	return uses, does
}

// commandDoes is what the command written as use does.
func commandDoes(use string) string {
	for _, c := range commandUses {
		if c.use == use {
			return c.does
		}
	}
	return ""
}

// commandName is the run of letters a command line starts with, read in
// small letters, so a name typed with Shift or Caps Lock held is still the
// command.
func commandName(s string) string {
	for i, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return strings.ToLower(s[:i])
		}
	}
	return strings.ToLower(s)
}

// tagWord is the tag being typed at the end of a line, without its #, or
// false when the last word is no tag. A ## word is a context: ctx says so,
// and the word is given without its ##.
func tagWord(e *editor, surface string) (w string, ctx, ok bool) {
	if e.caret != len(e.buf) || len(e.buf) == 0 {
		return "", false, false
	}
	line := string(e.buf)
	w = line[strings.LastIndexAny(line, " \t")+1:]
	if surface == askStamp || surface == askFilter {
		w = strings.TrimPrefix(w, "-")
	}
	if surface == askFilter && w != "" && strings.ContainsRune("^_<>", rune(w[0])) {
		w = w[1:]
	}
	if !strings.HasPrefix(w, "#") {
		return "", false, false
	}
	if surface == askFilter {
		if i := strings.LastIndexByte(w, '|'); i >= 0 {
			// A side without its own sigil takes the first side's.
			sig := "#"
			if strings.HasPrefix(w, "##") {
				sig = "##"
			}
			side := w[i+1:]
			switch {
			case strings.HasPrefix(side, "#"):
				w = side
			case strings.ContainsAny(side[:min(len(side), 1)], sigilStarts):
				return "", false, false
			default:
				w = sig + side
			}
		}
	}
	w, ctx = strings.CutPrefix(w, "##")
	if !ctx {
		w = w[1:]
	}
	w = strings.ToLower(w)
	if strings.ContainsAny(w, "=<>") || ctx && strings.HasPrefix(w, "#") {
		return "", false, false
	}
	return w, ctx, true
}

// tagOffers names the tags in use that the tag being typed could become,
// most used first, each with how many rows carry it, so the line shows
// what tags there are without a list of its own. A ##context names the
// contexts in use the same way. On the filter a word that names the one
// tag or context it could become offers nothing more: the count of rows
// carrying it would stand beside the count of matches and read as one
// contradicting the other, so the line is read back in words instead.
func (m *Model) tagOffers(e *editor, surface string) string {
	if colors := m.colorOffers(e, surface); colors != "" {
		return colors
	}
	typed, ctx, ok := tagWord(e, surface)
	if !ok {
		return ""
	}
	if ctx {
		var parts []string
		for _, name := range m.contexts {
			if strings.HasPrefix(name, typed) {
				parts = append(parts, fmt.Sprintf("##%s %d", name, m.contextUse[name]))
			}
		}
		if len(parts) == 0 {
			return "a new context"
		}
		if surface == askFilter && len(parts) == 1 && slices.Contains(m.contexts, typed) {
			return ""
		}
		return strings.Join(parts, " · ")
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
	if surface == askFilter && len(names) == 1 && names[0] == typed {
		return ""
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
			{Kind: outline.Insert, Row: id, Parent: filtersID, Pos: m.tree.PosLast(filtersID, false)},
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
	// And the keys other programs taught: help, save and quit, marks, a tick.
	"?":     ":help lists every key; / filters",
	"Z":     saved,
	"'":     noMarks,
	"`":     noMarks,
	"space": "x closes the row, v selects it, t picks it for Today",
}

const (
	saved   = "every change is saved as it is made; :q quits"
	noMarks = "Ctrl+O goes back along jumps and g; to the last change, so there are no marks"
)

// strayCommands are the commands the same hand types to save. Nothing is
// ever unsaved, so each says so and none acts.
var strayCommands = map[string]string{"w": saved, "write": saved, "wq": saved, "x": saved}

// sideSays spells out the marks on the sidebar's selected entry, which are
// too short to explain themselves: it is said where the eye already is,
// while the entry is the one being chosen.
func (m *Model) sideSays() string {
	var says []string
	for _, e := range m.side {
		if !e.same(m.sel) {
			continue
		}
		switch {
		case e.kind == kindStale:
			says = append(says, fmt.Sprintf("open and doing rows untouched for %d days", int(staleAfter.Hours()/24)))
		case e.kind == kindToday && e.done && e.count == 0:
			says = append(says, "✓ every pick is closed")
		case e.kind == kindProject:
			switch e.due {
			case 2:
				says = append(says, "@ a deadline is overdue")
			case 1:
				says = append(says, "@ a deadline within a week")
			}
			if e.idle {
				says = append(says, "∅ nothing left to do")
			} else if e.age > 0 {
				says = append(says, strconv.Itoa(e.age)+"d since anything changed")
			}
		}
	}
	return strings.Join(says, " · ")
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

// unreadStamp says why the stamp line stopped at a word, and what a word of
// its kind reads as, so the line is put right without leaving it.
func unreadStamp(w string) string {
	body := strings.TrimPrefix(w, "-")
	way := "the line reads #tag, ##context, @date, $state, *interval, ~colour and !"
	switch {
	case w != body && strings.HasPrefix(body, "$"):
		way = "a state is set, not taken off, as $open or $note"
	case w != body && (strings.HasPrefix(body, "@") || strings.HasPrefix(body, "*") || strings.HasPrefix(body, "~")):
		way = "-@, -@@, -* and -~ take the whole field off"
	case strings.HasPrefix(body, "~"):
		// On the stamp line ~ is a colour, not the filter's time.
		way = colourForm
	case sigilForm(body) != "":
		way = sigilForm(body)
	}
	return "cannot read " + w + ": " + way
}

// sigilForm says what a word of a sigil's kind reads as, for a word that
// starts like one and could not be read.
func sigilForm(w string) string {
	switch {
	case strings.HasPrefix(w, "@"):
		return "a day reads as @fri, @+3d, @dec4 or @2026-12-04"
	case strings.HasPrefix(w, "$"):
		return "a state is $open, $doing, $waiting, $done, $canceled, $parked or $note"
	case strings.HasPrefix(w, "*"):
		return "an interval reads as *3d, *2w or *monthly"
	case strings.HasPrefix(w, "##"):
		return "a context is a word starting with a letter, as ##home"
	case strings.HasPrefix(w, "#"):
		return "a tag starts with a letter"
	case strings.HasPrefix(w, "~"):
		return "a time reads as ~today, ~7d, ~2w, ~1m or ~1y"
	case strings.HasPrefix(w, "!"):
		return "the flag is ! alone"
	}
	return ""
}

// colourForm says what a colour reads as.
const colourForm = "a colour is a name, as ~teal or ~orange, or a hex value, as ~#3fa7c4"

// colorOffers names, while a ~colour is typed at the end of a row, the
// capture or the stamp line, every colour it could become, each drawn in
// itself, so the names there are can be read off the line. On the filter ~
// is a time, and it says nothing.
func (m *Model) colorOffers(e *editor, surface string) string {
	if surface == askFilter || e.caret != len(e.buf) || len(e.buf) == 0 {
		return ""
	}
	line := string(e.buf)
	typed, ok := strings.CutPrefix(line[strings.LastIndexAny(line, " \t")+1:], "~")
	if !ok {
		return ""
	}
	typed = strings.ToLower(typed)
	if strings.HasPrefix(typed, "#") {
		return "a hex colour, as ~#3fa7c4"
	}
	var parts []string
	for _, name := range sigil.ColorNames {
		if strings.HasPrefix(name, typed) {
			parts = append(parts, m.colorOf(name)+"~"+name+sgrReset+m.colors().dim)
		}
	}
	if len(parts) == 0 {
		return colourForm
	}
	return strings.Join(parts, " · ")
}
