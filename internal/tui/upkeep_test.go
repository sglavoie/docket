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
	if !strings.Contains(lap.m.View().Content, darkPalette.alert+"sync stuck") {
		t.Fatal("a stuck sync is not shown as an alarm")
	}
	// What came before the damage is here, and work goes on.
	lap.press("g", "g")
	lap.want("▌ · Alpha")
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
		"  · Alpha",
		"▌ · typed meanwhile",
	)
	// No second pass starts while the first is waiting.
	lap.m.Update(tickMsg{})
	if lap.m.syncHeld == nil {
		t.Fatal("the finished pass was dropped")
	}
	lap.press("esc")
	lap.want(
		"  · Alpha",
		"  · Beta",
		"▌ · typed meanwhile",
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
	// A day of its own, as the harness has, so no date sits across midnight
	// or a month's end from the clock the rows are drawn on.
	today := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
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
			// Deadlines spread around today, so a range filter keeps most
			// rows and has an order to sort into.
			set(id, "due", today.AddDate(0, 0, (i*7)%60-10).Format("2006-01-02"))
		}
	}
	if err := st.Commit(ops, today.UnixMilli()); err != nil {
		tb.Fatal(err)
	}
	tree, err := st.Load()
	if err != nil {
		tb.Fatal(err)
	}
	m := newModel(tree, st, nil, func() int64 { return today.UnixMilli() })
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// frameBudget is one frame at 60 a second. GitHub's shared runners, Linux
// and macOS alike, run these timings slower than any machine Docket is used
// on, so there alone it doubles; local runs keep the frame.
func frameBudget() time.Duration {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return 32 * time.Millisecond
	}
	return 16 * time.Millisecond
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
	budget := frameBudget()
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
	// n with no match further on reads every shown row, ancestors included.
	m.find = "^nowhere $open"
	if d := best("n"); d > budget {
		t.Errorf("finding the next match took %v", d)
	}
	if d := best("}"); d > budget {
		t.Errorf("moving to a sibling took %v", d)
	}
	frame(m, "o")
	if d := best("a"); d > budget {
		t.Errorf("typing a letter took %v", d)
	}
	// Preloading is outside the sample; every timed key still wraps and draws
	// the entire long buffer and scrolls its caret within the editing row.
	long := strings.Repeat("ordinary words and spaces ", 180)
	m.edit.buf = []rune(long)
	m.edit.caret = len(m.edit.buf)
	_, height := m.size()
	if len(m.edit.buf) < 4096 || len(m.edit.wrapped(m.editWidth(), nil).lines) <= height-2 {
		t.Fatal("long typing fixture must exceed both 4096 characters and the pane height")
	}
	if d := best("z"); d > budget {
		t.Errorf("typing in a long wrapped buffer took %v", d)
	}
	if string(m.edit.buf) != long+strings.Repeat("z", 20) {
		t.Fatal("timed letters were not inserted into the long buffer")
	}
	v := m.View()
	if v.Cursor == nil || v.Cursor.Y < 1 || v.Cursor.Y >= height-1 || v.Cursor.X < sideCols+4 || v.Cursor.X >= m.width {
		t.Fatalf("long typing caret is outside the visible pane: %+v", v.Cursor)
	}
	frame(m, "esc")
	frame(m, "m")
	if len(m.headings) == 0 {
		t.Fatal("move picker timing needs heading destinations")
	}
	if d := best("s"); d > budget {
		t.Errorf("typing a move-picker letter took %v", d)
	}
	frame(m, "esc")

	// The jump picker reads every row's text against each letter, and here
	// nearly every row holds it.
	frame(m, "f")
	if len(m.rowNames) < 9000 {
		t.Fatalf("jump picker timing needs every row offered, have %d", len(m.rowNames))
	}
	if d := best("s"); d > budget {
		t.Errorf("typing a jump-picker letter took %v", d)
	}
	if hits := m.offers(m.ask, "s"); len(hits) < 9000 {
		t.Fatalf("the timed letter matched only %d rows", len(hits))
	}
	frame(m, "esc")

	// Inside a row, a motion and a change each wrap and draw the line.
	frame(m, "w")
	if m.edit == nil || !m.edit.normal {
		t.Fatal("w did not step into the row")
	}
	if d := best("w"); d > budget {
		t.Errorf("a word motion inside a row took %v", d)
	}
	if d := best("~"); d > budget {
		t.Errorf("a change inside a row took %v", d)
	}
	frame(m, "esc")
	frame(m, "u")

	// A filter reads the whole outline again on each letter of its query.
	// retype times the query's last letter, put back after each sample.
	retype := func(last string) time.Duration {
		least := time.Hour
		for i := 0; i < 20; i++ {
			frame(m, "backspace")
			at := time.Now()
			frame(m, last)
			least = min(least, time.Since(at))
		}
		return least
	}
	filter := func(query string) {
		for _, r := range query {
			frame(m, string(r))
		}
	}

	// $next in context: each project's next action, under its project.
	frame(m, "/")
	filter("$next")
	if len(m.at.hits) == 0 {
		t.Fatal("$next timing needs next actions on screen")
	}
	if d := retype("t"); d > budget {
		t.Errorf("typing a $next filter took %v", d)
	}
	frame(m, "enter")
	if d := best("j"); d > budget {
		t.Errorf("moving the cursor under $next took %v", d)
	}

	// Ancestor text matches all project descendants in one contextual walk.
	frame(m, "/")
	frame(m, "esc")
	frame(m, "/")
	filter("^project")
	if len(m.at.matched) < 10000 {
		t.Fatalf("ancestor timing needs at least 10000 descendants, have %d", len(m.at.matched))
	}
	if d := retype("t"); d > budget {
		t.Errorf("typing an ancestor filter took %v", d)
	}
	frame(m, "enter")
	if d := best("j"); d > budget {
		t.Errorf("moving the cursor under an ancestor filter took %v", d)
	}

	// A deadline range, flat, so its matches are sorted soonest first.
	frame(m, "/")
	frame(m, "esc")
	frame(m, "/")
	frame(m, "ctrl+t")
	filter("@<+30d")
	if !m.at.flat || len(m.at.hits) < 1000 {
		t.Fatalf("sorted flat timing needs a flat list of many matches, have %d (flat %v)", len(m.at.hits), m.at.flat)
	}
	if d := retype("d"); d > budget {
		t.Errorf("typing a sorted flat filter took %v", d)
	}
	frame(m, "enter")
	if d := best("j"); d > budget {
		t.Errorf("moving the cursor in a sorted flat filter took %v", d)
	}

	// A term on the rows below walks the outline once more for its answer.
	frame(m, "/")
	frame(m, "esc")
	frame(m, "/")
	filter("_#alpha")
	if len(m.at.matched) != 40 {
		t.Fatalf("below timing needs every project to match, have %d", len(m.at.matched))
	}
	if d := retype("a"); d > budget {
		t.Errorf("typing a filter on the rows below took %v", d)
	}
	frame(m, "esc")
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

