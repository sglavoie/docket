package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

const helpText = `Essentials — move, fold and zoom
  j k  gg G      move; Home End reach the ends
  o O  A I       add below, above; type at the end, start
  x  !  t        close and reopen, flag, pick for Today
  /              filter; Enter keeps, Esc clears
  u  Ctrl+R      undo, redo; nothing asks for confirmation
  Ctrl+C  :q     quit
  :tutor         open a fresh Welcome tutorial; dd on its project removes it
  h l            fold, unfold; step to parent, first child
  Enter  Esc     zoom in; clear selection, filter, then zoom back out
  ] [  H         next, previous heading; parent
  } {  ) (       next, previous sibling; last, first sibling
  Ctrl+D Ctrl+U  half a pane; Ctrl+F Ctrl+B or PgDn PgUp a whole pane
  zc zo          fold, unfold the whole branch; a count or selection too
  zt zz zb       draw the cursor's row at the top, middle, bottom
  h in a list    show the row where it lives; Esc returns; l does nothing

Add and type
  o O            new row below, above; o on an open parent adds a first child
  A I  cc        type at end, start; retype text, keeping fields
  Enter          split inside a line; at its ends keep it (fresh rows chain)
  Backspace      at the start, join into the row above (no children)
  Up Down        by wrapped line, then go on typing in the next row
  Home Ctrl+A    start of the typed row; End Ctrl+E its end
  Ctrl+U         cut before the caret; Alt+Left/Right/Backspace by word
  Tab Shift+Tab  indent, outdent while typing; Esc keeps the row
  w b e  W B E   step into text by word; capitals count nonblank words
  Inside a row:  h l move by character; 0 ^ $ reach line ends; j k wrap
                 f F t T find a character; ; , repeat, reverse
                 i a I A type at, after the caret, at the start, at the end
                 d c y take motions or objects: dw, c2e, d$, yiw, ci"
                 iw aw iW aW words; i" a" quotes; i( a( ib i[ i{ i< brackets
                 x X D C s delete, change; r replaces; ~ switches case
                 p P put text after, before; v selects with motions or objects
                 * filters by the word at the caret; n N walk matches
                 u Ctrl+R undo, redo inside this visit; . repeats a text change
                 Esc ends typing, then keeps the visit as one outline undo
  Paste          several lines become rows, indentation as depth; one undo

States and sigils
  # @ $ * -      open the stamp line; Enter sets sigils on the row or selection
  !              flag; -! removes it on the stamp line
  @fri @+3d      deadline; @dec4 @jan5-2027 @eom by name; @@mon not before that day
  *2w *monthly   repeat: closing moves the date and reopens done descendants
  #tag #est=2h   tag or value; header sums estimates, headings show done/total
  $open $doing $waiting $done $canceled $parked $note    state
  -#tag -@ -@@ -*    remove fields on the stamp line
  @++1w @--3d    shift the row's deadline; @@++1w its start (stamp line only)
  Defaults       notes at top level, under tasks, or ending in a colon
                 other rows are tasks; $open makes a subtask; \#tag stays text
  While typing   readable sigils take accent colour, unreadable ones warning
                 Right accepts a dim suggestion; dates edit as YYYY-MM-DD

Counts and operators
  2 3 …          repeat motions or take rows for an act; Esc drops a count
  :numbers       show each row's distance from the cursor
  d y > <        delete, copy, indent, outdent; doubled keys take rows
                 a motion takes an inclusive range: d3j = 3dj, >j, y)
                 ic takes children, is the row and its siblings: dic, yis
                 row objects include folded subtrees; no lists or selections
  J K            reorder down, up; Today too (one row there)
  p P            put the register after, before, with fresh identities
  gp gP          put the last copy, even after a delete replaced the register
  .              repeat the last row change, including stamps and typed text
                 keeps its count unless given another; one undo step
  Copy/delete    include whole subtrees; write the register and clipboard
                 u restores deleted rows; the register stays

Selection
  v              pick a row and step down; again on a picked row clears it
  V              sweep with motions; V ends the sweep, Esc clears selections
  x ! t dd yy m L and stamps    act on the selection in one step
  >> << J K      move adjacent selected siblings together
  m Enter        with a blank query, move selected rows here, where o would add
  :all           select every shown row (matches only under a filter)
  /#old :all -#old #new    filter, select, retag with one stamp

Filter
  /              text and sigils; Enter keeps, Esc clears; Tab flat or context
  "a phrase"     match words together, ignoring case; -"a phrase" excludes
  n N            next, previous match; after Esc, use the last filter
  # $ @          any tag, task, deadline; -# untagged, -$ notes
  $next          each project's first workable task; excludes Inbox
  $first         first workable sibling per parent; includes Inbox
  $first -<$     next action under each heading and project
  $today         picked or starting today
  @overdue @week @<fri @>+1w    deadlines; flat results soonest first
  @@later ~7d    starts later; state changed (or row made) in the last week
  a|b            either: $open|doing, #home|!, milk|#food
  | ( ) -(       groups, with blanks: fix ( #docs | cache ), -( a | b )
  ^term _term    tests ancestors, descendants: ^#work, _@overdue
  <term >term    tests parent, direct children: <#work, >$waiting
  #est #est=2h #est<3 #est>3    any value, exact, at most, at least
  & &word &.     a row link, one to matching text, one to the cursor's row
  Held rows      edits never remove matches; change the query to read again
  :save name     keep a filter; f then /name opens it; :forget name drops it

Capture and travel
  a              capture to Inbox; %name files in a project or heading
                 Enter keeps and offers another; blank Enter or Esc ends
  f              go to a project, heading, list, saved filter or row by name
                 blank Enter opens Inbox; Up Down choose, Esc leaves
  m              move rows to a project, Inbox or heading by name
  Tab            sidebar focus; j k and paging keys visit; Enter works there
  L gl           link to a row, follow it; blank L Enter removes the link
                 with no row link, gl opens the first web address in the text
  Ctrl+O Ctrl+I  back, forward along jumps (f, sidebar visits, row links)
                 a terminal unable to distinguish Ctrl+I from Tab gives Tab
  g; g,         back, forward through the rows changed this session
  docket add     capture from the shell; no arguments reads one row per line
  docket ls      print a filter's matches; no filter prints the whole outline

Undo and commands
  u Ctrl+R       undo, redo across restarts; a count repeats the key
  Up Down        recall /, stamp and : lines beginning with what is typed
  :help          this reference; ] [ go to next, previous section
  :export [file] write the whole outline; default ~/docket-YYYY-MM-DD.txt
  :import file   read indented rows after the cursor; one undo step
                 in Projects, an Inbox line routes its children to Inbox
  :closed        hide or show done and canceled rows
  :s/old/new     replace text in row or selection; any separator, as s,a/b,c
  :archive       move closed work under the project's Archive: heading
  :fold [level]  fold all, or show N levels; :unfold opens all
  :sort [keys]   siblings once: @ deadline, ! flag, $ state; ties stay ordered
                 later keys break ties; no keys sorts by text
  :review        projects, longest untouched first
  :sync folder   share through a synced folder; :sync off stops
  Display        palette follows terminal background; focused rows are shaded
                 dates outside this year name the year
  More detail    SPEC.md describes the grammar; :tutor teaches it on live rows
  Return         Esc or any key other than a help motion closes this page`

