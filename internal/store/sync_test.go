package store

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/outline"
)

type machine struct {
	t    *testing.T
	s    *Store
	path string
	dir  string
	now  int64
}

func newMachine(t *testing.T, shared string) *machine {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docket.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m := &machine{t: t, s: s, path: path, dir: shared, now: 1000}
	t.Cleanup(func() { m.s.Close() })
	return m
}

func (m *machine) do(ops ...outline.Op) {
	m.t.Helper()
	m.now += 10
	if err := m.s.Commit(ops, m.now); err != nil {
		m.t.Fatal(err)
	}
}

func (m *machine) sync() int {
	m.t.Helper()
	n, err := m.s.Sync(m.dir)
	if err != nil {
		m.t.Fatal(err)
	}
	return n
}

// shape is the whole outline as text: each live row under its parent, with
// its fields, so two machines can be compared.
func (m *machine) shape() string {
	m.t.Helper()
	tree, err := m.s.Load()
	if err != nil {
		m.t.Fatal(err)
	}
	var b strings.Builder
	var walk func(parent, indent string)
	walk = func(parent, indent string) {
		for _, id := range tree.Children(parent) {
			r := tree.Row(id)
			var fields []string
			for k, v := range r.Fields {
				fields = append(fields, k+"="+v)
			}
			sort.Strings(fields)
			b.WriteString(indent + id + " " + strings.Join(fields, " ") + "\n")
			walk(id, indent+"  ")
		}
	}
	walk("", "")
	return b.String()
}

func insert(id, parent, pos, text string) []outline.Op {
	return []outline.Op{
		{Kind: outline.Insert, Row: id, Parent: parent, Pos: pos},
		{Kind: outline.Set, Row: id, Field: "text", Value: text},
	}
}

func set(id, field, value string) outline.Op {
	return outline.Op{Kind: outline.Set, Row: id, Field: field, Value: value}
}

func converge(t *testing.T, machines ...*machine) string {
	t.Helper()
	for range 3 {
		for _, m := range machines {
			m.sync()
		}
	}
	want := machines[0].shape()
	for _, m := range machines[1:] {
		if got := m.shape(); got != want {
			t.Fatalf("machines disagree:\n%s\nand:\n%s", want, got)
		}
	}
	for _, m := range machines {
		if n := m.sync(); n != 0 {
			t.Fatalf("a settled sync applied %d operations", n)
		}
	}
	return want
}

