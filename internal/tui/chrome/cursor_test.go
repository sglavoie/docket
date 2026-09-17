package chrome

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestClampCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		length int
		want   int
	}{
		{
			name:   "negative clamped to zero",
			cursor: -5,
			length: 10,
			want:   0,
		},
		{
			name:   "beyond length clamped to last",
			cursor: 15,
			length: 10,
			want:   9,
		},
		{
			name:   "valid cursor unchanged",
			cursor: 5,
			length: 10,
			want:   5,
		},
		{
			name:   "empty list returns zero",
			cursor: 5,
			length: 0,
			want:   0,
		},
		{
			name:   "cursor at last position unchanged",
			cursor: 9,
			length: 10,
			want:   9,
		},
		{
			name:   "cursor at first position unchanged",
			cursor: 0,
			length: 10,
			want:   0,
		},
		{
			name:   "cursor equals length clamped",
			cursor: 10,
			length: 10,
			want:   9,
		},
		{
			name:   "negative cursor empty list returns zero",
			cursor: -1,
			length: 0,
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampCursor(tt.cursor, tt.length)
			if got != tt.want {
				t.Errorf("ClampCursor(%d, %d) = %d, want %d", tt.cursor, tt.length, got, tt.want)
			}
		})
	}
}

// TestSelectedLineIsUnchangedByTheOutlinerBand holds the other list surfaces
// still. The Outliner's cursor row now carries a band under everything it drew,
// and SelectedLine is the whole-line authority every other list uses: a picker
// row, a Notes row, a Hub row are selected, not "the task you are on", so they
// keep repainting the line with the selection style. A band leaking in here would
// be the Outliner's own paint spreading to surfaces that never asked for it.
func TestSelectedLineIsUnchangedByTheOutlinerBand(t *testing.T) {
	for _, name := range []string{"dark", "light"} {
		t.Run(name, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: name})
			t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

			line := SelectedLine(theme.MutedStyle.Render("Renew passport"), 40)

			if want := theme.SelectedStyle.Width(40).Render("Renew passport"); line != want {
				t.Fatalf("SelectedLine no longer repaints the whole line:\nwant %q\ngot  %q", want, line)
			}
			if strings.Contains(line, "\x1b[7") {
				t.Fatalf("a selected list row wears the Outliner's cursor bar:\n%q", line)
			}
			band := lipgloss.NewStyle().Background(theme.ColorCursorRowBg).Render(" ")
			if params, _, _ := strings.Cut(strings.TrimPrefix(band, "\x1b["), "m"); strings.Contains(line, params) {
				t.Fatalf("a selected list row wears the Outliner's cursor band:\n%q", line)
			}
		})
	}
}
