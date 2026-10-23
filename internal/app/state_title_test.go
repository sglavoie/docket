package app

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

// TestFitStateChangeTitleCutsByColumns holds the status change's title to the
// columns the check-in note gives it: a CJK title is two columns a rune, so a
// rune count would leave it twice as wide as the note's cut. The expectations
// are what textfit.TruncateToVisualWidth gives at 24 columns, so a ZWJ family,
// combining accents, and flags are cut where the note would cut them.
func TestFitStateChangeTitleCutsByColumns(t *testing.T) {
	for _, tc := range []struct{ title, want string }{
		{"Short title", "Short title"},
		{"exactly twenty-four cols", "exactly twenty-four cols"},
		{"a title that runs past twenty-four columns", "a title that runs past …"},
		{"整理書架上的舊書並捐給圖書館的志工", "整理書架上的舊書並捐給…"},
		{"👩\u200d👩\u200d👧 family plans for the long weekend", "👩\u200d👩\u200d👧 family plans for the…"},
		{strings.Repeat("e\u0301", 25), strings.Repeat("e\u0301", 23) + "…"},
		{strings.Repeat("🇫🇷", 13), strings.Repeat("🇫🇷", 11) + "…"},
	} {
		got := fitStateChangeTitle(tc.title)
		if got != tc.want {
			t.Errorf("fitStateChangeTitle(%q) = %q, want %q", tc.title, got, tc.want)
		}
		if width := uniseg.StringWidth(got); width > stateChangeTitleWidth {
			t.Errorf("fitStateChangeTitle(%q) is %d columns, over %d", tc.title, width, stateChangeTitleWidth)
		}
	}
}
