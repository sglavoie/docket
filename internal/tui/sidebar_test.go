package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/store"
)

// two is a wide screen over two projects: Beta, empty, above Alpha, which
// holds a task due tomorrow and one waiting.
func two(t *testing.T) *harness {
	h := start(t).wide()
	h.press("o", `"Alpha"`, "enter", "tab", `"One @tom"`, "enter", `"Two $w"`, "esc")
	h.press("g", "g", "O", `"Beta"`, "esc")
	h.want(
		"▌ · Beta",
		"  ▾ Alpha",
		"    ○ One @tomorrow",
		"    ◔ Two",
	)
	return h
}

func TestSidebarStatesWhatNeedsAttention(t *testing.T) {
	h := two(t)
	h.wantSide(
		"Inbox",
		"Today",
		"Waiting 1",
		"Stale",
		"▌ Projects",
		"Beta ∅",
		"Alpha @",
	)
	for _, l := range h.lines() {
		if w := ansi.StringWidth(l); w > 100 {
			t.Fatalf("line is %d columns wide: %q", w, l)
		}
	}

	// Fifteen days on, the open task is stale and overdue, and the project
	// says how long it has sat.
	h.clock = h.clock.Add(15 * 24 * time.Hour)
	h.press("j")
	h.wantSide(
		"Inbox",
		"Today",
		"Waiting 1",
		"Stale 1",
		"▌ Projects",
		"Beta ∅",
		"Alpha @ 15d",
	)
}

func TestSidebarFocusShowsEachDestination(t *testing.T) {
	h := two(t)
	h.press("tab", "j", "j")
	if h.header() != "Alpha  · 2 to do" {
		t.Fatalf("header = %q", h.header())
	}
	h.want(
		"▌ ○ One @tomorrow",
		"  ◔ Two",
	)
	h.press("k", "k", "k", "k")
	if h.header() != "Waiting  · 1 waiting" {
		t.Fatalf("header = %q", h.header())
	}
	h.want(h.edge("▌ ◔ Two · Alpha", "0d"))

	// Back in the pane, closing the row leaves it in the list it was found
	// in; the list is read again when reopened.
	h.press("enter", "x")
	h.want(h.edge("▌ ✓ Two · Alpha", "0d"))
	h.wantSide(
		"Inbox",
		"Today",
		"▌ Waiting",
		"Stale",
		"Projects",
		"Beta ∅",
		"Alpha @",
	)
	h.press("tab", "j", "k", "tab")
	h.want("  Nothing is waiting. Stamp a row $waiting to list it here.")
}

func TestAListHasNoStructure(t *testing.T) {
	h := two(t)
	h.press("f", `"wait"`, "enter")
	h.want(h.edge("▌ ◔ Two · Alpha", "0d"))
	for _, k := range []string{"o", "J", ">"} {
		h.act(k)
		if h.status() == "" {
			t.Fatalf("%s was not refused", k)
		}
		h.want(h.edge("▌ ◔ Two · Alpha", "0d"))
	}
	// Enter works inside the row, and Esc returns to the same list.
	h.press("enter")
	if h.header() != "Alpha › Two" {
		t.Fatalf("header = %q", h.header())
	}
	h.press("o", `"a detail $note"`, "esc", "esc")
	h.want(h.edge("▌ ◔ Two +1 · Alpha", "0d"))
}

