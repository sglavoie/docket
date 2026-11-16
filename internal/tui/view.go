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

const (
	sgrReset   = "\x1b[0m"
	sgrBold    = "\x1b[1m"
	sgrDim     = "\x1b[2m"
	sgrReverse = "\x1b[7m"
	sgrAccent  = "\x1b[36m"
	sgrWarn    = "\x1b[33m"
	sgrAlert   = "\x1b[31m"
)

const helpText = `Docket

  2 3 …          count: repeat a motion or take rows; Esc drops the count
                 a count, operator, g or z that waits shows at the right
                 of the status line
  j k  gg G      move; Home End as gg G
  Ctrl+D Ctrl+U  move half a pane down, up (outlines, lists and filters)
  Ctrl+F Ctrl+B  move a whole pane down, up; PgDn PgUp too
  } {  ) (       next, previous sibling, passing over its rows; last, first
  H              the parent, folding nothing; 2H climbs two
  ] [            next, previous heading (a row ending in a colon)
  n N            next, previous match of the filter on screen, else of the one
                 last kept with Enter or dropped with Esc; this session only
                 all are motions: 3}, d}, y), VH and dn work; lists step by row
                 a row reached from afar (n N, a jump, the trail, a place
                 come back to) is shown mid-pane; one on screen stays put
  h l            fold, unfold; step to parent, first child
                 in lists and flat filters, h shows the row where it lives;
                 Esc returns to the held rows; l does nothing
  zc zo          fold, unfold the row's whole branch, every level under it;
                 the selection or a count of rows too
  zt zz zb       draw the cursor's row at the top, middle, bottom of the pane
  Enter  Esc     zoom into a row; Esc clears the selection, the filter, the zoom
  o O            new row below, above
  A I            type at the end, start of the row
  cc             retype text, keeping fields; one undo (Esc keeps even blank text)
                 c waits only for c; any other key cancels
                 A I cc drop counts; no selection; lists and filters too
  w b e  W B E   step into the row's text: 3w lands on the third word, b on the
                 last; see Inside a row below
  >> <<          indent, outdent; > and < are operators like d and y:
                 >j takes this row and the next, 3>> three rows, <) to the last
  J K            move the row down, up (also in Today; no selection there)
  x  !  t        done and back, flag, pick for Today in pick order
  m              move to a project, Inbox or heading
                 with a selection, m Enter moves it here, where o would open a row
  L  gl          link the row to another, found by name; gl follows the link
                 with no row link, gl opens the first web address in the text
                 a row shows → and the row it points at; blank L Enter is none
  .              repeat the last change to rows on the row or selection:
                 a stamp, a move, a link, :s, x ! t J K p P, d > < with what
                 completed them, and text typed after o O A I cc
                 the count it was made with is used again unless one is given
                 stamps read dates again today; one undo; lists and filters too
                 here, copies, motions and undo leave it remembered
                 nothing remembered or a deleted destination says so
                 remembered only this session; sidebar ignores .
  d y            delete or copy: dd yy take the row or selection with subtrees
                 a motion takes the inclusive range; no selection with motions
                 counts multiply: d3j = 3dj; Esc or another key cancels
                 ic is: row objects, for d y > < alike: ic the rows under
                 this row, is the row and all its siblings, each with what
                 is under it: dic empties a row, <ic dissolves a group,
                 yis copies a level; outlines only, no selection
                 both fill the session register and clipboard; y alone waits
                 copy keeps the cursor; one u restores a delete, keeping the register
  p P            put the register after, before the row as new rows at its level
                 keeps depth and fields like paste; one undo; lands on the last row
                 put N rows; nothing to put when empty; counts are dropped
                 no lists, filters or selection; sidebar ignores p P
                 empty places receive children; each put keeps the register
                 put makes new identities; m J K keep the row's identity
  gp gP          put the last copy instead: what y took, even after a delete
  v V            pick a row; sweep with motions (V ends, Esc clears)
                 x ! t dd yy m and stamps act on the selection
                 >> << J K move adjacent selected siblings together
  # @ $ * -      stamp: type sigils, Enter sets them (-#tag, -@, -@@, -* remove)
                 @++1w @--3d move the deadline from the day the row has, not
                 from today (d w m y); @@++1w the start day; no day, no change
  /              filter by text and sigils; -word excludes; Tab flat or in context
                 # any tag, $ any task; -# untagged, -$ notes
                 a|b is either: $open|doing, #home|!, milk|#food
                 each side uses its own sigil or the first side's; unreadable is text
                 | ( ) -( standing alone, a blank each side, group terms:
                 #home $open | #work !    fix ( #docs | cache )    -( a | b )
                 ~7d ~2w ~today: state changed (or row made) since then,
                 so $done ~7d is what was finished this week
                 $next: each project's first open or doing row not starting later
                 $today: a row in Today
                 @<fri @>+1w: open rows due by or from a day; flat lists soonest first
                 ^term tests ancestors, not this row, even above the pane root
                 ^#work|home, ^name $next; -^#work means no ancestor has #work
                 bare ^ waits for typing; ^^ is text; completion works after ^
                 _term tests the rows under this one, at any depth: _@overdue,
                 -_$open|doing (nothing left to do under it); __ is text
                 <term tests the parent only, >term the rows directly under:
                 <#work, >$waiting, ->$ (no task of its own); << >> are text
                 #est any value, #est=2h that one, #est<3 #est>3 at most, at least
                 & a row that links to another; &word to a row holding word;
                 &. to the row the cursor was on: the rows that point here
                 a filter shows every match, folded or not
  Up Down        on the / : and stamp lines, recall the lines kept there
                 before, those beginning with what is typed; kept across
                 restarts on this machine
  :              q, help (open this page), export [file],
                 import file (read such a file back as rows after the cursor;
                 in Projects its Inbox line is the Inbox again; one undo),
                 closed (hide or show done and canceled rows),
                 numbers (each row says how far it is from the cursor: a count),
                 all (select every row shown: /#old, :all, -#old #new retags),
                 s/old/new (change text in the row or selection, every place,
                 capitals as typed; one undo; any mark for the slash: s,a/b,c),
                 archive (move closed rows under the project's Archive: heading),
                 fold [level] (all, or show N levels), unfold (all),
                 sort [keys] (siblings or adjacent selected siblings, once),
                 @ soonest deadline (missing last), ! flagged first,
                 $ doing, open, waiting, notes, parked, done, canceled;
                 later keys break ties; no keys: text ignoring case
                 ties keep order; children travel; one undo; no lists or filters
                 review (projects, longest untouched first),
                 save name (keep the filter; f then /name opens it), forget name,
                 sync folder (share with other machines through a synced folder)
  a              capture to the Inbox; %name files in a project or heading
  f              go to a project, heading, list or saved filter by name,
                 or to any row by words of its text, shown where it lives
  Ctrl+O Ctrl+I  back along the places jumps left, and forward again
                 a jump is f, a sidebar visit or gl to a row; this session only
                 a terminal that cannot tell Ctrl+I from Tab gives Tab
  Tab            the sidebar: j k choose, Enter work in it
                 gg G and the paging keys move through it too
  u  Ctrl+R      undo, redo
  Ctrl+C         quit

While typing
  Enter          split inside a line; at ends, keep it (fresh rows chain)
  Backspace      at the start, join into the row above (no children)
                 blank fresh rows disappear; lists and filters never join
                 or split; each split or join is one undo step
  Up Down        move by wrapped line; at its ends, type in the next row
  Home Ctrl+A    start of the whole typed row
  End Ctrl+E     end of the whole typed row
  Ctrl+U         cut before the caret
  Esc            keep the row
  Tab Shift+Tab  indent, outdent
  Alt+Left Alt+Right Alt+Backspace    by word

Inside a row (after w b e W B E; the caret is a block)
  h l  0 ^ $     by character; start, end of the line
  w b e  W B E   by word; capitals count anything but a blank as one word
  f F t T  ; ,   to a character, or before it; again, and the other way
  j k            by drawn line of a wrapped row; the row is never left
  i a I A        type at the caret, after it, at the start, at the end;
                 Esc ends the typing with the caret back on a character
  d c y          take a motion or a text object: dw, c2e, d$, yiw, ct,
                 iw aw iW aW a word; i" a" i' quotes and backticks;
                 i( a( ib i[ i{ i< brackets
  x X D C s      delete a character, the one before, to the end; change those
  r ~            replace a character; switch case
  p P            put what d c y x last took after, before the caret
  v              select from the caret: motions and iw a" stretch it,
                 d x c s y ~ act on it; v or Esc ends it
  *              filter the pane by the word under the caret, a sigil whole,
                 as / and Enter would; n N walk the matches
  u Ctrl+R       undo, redo changes made in this row before leaving it
  .              repeat the last change made inside a row, with what was typed
  2 3 …          count: 3w, d2w, 2fx, 3x
  Esc            drop what is pending, else keep the row and return
                 everything done on one visit is one undo step

Sigils, typed anywhere in a row
  !              flag
  @fri @+3d @11-20 @today    deadline
  @@mon          not before that day: dimmed until then, and in Today on the day
  *2w *monthly   repeats: closing it moves it to its next day, still open
                 done rows under it reopen too, at every depth; u undoes all
  #tag           tag; #est=2h a tag with a value, one value to a name
  $doing $waiting $done $canceled $parked $open $note    state
  A row is a note at top-level, under a task, or when it ends in a colon.
  Other rows default to tasks; $open under a task makes a subtask.
  \#tag keeps a sigil as plain text.
  As you type, a sigil the line reads turns cyan and one it cannot turns
  yellow. The right arrow takes the dim suggestion at the end of the line.

Pasting several lines adds them as rows, indentation as depth.
Copy and export spell otherwise empty rows as $open, $note or their state.

A web address in a row is a link the terminal can open.
From a shell, docket add "Buy milk %home @fri" captures without opening
the app; with no words it reads one row a line from standard input.
docket ls '$next #home' prints the rows a filter matches, one a line with
a tab and where each lives; with no filter, the whole outline as :export.

Nothing asks for confirmation: undo takes anything back.
Press any key to return.`

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
func stateGlyph(state string, kids int, folded bool) string {
	switch state {
	case sigil.Open:
		return "○"
	case sigil.Doing:
		return sgrAccent + "●" + sgrReset
	case sigil.Waiting:
		return sgrWarn + "◔" + sgrReset
	case sigil.Done:
		return sgrDim + "✓" + sgrReset
	case sigil.Canceled:
		return sgrDim + "✕" + sgrReset
	case sigil.Parked:
		return sgrDim + "◌" + sgrReset
	}
	switch {
	case kids == 0:
		return " "
	case folded:
		return sgrDim + "▸" + sgrReset
	}
	return sgrDim + "▾" + sgrReset
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
			return sgrDim
		}
		return style
	}

	var out []word
	body := ""
	if onCursor {
		body = sgrBold
	}
	for _, w := range strings.Fields(l.Text) {
		out = append(out, word{paint(body), w})
	}
	if l.Flag {
		out = append(out, word{paint(sgrAlert + sgrBold), "!"})
	}
	if l.Due != "" {
		label, days := sigil.DueLabel(l.Due, m.today())
		style := sgrDim
		switch {
		case days < 0:
			style = sgrAlert
		case days == 0:
			style = sgrWarn
		}
		// A label such as "@Nov 20" stays in one piece.
		out = append(out, word{paint(style), label})
	}
	if l.Start != "" {
		label, _ := sigil.DueLabel(l.Start, m.today())
		out = append(out, word{sgrDim, "@" + label})
	}
	if l.Every != "" {
		out = append(out, word{sgrDim, "*" + l.Every})
	}
	for _, tag := range l.Tags {
		out = append(out, word{sgrDim, "#" + tag})
	}
	if to := r.Fields[fieldLink]; to != "" {
		// A link names the row it points at as that row reads now.
		name := "gone"
		if m.live(to) {
			name = ansi.Truncate(m.tree.Row(to).Text(), 28, "…")
		}
		out = append(out, word{sgrDim, "→ " + name})
	}

	// A task's glyph is its state, so the children it hides are counted
	// after it: when folded, and always in a flat view.
	kids := len(m.tree.Children(r.ID))
	if hides := m.isFolded(r.ID) || flat; kids > 0 && hides && l.State != "" {
		out = append(out, word{sgrDim, "+" + strconv.Itoa(kids)})
	}
	if flat {
		switch m.at.list {
		case kindWaiting:
			out = append(out, word{sgrDim, "·"}, word{sgrDim, days(m.now() - m.changedAt(r.ID))})
		case kindStale:
			out = append(out, word{sgrDim, "·"}, word{sgrDim, days(m.now() - r.Modified)})
		case kindReview:
			// A project parked or closed during the review has no line left.
			if st, active := m.stats[r.ID]; active {
				todo := strconv.Itoa(st.open) + " to do"
				if st.idle {
					todo = "∅"
				}
				out = append(out, word{sgrDim, "·"}, word{sgrDim, strconv.Itoa(st.age) + "d"}, word{sgrDim, "·"}, word{sgrDim, todo})
			}
		}
		if p := m.projectOf(r.ID); p != nil && p.ID != r.ID {
			out = append(out, word{sgrDim, "·"}, word{sgrDim, p.Text()})
		}
	}
	if m.at.query != "" && !m.at.matched[r.ID] {
		// An ancestor kept only to place a match.
		for i := range out {
			out[i].style = sgrDim
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
		text += ghost(text, m.suggest(&a.ed, a.kind), w-lw)
		line := sgrAccent + label + sgrReset + text
		room := w - lw - ansi.StringWidth(text) - 3
		switch {
		case room <= 3:
		case a.kind == askCapture && len(a.ed.buf) == 0:
		case a.kind == askCapture:
			// Where the line will land, so a %name is seen to have worked.
			to, _ := m.route(string(a.ed.buf))
			line += "   " + sgrDim + ansi.Truncate("files in "+to.label, room, "…") + sgrReset
		case a.kind == askFilter:
			shape := "in context"
			if m.at.flat {
				shape = "flat"
			}
			note := strconv.Itoa(len(m.at.matched)) + " match · " + shape + " · Tab switches"
			line += "   " + sgrDim + ansi.Truncate(note, room, "…") + sgrReset
		case a.kind == askStamp:
			note := strconv.Itoa(len(a.rows)) + " row"
			if len(a.rows) != 1 {
				note += "s"
			}
			line += "   " + sgrDim + ansi.Truncate(note+" · -#tag and -@ remove", room, "…") + sgrReset
		case picker(a.kind):
			// The first name is the one Enter takes.
			offer := sgrDim + "nothing matches" + sgrReset
			hits := m.offers(a, strings.TrimSpace(string(a.ed.buf)))
			from := max(0, min(a.choice, len(hits)-1))
			for i, e := range hits[from:] {
				// A row says where it lives, since its text alone may not.
				where := ""
				if e.hint != "" {
					where = " (" + e.hint + ")"
				}
				if i == 0 && from > 0 {
					offer = sgrDim + "‹ " + sgrReset + sgrBold + e.label + sgrReset + sgrDim + where + sgrReset
				} else if i == 0 {
					offer = sgrBold + e.label + sgrReset + sgrDim + where + sgrReset
				} else {
					offer += sgrDim + " · " + e.label + where + sgrReset
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
		return left + strings.Repeat(" ", gap) + sgrDim + right + sgrReset, -1
	}
	pw := ansi.StringWidth(pend)
	tail := ""
	if right != "" && ansi.StringWidth(left)+1+pw+2+ansi.StringWidth(right) <= w {
		tail = "  " + sgrDim + right + sgrReset
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
		editing.lines[last] += ghost(editing.lines[last], m.suggest(m.edit, surfaceRow), max(w-prefix(m.cursor), 1))
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
	if m.at.query != "" {
		header += "  / " + m.at.query
		if m.at.flat {
			header += "  (flat)"
		}
	} else if m.hideClosed && m.at.list == "" {
		header += "  · closed hidden"
	}
	lines = append(lines, sgrDim+ansi.Truncate(header, w, "…")+sgrReset)

	var caret *tea.Cursor
	for i := m.top; i < len(m.vis) && len(lines) < h; i++ {
		v := m.vis[i]
		r := m.tree.Row(v.id)
		glyph := stateGlyph(r.Fields[sigil.FieldState], len(m.tree.Children(v.id)), m.isFolded(v.id))
		gutter := " "
		if i == m.cursor {
			gutter = sgrAccent + "▌" + sgrReset
			if m.sideFocus {
				gutter = sgrDim + "▌" + sgrReset
			}
		}
		mark := " "
		if m.picked[v.id] {
			mark = sgrAccent + "┃" + sgrReset
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
				number = sgrDim + fmt.Sprintf("%*d ", numberCols-1, d) + sgrReset
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
			if n == 0 {
				lines = append(lines, number+gutter+mark+indent+glyph+" "+text)
			} else {
				// The cursor and selection marks run the height of the row.
				lines = append(lines, blank+gutter+mark+indent+"  "+text)
			}
		}
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(m.vis) == 0 {
		lines[1] = sgrDim + "  " + ansi.Truncate(m.emptyText(), max(w-2, 1), "…") + sgrReset
	}
	return lines, caret
}

func (m *Model) View() tea.View {
	w, h := m.size()
	if m.help {
		all := strings.Split(helpText, "\n")
		page := m.helpPage()
		m.clampHelp()
		out := all
		if len(all) > page {
			out = append(append([]string{}, all[m.helpTop:min(m.helpTop+page, len(all))]...),
				sgrDim+fmt.Sprintf("  j/k scroll · gg/G ends · %d-%d of %d · any other key closes",
					m.helpTop+1, min(m.helpTop+page, len(all)), len(all))+sgrReset+"  "+m.pending())
		}
		v := tea.NewView(strings.Join(out, "\n"))
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
			lines = append(lines, side[i]+sgrDim+" │ "+sgrReset+pane[i])
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

	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.Cursor = caret
	return v
}
