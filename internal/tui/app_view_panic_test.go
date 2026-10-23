package tui

import "testing"

func TestViewPanicTextSpellsTheQuitKeyLikeTheFooter(t *testing.T) {
	if got, want := viewPanicText("boom"), "internal error: boom\n\nPress Ctrl+C to quit."; got != want {
		t.Fatalf("panic frame = %q, want %q", got, want)
	}
}
