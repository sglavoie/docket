package tui

import (
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
		"▌   Beta",
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
		"Alpha !",
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
		"Alpha ! 15d",
	)
}

func TestSidebarFocusShowsEachDestination(t *testing.T) {
	h := two(t)
	h.press("tab", "j", "j")
	if h.header() != "Alpha" {
		t.Fatalf("header = %q", h.header())
	}
	h.want(
		"▌ ○ One @tomorrow",
		"  ◔ Two",
	)
	h.press("k", "k", "k", "k")
	if h.header() != "Waiting" {
		t.Fatalf("header = %q", h.header())
	}
	h.want("▌ ◔ Two · 0d · Alpha")

	// Back in the pane, closing the row leaves it in the list it was found
	// in; the list is read again when reopened.
	h.press("enter", "x")
	h.want("▌ ✓ Two · 0d · Alpha")
	h.wantSide(
		"Inbox",
		"Today",
		"▌ Waiting",
		"Stale",
		"Projects",
		"Beta ∅",
		"Alpha !",
	)
	h.press("tab", "j", "k", "tab")
	h.want("  Nothing is waiting.")
}

func TestAListHasNoStructure(t *testing.T) {
	h := two(t)
	h.press("p", `"wait"`, "enter")
	h.want("▌ ◔ Two · 0d · Alpha")
	for _, k := range []string{"o", "J", ">"} {
		h.press(k)
		if h.status() == "" {
			t.Fatalf("%s was not refused", k)
		}
		h.want("▌ ◔ Two · 0d · Alpha")
	}
	// Enter works inside the row, and Esc returns to the same list.
	h.press("enter")
	if h.header() != "Alpha › Two" {
		t.Fatalf("header = %q", h.header())
	}
	h.press("o", `"a detail $note"`, "esc", "esc")
	h.want("▌ ◔ Two +1 · 0d · Alpha")
}

func TestTodayIsPickedAndLapsesOvernight(t *testing.T) {
	h := two(t)
	h.press("j", "j", "t", "j", "t", "x")
	h.wantSide(
		"Inbox",
		"Today 1",
		"Waiting",
		"Stale",
		"▌ Projects",
		"Beta ∅",
		"Alpha !",
	)
	h.press("p", `"tod"`, "enter")
	h.want(
		"▌ ○ One @tomorrow · Alpha",
		"  ✓ Two · Alpha",
	)
	h.press("t")
	h.wantSide(
		"Inbox",
		"▌ Today",
		"Waiting",
		"Stale",
		"Projects",
		"Beta ∅",
		"Alpha !",
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
	if again.header() != "Today" {
		t.Fatalf("launched on %q", again.header())
	}

	// The next morning nothing is picked, and nothing was written to say so.
	next := resume(t, h.path, st, h.clock.Add(24*time.Hour)).wide()
	if next.header() != "Projects" {
		t.Fatalf("launched on %q", next.header())
	}
	next.wantSide(
		"Inbox",
		"Today",
		"Waiting",
		"Stale",
		"▌ Projects",
		"Beta ∅",
		"Alpha ! 1d",
	)
}

func TestCaptureFilesInTheInboxOrANamedProject(t *testing.T) {
	h := two(t)
	h.press("a", `"Buy milk"`)
	if got := h.lines()[len(h.lines())-1]; got != "Inbox › Buy milk" {
		t.Fatalf("capture line = %q", got)
	}
	if c := h.m.View().Cursor; c == nil || c.X != 16 || c.Y != 15 {
		t.Fatalf("caret = %+v", c)
	}
	h.press("enter", `"Fix the build ! %alp"`, "enter")
	if h.status() != "Inbox ›" || h.m.msg != "added to Alpha" {
		t.Fatalf("status = %q, msg = %q", h.status(), h.m.msg)
	}
	h.press(`"see %nowhere"`, "esc")
	h.want(
		"▌   Beta",
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
		"Alpha !",
	)
	h.press("p", `"inb"`, "enter")
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
	h.press("a", `"Stray thought"`, "enter", "enter", "p", `"inbox"`, "enter")
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
	h.press("p", `"beta"`, "enter")
	h.want("▌ ○ Stray thought")
	h.press("u", "p", `"inbox"`, "enter")
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
	h.press("j", "e", `" $parked"`, "esc", "k", "e", `" $done"`, "esc")
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
		"parked 1",
	)
	// It is still reachable by name.
	h.press("p", `"alp"`, "enter")
	if h.header() != "Alpha" {
		t.Fatalf("header = %q", h.header())
	}
}

func TestTheInboxRowIsNeverASibling(t *testing.T) {
	h := two(t)
	h.press("K")
	h.want(
		"▌   Beta",
		"  ▾ Alpha",
		"    ○ One @tomorrow",
		"    ◔ Two",
	)
	h.press(">")
	if h.m.msg != "no row above to indent under" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("O", `"First"`, "esc", "J")
	h.want(
		"    Beta",
		"▌   First",
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
		"▌   Beta",
		"  ▾ Alpha",
		"    ○ One @tomorrow",
		"    ◔ Two",
	)
}