// Rule 5 holds in a body of thousands of lines: typing in it, moving in it,
// and moving among the rows below it while it is drawn.
func TestALongBodyPaintsWithinAFrame(t *testing.T) {
	if testing.Short() {
		t.Skip("timing")
	}
	budget := frameBudget()
	m := crowd(t, 10000)
	frame(m, "gg")
	frame(m, "enter")
	if m.bodyRow() == "" || len(m.vis) == 0 {
		t.Fatal("long body timing needs a project open on its rows")
	}
	lines := make([]string, 5000)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d of the notes, ordinary words and spaces to wrap", i)
	}
	if _, err := m.tree.Apply(outline.Op{Kind: outline.Set, Row: m.bodyRow(), Field: fieldBody, Value: strings.Join(lines, "\n")}, m.now()); err != nil {
		t.Fatal(err)
	}
	best := func(k string) time.Duration {
		least := time.Hour
		for i := 0; i < 20; i++ {
			at := time.Now()
			frame(m, k)
			least = min(least, time.Since(at))
		}
		return least
	}
	if d := best("j"); d > budget {
		t.Errorf("moving among the rows under a long body took %v", d)
	}
	frame(m, "gg")
	frame(m, "k")
	if m.edit == nil || !m.edit.body {
		t.Fatal("k did not step into the body")
	}
	if d := best("k"); d > budget {
		t.Errorf("moving in a long body took %v", d)
	}
	frame(m, "A")
	if d := best("a"); d > budget {
		t.Errorf("typing in a long body took %v", d)
	}
	frame(m, "esc")
	frame(m, "esc")
	if !strings.Contains(m.tree.Row(m.at.root).Fields[fieldBody], lines[4979]+strings.Repeat("a", 20)+"\n"+lines[4980]) {
		t.Fatal("the timed letters were not kept in the body")
	}
}
