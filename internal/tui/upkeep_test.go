package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/store"
)

func TestASessionLeftOpenTakesEachDaysBackup(t *testing.T) {
	h := work(t)
	backups := filepath.Join(filepath.Dir(h.path), "backups")
	h.tick()
	if got, _ := filepath.Glob(filepath.Join(backups, "*.db")); len(got) != 0 {
		t.Fatalf("a snapshot was taken on the day of the launch, which has its own: %v", got)
	}
	h.clock = h.clock.Add(24 * time.Hour)
	h.tick()
	h.clock = h.clock.Add(24 * time.Hour)
	h.tick()
	h.tick()
	got, _ := filepath.Glob(filepath.Join(backups, "*.db"))
	if len(got) != 2 || filepath.Base(got[0]) != "docket-2026-11-16.db" || filepath.Base(got[1]) != "docket-2026-11-17.db" {
		t.Fatalf("snapshots = %v", got)
	}
}

func TestAStuckSyncFileIsNamed(t *testing.T) {
	shared := t.TempDir()
	desk := open(t, filepath.Join(t.TempDir(), "desk.db")).wide()
	lap := open(t, filepath.Join(t.TempDir(), "lap.db")).wide()
	desk.press(":", `"sync `+shared+`"`, "enter")
	desk.press("o", `"Alpha"`, "esc")
	desk.tick()
	files, _ := filepath.Glob(filepath.Join(shared, "*.ops"))
	if len(files) != 1 {
		t.Fatalf("files = %v", files)
	}
	f, err := os.OpenFile(files[0], os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, "damaged in transit")
	f.Close()
	desk.press("o", `"Beta"`, "esc")
	desk.tick()

	// One pass waits, since a file may be seen half copied.
	lap.press(":", `"sync `+shared+`"`, "enter")
	if lap.m.syncFailed {
		t.Fatalf("said at once: %q", lap.m.msg)
	}
	lap.tick()
	name := filepath.Base(files[0])
	if !strings.HasPrefix(lap.m.msg, "sync stuck: line ") || !strings.Contains(lap.m.msg, " of "+name+" cannot be read") {
		t.Fatalf("msg = %q", lap.m.msg)
	}
	if !strings.Contains(lap.m.View().Content, sgrAlert+"sync stuck") {
		t.Fatal("a stuck sync is not shown as an alarm")
	}
	// What came before the damage is here, and work goes on.
	lap.press("g", "g")
	lap.want("▌   Alpha")
}

func TestASyncThatEndsWhileTypingWaitsForTheRow(t *testing.T) {
	shared := t.TempDir()
	desk := open(t, filepath.Join(t.TempDir(), "desk.db")).wide()
	lap := open(t, filepath.Join(t.TempDir(), "lap.db")).wide()
	desk.press(":", `"sync `+shared+`"`, "enter", "o", `"Alpha"`, "esc")
	desk.tick()
	lap.press(":", `"sync `+shared+`"`, "enter")
	desk.press("o", `"Beta"`, "esc")
	desk.tick()

	// The pass starts, and a row is begun before the folder answers.
	_, cmd := lap.m.Update(tickMsg{})
	if cmd == nil || lap.m.syncIO == nil {
		t.Fatal("the tick started no pass")
	}
	work := lap.m.syncIO
	lap.m.Update(key("o"))
	for _, r := range "typed meanwhile" {
		lap.m.Update(key(string(r)))
	}
	lap.m.Update(work())
	if lap.m.edit == nil || string(lap.m.edit.buf) != "typed meanwhile" {
		t.Fatal("the pass interrupted typing")
	}
	lap.want(
		"    Alpha",
		"▌   typed meanwhile",
	)
	// No second pass starts while the first is waiting.
	lap.m.Update(tickMsg{})
	if lap.m.syncHeld == nil {
		t.Fatal("the finished pass was dropped")
	}
	lap.press("esc")
	lap.want(
		"    Alpha",
		"    Beta",
		"▌   typed meanwhile",
	)
}

// crowd fills a database with n rows across forty projects.
func crowd(tb testing.TB, n int) *Model {
	tb.Helper()
	st, err := store.Open(filepath.Join(tb.TempDir(), "docket.db"))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { st.Close() })
	var ops []outline.Op
	set := func(id, field, value string) {
		ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: field, Value: value})
	}
	top := ""
	for p := 0; p < 40; p++ {
		project := fmt.Sprintf("p%02d", p)
		top = outline.Between(top, "")
		ops = append(ops, outline.Op{Kind: outline.Insert, Row: project, Pos: top})
		set(project, "text", "Project "+project)
		pos := ""
		for i := 0; i < n/40; i++ {
			id := fmt.Sprintf("%s-%d", project, i)
			pos = outline.Between(pos, "")
			ops = append(ops, outline.Op{Kind: outline.Insert, Row: id, Parent: project, Pos: pos})
			title := "A task with a title of ordinary length"
			if i%10 == 0 {
				title = "Storage:"
			}
			set(id, "text", title)
			set(id, "state", "open")
			set(id, "tags", "alpha beta")
			set(id, "due", "2026-12-01")
		}
	}
	if err := st.Commit(ops, time.Now().UnixMilli()); err != nil {
		tb.Fatal(err)
	}
	tree, err := st.Load()
	if err != nil {
		tb.Fatal(err)
	}
	m := New(tree, st, nil)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// frame is one key and the drawing that follows it.
func frame(m *Model, k string) {
	m.Update(key(k))
	m.View()
}

// Rule 5: every keystroke paints within one frame. The outline here is far
// larger than one person's, and the budget is a frame at 60 a second, taken
// as the best of several so a busy machine does not fail it.
func TestAKeystrokePaintsWithinAFrame(t *testing.T) {
	if testing.Short() {
		t.Skip("timing")
	}
	const budget = 16 * time.Millisecond
	m := crowd(t, 10000)
	best := func(k string) time.Duration {
		least := time.Hour
		for i := 0; i < 20; i++ {
			if m.ask != nil {
				// Keep every sample matching headings, rather than measuring
				// an ever-longer query with no remaining matches.
				m.ask.ed = editor{}
			}
			at := time.Now()
			frame(m, k)
			least = min(least, time.Since(at))
		}
		return least
	}
	if d := best("j"); d > budget {
		t.Errorf("moving the cursor took %v", d)
	}
	frame(m, "o")
	if d := best("a"); d > budget {
		t.Errorf("typing a letter took %v", d)
	}
	frame(m, "esc")
	frame(m, "m")
	if len(m.headings) == 0 {
		t.Fatal("move picker timing needs heading destinations")
	}
	if d := best("s"); d > budget {
		t.Errorf("typing a move-picker letter took %v", d)
	}
}

func BenchmarkKeys(b *testing.B) {
	for _, n := range []int{5000, 50000} {
		m := crowd(b, n)
		b.Run(fmt.Sprintf("move/%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				frame(m, "j")
			}
		})
		frame(m, "o")
		b.Run(fmt.Sprintf("type/%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				frame(m, "a")
			}
		})
		frame(m, "esc")
		b.Run(fmt.Sprintf("close/%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				frame(m, "x")
			}
		})
	}
}
