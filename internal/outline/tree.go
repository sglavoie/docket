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
	FieldPin     = "pin" // a pinned row is drawn before its unpinned siblings
)

// Row is one line of the outline. The root of the tree is the empty ID.
type Row struct {
	ID       string
	Parent   string
	Pos      string
	Fields   map[string]string
	Created  int64 // unix milliseconds
	Modified int64
	Edited   int64 // when a field was last written; a move leaves it
}

// Quiet fields are view state written on a row, such as its place in
// Today: writing one is not an edit of the row.
var Quiet = map[string]bool{"todaypos": true}

func (r *Row) Text() string  { return r.Fields[FieldText] }
func (r *Row) Deleted() bool { return r.Fields[FieldDeleted] != "" }
func (r *Row) Pinned() bool  { return r.Fields[FieldPin] != "" }

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
// sibling order: the pinned rows first, then the rest, each group by pos.
// A pos is only ever compared within its group, so pinning and unpinning
// never move another row, and an unpinned row goes back where it was.
type Tree struct {
	rows map[string]*Row
	kids map[string][]string
	gen  uint64
}

// Gen counts the changes made to the tree. What is derived from the tree
// stays true for as long as the count holds.
func (t *Tree) Gen() uint64 { return t.gen }

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
	if a.Pinned() != b.Pinned() {
		return a.Pinned()
	}
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
	t.gen++
	switch op.Kind {
	case Insert:
		if t.rows[op.Row] != nil {
			return Op{}, fmt.Errorf("insert %s: row exists", op.Row)
		}
		if op.Parent != "" && t.rows[op.Parent] == nil {
			return Op{}, fmt.Errorf("insert under %s: %w", op.Parent, ErrUnknownRow)
		}
		r := &Row{ID: op.Row, Parent: op.Parent, Pos: op.Pos, Fields: map[string]string{}, Created: now, Modified: now, Edited: now}
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
		if !Quiet[op.Field] {
			r.Edited = now
		}
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
		t.gen++
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

// Gap is the pair of keys a row of one group, pinned or not, goes between
// to land directly after sibling, or as near after it as the group allows:
// first among the unpinned when sibling is pinned, last among the pinned
// when it is not. An empty sibling means first in the group. Rows in moving
// are passed over, since they are the ones being placed.
func (t *Tree) Gap(parent, sibling string, pinned bool, moving map[string]bool) (left, right string) {
	seen := sibling == ""
	for _, id := range t.kids[parent] {
		r := t.rows[id]
		if !moving[id] && r.Pinned() == pinned {
			if !seen {
				left = r.Pos
			} else {
				return left, r.Pos
			}
		}
		if id == sibling {
			seen = true
		}
	}
	// The group ends after sibling, or sibling is not there: its end.
	return left, ""
}

// PosAfter is a position for a row of the group directly after sibling, or
// first in the group when sibling is "".
func (t *Tree) PosAfter(parent, sibling string, pinned bool) string {
	return Between(t.Gap(parent, sibling, pinned, nil))
}

// PosBefore is a position for a row of the group directly before sibling.
func (t *Tree) PosBefore(parent, sibling string, pinned bool) string {
	i := t.index(parent, sibling)
	if i < 0 {
		return t.PosLast(parent, pinned)
	}
	prev := ""
	if i > 0 {
		prev = t.kids[parent][i-1]
	}
	return t.PosAfter(parent, prev, pinned)
}

// PosLast is a position after every child of parent in the group.
func (t *Tree) PosLast(parent string, pinned bool) string {
	left := ""
	for _, id := range t.kids[parent] {
		if r := t.rows[id]; r.Pinned() == pinned {
			left = r.Pos
		}
	}
	return Between(left, "")
}
