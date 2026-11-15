package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyTo opens a copy of m's database at a new path, as a second machine
// set up from the first one's file would.
func (m *machine) copyTo(t *testing.T, shared string) *machine {
	t.Helper()
	if _, err := m.s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "docket.db")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c := &machine{t: t, s: s, path: path, dir: shared, now: m.now}
	t.Cleanup(func() { c.s.Close() })
	return c
}

func TestACopiedDatabaseSyncsUnderItsOwnName(t *testing.T) {
	shared := t.TempDir()
	a := newMachine(t, shared)
	a.do(insert("t", "", "V", "Task")...)
	a.sync()

	b := a.copyTo(t, shared)
	if !b.s.Renamed() || b.s.device == a.s.device {
		t.Fatalf("the copy kept the name %s", a.s.device)
	}
	if a.s.Renamed() {
		t.Fatal("the original was renamed")
	}

	// Both work on, each unseen by the other, and nothing is lost.
	a.do(insert("x", "", "W", "Made on the first")...)
	a.do(set("t", "flag", "1"))
	b.do(insert("y", "", "X", "Made on the copy")...)
	b.do(set("t", "state", "doing"))
	got := converge(t, a, b)
	for _, want := range []string{"flag=1", "state=doing", "Made on the first", "Made on the copy"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q is missing:\n%s", want, got)
		}
	}
	files, _ := filepath.Glob(filepath.Join(shared, "*.ops"))
	if len(files) != 2 {
		t.Fatalf("files = %v", files)
	}

	// Reopened where it is, the copy keeps the name it took.
	name := b.s.device
	b.s.Close()
	var err error
	if b.s, err = Open(b.path); err != nil {
		t.Fatal(err)
	}
	if b.s.Renamed() || b.s.device != name {
		t.Fatalf("renamed again: %s, was %s", b.s.device, name)
	}
}

func TestACopyWritesOutWhatItsOriginNeverSynced(t *testing.T) {
	// The first machine never synced and is gone. All it made must still
	// reach a third machine, through the copy.
	a := newMachine(t, t.TempDir())
	a.do(insert("t", "", "V", "Made before the move")...)
	shared := t.TempDir()
	b := a.copyTo(t, shared)
	b.do(insert("u", "", "W", "Made after it")...)
	c := newMachine(t, shared)
	if got, want := converge(t, b, c), "t text=Made before the move\nu text=Made after it\n"; got != want {
		t.Fatalf("got:\n%s", got)
	}
	// What the copy inherited is written once, not at every pass.
	path := filepath.Join(shared, b.s.device+".ops")
	before, _ := os.ReadFile(path)
	b.sync()
	if after, _ := os.ReadFile(path); len(after) != len(before) {
		t.Fatalf("the file grew from %d to %d bytes with nothing new", len(before), len(after))
	}
}

func TestADatabaseOnAnotherMachineAtTheSamePathIsACopy(t *testing.T) {
	a := newMachine(t, t.TempDir())
	name := a.s.device
	a.s.Close()
	real := machineID
	machineID = func() string { return "another computer" }
	t.Cleanup(func() { machineID = real })
	var err error
	if a.s, err = Open(a.path); err != nil {
		t.Fatal(err)
	}
	if !a.s.Renamed() || a.s.device == name {
		t.Fatal("the database kept its name on another machine")
	}
}

// pass is one sync pass, with its report.
func (m *machine) pass() SyncReport {
	m.t.Helper()
	job, err := m.s.SyncBegin(m.dir)
	if err != nil {
		m.t.Fatal(err)
	}
	report, err := m.s.SyncEnd(job.Run())
	if err != nil {
		m.t.Fatal(err)
	}
	return report
}

func TestSyncSaysWhichFileCannotBeRead(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	a.do(insert("t", "", "V", "Task")...)
	a.sync()
	path := filepath.Join(shared, a.s.device+".ops")
	whole, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a.do(set("t", "flag", "1"))
	a.sync()
	later, _ := os.ReadFile(path)
	// The file arrives with a damaged line in its middle.
	damaged := string(whole) + "{\"s\": not json\n" + string(later[len(whole):])
	if err := os.WriteFile(path, []byte(damaged), 0o600); err != nil {
		t.Fatal(err)
	}

	// The first pass waits: a file being copied in can look like this.
	if r := b.pass(); len(r.Stuck) != 0 || r.Machines != 1 {
		t.Fatalf("report = %+v", r)
	}
	if got := b.shape(); got != "t text=Task\n" {
		t.Fatalf("got:\n%q", got)
	}
	// The second names the file and the line, and applies nothing past it.
	r := b.pass()
	if len(r.Stuck) != 1 || r.Stuck[0].File != a.s.device+".ops" || r.Stuck[0].Line != 3 {
		t.Fatalf("report = %+v", r)
	}
	if got := b.shape(); got != "t text=Task\n" {
		t.Fatalf("got:\n%q", got)
	}
	// Once the file is whole the rest arrives and the report clears.
	if err := os.WriteFile(path, later, 0o600); err != nil {
		t.Fatal(err)
	}
	if r := b.pass(); len(r.Stuck) != 0 || r.Changed != 1 {
		t.Fatalf("report = %+v", r)
	}
	if got := b.shape(); got != "t flag=1 text=Task\n" {
		t.Fatalf("got:\n%q", got)
	}
}

func TestSyncReadsAReplacedFileFromTheTop(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	a.do(insert("t", "", "V", "A long enough title to move the place well into the file")...)
	a.sync()
	b.sync()
	// The file is replaced by a shorter one holding something else.
	c := newMachine(t, t.TempDir())
	c.do(insert("u", "", "W", "Other")...)
	c.sync()
	other, err := os.ReadFile(filepath.Join(c.dir, c.s.device+".ops"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shared, a.s.device+".ops"), other, 0o600); err != nil {
		t.Fatal(err)
	}
	b.sync()
	if got := b.shape(); !strings.Contains(got, "u text=Other") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestDailyBackupSitsBesideTheDatabase(t *testing.T) {
	a := newMachine(t, t.TempDir())
	if err := a.s.DailyBackup("2026-11-15"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.path), "backups", "docket-2026-11-15.db")); err != nil {
		t.Fatal(err)
	}
}

func TestSyncCutsALineItNeverFinishedWriting(t *testing.T) {
	shared := t.TempDir()
	a, b := newMachine(t, shared), newMachine(t, shared)
	a.do(insert("t", "", "V", "Task")...)
	a.sync()
	// The machine died part way through a later write.
	path := filepath.Join(shared, a.s.device+".ops")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"s":"0000000001020-0000-`)
	f.Close()
	a.do(set("t", "flag", "1"))
	if got := converge(t, a, b); got != "t flag=1 text=Task\n" {
		t.Fatalf("got:\n%q", got)
	}
	if r := b.pass(); len(r.Stuck) != 0 {
		t.Fatalf("report = %+v", r)
	}
	if r := b.pass(); len(r.Stuck) != 0 {
		t.Fatalf("report = %+v", r)
	}
}
