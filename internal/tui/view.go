package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	sgrReset  = "\x1b[0m"
	sgrBold   = "\x1b[1m"
	sgrDim    = "\x1b[2m"
	sgrAccent = "\x1b[36m"
)

const helpText = `Docket

  j k  gg G      move
  h l            fold, unfold; step to parent, first child
  Enter  Esc     zoom into a row, back out
  o O            new row below, above
  e              edit the row
  > <            indent, outdent
  J K            move the row down, up
  dd             delete
  u  Ctrl+R      undo, redo
  Ctrl+C         quit

While typing
  Enter          keep the row and start the next
  Esc            keep the row
  Tab Shift+Tab  indent, outdent

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

func (m *Model) breadcrumb() string {
	if m.root == "" {
		return "Docket"
	}
	var parts []string
	for id := m.root; id != ""; {
		r := m.tree.Row(id)
		if r == nil {
			break
		}
		parts = append([]string{r.Text()}, parts...)
		id = r.Parent
	}
	return strings.Join(parts, " › ")
}

func (m *Model) statusLine(w int) string {
	left, right := m.msg, "? help"
	if m.edit != nil {
		right = ""
		if left == "" {
			left = "Enter next row · Esc done · Tab indent"
		}
	}
	left = ansi.Truncate(left, w, "…")
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + sgrDim + right + sgrReset
}

func (m *Model) View() tea.View {
	w, h := m.size()
	if m.help {
		v := tea.NewView(helpText)
		v.AltScreen = true
		return v
	}

	listH := max(h-2, 1)
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

		glyph := "•"
		if len(m.tree.Children(v.id)) > 0 {
			glyph = "▾"
			if m.folded[v.id] {
				glyph = "▸"
			}
		}
		gutter := " "
		if i == m.cursor {
			gutter = sgrAccent + "▌" + sgrReset
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
			text = ansi.Truncate(r.Text(), avail, "…")
			if i == m.cursor {
				text = sgrBold + text + sgrReset
			}
		}
		lines = append(lines, gutter+" "+strings.Repeat("  ", v.depth)+sgrDim+glyph+sgrReset+" "+text)
	}
	if len(m.vis) == 0 {
		lines[1] = sgrDim + "  Nothing here yet. Press o to add a row." + sgrReset
	}
	lines = append(lines, m.statusLine(w))

	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.Cursor = caret
	return v
}
