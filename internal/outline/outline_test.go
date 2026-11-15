package outline

import (
	"errors"
	"math/rand"
	"sort"
	"strings"
	"testing"
)

func TestBetweenKeepsOrderUnderRandomInsertion(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	keys := []string{}
	for i := 0; i < 3000; i++ {
		at := rng.Intn(len(keys) + 1)
		a, b := "", ""
		if at > 0 {
			a = keys[at-1]
		}
		if at < len(keys) {
			b = keys[at]
		}
		k := Between(a, b)
		if (a != "" && k <= a) || (b != "" && k >= b) {
			t.Fatalf("Between(%q, %q) = %q is out of range", a, b, k)
		}
		if strings.HasSuffix(k, "0") {
			t.Fatalf("Between(%q, %q) = %q ends in the lowest digit", a, b, k)
		}
		keys = append(keys, "")
		copy(keys[at+1:], keys[at:])
		keys[at] = k
	}
	if !sort.StringsAreSorted(keys) {
		t.Fatal("keys are not sorted")
	}
}

func TestBetweenAppendAndPrependStayShort(t *testing.T) {
	last, first := "", ""
	for i := 0; i < 1000; i++ {
		last = Between(last, "")
		next := Between("", first)
		if first != "" && next >= first {
			t.Fatalf("prepend %q is not before %q", next, first)
		}
		first = next
	}
	if len(last) > 40 || len(first) > 40 {
		t.Fatalf("keys grew too long: append %d, prepend %d", len(last), len(first))
	}
}

func build(t *testing.T) *Tree {
	t.Helper()
	tr := NewTree(nil)
	for _, op := range []Op{
		{Kind: Insert, Row: "a", Pos: "V"},
		{Kind: Insert, Row: "b", Pos: "W"},
		{Kind: Insert, Row: "a1", Parent: "a", Pos: "V"},
		{Kind: Insert, Row: "a2", Parent: "a", Pos: "W"},
	} {
		if _, err := tr.Apply(op, 1); err != nil {
			t.Fatal(err)
		}
	}
	return tr
}

func TestApplyReturnsInverse(t *testing.T) {
	tr := build(t)
	ops := []Op{
		{Kind: Set, Row: "a1", Field: FieldText, Value: "hello"},
		{Kind: Move, Row: "a2", Parent: "b", Pos: "V"},
		{Kind: Set, Row: "a", Field: FieldDeleted, Value: "1"},
		{Kind: Insert, Row: "c", Pos: "X"},
	}
	var inverse []Op
	for _, op := range ops {
		inv, err := tr.Apply(op, 2)
		if err != nil {
			t.Fatal(err)
		}
		inverse = append([]Op{inv}, inverse...)
	}
	if got := strings.Join(tr.Children(""), ","); got != "b,c" {
		t.Fatalf("top level after ops = %s", got)
	}
	for _, op := range inverse {
		if _, err := tr.Apply(op, 3); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Join(tr.Children(""), ","); got != "a,b" {
		t.Fatalf("top level after undo = %s", got)
	}
	if got := strings.Join(tr.Children("a"), ","); got != "a1,a2" {
		t.Fatalf("children of a after undo = %s", got)
	}
	if tr.Row("a1").Text() != "" {
		t.Fatalf("text was not restored")
	}
}

func TestMoveRefusesCycle(t *testing.T) {
	tr := build(t)
	for _, target := range []string{"a", "a1"} {
		_, err := tr.Apply(Op{Kind: Move, Row: "a", Parent: target, Pos: "V"}, 2)
		if !errors.Is(err, ErrCycle) {
			t.Fatalf("move a under %s: got %v, want ErrCycle", target, err)
		}
	}
}

func TestPositionsLandBetweenSiblings(t *testing.T) {
	tr := build(t)
	apply := func(op Op) {
		t.Helper()
		if _, err := tr.Apply(op, 2); err != nil {
			t.Fatal(err)
		}
	}
	apply(Op{Kind: Insert, Row: "mid", Pos: tr.PosAfter("", "a")})
	apply(Op{Kind: Insert, Row: "first", Pos: tr.PosBefore("", "a")})
	apply(Op{Kind: Insert, Row: "last", Pos: tr.PosLast("")})
	if got := strings.Join(tr.Children(""), ","); got != "first,a,mid,b,last" {
		t.Fatalf("order = %s", got)
	}
}
