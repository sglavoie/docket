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
// fold, or a dot when it has none; a task shows its state, and a folded task counts its
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
		return m.colors().dim + "·" + sgrReset
	case folded:
		return m.colors().dim + "▸" + sgrReset
	}
	return m.colors().dim + "▾" + sgrReset
}

// todayMark follows a row picked for Today wherever it is drawn but there.
const todayMark = "★"

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
	day := m.today().Format("2006-01-02")
	picked := r.Fields[sigil.FieldToday] == day
	if m.at.list == kindToday && !picked && (l.Start != day || l.State == "") {
		// Today holds its rows until it is opened again, so one unpicked
		// there recedes where it sits.
		quiet = true
	}
	paint := func(style string) string {
		if quiet {
			return m.colors().dim
		}
		return style
	}

	var out []word
	// A heading and a project carry the outline's shape: a heading is drawn
	// bold in the accent colour, a project bold. A heading with every task
	// under it done has nothing left to say, so it keeps its weight and
	// gives up its colour.
	body := ""
	under := m.totals[r.ID].under
	switch {
	case strings.HasSuffix(l.Text, ":") && under.total > 0 && under.done == under.total:
		body = m.colors().dim + sgrBold
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
	if strings.HasSuffix(l.Text, ":") && under.total > 0 {
		out = append(out, word{m.colors().dim, fmt.Sprintf("%d/%d", under.done, under.total)})
	}
	if l.Flag {
		out = append(out, word{paint(m.colors().alert + sgrBold), "!"})
	}
	if picked && m.at.list != kindToday {
		// t toggles the pick, so the row says which way the key will go.
		// Today itself holds nothing else, and says it of no row.
		out = append(out, word{paint(m.colors().accent), todayMark})
	}
	if l.Due != "" {
		label, days := sigil.DueLabel(l.Due, m.today())
		style := m.colors().dim
		// The colours the sidebar marks a project with, so the row behind
		// its mark is found: overdue is bold too, not told by colour alone.
		switch {
		case days < 0:
			style = m.colors().alert + sgrBold
		case days <= dueSoonDays:
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
	if m.at.since && m.at.matched[r.ID] {
		// A filter that asks when rows changed says when each one did.
		day := time.UnixMilli(m.changedAt(r.ID)).In(m.today().Location()).Format("2006-01-02")
		out = append(out, word{m.colors().dim, "~" + dayWords(day, m.today())})
	}
	if to := r.Fields[fieldLink]; to != "" {
		// A link names the row it points at as that row reads now.
		name := "gone"
		if m.live(to) {
			name = ansi.Truncate(m.tree.Row(to).Text(), 28, "…")
		}
		out = append(out, word{m.colors().dim, "→ " + name})
	}

	// A row that hides the rows under it counts them after its text: when
	// folded, and always in a flat view, where nothing folds. A note that
	// already says what it holds does not: a heading its progress, a project
	// under review its tasks.
	kids := len(m.tree.Children(r.ID))
	said := m.at.list == kindReview || strings.HasSuffix(l.Text, ":") && under.total > 0
	if kids > 0 && (flat || m.isFolded(r.ID)) && (l.State != "" || !said) {
		out = append(out, word{m.colors().dim, "+" + strconv.Itoa(kids)})
	}
	if flat {
		// Each thing said about a row keeps its dot, so a wrapped row never
		// leaves one hanging at the end of a line.
		also := func(text string) { out = append(out, word{m.colors().dim, "· " + text}) }
		switch m.at.list {
		case kindWaiting:
			also(days(m.now() - m.changedAt(r.ID)))
		case kindStale:
			also(days(m.now() - r.Modified))
		case kindReview:
			// A project parked or closed during the review has no line left.
			if st, active := m.stats[r.ID]; active {
				also(strconv.Itoa(st.age) + "d")
				if st.idle {
					also("∅")
				} else {
					also(strconv.Itoa(st.open) + " to do")
				}
			}
		}
		if where := m.whereFlat(r); where != "" {
			also(where)
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

// whereFlat is where a row in a flat view lives: its project, or, when the
// pane is inside that project and the header already names it, the nearest
// heading over the row there.
func (m *Model) whereFlat(r *outline.Row) string {
	p := m.projectOf(r.ID)
	if p == nil || p.ID == r.ID {
		return ""
	}
	if here := m.projectOf(m.at.root); m.at.root == "" || here == nil || here.ID != p.ID {
		return p.Text()
	}
	for id := r.Parent; id != "" && id != m.at.root; {
		up := m.tree.Row(id)
		if up == nil {
			break
		}
		if name, heading := strings.CutSuffix(up.Text(), ":"); heading {
			return name
		}
		id = up.Parent
	}
	return ""
}

// placeName is a place as one line: a list by its name, a row under the rows
// above it.
func (m *Model) placeName(p place) string {
	return strings.Join(m.placeParts(p), " › ")
}

// placeParts names a place from the project down. A heading drops its
// colon, as it does wherever else a path is written: the mark between two
// names already says one holds the other.
func (m *Model) placeParts(p place) []string {
	switch {
	case p.list != "":
		return []string{strings.ToUpper(p.list[:1]) + p.list[1:]}
	case p.root == "":
		return []string{"Projects"}
	}
	var parts []string
	for id := p.root; id != ""; {
		r := m.tree.Row(id)
		if r == nil {
			break
		}
		name := r.Text()
		if short := strings.TrimSuffix(name, ":"); short != "" {
			name = short
		}
		parts = append([]string{name}, parts...)
		id = r.Parent
	}
	return parts
}

// paneHeader says where the pane is: the place in the text colour under its
// dimmed ancestors, what it holds dimmed after it, a held filter in the
// accent colour since it hides rows, and the ruler at the right edge. A
// line too narrow for all of it gives up the ruler, then the rows above the
// place from the project down, since the place is what the pane shows.
func (m *Model) paneHeader(w int, ruler string) string {
	c := m.colors()
	parts := m.placeParts(m.at)
	summary := m.paneTotal.summary()
	if p := m.projectOf(m.at.root); p != nil && m.at.list == "" {
		// A project out of the sidebar says why, since nothing there marks it.
		switch state := p.Fields[sigil.FieldState]; state {
		case sigil.Parked, sigil.Done, sigil.Canceled:
			if summary != "" {
				state += " · " + summary
			}
			summary = state
		}
	}
	if done := m.paneTotal.done; m.at.list == kindToday && done > 0 {
		// Today is the list that is finished, so it says how far along it is.
		if summary == "" {
			summary = "all " + strconv.Itoa(done) + " done"
		} else {
			summary += " · " + strconv.Itoa(done) + " done"
		}
	}
	tail := ""
	if summary != "" {
		tail += c.dim + "  · " + summary + sgrReset
	}
	if m.at.query != "" {
		tail += "  " + c.accent + "/ " + m.at.query + sgrReset
		if m.at.flat {
			tail += c.dim + "  (flat)" + sgrReset
		}
	} else if m.hideClosed && m.at.list == "" {
		tail += c.dim + "  · closed hidden" + sgrReset
	}
	// The header with the first cut rows above the place left out.
	line := func(cut int) string {
		above := ""
		if cut > 0 {
			above = "… › "
		}
		for _, name := range parts[cut : len(parts)-1] {
			above += name + " › "
		}
		if above != "" {
			above = c.dim + above + sgrReset
		}
		return above + parts[len(parts)-1] + tail
	}
	header := line(0)
	if gap := w - ansi.StringWidth(header) - ansi.StringWidth(ruler); ruler != "" && gap >= 2 {
		return header + strings.Repeat(" ", gap) + c.dim + ruler + sgrReset
	}
	for cut := 1; cut < len(parts) && ansi.StringWidth(header) > w; cut++ {
		header = line(cut)
	}
	return ansi.Truncate(header, w, "…") + sgrReset
}

// ruler is where the pane is among its rows, as Vim's ruler says it, given
// how many rows are drawn and whether the last of them is whole. It says
// nothing when every row is on screen.
func (m *Model) ruler(drawn int, whole bool) string {
	end := m.top + drawn
	if !whole {
		end--
	}
	switch {
	case m.top == 0 && end >= len(m.vis):
		return ""
	case m.top == 0:
		return "Top"
	case end >= len(m.vis):
		return "Bot"
	}
	return strconv.Itoa(min(m.top*100/max(len(m.vis)-drawn, 1), 99)) + "%"
}

func (m *Model) emptyText() string {
	switch {
	case m.at.query != "" && m.ask == nil:
		// A held filter is the one thing between the pane and its rows.
		return "Nothing matches. Esc drops the filter."
	case m.at.query != "":
		return "Nothing matches."
	case m.at.list == kindToday:
		return "Nothing picked for today. Press t on a row to pick it."
	case m.at.list == kindWaiting:
		return "Nothing is waiting. Stamp a row $waiting to list it here."
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
			// The one word this line reads that no other does.
			line += "   " + m.colors().dim + ansi.Truncate("%name files it in a project or heading", room, "…") + sgrReset
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
			// The line read back in words, so its marks need not be remembered.
			found := matchCount(len(m.at.matched))
			note := found + " · " + shape + " · Tab switches"
			if strings.TrimSpace(string(a.ed.buf)) == "" {
				// A blank line hides nothing, so it counts nothing and names
				// what it reads instead.
				note = shape + " · Tab switches · words, #tag, $state, @date, ! · - excludes"
			} else if tags != "" {
				note = found + " · " + tags
			} else if reads := readBack(string(a.ed.buf), m.today()); reads != "" {
				note = found + " · " + shape + " · " + reads
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
		case a.kind == askCommand && a.choice > 0:
			// The command the walk stands on leads, as a picker's match does.
			uses, _ := commandMatches(a.draft)
			from := min(a.choice, len(uses)) - 1
			offer := sgrBold + uses[from] + sgrReset
			if from > 0 {
				offer = m.colors().dim + "‹ " + sgrReset + offer
			}
			for _, use := range uses[from+1:] {
				if ansi.StringWidth(offer) > room {
					break // the rest would not fit
				}
				offer += m.colors().dim + " · " + use + sgrReset
			}
			line += "   " + ansi.Truncate(offer, room, "…") + sgrReset
		case a.kind == askCommand:
			line += "   " + m.colors().dim + ansi.Truncate(commandOffers(string(a.ed.buf)), room, "…") + sgrReset
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
	case failing(left):
		// What did not reach the disk or the sync folder is the one message
		// that must not pass for a confirmation.
		left = m.colors().alert + left + sgrReset
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
	if m.msg == "" && left != "" && m.nextKeys() == "" {
		// A standing hint is dimmed, so a message is seen to be one.
		left = m.colors().dim + left + sgrReset
	}
	if n := len(m.picked); n > 0 {
		// The keys now act on these rows, so the count is in the colour
		// their marks are drawn in and only the hint is dim.
		count := m.colors().accent + strconv.Itoa(n) + " selected" + sgrReset + m.colors().dim
		right = count + " · Esc clears"
		if m.sweep != "" {
			// A sweep is a mode, and a mode is said while it lasts.
			right = count + " · V keeps · Esc clears"
		}
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
	lines = append(lines, "") // the header, once the rows say how far they reach

	var caret *tea.Cursor
	drawn, whole := 0, true // the rows on screen, and whether the last is all there
	for i := m.top; i < len(m.vis) && len(lines) < h; i++ {
		v := m.vis[i]
		drawn++
		r := m.tree.Row(v.id)
		state, kids := r.Fields[sigil.FieldState], len(m.tree.Children(v.id))
		if flat {
			kids = 0 // nothing folds in a flat view, so no row offers to
		}
		if e := m.edit; e != nil && e.row == v.id {
			// The row being typed shows what its line makes of it so far.
			notes := e.notes
			if e.fresh {
				notes = m.defaultsToNotes(r.Parent)
			}
			state = sigil.Parse(string(e.buf), notes, m.today()).State
		}
		glyph := m.stateGlyph(state, kids, m.isFolded(v.id))
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
				whole = false
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
	lines[0] = m.paneHeader(w, m.ruler(drawn, whole))
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
		v := tea.NewView(m.paintText(m.helpView()))
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
		// Too narrow for both: the sidebar takes the pane while it has focus,
		// no wider than keeps each count beside the name it counts.
		lines = m.sideLines(min(w, sidebarAlone), bodyH)
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
