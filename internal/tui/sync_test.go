package tui

import (
	"path/filepath"
	"testing"
)

func TestTwoMachinesShareAnOutline(t *testing.T) {
	shared := t.TempDir()
	desk := open(t, filepath.Join(t.TempDir(), "desk.db")).wide()
	lap := open(t, filepath.Join(t.TempDir(), "lap.db")).wide()

	desk.press(":", `"sync `+shared+`"`, "enter")
	if desk.m.msg != "synced with "+shared {
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
	)

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
	desk.press(":", `"sync off"`, "enter")
	if desk.m.msg != "sync is off" || desk.m.syncDir != "" {
		t.Fatalf("msg = %q", desk.m.msg)
	}
}