func TestTodayIsPickedAndKeepsWhatIsLeft(t *testing.T) {
	h := two(t)
	h.press("j", "j", "t", "j", "t", "x")
	h.wantSide(
		"Inbox",
		"Today 1",
		"Waiting",
		"Stale",
		"▌ Projects",
		"Beta ∅",
		"Alpha @",
	)
	h.press("f", `"tod"`, "enter")
	h.want(
		"▌ ○ One @tomorrow · Alpha",
		"  ✓ Two · Alpha",
	)
	h.press("t")
	h.wantSide(
		"Inbox",
		"▌ Today ✓",
		"Waiting",
		"Stale",
		"Projects",
		"Beta ∅",
		"Alpha @",
	)

	// A launch with something picked opens on Today.
	h.press("t")
	h.st.Close()
	st, err := store.Open(h.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	again := resume(t, h.path, st, h.clock).wide()
	if again.header() != "Today  · 1 to do · 1 done" {
		t.Fatalf("launched on %q", again.header())
	}

	// The next morning the pick left undone is still in Today and says
	// since when; the one closed yesterday is not. Nothing was written.
	next := resume(t, h.path, st, h.clock.Add(24*time.Hour)).wide()
	if next.header() != "Today  · 1 to do" {
		t.Fatalf("launched on %q", next.header())
	}
	next.want("▌ ○ One @today · since Nov 15 · Alpha")
	next.wantSide(
		"Inbox",
		"▌ Today 1",
		"Waiting",
		"Stale",
		"Projects",
		"Beta ∅",
		"Alpha @ 1d",
	)
	// One t lets it go, and Today is empty from then on.
	next.press("t")
	next.want("▌ ○ One @today · Alpha")
	if next.m.tree.Row(next.m.current()).Fields["today"] != "" {
		t.Fatal("t did not unpick the carried row")
	}
	next.wantSide(
		"Inbox",
		"▌ Today",
		"Waiting",
		"Stale",
		"Projects",
		"Beta ∅",
		"Alpha @",
	)
}

func TestTodayIsReordered(t *testing.T) {
	three := func(t *testing.T, startDay bool) *harness {
		h := start(t).wide()
		h.press("o", `"Alpha"`, "enter", "tab", `"One"`, "enter", `"Two"`, "enter", `"Three"`, "esc")
		if startDay {
			h.press("o", `"Scheduled @@today"`, "esc", "o", `"Later @@today"`, "esc")
		}
		// Pick Three, One, Two, against their outline order.
		h.press("g", "g", "j", "j", "j", "t", "k", "k", "t", "j", "t", "f", `"today"`, "enter")
		return h
	}
	t.Run("picks moves undo and restart", func(t *testing.T) {
		h := three(t, false)
		h.want("▌ ○ Three · Alpha", "  ○ One · Alpha", "  ○ Two · Alpha")
		positions := map[string]string{}
		for _, id := range h.m.at.rows {
			positions[id] = h.m.tree.Row(id).Pos
		}
		h.press("j", "K")
		h.want("▌ ○ One · Alpha", "  ○ Three · Alpha", "  ○ Two · Alpha")
		h.press("u")
		h.want("  ○ Three · Alpha", "▌ ○ One · Alpha", "  ○ Two · Alpha")
		h.press("ctrl+r", "J")
		h.want("  ○ Three · Alpha", "▌ ○ One · Alpha", "  ○ Two · Alpha")
		h.press("J")
		h.want("  ○ Three · Alpha", "  ○ Two · Alpha", "▌ ○ One · Alpha")
		steps := len(h.m.undo)
		h.press("J")
		if len(h.m.undo) != steps {
			t.Fatal("moving past the end made an undo step")
		}
		h.press("f", `"alpha"`, "enter")
		h.want("▌ ○ One ★", "  ○ Two ★", "  ○ Three ★")
		for id, pos := range positions {
			if row := h.m.tree.Row(id); row.Pos != pos || row.Parent != h.m.at.root {
				t.Fatal("Today changed outline structure")
			}
		}
		h.press("f", `"today"`, "enter")
		h.want("  ○ Three · Alpha", "  ○ Two · Alpha", "▌ ○ One · Alpha")
		h.st.Close()
		again := open(t, h.path).wide()
		again.want("▌ ○ Three · Alpha", "  ○ Two · Alpha", "  ○ One · Alpha")
		again.press("G", "u")
		again.want("  ○ Three · Alpha", "▌ ○ One · Alpha", "  ○ Two · Alpha")
		again.press("ctrl+r")
		again.want("  ○ Three · Alpha", "  ○ Two · Alpha", "▌ ○ One · Alpha")
	})
	t.Run("start day rows initialize in one undo step", func(t *testing.T) {
		h := three(t, true)
		h.want("▌ ○ Three · Alpha", "  ○ One · Alpha", "  ○ Two · Alpha", "  ○ Scheduled · Alpha", "  ○ Later · Alpha")
		before := map[string]string{}
		for _, id := range h.m.at.rows {
			before[id] = h.m.tree.Row(id).Fields[fieldTodayPos]
		}
		steps := len(h.m.undo)
		h.press("G", "K")
		h.want("  ○ Three · Alpha", "  ○ One · Alpha", "  ○ Two · Alpha", "▌ ○ Later · Alpha", "  ○ Scheduled · Alpha")
		if len(h.m.undo) != steps+1 {
			t.Fatal("initializing and moving did not form one undo step")
		}
		for _, id := range h.m.at.rows {
			if h.m.tree.Row(id).Fields[fieldTodayPos] == "" {
				t.Fatal("a row was left without a place")
			}
		}
		h.press("u")
		h.want("  ○ Three · Alpha", "  ○ One · Alpha", "  ○ Two · Alpha", "  ○ Scheduled · Alpha", "▌ ○ Later · Alpha")
		for id, pos := range before {
			if h.m.tree.Row(id).Fields[fieldTodayPos] != pos {
				t.Fatal("undo did not restore the original place")
			}
		}
		h.press("ctrl+r", "J")
		h.want("  ○ Three · Alpha", "  ○ One · Alpha", "  ○ Two · Alpha", "  ○ Scheduled · Alpha", "▌ ○ Later · Alpha")
	})
	t.Run("group toggle retains earlier picks and replaces stale positions", func(t *testing.T) {
		h := three(t, false)
		threeID := h.m.current()
		old := h.m.tree.Row(threeID).Fields[fieldTodayPos]
		h.press("t")
		if h.m.tree.Row(threeID).Fields[fieldTodayPos] != old {
			t.Fatal("unpick cleared the stored position")
		}
		// A mixed selection picks only its unpicked member, at the end.
		h.press("v", "v", "t", "esc")
		h.want("  ○ One · Alpha", "▌ ○ Two · Alpha", "  ○ Three · Alpha")
		h.press("u")
		h.want("  ○ Three · Alpha", "  ○ One · Alpha", "▌ ○ Two · Alpha")
		if h.m.tree.Row(threeID).Fields[fieldTodayPos] != old || h.m.tree.Row(threeID).Fields["today"] != "" {
			t.Fatal("undo did not restore both pick fields")
		}
		// The next day keeps the picks left undone, in their order, below
		// the day's own picks.
		h.clock = h.clock.Add(24 * time.Hour)
		h.press("f", `"alpha"`, "enter", "g", "g", "j", "j", "t", "f", `"today"`, "enter")
		h.want("  ○ Three · Alpha", todayRule(h), "  ○ One · since Nov 15 · Alpha", "▌ ○ Two · since Nov 15 · Alpha")
		if h.m.tree.Row(threeID).Fields[fieldTodayPos] == old {
			t.Fatal("new day retained the old position")
		}
		h.press("g", "g", "v", "v", "v", "t")
		for _, id := range h.m.at.rows {
			if h.m.tree.Row(id).Fields["today"] != "" || h.m.tree.Row(id).Fields[fieldTodayPos] == "" {
				t.Fatal("group unpick did not retain positions")
			}
		}
	})
	t.Run("retained unpicked rows with equal positions", func(t *testing.T) {
		h := three(t, false)
		// Unpicked rows remain in the snapshot. Repicking can reuse their keys.
		h.press("G", "t", "k", "t", "t", "g", "g", "t", "t")
		h.want("  ○ One · Alpha", "  ○ Two · Alpha", "▌ ○ Three · Alpha")
		h.press("K")
		h.want("  ○ One · Alpha", "▌ ○ Three · Alpha", "  ○ Two · Alpha")
		h.press("u")
		h.want("  ○ One · Alpha", "  ○ Two · Alpha", "▌ ○ Three · Alpha")
	})
	t.Run("refusals preserve rows and history", func(t *testing.T) {
		h := three(t, false)
		before, steps := h.rows(), len(h.m.undo)
		for _, k := range []string{"o", "O", ">", "<"} {
			h.act(k)
			if h.m.msg == "" || h.rows() != before || len(h.m.undo) != steps {
				t.Fatalf("Today did not refuse %s", k)
			}
		}
		h.press("v")
		for _, k := range []string{"J", "K"} {
			h.press(k)
			if h.m.msg != "that works on one row: press Esc to clear the selection" || len(h.m.undo) != steps {
				t.Fatalf("Today selection did not refuse %s: %s", k, h.m.msg)
			}
		}
		h.press("esc", "/", `"One"`, "enter", "J")
		if h.m.msg != "clear the filter first (Esc)" || len(h.m.undo) != steps {
			t.Fatal("filtered Today allowed reorder")
		}
		h.press("esc", "$", `"waiting"`, "enter", "f", `"waiting"`, "enter")
		before, steps = h.rows(), len(h.m.undo)
		for _, k := range []string{"J", "K"} {
			h.press(k)
			if h.m.msg != "a list has no order: h shows the row where it lives, Enter works inside it" || h.rows() != before || len(h.m.undo) != steps {
				t.Fatalf("Waiting did not refuse %s", k)
			}
		}
		h.clock = h.clock.Add(15 * 24 * time.Hour)
		for _, keys := range [][]string{{"f", `"stale"`, "enter"}, {":", `"review"`, "enter"}} {
			h.press(keys...)
			before, steps = h.rows(), len(h.m.undo)
			for _, k := range []string{"J", "K"} {
				h.press(k)
				if h.m.msg != "a list has no order: h shows the row where it lives, Enter works inside it" || h.rows() != before || len(h.m.undo) != steps {
					t.Fatalf("%s did not refuse %s", h.header(), k)
				}
			}
		}
	})
	t.Run("failed initialization and pick are rolled back", func(t *testing.T) {
		h := three(t, true)
		h.press("G")
		before, steps := h.rows(), len(h.m.undo)
		scheduled := h.m.current()
		h.m.backend = &flaky{Backend: h.m.backend, fail: true}
		for _, k := range []string{"K", "t"} {
			h.press(k)
			if h.rows() != before || len(h.m.undo) != steps || !strings.Contains(h.m.msg, "not saved") {
				t.Fatalf("failed %s changed the view or history", k)
			}
			if row := h.m.tree.Row(scheduled); row.Fields[fieldTodayPos] != "" || row.Fields["today"] != "" {
				t.Fatalf("failed %s left fields behind", k)
			}
		}
	})
}

func TestCaptureFilesInTheInboxOrANamedProject(t *testing.T) {
	h := two(t)
	h.press("a", `"Buy milk"`)
	if got := h.lines()[len(h.lines())-1]; got != "Inbox › Buy milk   files in Inbox" {
		t.Fatalf("capture line = %q", got)
	}
	if c := h.m.View().Cursor; c == nil || c.X != 16 || c.Y != 15 {
		t.Fatalf("caret = %+v", c)
	}
	h.press("enter", `"Fix the build ! %alp"`, "enter")
	if h.status() != "Inbox ›    added to Alpha" {
		t.Fatalf("status = %q, msg = %q", h.status(), h.m.msg)
	}
	h.press(`"see %nowhere"`, "esc")
	h.want(
		"▌ · Beta",
		"  ▾ Alpha",
		"    ○ One @tomorrow",
		"    ◔ Two",
		"    ○ Fix the build !",
	)
	h.wantSide(
		"Inbox 2",
		"Today",
		"Waiting 1",
		"Stale",
		"▌ Projects",
		"Beta ∅",
		"Alpha @",
	)
	h.press("f", `"inb"`, "enter")
	h.want(
		"▌ ○ Buy milk",
		"  ○ see %nowhere",
	)
	// One capture is one undo step.
	h.press("u")
	h.want("▌ ○ Buy milk")
}

func TestMoveARowToAProject(t *testing.T) {
	h := two(t)
	h.press("a", `"Stray thought"`, "enter", "enter", "f", `"inbox"`, "enter")
	h.want("▌ ○ Stray thought")
	h.press("m", `"be"`)
	if got := h.lines()[len(h.lines())-1]; got != "Move to › be   Beta" {
		t.Fatalf("picker line = %q", got)
	}
	h.press("enter")
	if h.m.msg != "moved to Beta" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.want("  The Inbox is empty. Press a to capture a thought.")
	h.press("f", `"beta"`, "enter")
	h.want("▌ ○ Stray thought")
	h.press("u", "f", `"inbox"`, "enter")
	h.want("▌ ○ Stray thought")

	// A picker with no match acts on nothing, and Esc leaves it.
	h.press("m", `"zzz"`, "enter")
	if h.m.msg != "nothing matches zzz" || h.m.ask == nil {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("esc")
	h.want("▌ ○ Stray thought")
}

func TestParkedAndClosedProjectsLeaveTheSidebar(t *testing.T) {
	h := two(t)
	h.press("j", "A", `" $parked"`, "esc", "k", "A", `" $done"`, "esc")
	h.want(
		"▌ ✓ Beta",
		"  ◌ Alpha",
		"    ○ One @tomorrow",
		"    ◔ Two",
	)
	// Parked work is in no computed list, however long it sits.
	h.clock = h.clock.Add(30 * 24 * time.Hour)
	h.press("j")
	h.wantSide(
		"Inbox",
		"Today",
		"Waiting",
		"Stale",
		"▌ Projects",
		"1 parked project",
	)
	// It is still reachable by name.
	h.press("f", `"alp"`, "enter")
	if h.header() != "Alpha  · parked · 2 to do" {
		t.Fatalf("header = %q", h.header())
	}
	// The count stands for it in the sidebar, which has no entry to mark.
	h.wantSide(
		"Inbox",
		"Today",
		"Waiting",
		"Stale",
		"Projects",
		"▌ 1 parked project",
	)
}

func TestTheInboxRowIsNeverASibling(t *testing.T) {
	h := two(t)
	h.press("K")
	h.want(
		"▌ · Beta",
		"  ▾ Alpha",
		"    ○ One @tomorrow",
		"    ◔ Two",
	)
	h.press(">", ">")
	if h.m.msg != "no row above to indent under" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("O", `"First"`, "esc", "J")
	h.want(
		"  · Beta",
		"▌ · First",
		"  ▾ Alpha",
		"    ○ One @tomorrow",
		"    ◔ Two",
	)
}

func TestANarrowTerminalShowsTheSidebarOnlyWhileFocused(t *testing.T) {
	h := two(t)
	h.m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	if strings.Contains(h.lines()[0], "Inbox") {
		t.Fatal("sidebar drawn in a narrow terminal")
	}
	h.press("tab")
	if !strings.Contains(h.lines()[0], "Inbox") {
		t.Fatalf("sidebar not drawn while focused:\n%s", strings.Join(h.lines(), "\n"))
	}
	h.press("j", "k", "enter")
	h.want(
		"▌ · Beta",
		"  ▾ Alpha",
		"    ○ One @tomorrow",
		"    ◔ Two",
	)
}

func TestMoveReachesAHeading(t *testing.T) {
	fixture := func(t *testing.T) *harness {
		h := start(t).wide()
		h.pasteText("Alpha\n\tStorage:\n\t\tExisting\n\tFirst\n\tSecond")
		return h
	}
	children := func(h *harness, id string, want ...string) {
		h.t.Helper()
		var got []string
		for _, child := range h.m.tree.Children(id) {
			got = append(got, h.m.tree.Row(child).Text())
		}
		if !slices.Equal(got, want) {
			h.t.Fatalf("children = %q, want %q", got, want)
		}
	}
	t.Run("move and selection append in order", func(t *testing.T) {
		h := fixture(t)
		heading := h.m.headings[0].id
		h.press("m", `"stor"`)
		if got := h.status(); got != "Move to › stor   Alpha › Storage" {
			t.Fatalf("picker = %q", got)
		}
		h.press("enter")
		children(h, heading, "Existing", "Second")
		h.press("u", "f", `"alpha"`, "enter", "g", "g")
		h.press("j", "j", "v", "v")
		h.press("m", `"stor"`, "enter")
		children(h, heading, "Existing", "First", "Second")
		h.press("u", "esc")
		children(h, heading, "Existing")
		h.want("  ▾ Storage: 0/1", "    ○ Existing", "▌ ○ First", "  ○ Second")
	})
	t.Run("equal ranks prefer project then Down reaches heading", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("Cold Storage\nAlpha\n\tStorage:\n\tStray")
		hits := match("storage", 3, h.m.targets(false))
		if len(hits) != 2 || hits[0].kind != kindProject || hits[1].kind != kindHeading {
			t.Fatalf("matches = %+v", hits)
		}
		project, heading := hits[0].id, hits[1].id
		h.press("m", `"storage"`, "enter")
		children(h, project, "Stray")
		h.press("u", "m", `"storage"`, "down", "enter")
		children(h, heading, "Stray")
	})
	t.Run("nested capture and a jump to the heading", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("Alpha\n\tTUI:\n\t\tSidebar:\n\t\t\tExisting\n\tStray")
		heading := h.m.headings[1].id
		if h.m.headings[1].label != "Alpha › TUI › Sidebar" {
			t.Fatalf("heading = %+v", h.m.headings[1])
		}
		h.press("a", `"New %sidebar"`)
		if got := h.status(); got != "Inbox › New %sidebar   files in Alpha › TUI › Sidebar" {
			t.Fatalf("capture = %q", got)
		}
		h.press("enter", "enter")
		children(h, heading, "Existing", "New")
		newID := h.m.tree.Children(heading)[1]
		if h.m.tree.Row(newID).Fields["state"] != "open" {
			t.Fatal("capture under a heading must default to a task")
		}
		h.press("u")
		children(h, heading, "Existing")
		// The jump picker reaches the heading, inside its project: the
		// sidebar stays on the project and Esc backs out to it.
		h.press("f", `"sidebar"`, "enter")
		if h.header() != "Alpha › TUI › Sidebar  · 1 to do" {
			t.Fatalf("header = %q", h.header())
		}
		h.want("▌ ○ Existing")
		if h.m.sel.kind != kindProject || h.m.sel.id != h.m.projectOf(heading).ID {
			t.Fatalf("sidebar selection = %+v", h.m.sel)
		}
		h.press("esc")
		if h.header() != "Alpha  · 2 to do" {
			t.Fatalf("Esc went to %q", h.header())
		}
	})
	t.Run("cycles fail without moving any selected row", func(t *testing.T) {
		h := fixture(t)
		h.press("f", `"alpha"`, "enter", "g", "g", "m", `"storage"`, "enter")
		if h.m.msg != "a row cannot move under itself" {
			t.Fatalf("self move = %q", h.m.msg)
		}
		h.press("f", `"projects"`, "enter", "g", "g", "m", `"storage"`, "enter")
		if h.m.msg != "a row cannot move under itself" {
			t.Fatalf("descendant move = %q", h.m.msg)
		}
		// First move would succeed; the second would create a cycle.
		h = start(t).wide()
		h.pasteText("Alpha\n\tFirst\n\tStorage:\n\t\tNested:")
		h.press("f", `"alpha"`, "enter", "g", "g", "v", "v", "m", `"nested"`, "enter")
		if h.m.msg != "a row cannot move under itself" {
			t.Fatalf("selected move = %q", h.m.msg)
		}
		children(h, h.m.at.root, "First", "Storage:")
		children(h, h.m.headings[1].id)
	})
	t.Run("live headings in Inbox and at any depth", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("Alpha:\n\tTask\n\t\tDeep:\n\tGone:\n\t\tHidden:\n\tClosed: $done\n\tParked: $parked")
		h.press("f", `"alpha"`, "enter", "g", "g", "j", "j", "d", "d")
		h.press("a", `"Inbox heading:"`, "enter", "enter")
		var labels []string
		for _, e := range h.m.headings {
			labels = append(labels, e.label)
		}
		want := []string{"Inbox › Inbox heading", "Alpha › Task › Deep", "Alpha › Closed", "Alpha › Parked"}
		if !slices.Equal(labels, want) {
			t.Fatalf("headings = %q, want %q", labels, want)
		}
		h.press("a", `"Filed %heading"`, "enter", "enter")
		children(h, h.m.headings[0].id, "Filed")
	})
	t.Run("picker letters reuse scan and edits invalidate it", func(t *testing.T) {
		h := fixture(t)
		// Poison the derived value, leaving the tree intact. A new derivation,
		// even one that reuses the slice's storage, would replace this label.
		h.m.headings[0].label = "Cached heading"
		scanned := h.m.scanned
		h.press("m", `"cached"`)
		if h.m.scanned != scanned || h.m.headings[0].label != "Cached heading" {
			t.Fatal("typing in the picker derived headings again")
		}
		if got := h.status(); got != "Move to › cached   Cached heading" {
			t.Fatalf("picker did not use cached headings: %q", got)
		}
		h.press("esc", "f", `"alpha"`, "enter", "g", "g", "A", "ctrl+u", `"Renamed:"`, "esc")
		if h.m.headings[0].label != "Alpha › Renamed" {
			t.Fatalf("edit did not refresh headings: %+v", h.m.headings)
		}
	})
}

