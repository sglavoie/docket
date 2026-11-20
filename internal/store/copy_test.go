package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/outline"
)

func TestCopyGoesUnderTheDeviceAndSkipsWhatIsUnchanged(t *testing.T) {
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
	cloud := filepath.Join(t.TempDir(), "Docket Backups")
	mine := filepath.Join(cloud, s.device)
	names := func() string {
		t.Helper()
		found, err := filepath.Glob(filepath.Join(mine, "*"))
		if err != nil {
			t.Fatal(err)
		}
		for i := range found {
			found[i] = filepath.Base(found[i])
		}
		return strings.Join(found, " ")
	}
	copyNow := func(day string, force bool) bool {
		t.Helper()
		job, err := s.CopyBegin(cloud, day, 2, force)
		if err != nil {
			t.Fatal(err)
		}
		if job == nil {
			return false
		}
		if err := s.CopyEnd(job.Run()); err != nil {
			t.Fatal(err)
		}
		return true
	}

	commit("a", "morning")
	if !copyNow("2026-11-15", false) {
		t.Fatal("a first copy was skipped")
	}
	if copyNow("2026-11-15", false) {
		t.Fatal("a copy was made though nothing changed")
	}
	if !copyNow("2026-11-15", true) {
		t.Fatal("a copy asked for was skipped")
	}
	// A later copy the same day replaces that day's.
	commit("b", "afternoon")
	if !copyNow("2026-11-15", false) {
		t.Fatal("a change was not copied")
	}
	snap, err := Open(filepath.Join(mine, "docket-2026-11-15.db"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := snap.Load()
	snap.Close()
	if err != nil {
		t.Fatal(err)
	}
	if tree.Row("b") == nil || tree.Row("b").Text() != "afternoon" {
		t.Fatal("the day's copy is not the database as it is now")
	}

	for _, day := range []string{"2026-11-16", "2026-11-17"} {
		commit("c"+day, day)
		copyNow(day, false)
	}
	if got := names(); got != "docket-2026-11-16.db docket-2026-11-17.db" {
		t.Fatalf("copies = %q", got)
	}
	// Nothing is left half written, here or beside the database.
	if left, _ := filepath.Glob(filepath.Join(dir, ".docket-copy-*")); len(left) > 0 {
		t.Fatalf("left beside the database: %v", left)
	}
	// Another folder is owed a copy even when nothing changed.
	cloud = filepath.Join(t.TempDir(), "elsewhere")
	mine = filepath.Join(cloud, s.device)
	if !copyNow("2026-11-17", false) {
		t.Fatal("a new folder was not given a copy")
	}
	if _, err := os.Stat(filepath.Join(mine, "docket-2026-11-17.db")); err != nil {
		t.Fatal(err)
	}
}
