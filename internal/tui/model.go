// Package tui is the terminal client of the outline. It holds view state
// only: the zoom, the folds, the cursor, and one row being edited.
package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/store"
)

// Backend is what the client needs from storage.
type Backend interface {
	Commit(ops []outline.Op, now int64) error
	SetFold(id string, folded bool) error
}

type visRow struct {
	id    string
	depth int
}

// editor is the one row being typed into. Its operations are already applied
// to the tree and are stored only when the edit ends, so one edit is one
// transaction and one undo step.
type editor struct {
	row        string
	buf        []rune
	caret, off int
	fresh      bool   // the row was created by this edit
	back       string // where the cursor returns if a fresh row is abandoned
	ops, inv   []outline.Op
}

type Model struct {
	tree    *outline.Tree
	backend Backend
	now     func() int64
	newID   func() string

	width, height int
	root          string
	zoom          []string // the roots Esc returns to
	folded        map[string]bool
	vis           []visRow
	cursor, top   int

	undo, redo [][]outline.Op
	prefix     string // a pending g or d
	edit       *editor
	help       bool
	msg        string
}

func New(tree *outline.Tree, backend Backend, folds map[string]bool) *Model {
	if folds == nil {
		folds = map[string]bool{}
	}
	m := &Model{
		tree:    tree,
		backend: backend,
		folded:  folds,
		now:     func() int64 { return time.Now().UnixMilli() },
		newID:   store.NewID,
	}
	m.rebuild()
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

// rebuild recomputes the visible rows and keeps the cursor inside them.
func (m *Model) rebuild() {
	m.vis = m.vis[:0]
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		for _, id := range m.tree.Children(parent) {
			m.vis = append(m.vis, visRow{id, depth})
			if !m.folded[id] {
				walk(id, depth+1)
			}
		}
	}
	walk(m.root, 0)
	m.cursor = max(0, min(m.cursor, len(m.vis)-1))
}

func (m *Model) current() string {
	if m.cursor < len(m.vis) {
		return m.vis[m.cursor].id
	}
	return ""
}

// seek puts the cursor on id when it is visible.
func (m *Model) seek(id string) {
	for i, v := range m.vis {
		if v.id == id {
			m.cursor = i
			return
		}
	}
}

func (m *Model) setFold(id string, folded bool) {
	if m.folded[id] == folded {
		return
	}
	if folded {
		m.folded[id] = true
	} else {
		delete(m.folded, id)
	}
	if err := m.backend.SetFold(id, folded); err != nil {
		m.msg = "not saved: " + err.Error()
	}
}

// apply runs ops against the tree and returns the group that undoes them,
// newest first. A failure leaves the tree as it was.
func (m *Model) apply(ops []outline.Op) ([]outline.Op, error) {
	now := m.now()
	inv := make([]outline.Op, 0, len(ops))
	for _, op := range ops {
		undo, err := m.tree.Apply(op, now)
		if err != nil {
			for _, u := range inv {
				m.tree.Apply(u, now)
			}
			return nil, err
		}
		inv = append([]outline.Op{undo}, inv...)
	}
	return inv, nil
}

// record stores an applied group and makes it the next undo step.
func (m *Model) record(ops, inv []outline.Op) {
	if err := m.backend.Commit(ops, m.now()); err != nil {
		m.msg = "not saved: " + err.Error()
	}
	m.undo = append(m.undo, inv)
	m.redo = nil
}

func (m *Model) do(ops ...outline.Op) bool {
	inv, err := m.apply(ops)
	if err != nil {
		m.msg = err.Error()
		return false
	}
	m.record(ops, inv)
	return true
}

// replay moves one step between the undo and redo stacks.
func (m *Model) replay(from, to *[][]outline.Op, empty string) {
	if len(*from) == 0 {
		m.msg = empty
		return
	}
	group := (*from)[len(*from)-1]
	inv, err := m.apply(group)
	if err != nil {
		m.msg = err.Error()
		return
	}
	*from = (*from)[:len(*from)-1]
	if err := m.backend.Commit(group, m.now()); err != nil {
		m.msg = "not saved: " + err.Error()
	}
	*to = append(*to, inv)
	m.rebuild()
	m.seek(group[len(group)-1].Row)
}

