package store

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/outline"
)

func TestCommitSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docket.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ops := []outline.Op{
		{Kind: outline.Insert, Row: "a", Pos: "V"},
		{Kind: outline.Set, Row: "a", Field: outline.FieldText, Value: "Project"},
		{Kind: outline.Insert, Row: "b", Pos: "W"},
		{Kind: outline.Set, Row: "b", Field: outline.FieldText, Value: "Child"},
		{Kind: outline.Move, Row: "b", Parent: "a", Pos: "V"},
		{Kind: outline.Insert, Row: "c", Pos: "X"},
		{Kind: outline.Set, Row: "c", Field: outline.FieldDeleted, Value: "1"},
	}
	if err := s.Commit(ops, 1000); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFold("a", true); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tree, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tree.Children(""), ","); got != "a" {
		t.Fatalf("top level = %q", got)
	}
	if got := strings.Join(tree.Children("a"), ","); got != "b" {
		t.Fatalf("children of a = %q", got)
	}
	if tree.Row("b").Text() != "Child" || !tree.Row("c").Deleted() {
		t.Fatal("fields were not restored")
	}
	folds, err := s.Folds()
	if err != nil || !folds["a"] {
		t.Fatalf("folds = %v, %v", folds, err)
	}
}

func TestStampsStayOrderedAcrossReopenAndClockStepBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docket.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Commit([]outline.Op{{Kind: outline.Insert, Row: "a", Pos: "V"}}, 5000); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// The wall clock went backwards between sessions.
	if err := s.Commit([]outline.Op{{Kind: outline.Insert, Row: "b", Pos: "W"}}, 4000); err != nil {
		t.Fatal(err)
	}
	rs, err := s.db.Query(`SELECT stamp FROM ops ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	prev := ""
	for rs.Next() {
		var stamp string
		if err := rs.Scan(&stamp); err != nil {
			t.Fatal(err)
		}
		if stamp <= prev {
			t.Fatalf("stamp %q does not follow %q", stamp, prev)
		}
		prev = stamp
	}
}
