package app

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestFitStateChangeTitleCutsByColumns holds the status change's title to the
// columns the check-in note gives it: a CJK title is two columns a rune, so a
// rune count would leave it twice as wide as the note's cut.
func TestFitStateChangeTitleCutsByColumns(t *testing.T) {
	for _, tc := range []struct{ title, want string }{
		{"Short title", "Short title"},
		{"exactly twenty-four cols", "exactly twenty-four cols"},
		{"a title that runs past twenty-four columns", "a title that runs past …"},
		{"整理書架上的舊書並捐給圖書館的志工", "整理書架上的舊書並捐給…"},
	} {
		got := fitStateChangeTitle(tc.title)
		if got != tc.want {
			t.Errorf("fitStateChangeTitle(%q) = %q, want %q", tc.title, got, tc.want)
		}
		if width := ansi.StringWidth(got); width > stateChangeTitleWidth {
			t.Errorf("fitStateChangeTitle(%q) is %d columns, over %d", tc.title, width, stateChangeTitleWidth)
		}
	}
}