func (m *Model) size() (int, int) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}

// numberCols is the width :numbers takes at the left of the pane.
const numberCols = 3

func (m *Model) numberCols() int {
	if m.numbers {
		return numberCols
	}
	return 0
}

// stateGlyph is the one column that says what a row is. A note shows its
// fold, or nothing; a task shows its state, and a folded task counts its
// children after its text instead.
func (m *Model) stateGlyph(state string, kids int, folded bool) string {
	switch state {
	case sigil.Open:
		return "○"
	case sigil.Doing:
		return m.colors().accent + "●" + sgrReset
	case sigil.Waiting:
		return m.colors().warn + "◔" + sgrReset
	case sigil.Done:
		return m.colors().dim + "✓" + sgrReset
	case sigil.Canceled:
		return m.colors().dim + "✕" + sgrReset
	case sigil.Parked:
		return m.colors().dim + "◌" + sgrReset
	}
	switch {
	case kids == 0:
		return " "
	case folded:
		return m.colors().dim + "▸" + sgrReset
	}
	return m.colors().dim + "▾" + sgrReset
}

// isFolded reports a row drawn folded. A filter shows every match, so
// nothing is folded while one is on.
func (m *Model) isFolded(id string) bool { return m.folded[id] && m.at.query == "" }

// word is one styled word of a row. A row is drawn as words so that it can
// wrap at any of them and keep each word's colour on every line.
type word struct {
	style, text string
}