func (m *Model) indentOp(id string) (outline.Op, bool) {
	prev := m.tree.PrevSibling(id)
	if prev == "" {
		m.msg = "no row above to indent under"
		return outline.Op{}, false
	}
	return outline.Op{Kind: outline.Move, Row: id, Parent: prev, Pos: m.tree.PosLast(prev)}, true
}

func (m *Model) outdentOp(id string) (outline.Op, bool) {
	r := m.tree.Row(id)
	if r.Parent == m.root {
		m.msg = "already at the top level"
		return outline.Op{}, false
	}
	p := m.tree.Row(r.Parent)
	return outline.Op{Kind: outline.Move, Row: id, Parent: p.Parent, Pos: m.tree.PosAfter(p.Parent, p.ID)}, true
}

// shift indents or outdents id, inside the open edit when there is one.
func (m *Model) shift(id string, indent bool) {
	op, ok := m.outdentOp(id)
	if indent {
		op, ok = m.indentOp(id)
	}
	if !ok {
		return
	}
	if m.edit != nil {
		inv, err := m.apply([]outline.Op{op})
		if err != nil {
			m.msg = err.Error()
			return
		}
		m.edit.ops = append(m.edit.ops, op)
		m.edit.inv = append(inv, m.edit.inv...)
	} else if !m.do(op) {
		return
	}
	if indent {
		m.setFold(op.Parent, false)
	}
	m.rebuild()
	m.seek(id)
}

func (m *Model) reorder(id string, down bool) {
	r := m.tree.Row(id)
	var pos string
	if down {
		next := m.tree.NextSibling(id)
		if next == "" {
			return
		}
		pos = m.tree.PosAfter(r.Parent, next)
	} else {
		prev := m.tree.PrevSibling(id)
		if prev == "" {
			return
		}
		pos = m.tree.PosBefore(r.Parent, prev)
	}
	if m.do(outline.Op{Kind: outline.Move, Row: id, Parent: r.Parent, Pos: pos}) {
		m.rebuild()
		m.seek(id)
	}
}

// newRow opens a blank row for typing: above the cursor, as the first child
// of an open parent, or else directly below.
func (m *Model) newRow(above bool) {
	cur := m.current()
	parent, pos := m.root, ""
	switch {
	case cur == "":
		pos = m.tree.PosLast(m.root)
	case above:
		parent = m.tree.Row(cur).Parent
		pos = m.tree.PosBefore(parent, cur)
	case len(m.tree.Children(cur)) > 0 && !m.folded[cur]:
		parent = cur
		pos = m.tree.PosAfter(cur, "")
	default:
		parent = m.tree.Row(cur).Parent
		pos = m.tree.PosAfter(parent, cur)
	}
	op := outline.Op{Kind: outline.Insert, Row: m.newID(), Parent: parent, Pos: pos}
	inv, err := m.apply([]outline.Op{op})
	if err != nil {
		m.msg = err.Error()
		return
	}
	m.rebuild()
	m.seek(op.Row)
	m.edit = &editor{row: op.Row, fresh: true, back: cur, ops: []outline.Op{op}, inv: inv}
}

func (m *Model) editRow(id string) {
	buf := []rune(m.tree.Row(id).Text())
	m.edit = &editor{row: id, buf: buf, caret: len(buf)}
}

// finishEdit ends the open edit and reports whether its row was kept. There
// is no cancel: the text stands, undo takes it back, and only a fresh row
// left blank disappears.
func (m *Model) finishEdit() bool {
	e := m.edit
	m.edit = nil
	text := strings.TrimSpace(string(e.buf))
	if e.fresh && text == "" {
		m.tree.Forget(e.row)
		m.rebuild()
		m.seek(e.back)
		return false
	}
	if text != m.tree.Row(e.row).Text() {
		op := outline.Op{Kind: outline.Set, Row: e.row, Field: outline.FieldText, Value: text}
		if inv, err := m.apply([]outline.Op{op}); err == nil {
			e.ops = append(e.ops, op)
			e.inv = append(inv, e.inv...)
		}
	}
	if len(e.ops) > 0 {
		m.record(e.ops, e.inv)
	}
	m.rebuild()
	m.seek(e.row)
	return true
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.PasteMsg:
		if m.edit != nil {
			m.edit.insert([]rune(strings.Join(strings.Fields(msg.Content), " ")))
		}
	case tea.KeyPressMsg:
		m.msg = ""
		if msg.String() == "ctrl+c" {
			if m.edit != nil {
				m.finishEdit()
			}
			return m, tea.Quit
		}
		switch {
		case m.help:
			m.help = false
		case m.edit != nil:
			m.editKey(msg)
		default:
			m.normalKey(msg.String())
		}
	}
	return m, nil
}