func TestSidebarPagesLikeThePane(t *testing.T) {
	h := start(t).wide() // 16 rows: a page is 14 entries, half is 7
	keys := []string{"o"}
	for i := 1; i <= 20; i++ {
		if i > 1 {
			keys = append(keys, "enter")
		}
		keys = append(keys, fmt.Sprintf(`"P%02d"`, i))
	}
	h.press(append(keys, "esc")...)

	// Entries: Inbox, Today, Waiting, Stale, Projects, then P01 to P20.
	h.press("tab")
	for _, step := range []struct{ key, header string }{
		{"ctrl+d", "P07"},
		{"ctrl+f", "P20"},
		{"ctrl+u", "P13"},
		{"ctrl+b", "Stale"},
		{"G", "P20"},
		{"g", "P20"},
		{"g", "Inbox"},
		{"pgdown", "P10"},
		{"pgup", "Inbox"},
		{"g", "Inbox"},
		{"j", "Today"},
		{"end", "P20"},
		{"home", "Inbox"},
	} {
		h.press(step.key)
		if got := h.header(); got != step.header {
			t.Fatalf("after %s, header = %q, want %q", step.key, got, step.header)
		}
	}
}

// A sidebar taller than its pane counts what is scrolled out at each edge,
// and never draws a count over the selection.
func TestSidebarCountsWhatIsScrolledOut(t *testing.T) {
	h := start(t).wide()
	h.press("o")
	for _, name := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		h.press(`"`+name+`"`, "enter")
	}
	h.press("esc")
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 9})
	side := func() []string { return strings.Split(h.side(), "\n") }
	if got := side(); got[0] != "Inbox" || got[len(got)-1] != "↓ 7" {
		t.Fatalf("at the top:\n%s", h.side())
	}
	h.press("tab", "G")
	if got := side(); got[0] != "↑ 6" || got[len(got)-1] != "▌ H ∅" {
		t.Fatalf("at the end:\n%s", h.side())
	}
	h.press("k", "k", "k", "k")
	got := side()
	if got[0] != "↑ 5" || got[len(got)-1] != "↓ 2" || !slices.Contains(got, "▌ D ∅") {
		t.Fatalf("in the middle:\n%s", h.side())
	}
}

