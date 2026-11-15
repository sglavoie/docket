// Package outline is the whole durable model: a tree of rows, changed only by
// operations. It knows nothing about terminals or SQL.
package outline

import (
	"errors"
	"fmt"
	"sort"
)

// Field names. A field is a string value on a row; the empty string is unset.
const (
	FieldText    = "text"
	FieldDeleted = "deleted"
)

// Row is one line of the outline. The root of the tree is the empty ID.
type Row struct {
	ID       string
	Parent   string
	Pos      string
	Fields   map[string]string
	Created  int64 // unix milliseconds
	Modified int64
}

func (r *Row) Text() string  { return r.Fields[FieldText] }
func (r *Row) Deleted() bool { return r.Fields[FieldDeleted] != "" }

type Kind string

const (
	Insert Kind = "insert"
	Set    Kind = "set"
	Move   Kind = "move"
)

// Op is one change to one row. Insert and Move carry Parent and Pos; Set
// carries Field and Value. There is no delete: a row is retired by setting
// FieldDeleted, which is what makes every operation reversible by another.
type Op struct {
	Kind   Kind
	Row    string
	Field  string
	Value  string
	Parent string
	Pos    string
}

var (
	ErrUnknownRow = errors.New("unknown row")
	ErrCycle      = errors.New("a row cannot move under itself")
)

// Tree is the in-memory outline. Children lists hold live rows only, in
// sibling order.
type Tree struct {
	rows map[string]*Row
	kids map[string][]string
}

func NewTree(rows []*Row) *Tree {
	t := &Tree{rows: make(map[string]*Row, len(rows)), kids: map[string][]string{}}
	for _, r := range rows {
		if r.Fields == nil {
			r.Fields = map[string]string{}
		}
		t.rows[r.ID] = r
	}
	for _, r := range rows {
		if !r.Deleted() {
			t.kids[r.Parent] = append(t.kids[r.Parent], r.ID)
		}
	}
	for _, k := range t.kids {
		sort.Slice(k, func(i, j int) bool { return t.less(t.rows[k[i]], t.rows[k[j]]) })
	}
	return t
}

func (t *Tree) less(a, b *Row) bool {
	if a.Pos != b.Pos {
		return a.Pos < b.Pos
	}
	return a.ID < b.ID
}

// Row returns the row, or nil.
func (t *Tree) Row(id string) *Row { return t.rows[id] }

// Children returns the live children in order. The slice is the tree's own.
func (t *Tree) Children(id string) []string { return t.kids[id] }

// Contains reports whether ancestor is id or one of its ancestors.
func (t *Tree) Contains(ancestor, id string) bool {
	for id != "" {
		if id == ancestor {
			return true
		}
		r := t.rows[id]
		if r == nil {
			return false
		}
		id = r.Parent
	}
	return ancestor == ""
}

func (t *Tree) link(r *Row) {
	if r.Deleted() {
		return
	}
	k := t.kids[r.Parent]
	i := sort.Search(len(k), func(i int) bool { return t.less(r, t.rows[k[i]]) })
	k = append(k, "")
	copy(k[i+1:], k[i:])
	k[i] = r.ID
	t.kids[r.Parent] = k
}

func (t *Tree) unlink(r *Row) {
	k := t.kids[r.Parent]
	for i, id := range k {
		if id == r.ID {
			t.kids[r.Parent] = append(k[:i], k[i+1:]...)
			return
		}
	}
}

// Apply performs op and returns the operation that undoes it.
func (t *Tree) Apply(op Op, now int64) (Op, error) {
	switch op.Kind {
	case Insert:
		if t.rows[op.Row] != nil {
			return Op{}, fmt.Errorf("insert %s: row exists", op.Row)
		}
		if op.Parent != "" && t.rows[op.Parent] == nil {
			return Op{}, fmt.Errorf("insert under %s: %w", op.Parent, ErrUnknownRow)
		}
		r := &Row{ID: op.Row, Parent: op.Parent, Pos: op.Pos, Fields: map[string]string{}, Created: now, Modified: now}
		t.rows[r.ID] = r
		t.link(r)
		return Op{Kind: Set, Row: r.ID, Field: FieldDeleted, Value: "1"}, nil

	case Set:
		r := t.rows[op.Row]
		if r == nil {
			return Op{}, fmt.Errorf("set %s: %w", op.Row, ErrUnknownRow)
		}
		prev := r.Fields[op.Field]
		t.unlink(r)
		if op.Value == "" {
			delete(r.Fields, op.Field)
		} else {
			r.Fields[op.Field] = op.Value
		}
		t.link(r)
		r.Modified = now
		return Op{Kind: Set, Row: r.ID, Field: op.Field, Value: prev}, nil

	case Move:
		r := t.rows[op.Row]
		if r == nil {
			return Op{}, fmt.Errorf("move %s: %w", op.Row, ErrUnknownRow)
		}
		if op.Parent != "" && t.rows[op.Parent] == nil {
			return Op{}, fmt.Errorf("move under %s: %w", op.Parent, ErrUnknownRow)
		}
		if op.Parent != "" && t.Contains(r.ID, op.Parent) {
			return Op{}, ErrCycle
		}
		inv := Op{Kind: Move, Row: r.ID, Parent: r.Parent, Pos: r.Pos}
		t.unlink(r)
		r.Parent, r.Pos = op.Parent, op.Pos
		t.link(r)
		r.Modified = now
		return inv, nil
	}
	return Op{}, fmt.Errorf("unknown operation %q", op.Kind)
}

// Forget drops a row that was inserted but never committed.
func (t *Tree) Forget(id string) {
	if r := t.rows[id]; r != nil {
		t.unlink(r)
		delete(t.rows, id)
	}
}

func (t *Tree) index(parent, id string) int {
	for i, k := range t.kids[parent] {
		if k == id {
			return i
		}
	}
	return -1
}

// PrevSibling and NextSibling return the neighbouring live row, or "".
func (t *Tree) PrevSibling(id string) string {
	r := t.rows[id]
	if r == nil {
		return ""
	}
	if i := t.index(r.Parent, id); i > 0 {
		return t.kids[r.Parent][i-1]
	}
	return ""
}

func (t *Tree) NextSibling(id string) string {
	r := t.rows[id]
	if r == nil {
		return ""
	}
	k := t.kids[r.Parent]
	if i := t.index(r.Parent, id); i >= 0 && i+1 < len(k) {
		return k[i+1]
	}
	return ""
}

func (t *Tree) posAt(parent string, i int) string {
	k := t.kids[parent]
	if i < 0 || i >= len(k) {
		return ""
	}
	return t.rows[k[i]].Pos
}

// PosAfter is a position directly after sibling, or first when sibling is "".
func (t *Tree) PosAfter(parent, sibling string) string {
	i := t.index(parent, sibling)
	return Between(t.posAt(parent, i), t.posAt(parent, i+1))
}

// PosBefore is a position directly before sibling.
func (t *Tree) PosBefore(parent, sibling string) string {
	i := t.index(parent, sibling)
	if i < 0 {
		return t.PosLast(parent)
	}
	return Between(t.posAt(parent, i-1), t.posAt(parent, i))
}

// PosLast is a position after every child of parent.
func (t *Tree) PosLast(parent string) string {
	return Between(t.posAt(parent, len(t.kids[parent])-1), "")
}