func (m *Model) normalKey(k string) {
	if m.prefix != "" {
		seq := m.prefix + k
		m.prefix = ""
		switch seq {
		case "gg":
			m.cursor = 0
		case "dd":
			if cur := m.current(); cur != "" {
				if m.do(outline.Op{Kind: outline.Set, Row: cur, Field: outline.FieldDeleted, Value: "1"}) {
					m.rebuild()
				}
			}
		}
		return
	}

	cur := m.current()
	switch k {
	case "g", "d":
		m.prefix = k
	case "j", "down":
		m.cursor = min(m.cursor+1, max(len(m.vis)-1, 0))
	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
	case "G":
		m.cursor = max(len(m.vis)-1, 0)
	case "o":
		m.newRow(false)
	case "O":
		m.newRow(true)
	case "u":
		m.replay(&m.undo, &m.redo, "nothing to undo")
	case "ctrl+r":
		m.replay(&m.redo, &m.undo, "nothing to redo")
	case "?":
		m.help = true
	case "esc":
		if n := len(m.zoom); n > 0 {
			was := m.root
			m.root, m.zoom = m.zoom[n-1], m.zoom[:n-1]
			m.rebuild()
			m.seek(was)
		}
	}
	if cur == "" {
		return
	}

	switch k {
	case "e":
		m.editRow(cur)
	case "enter":
		m.zoom = append(m.zoom, m.root)
		m.root, m.cursor = cur, 0
		m.rebuild()
	case "h", "left":
		if len(m.tree.Children(cur)) > 0 && !m.folded[cur] {
			m.setFold(cur, true)
			m.rebuild()
		} else if p := m.tree.Row(cur).Parent; p != m.root {
			m.seek(p)
		}
	case "l", "right":
		if m.folded[cur] {
			m.setFold(cur, false)
			m.rebuild()
		} else if kids := m.tree.Children(cur); len(kids) > 0 {
			m.seek(kids[0])
		}
	case ">":
		m.shift(cur, true)
	case "<":
		m.shift(cur, false)
	case "J":
		m.reorder(cur, true)
	case "K":
		m.reorder(cur, false)
	}
}

func (m *Model) editKey(msg tea.KeyPressMsg) {
	e := m.edit
	switch msg.String() {
	case "enter":
		// A fresh row chains into the next one, so a list is typed without
		// leaving the keyboard's home row.
		if m.finishEdit() && e.fresh {
			m.newRow(false)
		}
	case "esc":
		m.finishEdit()
	case "tab":
		m.shift(e.row, true)
	case "shift+tab":
		m.shift(e.row, false)
	case "backspace":
		if len(e.buf) == 0 && e.fresh {
			m.finishEdit()
		} else if e.caret > 0 {
			e.buf = append(e.buf[:e.caret-1], e.buf[e.caret:]...)
			e.caret--
		}
	case "delete":
		if e.caret < len(e.buf) {
			e.buf = append(e.buf[:e.caret], e.buf[e.caret+1:]...)
		}
	case "left":
		e.caret = max(e.caret-1, 0)
	case "right":
		e.caret = min(e.caret+1, len(e.buf))
	case "home", "ctrl+a":
		e.caret = 0
	case "end", "ctrl+e":
		e.caret = len(e.buf)
	case "ctrl+u":
		e.buf = e.buf[e.caret:]
		e.caret = 0
	case "ctrl+k":
		e.buf = e.buf[:e.caret]
	case "ctrl+w":
		i := e.caret
		for i > 0 && e.buf[i-1] == ' ' {
			i--
		}
		for i > 0 && e.buf[i-1] != ' ' {
			i--
		}
		e.buf = append(e.buf[:i], e.buf[e.caret:]...)
		e.caret = i
	default:
		if msg.Text != "" && msg.Mod&tea.ModCtrl == 0 {
			e.insert([]rune(msg.Text))
		}
	}
}

func (e *editor) insert(text []rune) {
	buf := make([]rune, 0, len(e.buf)+len(text))
	buf = append(buf, e.buf[:e.caret]...)
	buf = append(buf, text...)
	e.buf = append(buf, e.buf[e.caret:]...)
	e.caret += len(text)
}
