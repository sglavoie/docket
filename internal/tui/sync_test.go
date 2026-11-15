package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTwoMachinesShareAnOutline(t *testing.T) {
	shared := t.TempDir()
	desk := open(t, filepath.Join(t.TempDir(), "desk.db")).wide()
	lap := open(t, filepath.Join(t.TempDir(), "lap.db")).wide()

	desk.press(":", `"sync `+shared+`"`, "enter")
	if desk.m.msg != "synced with 0 other machines" {
		t.Fatalf("msg = %q", desk.m.msg)
	}
	desk.press("o", `"Alpha"`, "enter", "tab", `"Write the spec #doc"`, "enter", `"Ship it @fri"`, "esc")
	desk.m.Update(tickMsg{}) // the minute tick carries it out

	lap.press(":", `"sync `+shared+`"`, "enter")
	lap.want(
		"▌ ▾ Alpha",
		"    ○ Write the spec #doc",
		"    ○ Ship it @fri",
	)
	lap.wantSide(
		"Inbox",
		"Today",
		"Waiting",
		"Stale",
		"▌ Projects",
		"Alpha !",
		lap.m.syncNote,
	)
	if !strings.HasPrefix(lap.m.syncNote, "synced ") || !strings.HasPrefix(lap.m.msg, "synced: ") {
		t.Fatalf("note = %q, msg = %q", lap.m.syncNote, lap.m.msg)
	}

	// Each machine changes a different field of the same row, and the lap
	// adds a row; neither has seen the other's change.
	lap.press("j", "x", "o", `"Announce"`, "esc")
	desk.press("k", "e", `" v2"`, "esc")
	lap.m.Update(tickMsg{})
	desk.m.Update(tickMsg{})
	lap.m.Update(tickMsg{})
	for _, h := range []*harness{desk, lap} {
		h.press("g", "g")
		h.want(
			"▌ ▾ Alpha",
			"    ✓ Write the spec v2 #doc",
			"    ○ Announce",
			"    ○ Ship it @fri",
		)
	}
	if lap.m.msg != "" && desk.m.msg != "" {
		t.Fatalf("msgs = %q, %q", lap.m.msg, desk.m.msg)
	}

	// Undo on one machine is a new change, and reaches the other.
	lap.press("j", "u")
	lap.m.Update(tickMsg{})
	desk.m.Update(tickMsg{})
	desk.want(
		"▌ ▾ Alpha",
		"    ✓ Write the spec v2 #doc",
		"    ○ Ship it @fri",
	)

	// Sync waits while a line is being typed, and can be turned off.
	desk.press("o", `"half typed"`)
	lap.press("g", "g", "j", "!")
	lap.m.Update(tickMsg{})
	desk.m.Update(tickMsg{})
	if string(desk.m.edit.buf) != "half typed" {
		t.Fatal("sync interrupted typing")
	}
	desk.press("esc")
	desk.m.Update(tickMsg{})
	desk.want(
		"  ▾ Alpha",
		"▌   ○ half typed",
		"    ✓ Write the spec v2 ! #doc",
		"    ○ Ship it @fri",
	)
	// A saved filter is a row like any other, so it travels too.
	desk.press("/", `"#doc"`, "enter", ":", `"save docs"`, "enter", "esc")
	desk.m.Update(tickMsg{})
	lap.m.Update(tickMsg{})
	lap.press("p", `"/docs"`, "enter")
	lap.want(
		"  ▾ Alpha",
		"▌   ✓ Write the spec v2 ! #doc",
	)

	desk.press(":", `"sync off"`, "enter")
	if desk.m.msg != "sync is off" || desk.m.syncDir != "" || desk.m.syncNote != "" {
		t.Fatalf("msg = %q", desk.m.msg)
	}
}

func TestAFailedSyncIsSaidAndWorkGoesOn(t *testing.T) {
	h := work(t)
	notAFolder := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notAFolder, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h.press(":", `"sync `+notAFolder+`"`, "enter")
	if !strings.HasPrefix(h.m.msg, "sync failed: ") {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("x")
	h.want(
		"  ▾ Beta",
		"▌   ✓ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	h.wantSide(
		"Inbox",
		"Today",
		"Waiting",
		"Stale",
		"▌ Projects",
		"Beta ∅",
		"Alpha !",
		"sync failed",
	)
	if !strings.Contains(h.m.View().Content, sgrAlert+"sync failed") {
		t.Fatal("a failed sync is not shown as an alarm")
	}
}

func TestTwoWindowsOnOneDatabaseStayInStep(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docket.db")
	one, two := open(t, path), open(t, path)
	n := 0
	two.m.newID = func() string { n++; return fmt.Sprintf("second-window-%03d", n) }

	one.press("o", `"Alpha"`, "enter", "tab", `"From the first window"`, "esc")
	// The second window sees it at its next key, here a harmless one.
	two.press("g", "g")
	two.want(
		"▌ ▾ Alpha",
		"    ○ From the first window",
	)
	two.press("j", "o", `"From the second"`, "esc", "k", "x")
	// The first window, mid-typing, is left alone; its row is not stored
	// yet. When the row is kept it catches up, and nothing is lost.
	one.press("o", `"typed meanwhile"`)
	one.m.Update(tickMsg{})
	if one.m.edit == nil || string(one.m.edit.buf) != "typed meanwhile" {
		t.Fatal("the other window's change interrupted typing")
	}
	one.press("esc")
	want := []string{
		"  ▾ Alpha",
		"    ✓ From the first window",
		"▌   ○ typed meanwhile",
		"    ○ From the second",
	}
	one.want(want...)
	two.press("j")
	two.want(want...)

	// Undo in one window is seen in the other.
	one.press("u")
	two.press("k")
	two.want(
		"  ▾ Alpha",
		"▌   ✓ From the first window",
		"    ○ From the second",
	)
}