// rowWords is a row's text followed by its fields, each spelled with the
// sigil that writes it, then what the view adds: the children a task hides,
// and in a flat view its age and project. A closed or parked row is dimmed
// whole and raises no alarm.
func (m *Model) rowWords(r *outline.Row, onCursor, flat bool) []word {
	l := sigil.FromFields(r.Fields)
	quiet := l.Closed() || l.State == sigil.Parked || l.Later(m.today())
	paint := func(style string) string {
		if quiet {
			return m.colors().dim
		}
		return style
	}

	var out []word
	// A heading and a project carry the outline's shape: a heading is drawn
	// bold in the accent colour, a project bold.
	body := ""
	switch {
	case strings.HasSuffix(l.Text, ":"):
		body = m.colors().accent + sgrBold
	case r.Parent == "" || onCursor:
		body = sgrBold
	}
	// A match is drawn with what the filter found in it.
	var marked map[int]bool
	marking := m.at.matched[r.ID] && len(m.at.marks) > 0
	if marking {
		marked = markWords(l.Text, m.at.marks)
	}
	mark := m.colors().accent + sgrUnderline
	for i, w := range strings.Fields(l.Text) {
		if marked[i] {
			out = append(out, word{mark + body, w})
			continue
		}
		out = append(out, word{paint(body), w})
	}
	if total := m.totals[r.ID].under; strings.HasSuffix(l.Text, ":") && total.total > 0 {
		out = append(out, word{m.colors().dim, fmt.Sprintf("%d/%d", total.done, total.total)})
	}
	if l.Flag {
		out = append(out, word{paint(m.colors().alert + sgrBold), "!"})
	}
	if l.Due != "" {
		label, days := sigil.DueLabel(l.Due, m.today())
		style := m.colors().dim
		switch {
		case days < 0:
			style = m.colors().alert
		case days == 0:
			style = m.colors().warn
		}
		// A label such as "@Nov 20" stays in one piece.
		out = append(out, word{paint(style), label})
	}
	if l.Start != "" {
		label, _ := sigil.DueLabel(l.Start, m.today())
		out = append(out, word{m.colors().dim, "@" + label})
	}
	if l.Every != "" {
		out = append(out, word{m.colors().dim, "*" + l.Every})
	}
	for _, tag := range l.Tags {
		if marking && markTag(tag, m.at.marks) {
			out = append(out, word{mark, "#" + tag})
			continue
		}
		out = append(out, word{m.colors().dim, "#" + tag})
	}
	if to := r.Fields[fieldLink]; to != "" {
		// A link names the row it points at as that row reads now.
		name := "gone"
		if m.live(to) {
			name = ansi.Truncate(m.tree.Row(to).Text(), 28, "…")
		}
		out = append(out, word{m.colors().dim, "→ " + name})
	}

	// A task's glyph is its state, so the children it hides are counted
	// after it: when folded, and always in a flat view.
	kids := len(m.tree.Children(r.ID))
	if hides := m.isFolded(r.ID) || flat; kids > 0 && hides && l.State != "" {
		out = append(out, word{m.colors().dim, "+" + strconv.Itoa(kids)})
	}
	if flat {
		switch m.at.list {
		case kindWaiting:
			out = append(out, word{m.colors().dim, "·"}, word{m.colors().dim, days(m.now() - m.changedAt(r.ID))})
		case kindStale:
			out = append(out, word{m.colors().dim, "·"}, word{m.colors().dim, days(m.now() - r.Modified)})
		case kindReview:
			// A project parked or closed during the review has no line left.
			if st, active := m.stats[r.ID]; active {
				todo := strconv.Itoa(st.open) + " to do"
				if st.idle {
					todo = "∅"
				}
				out = append(out, word{m.colors().dim, "·"}, word{m.colors().dim, strconv.Itoa(st.age) + "d"}, word{m.colors().dim, "·"}, word{m.colors().dim, todo})
			}
		}
		if p := m.projectOf(r.ID); p != nil && p.ID != r.ID {
			out = append(out, word{m.colors().dim, "·"}, word{m.colors().dim, p.Text()})
		}
	}
	if m.at.query != "" && !m.at.matched[r.ID] {
		// An ancestor kept only to place a match.
		for i := range out {
			out[i].style = m.colors().dim
		}
	}
	return out
}

