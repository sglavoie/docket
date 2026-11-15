package tui

import (
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
  h l            fold, unfold; step to parent, first child
  Enter  Esc     zoom into a row, back out
  o O            new row below, above
  e              edit the row
  > <            indent, outdent
  J K            move the row down, up
  x  !  t        done and back, flag, pick for Today
  m              move the row to a project
  a              capture a line to the Inbox; %name files it in a project
  p              go to a project or list by name
  Tab            the sidebar: j k choose, Enter work in it
  dd             delete
  u  Ctrl+R      undo, redo
  Ctrl+C         quit

While typing
  Enter          keep the row and start the next
  Esc            keep the row
  Tab Shift+Tab  indent, outdent

Sigils, typed anywhere in a row
  !              flag
  @fri @+3d @11-20 @today    deadline
  #tag           tag
  $doing $waiting $done $canceled $parked $open $note    state
  A row is a task unless it is top-level or ends in a colon.
  \#tag keeps a sigil as plain text.

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

// window returns the part of the buffer that fits in avail columns with the
// caret inside it, and the caret's column within that part.
func (e *editor) window(avail int) (string, int) {
	if e.caret < e.off {
		e.off = e.caret
	}
	for e.off < e.caret && ansi.StringWidth(string(e.buf[e.off:e.caret])) > avail-1 {
		e.off++
	}
	return ansi.Truncate(string(e.buf[e.off:]), avail, ""), ansi.StringWidth(string(e.buf[e.off:e.caret]))
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

// rowBody is a row's text followed by its fields, each spelled with the
// sigil that writes it. A closed or parked row is dimmed whole and raises no
// alarm.
func (m *Model) rowBody(r *outline.Row, onCursor bool) string {
	l := sigil.FromFields(r.Fields)
	closed := l.Closed() || l.State == sigil.Parked
	paint := func(style, s string) string {
		if closed {
			style = sgrDim
		}
		return style + s + sgrReset
	}

	var b strings.Builder
	switch {
	case closed:
		b.WriteString(sgrDim + l.Text + sgrReset)
	case onCursor:
		b.WriteString(sgrBold + l.Text + sgrReset)
	default:
		b.WriteString(l.Text)
	}
	if l.Flag {
		b.WriteString(" " + paint(sgrAlert+sgrBold, "!"))
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
		b.WriteString(" " + paint(style, label))
	}
	for _, tag := range l.Tags {
		b.WriteString(" " + paint(sgrDim, "#"+tag))
	}
	return b.String()
}

func (m *Model) breadcrumb() string {
	switch {
	case m.at.list != "":
		return strings.ToUpper(m.at.list[:1]) + m.at.list[1:]
	case m.at.root == "":
		return "Projects"
	}
	var parts []string
	for id := m.at.root; id != ""; {
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
	case m.at.list == kindToday:
		return "Nothing picked for today. Press t on a row to pick it."
	case m.at.list == kindWaiting:
		return "Nothing is waiting."
	case m.at.list == kindStale:
		return "Nothing has gone stale."
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

// listNote is what a row in a list says beyond itself: how long it has
// waited or sat untouched, and which project it lives in.
func (m *Model) listNote(r *outline.Row) string {
	var parts []string
	switch m.at.list {
	case kindWaiting:
		parts = append(parts, days(m.now()-m.changedAt(r.ID)))
	case kindStale:
		parts = append(parts, days(m.now()-r.Modified))
	}
	if p := m.projectOf(r.ID); p != nil && p.ID != r.ID {
		parts = append(parts, p.Text())
	}
	if len(parts) == 0 {
		return ""
	}
	return sgrDim + " · " + strings.Join(parts, " · ") + sgrReset
}

// statusLine is the bottom row, and the caret's column when it is asking.
func (m *Model) statusLine(w int) (string, int) {
	if a := m.ask; a != nil {
		label := map[string]string{askCapture: "Inbox › ", askJump: "Go to › ", askMove: "Move to › "}[a.kind]
		lw := ansi.StringWidth(label)
		text, col := a.ed.window(max(w-lw, 1))
		line := sgrAccent + label + sgrReset + text
		if room := w - lw - ansi.StringWidth(text) - 3; a.kind != askCapture && room > 3 {
			// The first name is the one Enter takes.
			offer := sgrDim + "nothing matches" + sgrReset
			for i, e := range match(m.targets(a.kind == askJump), strings.TrimSpace(string(a.ed.buf)), 3) {
				if i == 0 {
					offer = sgrBold + e.label + sgrReset
				} else {
					offer += sgrDim + " · " + e.label + sgrReset
				}
			}
			line += "   " + ansi.Truncate(offer, room, "…") + sgrReset
		}
		return line, lw + col
	}

	left, right := m.msg, "? help"
	switch {
	case left != "":
	case m.edit != nil:
		left = "Enter next row · Esc done · Tab indent"
	case m.sideFocus:
		left = "j k choose · Enter work in it"
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
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+listH {
		m.top = m.cursor - listH + 1
	}
	m.top = max(0, min(m.top, len(m.vis)-listH))

	lines := make([]string, 0, h)
	lines = append(lines, sgrDim+ansi.Truncate(m.breadcrumb(), w, "…")+sgrReset)

	var caret *tea.Cursor
	for i := m.top; i < m.top+listH; i++ {
		if i >= len(m.vis) {
			lines = append(lines, "")
			continue
		}
		v := m.vis[i]
		r := m.tree.Row(v.id)

		kids := len(m.tree.Children(v.id))
		glyph := stateGlyph(r.Fields[sigil.FieldState], kids, m.folded[v.id])
		gutter := " "
		if i == m.cursor {
			gutter = sgrAccent + "▌" + sgrReset
			if m.sideFocus {
				gutter = sgrDim + "▌" + sgrReset
			}
		}
		prefixW := 2 + 2*v.depth + 2
		avail := max(w-prefixW, 1)

		var text string
		if m.edit != nil && m.edit.row == v.id {
			var col int
			text, col = m.edit.window(avail)
			caret = tea.NewCursor(prefixW+col, len(lines))
			caret.Shape = tea.CursorBar
		} else {
			text = m.rowBody(r, i == m.cursor)
			// A task's glyph is its state, so the children it hides are
			// counted after it: when folded, and always in a list.
			if hides := m.folded[v.id] || m.at.list != ""; kids > 0 && hides && r.Fields[sigil.FieldState] != "" {
				text += sgrDim + " +" + strconv.Itoa(kids) + sgrReset
			}
			if m.at.list != "" {
				text += m.listNote(r)
			}
			text = ansi.Truncate(text, avail, "…") + sgrReset
		}
		lines = append(lines, gutter+" "+strings.Repeat("  ", v.depth)+glyph+" "+text)
	}
	if len(m.vis) == 0 {
		lines[1] = sgrDim + "  " + ansi.Truncate(m.emptyText(), max(w-2, 1), "…") + sgrReset
	}
	return lines, caret
}

func (m *Model) View() tea.View {
	w, h := m.size()
	if m.help {
		v := tea.NewView(helpText)
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
