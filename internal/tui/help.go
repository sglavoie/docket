package tui

import (
	_ "embed"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// help.txt is the reference as it is written: "## " opens a section and
// ":: " gives it a subtitle, "== " opens a group, and a line holding a tab is
// keys, then what they do. A line starting with the tab is a note on the line
// above. Backticks set off the keys named inside a description.
//
//go:embed help.txt
var helpSource string

type helpKind int

const (
	helpSection helpKind = iota
	helpGroup
	helpItem
)

// helpEntry is one line of the reference. A section keeps its title in keys
// and its subtitle in text; a note is an item without keys.
type helpEntry struct {
	kind       helpKind
	keys, text string
}

var helpDoc = readHelp(helpSource)

func readHelp(src string) []helpEntry {
	var doc []helpEntry
	for _, line := range strings.Split(src, "\n") {
		keys, text, item := strings.Cut(line, "\t")
		switch {
		case item:
			doc = append(doc, helpEntry{helpItem, keys, text})
		case strings.HasPrefix(line, "## "):
			doc = append(doc, helpEntry{kind: helpSection, keys: line[3:]})
		case strings.HasPrefix(line, ":: ") && len(doc) > 0:
			doc[len(doc)-1].text = line[3:]
		case strings.HasPrefix(line, "== "):
			doc = append(doc, helpEntry{kind: helpGroup, keys: line[3:]})
		}
	}
	return doc
}

const (
	helpKeyCols = 15 // the column the keys are read down
	helpMaxCols = 88 // a line longer than this is hard to read back
	helpStacked = 48 // narrower than this, keys sit above what they do
)

// helpPage is the reference laid out for one width: its lines, and for each
// section its title and the line it starts on.
type helpPage struct {
	lines  []string
	heads  []int
	titles []string
}

// helpWidth is the width the reference is drawn in: the pane's, beside the
// rail of sections when there is room for one.
func (m *Model) helpWidth() (pane int, rail bool) {
	w, _ := m.size()
	if w >= sidebarFrom {
		return w - sidebarCols - 3, true
	}
	return w, false
}

func (m *Model) helpLayout() helpPage {
	paneW, _ := m.helpWidth()
	w := min(paneW, helpMaxCols)
	c := m.colors()
	indent := 2 + helpKeyCols + 2
	stacked := w < helpStacked
	if stacked {
		indent = 4
	}
	margin := strings.Repeat(" ", indent)

	var p helpPage
	for _, e := range helpDoc {
		switch e.kind {
		case helpSection:
			if len(p.lines) > 0 {
				p.lines = append(p.lines, "")
			}
			p.heads = append(p.heads, len(p.lines))
			p.titles = append(p.titles, e.keys)
			title := c.accent + sgrBold + e.keys + sgrReset
			if e.text != "" {
				title += "  " + c.dim + e.text + sgrReset
			}
			p.lines = append(p.lines, title, "")
		case helpGroup:
			p.lines = append(p.lines, "", "  "+sgrBold+e.keys+sgrReset)
		case helpItem:
			plain, strong := "", sgrBold
			if e.keys == "" {
				plain, strong = c.dim, c.dim+sgrBold
			}
			body := helpWrap(e.text, max(w-indent, 8), plain, strong)
			used := ansi.StringWidth(e.keys)
			switch {
			case e.keys == "":
			case stacked || used > helpKeyCols:
				p.lines = append(p.lines, "  "+c.accent+e.keys+sgrReset)
			default:
				body[0] = "  " + c.accent + e.keys + sgrReset + strings.Repeat(" ", helpKeyCols-used+2) + body[0]
				p.lines = append(p.lines, body[0])
				body = body[1:]
			}
			for _, line := range body {
				p.lines = append(p.lines, margin+line)
			}
		}
	}
	for i, line := range p.lines {
		p.lines[i] = ansi.Truncate(line, paneW, "…")
	}
	return p
}

// helpWrap lays a description out in lines of at most width columns. What
// stands between backticks is a key and is drawn strong, the rest plain.
func helpWrap(text string, width int, plain, strong string) []string {
	lines := []string{""}
	used := 0
	key := false
	for _, raw := range strings.Fields(text) {
		var styled strings.Builder
		cols := 0
		for i, part := range strings.Split(raw, "`") {
			if i > 0 {
				key = !key
			}
			if part == "" {
				continue
			}
			style := plain
			if key {
				style = strong
			}
			if style != "" {
				part = style + part + sgrReset
			}
			styled.WriteString(part)
			cols += ansi.StringWidth(part)
		}
		if used > 0 && used+1+cols > width {
			lines, used = append(lines, ""), 0
		}
		if used > 0 {
			lines[len(lines)-1] += " "
			used++
		}
		lines[len(lines)-1] += styled.String()
		used += cols
	}
	return lines
}

func (m *Model) openHelp() {
	m.help, m.helpTop, m.helpG = true, 0, false
}

// helpRows is the number of help lines visible at once, under the header.
func (m *Model) helpRows() int {
	_, h := m.size()
	return max(h-2, 1)
}

// helpKey scrolls the help; any key it does not know closes it.
func (m *Model) helpKey(k string) {
	g := m.helpG
	m.helpG = false
	rows := m.helpRows()
	p := m.helpLayout()
	switch k {
	case "]":
		for _, head := range p.heads {
			if head > m.helpTop {
				m.helpTop = head
				break
			}
		}
	case "[":
		for i := len(p.heads) - 1; i >= 0; i-- {
			if p.heads[i] < m.helpTop {
				m.helpTop = p.heads[i]
				break
			}
		}
	case "j", "down", "ctrl+n":
		m.helpTop++
	case "k", "up", "ctrl+p":
		m.helpTop--
	case "ctrl+d":
		m.helpTop += rows / 2
	case "ctrl+u":
		m.helpTop -= rows / 2
	case "pgdown", "ctrl+f", "space":
		m.helpTop += rows
	case "pgup", "ctrl+b":
		m.helpTop -= rows
	case "G", "end":
		m.helpTop = 1 << 30
	case "home":
		m.helpTop = 0
	case "g":
		if g {
			m.helpTop = 0
		} else {
			m.helpG = true
		}
	default:
		m.help = false
	}
	m.clampHelp(p)
}

// clampHelp keeps the final heading reachable even when its section is
// shorter than a page. The view pads the remaining space above its footer.
func (m *Model) clampHelp(p helpPage) {
	last := p.heads[len(p.heads)-1]
	m.helpTop = max(min(m.helpTop, max(last, len(p.lines)-m.helpRows())), 0)
}

// helpView draws the reference as the app draws itself: the sections where
// the sidebar is, the one being read in the pane under a header, and a
// status line.
func (m *Model) helpView() string {
	w, _ := m.size()
	c := m.colors()
	rows := m.helpRows()
	paneW, rail := m.helpWidth()
	p := m.helpLayout()
	m.clampHelp(p)

	at := 0
	for i, head := range p.heads {
		if head <= m.helpTop {
			at = i
		}
	}
	header := c.dim + "Help › " + sgrReset + p.titles[at]
	if !rail {
		header += c.dim + "  · " + strconv.Itoa(at+1) + " of " + strconv.Itoa(len(p.titles)) + sgrReset
	}
	pane := []string{ansi.Truncate(header, paneW, "…") + sgrReset}
	end := min(m.helpTop+rows, len(p.lines))
	pane = append(pane, p.lines[m.helpTop:end]...)
	for len(pane) < rows+1 {
		pane = append(pane, "")
	}

	out := pane
	if rail {
		out = make([]string, len(pane))
		for i := range pane {
			side := ""
			if i < len(p.titles) {
				side = "  " + ansi.Truncate(p.titles[i], sidebarCols-2, "…")
				if i == at {
					side = c.accent + "▌" + sgrReset + " " + sgrBold + side[2:] + sgrReset
					side = m.tint(side, sidebarCols, c.cursor)
				}
			}
			side += strings.Repeat(" ", max(sidebarCols-ansi.StringWidth(side), 0))
			out[i] = side + c.dim + " │ " + sgrReset + pane[i]
		}
	}

	// Where the page is in the reference, as Vim's ruler says it.
	where := strconv.Itoa(m.helpTop*100/max(len(p.lines)-rows, 1)) + "%"
	switch {
	case len(p.lines) <= rows:
		where = "All"
	case m.helpTop == 0:
		where = "Top"
	case end == len(p.lines):
		where = "Bot"
	}
	right := c.dim + where + sgrReset
	if pend := m.pending(); pend != "" {
		right = sgrBold + pend + sgrReset + "  " + right
	}
	left := ansi.Truncate("j k scroll · ] [ section · any other key closes", max(w-ansi.StringWidth(right)-2, 0), "…")
	gap := max(w-ansi.StringWidth(left)-ansi.StringWidth(right), 0)
	out = append(out, c.dim+left+sgrReset+strings.Repeat(" ", gap)+right)
	return strings.Join(out, "\n")
}