// wrap lays words out in lines of at most width columns. A word wider than
// a line is cut across lines, since nothing may be lost.
func wrap(words []word, width int) []string {
	lines := []string{""}
	used := 0
	link := ""
	put := func(w word) {
		if used > 0 {
			lines[len(lines)-1] += " "
			used++
		}
		text := w.text
		if w.style != "" {
			text = w.style + w.text + sgrReset
		}
		if link != "" {
			// Every piece of an address carries the whole of it, so one
			// broken across lines still opens.
			text = ansi.SetHyperlink(link) + text + ansi.ResetHyperlink()
		}
		lines[len(lines)-1] += text
		used += ansi.StringWidth(w.text)
	}
	for _, w := range words {
		link = linkOf(w.text)
		for ansi.StringWidth(w.text) > width {
			// Fill what is left of this line, or start a new one.
			room := width - used
			if used > 0 {
				room--
			}
			if room < 1 {
				lines, used = append(lines, ""), 0
				continue
			}
			head := ansi.Truncate(w.text, room, "")
			if head == "" {
				// A character wider than the room left.
				lines, used = append(lines, ""), 0
				if width < 2 {
					break
				}
				continue
			}
			put(word{w.style, head})
			w.text = strings.TrimPrefix(w.text, head)
			lines, used = append(lines, ""), 0
		}
		need := ansi.StringWidth(w.text)
		if used > 0 && used+1+need > width {
			lines, used = append(lines, ""), 0
		}
		if w.text != "" {
			put(w)
		}
	}
	if n := len(lines); n > 1 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// linkOf is the web address a word is, without the punctuation a sentence
// leaves after it, or nothing.
func linkOf(w string) string {
	if !strings.HasPrefix(w, "https://") && !strings.HasPrefix(w, "http://") {
		return ""
	}
	return strings.TrimRight(w, ".,;:!?)")
}

func (m *Model) breadcrumb() string {
	return m.placeName(m.at)
}

func (m *Model) placeName(p place) string {
	switch {
	case p.list != "":
		return strings.ToUpper(p.list[:1]) + p.list[1:]
	case p.root == "":
		return "Projects"
	}
	var parts []string
	for id := p.root; id != ""; {
		r := m.tree.Row(id)
		if r == nil {
			break
		}
		parts = append([]string{r.Text()}, parts...)
		id = r.Parent
	}
	return strings.Join(parts, " › ")
}

func (m *Model) emptyText() string {
	switch {
	case m.at.query != "":
		return "Nothing matches."
	case m.at.list == kindToday:
		return "Nothing picked for today. Press t on a row to pick it."
	case m.at.list == kindWaiting:
		return "Nothing is waiting."
	case m.at.list == kindStale:
		return "Nothing has gone stale."
	case m.at.list == kindReview:
		return "No active projects to review."
	case m.hideClosed && len(m.tree.Children(m.at.root)) > 0 && m.at.root != "":
		return "Nothing open here. :closed shows the closed rows."
	case m.at.root == inboxID:
		return "The Inbox is empty. Press a to capture a thought."
	case m.at.root == "":
		return "No projects yet. Press o to add one."
	}
	return "Nothing here yet. Press o to add a row."
}

func days(ms int64) string {
	return strconv.FormatInt(ms/(24*time.Hour).Milliseconds(), 10) + "d"
}

// statusLine is the bottom row, and the caret's column when it is asking.
func (m *Model) statusLine(w int) (string, int) {
	if a := m.ask; a != nil {
		label := askLabels[a.kind]
		lw := ansi.StringWidth(label)
		text, col := a.ed.window(max(w-lw, 1), m.styler(a.kind))
		text += m.ghost(text, m.suggest(&a.ed, a.kind), w-lw)
		line := m.colors().accent + label + sgrReset + text
		room := w - lw - ansi.StringWidth(text) - 3
		// The tags there are, while one is typed.
		tags := ""
		if a.kind == askCapture || a.kind == askFilter || a.kind == askStamp {
			tags = m.tagOffers(&a.ed, a.kind)
		}
		switch {
		case room <= 3:
		case a.kind == askCapture && len(a.ed.buf) == 0:
		case a.kind == askCapture:
			// Where the line will land, so a %name is seen to have worked.
			to, _ := m.route(string(a.ed.buf))
			note := "files in " + to.label
			if tags != "" {
				note += " · " + tags
			}
			line += "   " + m.colors().dim + ansi.Truncate(note, room, "…") + sgrReset
		case a.kind == askFilter:
			shape := "in context"
			if m.at.flat {
				shape = "flat"
			}
			note := strconv.Itoa(len(m.at.matched)) + " match · " + shape + " · Tab switches"
			if tags != "" {
				note = strconv.Itoa(len(m.at.matched)) + " match · " + tags
			}
			line += "   " + m.colors().dim + ansi.Truncate(note, room, "…") + sgrReset
		case a.kind == askStamp:
			note := strconv.Itoa(len(a.rows)) + " row"
			if len(a.rows) != 1 {
				note += "s"
			}
			hint := " · -#tag and -@ remove"
			if tags != "" {
				hint = " · " + tags
			}
			line += "   " + m.colors().dim + ansi.Truncate(note+hint, room, "…") + sgrReset
		case picker(a.kind):
			// The first name is the one Enter takes.
			offer := m.colors().dim + "nothing matches" + sgrReset
			hits := m.offers(a, strings.TrimSpace(string(a.ed.buf)))
			from := max(0, min(a.choice, len(hits)-1))
			for i, e := range hits[from:] {
				// A row says where it lives, since its text alone may not.
				where := ""
				if e.hint != "" {
					where = " (" + e.hint + ")"
				}
				if i == 0 && from > 0 {
					offer = m.colors().dim + "‹ " + sgrReset + sgrBold + e.label + sgrReset + m.colors().dim + where + sgrReset
				} else if i == 0 {
					offer = sgrBold + e.label + sgrReset + m.colors().dim + where + sgrReset
				} else {
					offer += m.colors().dim + " · " + e.label + where + sgrReset
				}
				if ansi.StringWidth(offer) > room {
					break // the rest would not fit
				}
			}
			line += "   " + ansi.Truncate(offer, room, "…") + sgrReset
		}
		return line, lw + col
	}

	left, right := m.msg, ":help"
	switch {
	case left != "":
	case m.nextKeys() != "":
		left = m.colors().dim + m.nextKeys() + sgrReset
	case m.edit != nil && m.edit.normal:
		left = m.edit.textStatus()
	case m.edit != nil:
		left = "Enter next row · Esc done · Tab indent"
		if m.edit.inside {
			left = "Esc back to the caret · Enter next row · Tab indent"
		}
		if m.suggest(m.edit, surfaceRow) != "" {
			left += " · → completes"
		}
		if tags := m.tagOffers(m.edit, surfaceRow); tags != "" {
			left = tags
			if m.suggest(m.edit, surfaceRow) != "" {
				left += " · → completes"
			}
		}
	case m.sideFocus:
		left = "j k choose · Enter work in it"
	}
	if n := len(m.picked); n > 0 {
		right = strconv.Itoa(n) + " selected · Esc clears"
	}
	if m.edit != nil {
		right = ""
	}
	// What is typed and not yet complete sits at the right, as typed, and
	// is the last thing a narrow line gives up.
	pend := ansi.Truncate(m.pending(), w, "…")
	if pend == "" {
		left = ansi.Truncate(left, w, "…")
		gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
		if gap < 1 {
			return left, -1
		}
		return left + strings.Repeat(" ", gap) + m.colors().dim + right + sgrReset, -1
	}
	pw := ansi.StringWidth(pend)
	tail := ""
	if right != "" && ansi.StringWidth(left)+1+pw+2+ansi.StringWidth(right) <= w {
		tail = "  " + m.colors().dim + right + sgrReset
	}
	left = ansi.Truncate(left, max(w-pw-1-ansi.StringWidth(tail), 0), "…")
	gap := max(w-ansi.StringWidth(left)-pw-ansi.StringWidth(tail), 0)
	return left + strings.Repeat(" ", gap) + sgrBold + pend + sgrReset + tail, -1
}

// pending is the keys typed so far that wait for more: a count, an operator,
// a g or z, in the order they were typed. Empty when nothing waits.
func (m *Model) pending() string {
	num := func(n int) string {
		if n > 0 {
			return strconv.Itoa(n)
		}
		return ""
	}
	switch {
	case m.help:
		if m.helpG {
			return "g"
		}
		return ""
	case m.ask != nil:
		return ""
	case m.edit != nil:
		if e := m.edit; e.normal {
			return num(e.opCount) + e.pend + num(e.count)
		}
		return ""
	}
	return num(m.opCount) + m.operator + num(m.count) + m.prefix
}

// paneLines draws the outline pane: a header and the rows, h lines in all.
func (m *Model) paneLines(w, h int) ([]string, *tea.Cursor) {
	listH := max(h-1, 1)
	flat := m.at.list != "" || (m.at.query != "" && m.at.flat)
	prefix := func(i int) int { return m.numberCols() + 2 + 2*m.vis[i].depth + 2 }

	var editing editLayout
	if m.edit != nil {
		editing = m.edit.wrapped(max(w-prefix(m.cursor), 1), m.styler(surfaceRow))
		last := len(editing.lines) - 1
		editing.lines[last] += m.ghost(editing.lines[last], m.suggest(m.edit, surfaceRow), max(w-prefix(m.cursor), 1))
		p := editing.pos[m.edit.caret]
		m.edit.top = max(0, min(m.edit.top, len(editing.lines)-listH))
		m.edit.top = max(0, min(m.edit.top, p.y))
		if p.y >= m.edit.top+listH {
			m.edit.top = p.y - listH + 1
		}
	}
	// Reuse the editing layout for height accounting and drawing.
	rowLines := func(i int) []string {
		v := m.vis[i]
		if m.edit != nil && m.edit.row == v.id {
			return editing.lines
		}
		return wrap(m.rowWords(m.tree.Row(v.id), i == m.cursor, flat), max(w-prefix(i), 1))
	}
	span := func(from, to int) int {
		used := 0
		for i := from; i <= to && used <= listH; i++ {
			used += len(rowLines(i))
		}
		return used
	}

	// A row landed on from afar, and not on screen, is shown mid-pane; zt, zz
	// and zb place the cursor's row wherever it is.
	afar := m.centre && m.cursor < len(m.vis) && (m.cursor < m.top || span(m.top, m.cursor) > listH)
	if m.scroll != 0 && m.cursor < len(m.vis) || afar {
		m.top = m.cursor
		above := listH - len(rowLines(m.cursor))
		switch m.scroll {
		case 't':
			above = 0
		case 'z', 0:
			above /= 2
		}
		for m.top > 0 {
			n := len(rowLines(m.top - 1))
			if n > above {
				break
			}
			m.top, above = m.top-1, above-n
		}
	}
	m.centre, m.scroll = false, 0
	// Keep the cursor's row whole on screen, and leave no gap at the end.
	m.top = max(0, min(m.top, m.cursor))
	for m.top < m.cursor && span(m.top, m.cursor) > listH {
		m.top++
	}
	for m.top > 0 && span(m.top-1, len(m.vis)-1) <= listH {
		m.top--
	}

	lines := make([]string, 0, h)
	header := m.breadcrumb()
	if summary := m.paneTotal.summary(); summary != "" {
		header += "  · " + summary
	}
	if m.at.query != "" {
		header += "  / " + m.at.query
		if m.at.flat {
			header += "  (flat)"
		}
	} else if m.hideClosed && m.at.list == "" {
		header += "  · closed hidden"
	}
	lines = append(lines, m.colors().dim+ansi.Truncate(header, w, "…")+sgrReset)

	var caret *tea.Cursor
	for i := m.top; i < len(m.vis) && len(lines) < h; i++ {
		v := m.vis[i]
		r := m.tree.Row(v.id)
		glyph := m.stateGlyph(r.Fields[sigil.FieldState], len(m.tree.Children(v.id)), m.isFolded(v.id))
		gutter := " "
		if i == m.cursor {
			gutter = m.colors().accent + "▌" + sgrReset
			if m.sideFocus {
				gutter = m.colors().dim + "▌" + sgrReset
			}
		}
		mark := " "
		if m.picked[v.id] {
			mark = m.colors().accent + "┃" + sgrReset
		}
		indent := strings.Repeat("  ", v.depth)
		// With :numbers, each row says how far it is from the cursor, so a
		// count is read off the screen and not guessed.
		number, blank := "", strings.Repeat(" ", m.numberCols())
		if m.numbers {
			number = blank
			if i != m.cursor {
				d := i - m.cursor
				if d < 0 {
					d = -d
				}
				number = m.colors().dim + fmt.Sprintf("%*d ", numberCols-1, d) + sgrReset
			}
		}

		isEdit := m.edit != nil && m.edit.row == v.id
		for n, text := range rowLines(i) {
			if isEdit && n < m.edit.top {
				continue
			}
			if len(lines) == h {
				break
			}
			if isEdit && n == editing.pos[m.edit.caret].y {
				caret = tea.NewCursor(prefix(i)+editing.pos[m.edit.caret].x, len(lines))
				caret.Shape = tea.CursorBar
				if m.edit.normal {
					caret.Shape = tea.CursorBlock
				}
			}
			line := blank + gutter + mark + indent + "  " + text
			if n == 0 {
				line = number + gutter + mark + indent + glyph + " " + text
			}
			// Selection wins over the cursor; only the focused pane is live.
			background := ""
			if m.picked[v.id] {
				background = m.colors().selection
			} else if i == m.cursor && !m.sideFocus {
				background = m.colors().cursor
			}
			lines = append(lines, m.tint(line, w, background))
		}
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(m.vis) == 0 {
		lines[1] = m.colors().dim + "  " + ansi.Truncate(m.emptyText(), max(w-2, 1), "…") + sgrReset
	}
	return lines, caret
}

func (m *Model) View() tea.View {
	w, h := m.size()
	if m.help {
		all := strings.Split(helpText, "\n")
		page := m.helpPage()
		m.clampHelp()
		end := min(m.helpTop+page, len(all))
		out := append([]string{}, all[m.helpTop:end]...)
		for len(out) < page {
			out = append(out, "")
		}
		out = append(out, m.colors().dim+fmt.Sprintf("  j/k scroll · ]/[ sections · gg/G ends · %d-%d of %d · any other key closes",
			m.helpTop+1, end, len(all))+sgrReset+"  "+m.pending())
		v := tea.NewView(m.paintText(strings.Join(out, "\n")))
		v.AltScreen = true
		return v
	}

	bodyH := max(h-1, 2)
	var lines []string
	var caret *tea.Cursor
	switch {
	case w >= sidebarFrom:
		side := m.sideLines(sidebarCols, bodyH)
		pane, c := m.paneLines(w-sidebarCols-3, bodyH)
		for i := range pane {
			lines = append(lines, side[i]+m.colors().dim+" │ "+sgrReset+pane[i])
		}
		if c != nil {
			c.X += sidebarCols + 3
			caret = c
		}
	case m.sideFocus:
		// Too narrow for both: the sidebar takes the pane while it has focus.
		lines = m.sideLines(w, bodyH)
	default:
		lines, caret = m.paneLines(w, bodyH)
	}

	status, col := m.statusLine(w)
	if col >= 0 {
		caret = tea.NewCursor(col, len(lines))
		caret.Shape = tea.CursorBar
	}
	lines = append(lines, status)

	v := tea.NewView(m.paintText(strings.Join(lines, "\n")))
	v.AltScreen = true
	v.Cursor = caret
	return v
}