// A project's sidebar entry is its top-level row, so dd takes it there as it
// does in Projects, and undo and redo keep the focus and follow the project.
func TestTheSidebarDeletesAProjectAndUndoesIt(t *testing.T) {
	h := start(t).wide()
	h.press("o", `"Alpha"`, "enter", `"Beta"`, "esc")
	h.press(":", `"tutor"`, "enter", "tab", "d")
	if got := h.status(); !strings.HasPrefix(got, "j k choose · Enter work in it · A I cc rename it · dd delete it") || !strings.HasSuffix(got, " d") {
		t.Fatalf("a waiting d is not shown: %q", got)
	}
	h.press("d")
	if h.m.msg != "deleted 47 rows" || !h.m.sideFocus {
		t.Fatalf("msg = %q, focus = %v", h.m.msg, h.m.sideFocus)
	}
	h.wantSide("Inbox", "Today", "Waiting", "Stale", "Projects", "Alpha ∅", "▌ Beta ∅")
	h.press("u")
	if h.m.msg != "undo: 47 rows back" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.wantSide("Inbox", "Today", "Waiting", "Stale", "Projects", "Alpha ∅", "Beta ∅", "▌ Welcome")
	h.press("ctrl+r")
	if h.m.msg != "redo: 47 rows gone" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.wantSide("Inbox", "Today", "Waiting", "Stale", "Projects", "Alpha ∅", "▌ Beta ∅")

	// Anything else in the sidebar is not a row, and says what works.
	h.press("k", "k", "d", "d")
	if h.m.msg != "only a project is deleted here: Enter, then dd takes rows" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("d", "j", "d")
	h.wantSide("Inbox", "Today", "Waiting", "Stale", "Projects", "▌ Alpha ∅", "Beta ∅")
	h.press("enter", "/", `"x"`, "enter", ":", `"save x"`, "enter", "esc", "tab", "g", "g", "j", "j", "j", "j", "d", "d")
	if h.m.msg != "a saved filter is not deleted here: :forget x drops it" {
		t.Fatalf("msg = %q\n%s", h.m.msg, h.side())
	}
}