func TestSyncCarriesWorkBetweenMachines(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	a.do(insert("p", "", "V", "Project")...)
	a.do(insert("t", "p", "V", "Task")...)
	a.sync()
	if n := b.sync(); n != 4 {
		t.Fatalf("b applied %d operations, want 4", n)
	}

	// Different fields of one row, changed on each machine before either
	// has seen the other: both changes stand.
	a.do(set("t", "text", "Task, renamed"))
	b.do(set("t", "state", "done"))
	got := converge(t, a, b)
	want := "p text=Project\n  t state=done text=Task, renamed\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSyncSameFieldTakesTheLaterWrite(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	a.do(insert("t", "", "V", "Task")...)
	converge(t, a, b)

	a.now, b.now = 5000, 6000
	a.do(set("t", "text", "from a, earlier"))
	b.do(set("t", "text", "from b, later"))
	if got := converge(t, a, b); got != "t text=from b, later\n" {
		t.Fatalf("got:\n%s", got)
	}
	// What a machine writes after a sync is newer than all it has seen,
	// even with its wall clock behind.
	a.now = 100
	a.do(set("t", "text", "a again"))
	if got := converge(t, a, b); got != "t text=a again\n" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestSyncOrdersWritesMadeInTheSameMillisecond(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	// Many operations in one millisecond on a, then one on b at that same
	// millisecond after b has seen them: b's is still the later write.
	ops := insert("t", "", "V", "Task")
	for range 20 {
		ops = append(ops, set("t", "state", "open"))
	}
	if err := a.s.Commit(ops, 7000); err != nil {
		t.Fatal(err)
	}
	a.sync()
	b.sync()
	if err := b.s.Commit([]outline.Op{set("t", "state", "done")}, 7000); err != nil {
		t.Fatal(err)
	}
	if got := converge(t, a, b); got != "t state=done text=Task\n" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestSyncDeletesAndMovesPerRow(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	a.do(insert("p", "", "V", "P")...)
	a.do(insert("q", "", "W", "Q")...)
	a.do(insert("t", "p", "V", "T")...)
	converge(t, a, b)

	a.do(outline.Op{Kind: outline.Move, Row: "t", Parent: "q", Pos: "V"})
	b.do(set("t", "flag", "1"))
	b.do(set("p", "deleted", "1"))
	if got := converge(t, a, b); got != "q text=Q\n  t flag=1 text=T\n" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestSyncBreaksAMoveLoopTheSameWayEverywhere(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	a.do(insert("x", "", "V", "X")...)
	a.do(insert("y", "", "W", "Y")...)
	converge(t, a, b)

	// Each is valid alone; together x and y are each other's parent.
	a.now, b.now = 5000, 6000
	a.do(outline.Op{Kind: outline.Move, Row: "x", Parent: "y", Pos: "V"})
	b.do(outline.Op{Kind: outline.Move, Row: "y", Parent: "x", Pos: "V"})
	// The row that moved last is shown at the top; nothing is lost.
	if got := converge(t, a, b); got != "y text=Y\n  x text=X\n" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestSyncWaitsForARowThatHasNotArrived(t *testing.T) {
	shared, lagging := t.TempDir(), t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	c := newMachine(t, lagging)
	a.do(insert("t", "", "V", "Task")...)
	a.sync()
	b.sync()
	b.do(set("t", "state", "doing"))
	b.do(insert("n", "t", "V", "a note under it")...)
	b.sync()

	// c's folder has b's file and not yet a's, as a file syncer may deliver.
	copyFile := func(name string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(shared, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lagging, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	copyFile(b.s.device + ".ops")
	c.sync()
	if got := c.shape(); got != "" {
		t.Fatalf("c shows rows whose parent it has not received:\n%s", got)
	}
	copyFile(a.s.device + ".ops")
	c.sync()
	if got, want := c.shape(), b.shape(); got != want {
		t.Fatalf("c:\n%s\nb:\n%s", got, want)
	}
}

func TestSyncIgnoresALineStillBeingCopied(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	a.do(insert("t", "", "V", "Task")...)
	a.sync()
	path := filepath.Join(shared, a.s.device+".ops")
	whole, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Only the first line and half of the second have arrived.
	cut := strings.Index(string(whole), "\n") + 1
	if err := os.WriteFile(path, whole[:cut+10], 0o600); err != nil {
		t.Fatal(err)
	}
	b.sync()
	if got := b.shape(); got != "t \n" {
		t.Fatalf("got:\n%q", got)
	}
	if err := os.WriteFile(path, whole, 0o600); err != nil {
		t.Fatal(err)
	}
	b.sync()
	if got := b.shape(); got != "t text=Task\n" {
		t.Fatalf("got:\n%q", got)
	}
}

func TestSyncRewritesItsFileIfTheFolderIsNew(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	a := newMachine(t, first)
	a.do(insert("t", "", "V", "Task")...)
	a.sync()
	a.dir = second
	a.sync()
	b := newMachine(t, second)
	b.sync()
	if got := b.shape(); got != "t text=Task\n" {
		t.Fatalf("got:\n%q", got)
	}
}

func TestSyncGivesBackWhatARestoredBackupLacks(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	a.do(insert("t", "", "V", "Task")...)
	converge(t, a, b)
	backups := filepath.Join(filepath.Dir(a.path), "backups")
	if err := a.s.Backup(backups, "2026-11-15", 3); err != nil {
		t.Fatal(err)
	}
	a.do(set("t", "text", "Task, edited after the backup"))
	a.do(insert("u", "", "W", "Made after the backup")...)
	want := converge(t, a, b)

	// The database is replaced by the snapshot.
	a.s.Close()
	snapshot, err := os.ReadFile(filepath.Join(backups, "docket-2026-11-15.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		os.Remove(a.path + suffix)
	}
	if err := os.WriteFile(a.path, snapshot, 0o600); err != nil {
		t.Fatal(err)
	}
	if a.s, err = Open(a.path); err != nil {
		t.Fatal(err)
	}
	if got := a.shape(); got != "t text=Task\n" {
		t.Fatalf("the restore did not take:\n%s", got)
	}
	if got := converge(t, a, b); got != want {
		t.Fatalf("after restore and sync:\n%s\nwant:\n%s", got, want)
	}
	// And work goes on from there.
	a.now = 9000
	a.do(set("u", "flag", "1"))
	if got := converge(t, a, b); !strings.Contains(got, "u flag=1") {
		t.Fatalf("got:\n%s", got)
	}
}
