package tui

import "testing"

func TestClosingTheLastPickSaysTheDayIsDone(t *testing.T) {
	h := two(t)
	h.press("j", "j", "t", "j", "t")
	// Closing a pick with another still open is an ordinary close.
	h.press("x")
	if h.status() != "" {
		t.Fatalf("status = %q", h.status())
	}
	// Closing the last one marks the end of the day, once.
	h.press("k", "x")
	if h.status() != "today is done: every pick is closed" {
		t.Fatalf("status = %q", h.status())
	}
	h.press("j")
	if h.status() != "" {
		t.Fatalf("status = %q", h.status())
	}
	// An undo says what it took back, and its redo what it made again.
	h.press("u")
	if h.status() == "today is done: every pick is closed" {
		t.Fatal("an undo said the day was done")
	}
	h.press("ctrl+r")
	if h.status() == "today is done: every pick is closed" {
		t.Fatal("a redo said the day was done")
	}
	// Reopening and closing again ends the day again.
	h.press("g", "g", "j", "j", "x", "x")
	if h.status() != "today is done: every pick is closed" {
		t.Fatalf("status = %q", h.status())
	}
}

func TestTheInboxCountsWhatIsLeftInIt(t *testing.T) {
	h := two(t)
	h.press("a", `"milk"`, "enter", `"eggs"`, "enter", "enter")
	h.wantSide(
		"Inbox 2",
		"Today",
		"Waiting 1",
		"Stale",
		"▌ Projects",
		"Beta ∅",
		"Alpha @",
	)
	h.press("f", "enter", "x")
	if h.header() != "Inbox  · 1 to do" {
		t.Fatalf("header = %q", h.header())
	}
	h.wantSide(
		"▌ Inbox 1",
		"Today",
		"Waiting 1",
		"Stale",
		"Projects",
		"Beta ∅",
		"Alpha @",
	)
}
