package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
	"github.com/sglavoie/docket/internal/store"
)

// now is Friday 2026-11-20 at noon, in UTC, so the window runs from
// Saturday 2026-10-24 to today.
var now = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

// at is a time on a day of November 2026.
func at(day, hour int) int64 {
	return time.Date(2026, 11, day, hour, 0, 0, 0, time.UTC).UnixMilli()
}

const secret = "secret-row-text"

type fixture struct {
	t *testing.T
	s *store.Store
}

func (f fixture) commit(ms int64, ops ...outline.Op) {
	f.t.Helper()
	if err := f.s.Commit(ops, ms); err != nil {
		f.t.Fatal(err)
	}
}

func insert(id, parent, text string) []outline.Op {
	return []outline.Op{
		{Kind: outline.Insert, Row: id, Parent: parent, Pos: "m" + id},
		{Kind: outline.Set, Row: id, Field: sigil.FieldText, Value: secret + " " + text},
	}
}

func set(id, field, value string) outline.Op {
	return outline.Op{Kind: outline.Set, Row: id, Field: field, Value: value}
}

// build writes a month of use through the store, at set times:
//
//   - Nov 1: the Inbox, the filters, projects P (active) and Q (parked), and
//     an Inbox row left unfiled.
//   - Nov 5: an import of 600 rows under P.
//   - Nov 10: a picks a row in the Inbox and closes it; b is picked under P
//     and left open; c is added under P with a child, and both are deleted
//     the next day; d goes done and is reopened by an undo.
//   - Nov 12: b, carried two days, is closed.
//   - Nov 15: nothing.
//   - Nov 18: e goes doing, with an estimate.
func build(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docket.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{t, s}

	f.commit(at(1, 9),
		outline.Op{Kind: outline.Insert, Row: inboxID, Pos: "0"},
		outline.Op{Kind: outline.Insert, Row: filtersID, Pos: "0"},
		outline.Op{Kind: outline.Insert, Row: "saved", Parent: filtersID, Pos: "a"})
	f.commit(at(1, 10), append(insert("P", "", "project"), insert("Q", "", "parked")...)...)
	f.commit(at(1, 10), set("Q", sigil.FieldState, sigil.Parked))
	f.commit(at(1, 11), append(insert("unfiled", inboxID, "unfiled"), set("unfiled", sigil.FieldState, sigil.Open))...)

	var bulk []outline.Op
	for i := range 600 {
		bulk = append(bulk, outline.Op{Kind: outline.Insert, Row: "bulk" + strconv.Itoa(i), Parent: "P", Pos: "b"})
	}
	f.commit(at(5, 9), bulk...)

	f.commit(at(10, 8), append(insert("a", inboxID, "a"), set("a", sigil.FieldState, sigil.Open))...)
	f.commit(at(10, 8), append(insert("b", "P", "b"), set("b", sigil.FieldState, sigil.Open))...)
	f.commit(at(10, 9), set("a", sigil.FieldToday, "2026-11-10"), set("b", sigil.FieldToday, "2026-11-10"))
	f.commit(at(10, 15), set("a", sigil.FieldState, sigil.Done))
	f.commit(at(10, 16), append(insert("c", "P", "c"), insert("c1", "c", "c1")...)...)
	f.commit(at(10, 16), append(insert("d", "P", "d"), set("d", sigil.FieldState, sigil.Open))...)
	f.commit(at(10, 17), set("d", sigil.FieldState, sigil.Done))
	f.commit(at(10, 17), set("d", sigil.FieldState, sigil.Open))
	f.commit(at(11, 9), set("c", outline.FieldDeleted, "1"))
	f.commit(at(12, 14), set("b", sigil.FieldState, sigil.Done))

	f.commit(at(18, 9), append(insert("e", "P", "e"),
		set("e", sigil.FieldState, sigil.Doing), set("e", sigil.FieldTags, "est=2h deep"))...)

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func dayOf(t *testing.T, r *report, name string) []int {
	t.Helper()
	for _, d := range r.days {
		if d.start.Format(dayFormat) == name {
			return r.figures(d)
		}
	}
	t.Fatalf("no day %s in the window", name)
	return nil
}

func TestStatsReadsEachDayFromTheLog(t *testing.T) {
	path := build(t)
	db, err := openReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r, err := collect(db, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.days) != 28 || r.days[0].start.Format(dayFormat) != "2026-10-24" || r.days[27].start.Format(dayFormat) != "2026-11-20" {
		t.Fatalf("window runs %s to %s over %d days", r.days[0].start, r.days[len(r.days)-1].start, len(r.days))
	}

	//               done picks closed carried added inbox
	for _, tc := range []struct {
		name string
		day  string
		want []int
	}{
		// The fixed rows and the saved filter are not added rows.
		{"set-up", "2026-11-01", []int{0, 0, 0, 0, 3, 1}},
		// Two picks, one closed the same day; d's undone close is no close;
		// c and its child, deleted later, are left out.
		{"same-day close", "2026-11-10", []int{1, 2, 1, 0, 3, 1}},
		{"carried one day", "2026-11-11", []int{0, 0, 0, 1, 0, 0}},
		{"carried two days and closed", "2026-11-12", []int{1, 0, 0, 1, 0, 0}},
		{"no longer carried", "2026-11-13", []int{0, 0, 0, 0, 0, 0}},
		{"empty", "2026-11-15", []int{0, 0, 0, 0, 0, 0}},
		{"import", "2026-11-05", []int{0, 0, 0, 0, 600, 0}},
	} {
		got := dayOf(t, r, tc.day)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s (%s): got %v, want %v", tc.name, tc.day, got, tc.want)
		}
	}

	byDay := map[string]*day{}
	for _, d := range r.days {
		byDay[d.start.Format(dayFormat)] = d
	}
	if !byDay["2026-11-05"].imported() || byDay["2026-11-10"].imported() {
		t.Error("only the day of 600 inserts is an import")
	}
	if byDay["2026-11-15"].ops != 0 || byDay["2026-11-10"].ops == 0 {
		t.Error("the empty day has no changes, and Nov 10 has some")
	}

	if r.inboxLeft != 1 || r.inboxOldest != 19 {
		t.Errorf("Inbox: %d left, oldest %d days; want 1 and 19", r.inboxLeft, r.inboxOldest)
	}
	if r.projects != 1 || r.doing != 1 || r.estimated != 1 {
		t.Errorf("projects %d, doing %d, estimated %d; want 1, 1, 1", r.projects, r.doing, r.estimated)
	}
}