// The sidebar is there to show what needs attention, so the project left
// alone longest is drawn red, however young, and the gradient that leads to
// it is drawn against that age.
func TestANeglectedProjectsAgeStandsOut(t *testing.T) {
	h := two(t)
	h.clock = h.clock.Add(3 * 24 * time.Hour)
	h.press("j")
	if raw := h.m.View().Content; !strings.Contains(raw, ageStyle(darkPalette, 1, 1)+"3d") {
		t.Fatalf("the oldest age is not red:\n%q", raw)
	}
	h.clock = h.clock.Add(12 * 24 * time.Hour)
	h.press("k")
	if raw := h.m.View().Content; !strings.Contains(raw, ageStyle(darkPalette, 1, 1)+"15d") {
		t.Fatalf("the oldest age is not red:\n%q", raw)
	}
}

// Saved filters are the user's own places, set apart from the four lists
// by a blank line, as Projects is.
func TestSavedFiltersAreAGroupOfTheirOwn(t *testing.T) {
	h := two(t)
	h.press("/", `"One"`, "enter", ":", `"save one"`, "enter", "esc")
	var side []string
	for _, l := range h.lines()[:9] {
		side = append(side, strings.Join(strings.Fields(string([]rune(l)[:sidebarCols])), " "))
	}
	want := []string{"Inbox", "Today", "Waiting 1", "Stale", "", "/one", "", "▌ Projects"}
	if got := side[:len(want)]; !slices.Equal(got, want) {
		t.Fatalf("sidebar = %q\nwant %q", got, want)
	}
}

