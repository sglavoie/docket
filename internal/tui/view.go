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
	sgrReset  = "\x1b[0m"
	sgrBold   = "\x1b[1m"
	sgrDim    = "\x1b[2m"
	sgrAccent = "\x1b[36m"
	sgrWarn   = "\x1b[33m"
	sgrAlert  = "\x1b[31m"
)

const helpText = `Docket

  j k  gg G      move
  Ctrl+D Ctrl+U  move half a pane down, up (outlines, lists and filters)
  h l            fold, unfold; step to parent, first child
                 in lists and flat filters, h shows the row where it lives;
                 Esc returns to the held rows; l does nothing
  Enter  Esc     zoom into a row; Esc clears the selection, the filter, the zoom
  o O            new row below, above
  e              edit the row
  > <            indent, outdent
  J K            move the row down, up (also in Today; no selection there)
  x  !  t        done and back, flag, pick for Today in pick order
  m              move to a project, Inbox or heading
                 with a selection, m Enter moves it here, where o would open a row
  .              repeat the last stamp or named move on the row or selection
                 stamps read dates again today; one undo; lists and filters too
                 here, unreadable stamps and undo leave it remembered
                 nothing remembered or a deleted destination says so
                 remembered only this session; sidebar ignores .
  y              copy the row or selection, including subtrees
  v              select the row; x ! t dd m and stamps act on the selection
                 > < J K move adjacent selected siblings together
  # @ $ * -      stamp: type sigils, Enter sets them (-#tag, -@, -@@, -* remove)
  /              filter by text and sigils; -word excludes; Tab flat or in context
                 # any tag, $ any task; -# untagged, -$ notes
                 a|b is either: $open|doing, #home|!, milk|#food
                 each side uses its own sigil or the first side's; unreadable is text
                 ~7d ~2w ~today: state changed (or row made) since then,
                 so $done ~7d is what was finished this week
                 $next: each project's first open or doing row not starting later
                 @<fri @>+1w: open rows due by or from a day; flat lists soonest first
                 ^term tests ancestors, not this row, even above the pane root
                 ^#work|home, ^name $next; -^#work means no ancestor has #work
                 bare ^ waits for typing; ^^ is text; completion works after ^
                 a filter shows every match, folded or not
  :              q, help (open this page), export [file],
                 closed (hide or show done and canceled rows),
                 all (select every row shown: /#old, :all, -#old #new retags),
                 archive (move closed rows under the project's Archive: heading),
                 fold [level] (all, or show N levels), unfold (all),
                 sort [keys] (siblings or adjacent selected siblings, once),
                 @ soonest deadline (missing last), ! flagged first,
                 $ doing, open, waiting, notes, parked, done, canceled;
                 later keys break ties; no keys: text ignoring case
                 ties keep order; children travel; one undo; no lists or filters
                 review (projects, longest untouched first),
                 save name (keep the filter; p then /name opens it), forget name,
                 sync folder (share with other machines through a synced folder)
  a              capture to the Inbox; %name files in a project or heading
  p              go to a project, heading, list or saved filter by name
                 blank p Enter goes back to the place the last jump left
  Tab            the sidebar: j k choose, Enter work in it
  dd             delete
  u  Ctrl+R      undo, redo
  Ctrl+C         quit

While typing
  Enter          split inside a line; at ends, keep it (fresh rows chain)
  Backspace      at the start, join into the row above (no children)
                 blank fresh rows disappear; lists and filters never join
                 or split; each split or join is one undo step
  Up Down        keep the row and type in the one above, below
  Ctrl+U         cut before the caret
  Esc            keep the row
  Tab Shift+Tab  indent, outdent
  Alt+Left Alt+Right Alt+Backspace    by word

Sigils, typed anywhere in a row
  !              flag
  @fri @+3d @11-20 @today    deadline
  @@mon          not before that day: dimmed until then, and in Today on the day
  *2w *monthly   repeats: closing it moves it to its next day, still open
  #tag           tag
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
		case a.kind == askJump || a.kind == askMove:
			// The first name is the one Enter takes.
			offer := sgrDim + "nothing matches" + sgrReset
			hits := m.offers(a, strings.TrimSpace(string(a.ed.buf)))
			from := max(0, min(a.choice, len(hits)-1))
			for i, e := range hits[from:] {
				if i == 0 && from > 0 {
					offer = sgrDim + "‹ " + sgrReset + sgrBold + e.label + sgrReset
				} else if i == 0 {
					offer = sgrBold + e.label + sgrReset
				} else {
					offer += sgrDim + " · " + e.label + sgrReset
				}
			}
			line += "   " + ansi.Truncate(offer, room, "…") + sgrReset
		}
		return line, lw + col
	}

	left, right := m.msg, ":help"
	switch {
	case left != "":
	case m.edit != nil:
		left = "Enter next row · Esc done · Tab indent"
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
	left = ansi.Truncate(left, w, "…")
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return left, -1
	}
	return left + strings.Repeat(" ", gap) + sgrDim + right + sgrReset, -1
}

// paneLines draws the outline pane: a header and the rows, h lines in all.
func (m *Model) paneLines(w, h int) ([]string, *tea.Cursor) {
	listH := max(h-1, 1)
	flat := m.at.list != "" || (m.at.query != "" && m.at.flat)
	prefix := func(i int) int { return 2 + 2*m.vis[i].depth + 2 }

	// rowLines is a row as drawn: wrapped to the pane, or the one line
	// being typed.
	rowLines := func(i int) []string {
		v := m.vis[i]
		if m.edit != nil && m.edit.row == v.id {
			return []string{""}
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

		if m.edit != nil && m.edit.row == v.id {
			avail := max(w-prefix(i), 1)
			text, col := m.edit.window(avail, m.styler(surfaceRow))
			text += ghost(text, m.suggest(m.edit, surfaceRow), avail)
			caret = tea.NewCursor(prefix(i)+col, len(lines))
			caret.Shape = tea.CursorBar
			lines = append(lines, gutter+mark+indent+glyph+" "+text)
			continue
		}
		for n, text := range rowLines(i) {
			if len(lines) == h {
				break
			}
			if n == 0 {
				lines = append(lines, gutter+mark+indent+glyph+" "+text)
			} else {
				// The cursor and selection marks run the height of the row.
				lines = append(lines, gutter+mark+indent+"  "+text)
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
					m.helpTop+1, min(m.helpTop+page, len(all)), len(all))+sgrReset)
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