func TestStatsPrintsCountsOnlyAndWritesNothing(t *testing.T) {
	path := build(t)
	before := snapshot(t, filepath.Dir(path))

	var out bytes.Buffer
	if err := run(path, now, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, secret) {
		t.Errorf("a row's text was printed:\n%s", text)
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	// A header, 28 days, the medians, a blank line and four lines below.
	if len(lines) != 1+28+1+1+4 {
		t.Fatalf("%d lines:\n%s", len(lines), text)
	}
	for _, want := range []string{
		"2026-10-24 Sat",
		"2026-11-20 Fri",
		"import, not in the medians",
		"Inbox: 1 not yet filed, the oldest 19 days old",
		"Active projects: 1",
		"Doing: 1",
		"Open tasks with #est: 1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
	// Of the 27 days counted, Nov 1, 10, 11, 12 and 18 changed the outline.
	if med := lines[29]; !strings.HasPrefix(med, "median") || !strings.HasSuffix(med, "5 of 27 days") {
		t.Errorf("medians line: %q", med)
	}

	if after := snapshot(t, filepath.Dir(path)); after != before {
		t.Errorf("the database folder changed:\n%s\n%s", before, after)
	}
}

func TestStatsNeverCreatesADatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.db")
	if err := run(path, now, &bytes.Buffer{}); err == nil {
		t.Fatal("a missing database was opened")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a missing database was created: %v", err)
	}
}

func TestStatsRefusesWrites(t *testing.T) {
	db, err := openReadOnly(build(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM ops`); err == nil {
		t.Fatal("the connection accepted a write")
	}
}

func TestMedian(t *testing.T) {
	for _, tc := range []struct {
		in   []int
		want string
	}{{nil, "-"}, {[]int{3, 1, 2}, "2"}, {[]int{4, 1, 2, 3}, "2.5"}, {[]int{0, 0, 7, 0}, "0"}} {
		if got := median(tc.in); got != tc.want {
			t.Errorf("median(%v) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

// snapshot names every file in a folder with a hash of its contents.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s %d %x\n", e.Name(), len(data), sha256.Sum256(data))
	}
	return b.String()
}

// A docket still open on the database holds recent changes in its log; they
// are read, and the log is left as it was.
func TestStatsReadsWhatAnOpenDocketHasNotCheckpointed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docket.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f := fixture{t, s}
	f.commit(at(20, 9), outline.Op{Kind: outline.Insert, Row: inboxID, Pos: "0"})
	f.commit(at(20, 10), insert("x", inboxID, "x")...)
	if _, err := os.Stat(path + "-wal"); err != nil {
		t.Fatalf("no log beside an open database: %v", err)
	}
	var out bytes.Buffer
	if err := run(path, now, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Inbox: 1 not yet filed, the oldest 0 days old") {
		t.Errorf("the open docket's changes were not read:\n%s", out.String())
	}
	f.commit(at(20, 11), set("x", sigil.FieldState, sigil.Doing))
}