// todayRule is the line Today draws between the day's own rows and the
// picks carried from an earlier day, as wide as the pane allows.
func todayRule(h *harness) string {
	w := h.m.width
	if w >= sidebarFrom {
		w -= sidebarCols + 3
	}
	return "  " + strings.Repeat("─", w-4)
}

func TestTodayDrawsALineAboveWhatWasCarried(t *testing.T) {
	h := start(t).wide()
	h.press("o", `"Alpha"`, "enter", "tab", `"One"`, "enter", `"Two"`, "enter", `"Three"`, "enter", `"Four"`, "esc")
	// One and Two are picked today; tomorrow they are carried.
	h.press("g", "g", "j", "t", "j", "t")
	h.clock = h.clock.Add(24 * time.Hour)
	// The next day Three is picked, and Four starts on it.
	h.press("j", "t", "j", "A", `" @@today"`, "esc", "f", `"today"`, "enter")
	h.want(
		"▌ ○ Three · Alpha",
		"  ○ Four · Alpha",
		todayRule(h),
		"  ○ One · since Nov 15 · Alpha",
		"  ○ Two · since Nov 15 · Alpha",
	)
	// J and K keep each row on its side of the line, and say why.
	h.press("j", "J")
	if h.status() != "today's rows stay above the line: J K order them there" {
		t.Fatalf("status = %q", h.status())
	}
	h.press("j", "K")
	if h.status() != "carried picks stay below the line: t t picks it for today" {
		t.Fatalf("status = %q", h.status())
	}
	h.press("j", "K")
	h.want(
		"  ○ Three · Alpha",
		"  ○ Four · Alpha",
		todayRule(h),
		"▌ ○ Two · since Nov 15 · Alpha",
		"  ○ One · since Nov 15 · Alpha",
	)
	// Unpicked, a carried row keeps its place, dimmed; picked again for
	// today it joins the day's own rows, after them.
	h.press("t")
	h.want(
		"  ○ Three · Alpha",
		"  ○ Four · Alpha",
		todayRule(h),
		"▌ ○ Two · Alpha",
		"  ○ One · since Nov 15 · Alpha",
	)
	h.press("t")
	h.want(
		"  ○ Three · Alpha",
		"  ○ Four · Alpha",
		"▌ ○ Two · Alpha",
		todayRule(h),
		"  ○ One · since Nov 15 · Alpha",
	)
	// With nothing of the day's own, there is no line to draw.
	h = start(t).wide()
	h.press("o", `"Alpha"`, "enter", "tab", `"One"`, "esc", "t")
	h.clock = h.clock.Add(24 * time.Hour)
	h.press("f", `"today"`, "enter")
	h.want("▌ ○ One · since Nov 15 · Alpha")
}

