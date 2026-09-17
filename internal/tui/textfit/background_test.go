package textfit

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// band is the background these tests lay, and armed is the sequence that arms it.
// Reading both off lipgloss rather than spelling an escape keeps the test honest
// about what the production seam actually emits.
func band() (color.Color, string) {
	bg := lipgloss.Color("#1f2a4b")
	return bg, ansi.NewStyle().BackgroundColor(bg).String()
}

// TestPaintBackgroundLeavesEveryVisibleCharacterAlone is the whole promise: the
// background is laid under assembled text, so the text's own bytes are not
// rewritten and its plain reading is unchanged.
func TestPaintBackgroundLeavesEveryVisibleCharacterAlone(t *testing.T) {
	bg, _ := band()
	content := lipgloss.NewStyle().Bold(true).Render("⊡ Today") + " " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render("overdue")

	painted := PaintBackground(content, bg)

	if got, want := ansi.Strip(painted), ansi.Strip(content); got != want {
		t.Fatalf("painting changed the row's text: got %q, want %q", got, want)
	}
	if got, want := lipgloss.Width(painted), lipgloss.Width(content); got != want {
		t.Fatalf("painting changed the row's width: got %d, want %d", got, want)
	}
}

// TestPaintBackgroundRearmsAfterEveryReset is why the seam exists. A style
// applied over already-styled text ends at that text's first reset, so the band
// re-arms after each one and the cells past the first segment are painted too.
func TestPaintBackgroundRearmsAfterEveryReset(t *testing.T) {
	bg, armed := band()
	plain := lipgloss.NewStyle()
	content := plain.Bold(true).Render("one") + plain.Italic(true).Render("two") +
		plain.Underline(true).Render("three")

	if resets := strings.Count(content, ansi.ResetStyle); resets < 2 {
		t.Fatalf("fixture carried %d resets, want a run the band has to re-arm inside", resets)
	}

	painted := PaintBackground(content, bg)

	if !strings.HasPrefix(painted, armed) {
		t.Fatalf("band was not armed before the first segment:\n%q", painted)
	}
	if !strings.HasSuffix(painted, ansi.ResetStyle) {
		t.Fatalf("band was left open at the end of the run:\n%q", painted)
	}
	// Removing the arms gives the row back byte for byte, so the band is additive
	// and no segment's own sequence was rewritten to make room for it.
	if got := strings.ReplaceAll(painted, armed, ""); got != content {
		t.Fatalf("painting rewrote the row:\ngot  %q\nwant %q", got, content)
	}
	// Every reset but the one closing the run is followed by a fresh arm, which is
	// what keeps the cells past the first segment painted.
	for rest := painted; ; {
		idx := strings.Index(rest, ansi.ResetStyle)
		if idx < 0 {
			break
		}
		rest = rest[idx+len(ansi.ResetStyle):]
		if rest == "" {
			break
		}
		if !strings.HasPrefix(rest, armed) {
			t.Fatalf("a reset was not followed by a fresh arm:\n%q", painted)
		}
	}
}

// TestPaintBackgroundClosesAnUnresetRun covers the other end: text carrying no
// reset of its own still gets one, so the band stops at the row's edge.
func TestPaintBackgroundClosesAnUnresetRun(t *testing.T) {
	bg, armed := band()

	painted := PaintBackground("   ", bg)

	if want := armed + "   " + ansi.ResetStyle; painted != want {
		t.Fatalf("bare run painted %q, want %q", painted, want)
	}
}

// TestPaintBackgroundReadsBothSpellingsOfAReset holds nextReset to the pair it
// claims to recognize: lipgloss closes a style with one or the other, and a
// missed reset would leave the rest of the row unpainted.
func TestPaintBackgroundReadsBothSpellingsOfAReset(t *testing.T) {
	bg, armed := band()
	for _, reset := range []string{ansi.ResetStyle, "\x1b[0m"} {
		painted := PaintBackground("a"+reset+"b", bg)
		if got, want := strings.Count(painted, armed), 2; got != want {
			t.Fatalf("reset %q armed the band %d times, want %d:\n%q", reset, got, want, painted)
		}
	}
}

// TestPaintBackgroundWithNoColorIsNoPaint is what lets one render seam serve the
// rows that wear a band and the rows that do not.
func TestPaintBackgroundWithNoColorIsNoPaint(t *testing.T) {
	content := lipgloss.NewStyle().Bold(true).Render("Ship the release")
	if got := PaintBackground(content, nil); got != content {
		t.Fatalf("a nil background painted something: %q", got)
	}
	bg, _ := band()
	if got := PaintBackground("", bg); got != "" {
		t.Fatalf("an empty run painted something: %q", got)
	}
}
