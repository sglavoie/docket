package store

import (
	"os"
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

func TestBackupIsDailyWholeAndPruned(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "docket.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	commit := func(id, text string) {
		t.Helper()
		ops := []outline.Op{
			{Kind: outline.Insert, Row: id, Pos: "V"},
			{Kind: outline.Set, Row: id, Field: outline.FieldText, Value: text},
		}
		if err := s.Commit(ops, 1000); err != nil {
			t.Fatal(err)
		}
	}
	backups := filepath.Join(dir, "backups")
	names := func() string {
		t.Helper()
		found, err := filepath.Glob(filepath.Join(backups, "*"))
		if err != nil {
			t.Fatal(err)
		}
		for i := range found {
			found[i] = filepath.Base(found[i])
		}
		return strings.Join(found, " ")
	}

	commit("a", "first day")
	if err := s.Backup(backups, "2026-11-15", 2); err != nil {
		t.Fatal(err)
	}
	// A second launch the same day leaves that day's snapshot alone.
	commit("b", "later the same day")
	if err := s.Backup(backups, "2026-11-15", 2); err != nil {
		t.Fatal(err)
	}
	snap, err := Open(filepath.Join(backups, "docket-2026-11-15.db"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := snap.Load()
	snap.Close()
	if err != nil {
		t.Fatal(err)
	}
	if tree.Row("a") == nil || tree.Row("a").Text() != "first day" || tree.Row("b") != nil {
		t.Fatal("the snapshot is not the database as it was at the first launch of the day")
	}

	for _, day := range []string{"2026-11-16", "2026-11-17"} {
		if err := s.Backup(backups, day, 2); err != nil {
			t.Fatal(err)
		}
	}
	if got := names(); got != "docket-2026-11-16.db docket-2026-11-17.db" {
		t.Fatalf("backups = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "docket.db")); err != nil {
		t.Fatal(err)
	}
}

func TestTwoWindowsOnOneDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docket.db")
	open := func() *Store {
		t.Helper()
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		if _, err := s.Load(); err != nil {
			t.Fatal(err)
		}
		return s
	}
	one, two := open(), open()
	stale := func(s *Store) bool {
		t.Helper()
		v, err := s.Stale()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if stale(one) || stale(two) {
		t.Fatal("a fresh window is stale")
	}

	// One window writes; the other is told, and is not the one that wrote.
	if err := one.Commit([]outline.Op{
		{Kind: outline.Insert, Row: "a", Pos: "V"},
		{Kind: outline.Set, Row: "a", Field: "text", Value: "from one"},
	}, 9000); err != nil {
		t.Fatal(err)
	}
	if stale(one) || !stale(two) {
		t.Fatal("the wrong window is stale")
	}

	// The other writes without looking first, with its wall clock behind.
	// Its write is still the newer one, and it still knows it must look.
	if err := two.Commit([]outline.Op{{Kind: outline.Set, Row: "a", Field: "text", Value: "from two"}}, 100); err != nil {
		t.Fatal(err)
	}
	if !stale(two) {
		t.Fatal("writing hid the other window's change")
	}
	tree, err := two.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stale(two) || tree.Row("a").Text() != "from two" {
		t.Fatalf("after a fresh read: stale=%v text=%q", stale(two), tree.Row("a").Text())
	}
	if !stale(one) {
		t.Fatal("the first window was not told")
	}

	rs, err := one.db.Query(`SELECT stamp FROM ops ORDER BY seq`)
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