// A project's sidebar entry is its top-level row, so its name is typed in
// the entry with the keys that type in that row in Projects, as one undo
// step, and the focus stays in the sidebar.
func TestTheSidebarRenamesAProject(t *testing.T) {
	h := two(t)
	h.press("tab", "j", "j", "A", `" Two"`)
	if got := h.status(); got != "Enter or Esc done" {
		t.Fatalf("status = %q", got)
	}
	h.wantSide("Inbox", "Today", "Waiting 1", "Stale", "Projects", "Beta ∅", "▌ Alpha Two")
	if c := h.m.View().Cursor; c == nil || c.X != 2+len("Alpha Two") || c.Y != 7 {
		t.Fatalf("caret = %+v", c)
	}
	// Nothing carries the typing out of the name: not Tab, not Enter.
	h.press("tab", "enter")
	if h.m.edit != nil || !h.m.sideFocus || h.m.tree.Row(h.m.sel.id).Text() != "Alpha Two" {
		t.Fatalf("edit = %v, focus = %v, %q", h.m.edit, h.m.sideFocus, h.m.tree.Row(h.m.sel.id).Text())
	}
	h.wantSide("Inbox", "Today", "Waiting 1", "Stale", "Projects", "Beta ∅", "▌ Alpha Two @")
	if !strings.HasPrefix(h.header(), "Alpha Two") {
		t.Fatalf("header = %q", h.header())
	}
	h.press("u")
	h.wantSide("Inbox", "Today", "Waiting 1", "Stale", "Projects", "Beta ∅", "▌ Alpha @")
	h.press("ctrl+r")

	// c with a motion changes the name from its start; cc retypes it all.
	h.press("c", "w", `"Uno"`, "esc")
	h.wantSide("Inbox", "Today", "Waiting 1", "Stale", "Projects", "Beta ∅", "▌ Uno Two @")
	h.press("I", `"The "`, "esc")
	h.wantSide("Inbox", "Today", "Waiting 1", "Stale", "Projects", "Beta ∅", "▌ The Uno Two @")

	// A word motion steps into the name, and the grammar there stays in it.
	h.press("w")
	h.wantTyping("The Uno Two", 0)
	h.press("w", "j", "k")
	h.wantTyping("The Uno Two", 4)
	h.press("d", "w", "esc")
	h.wantSide("Inbox", "Today", "Waiting 1", "Stale", "Projects", "Beta ∅", "▌ The Two @")

	h.press("c", "i", "w", `"One"`, "esc")
	h.wantSide("Inbox", "Today", "Waiting 1", "Stale", "Projects", "Beta ∅", "▌ One Two @")
	h.press("c", "i", "w", `"The"`, "esc")

	// An empty project's name is typed as well, with nothing in the pane.
	h.press("k", "c", "c", `"Gamma"`, "enter")
	h.wantSide("Inbox", "Today", "Waiting 1", "Stale", "Projects", "▌ Gamma ∅", "The Two @")

	// A name typed out of the list gives the selection to the entry in its
	// place, as dd does.
	h.press("A", `" $p"`, "enter")
	h.wantSide("Inbox", "Today", "Waiting 1", "Stale", "Projects", "▌ The Two @", "1 parked project")

	// Anything else in the sidebar has no name to type.
	h.press("k", "A")
	if h.m.edit != nil || h.m.msg != "only a project is renamed here: Enter, then A or I types in rows" {
		t.Fatalf("edit = %v, msg = %q", h.m.edit, h.m.msg)
	}
}

// Too narrow for both, the sidebar alone takes the name being typed, and
// a long one wraps inside it.
func TestANarrowSidebarRenamesAProject(t *testing.T) {
	h := two(t)
	h.m.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	h.press("tab", "j", "A", `" is a project with a long name"`)
	got := h.lines()
	if got[6] != "▌ Beta is a project with" || got[7] != "▌  a long name          " {
		t.Fatalf("sidebar:\n%s", strings.Join(got, "\n"))
	}
	if c := h.m.View().Cursor; c == nil || c.X != 2+len(" a long name") || c.Y != 7 {
		t.Fatalf("caret = %+v", c)
	}
	h.press("enter")
	if text := h.m.tree.Row(h.m.sel.id).Text(); text != "Beta is a project with a long name" || !h.m.sideFocus {
		t.Fatalf("%q, focus = %v", text, h.m.sideFocus)
	}
}
