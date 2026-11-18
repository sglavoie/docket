package tui

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var sgrRGB = regexp.MustCompile(`\x1b\[[34]8;2;(\d+);(\d+);(\d+)m`)

// rgbOf reads the colour of a true-colour SGR sequence.
func rgbOf(t *testing.T, sgr string) [3]float64 {
	t.Helper()
	m := sgrRGB.FindStringSubmatch(sgr)
	if m == nil {
		t.Fatalf("not a true-colour sequence: %q", sgr)
	}
	var c [3]float64
	for i := range c {
		n, _ := strconv.Atoi(m[i+1])
		c[i] = float64(n)
	}
	return c
}

func linear(v float64) float64 {
	v /= 255
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func luminance(c [3]float64) float64 {
	return 0.2126*linear(c[0]) + 0.7152*linear(c[1]) + 0.0722*linear(c[2])
}

// contrast is the WCAG contrast ratio of two colours.
func contrast(a, b [3]float64) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// apart is the CIE76 distance between two colours in Lab: about 2 is the
// least an eye tells apart, and above 10 two colours are not taken for one.
func apart(a, b [3]float64) float64 {
	lab := func(c [3]float64) [3]float64 {
		r, g, bl := linear(c[0]), linear(c[1]), linear(c[2])
		x := (0.4124*r + 0.3576*g + 0.1805*bl) / 0.95047
		y := 0.2126*r + 0.7152*g + 0.0722*bl
		z := (0.0193*r + 0.1192*g + 0.9505*bl) / 1.08883
		f := func(t float64) float64 {
			if t > 0.008856 {
				return math.Cbrt(t)
			}
			return 7.787*t + 16.0/116
		}
		return [3]float64{116*f(y) - 16, 500 * (f(x) - f(y)), 200 * (f(y) - f(z))}
	}
	p, q := lab(a), lab(b)
	return math.Sqrt((p[0]-q[0])*(p[0]-q[0]) + (p[1]-q[1])*(p[1]-q[1]) + (p[2]-q[2])*(p[2]-q[2]))
}

// A selected row is not taken for the cursor, nor a copy's flash for a
// selection, and what is written on each stays readable.
func TestTheSelectionIsToldFromTheCursor(t *testing.T) {
	for name, p := range map[string]palette{"dark": darkPalette, "light": lightPalette} {
		cursor, selection, flash := rgbOf(t, p.cursor), rgbOf(t, p.selection), rgbOf(t, p.flash)
		if d := apart(cursor, selection); d < 10 {
			t.Errorf("%s: the selection is %.1f from the cursor, want at least 10", name, d)
		}
		if d := apart(selection, flash); d < 10 {
			t.Errorf("%s: a copy's flash is %.1f from the selection, want at least 10", name, d)
		}
		for what, bg := range map[string][3]float64{"cursor": cursor, "selection": selection} {
			if c := contrast(rgbOf(t, p.text), bg); c < 7 {
				t.Errorf("%s: text on the %s is %.2f to 1, want 7", name, what, c)
			}
		}
		if c := contrast(rgbOf(t, p.dimSelected), selection); c < 4.5 {
			t.Errorf("%s: dim on the selection is %.2f to 1, want 4.5", name, c)
		}
		if c := contrast(rgbOf(t, p.text), flash); c < 4.5 {
			t.Errorf("%s: text on a copy's flash is %.2f to 1, want 4.5", name, c)
		}
	}
}

// A project's colour never reads as a signal: no colour name stands near
// the alert, the warning or a step of the age gradient, so a project named
// in red is not taken for one overdue.
func TestNoColourNameReadsAsASignal(t *testing.T) {
	for i, p := range []palette{darkPalette, lightPalette} {
		signals := map[string][3]float64{"alert": rgbOf(t, p.alert), "warning": rgbOf(t, p.warn)}
		for step, age := range p.ages {
			signals["age "+strconv.Itoa(step)] = age
		}
		for name, shades := range namedColors {
			c := rgbOf(t, shades[i])
			for what, s := range signals {
				if d := apart(c, s); d < 14 {
					t.Errorf("palette %d: %s is %.1f from the %s, want at least 14", i, name, d, what)
				}
			}
		}
	}
}

// Waiting and Stale say each row's age in a column at the pane's right
// edge, as wide as the widest age, so the ages are read down the list.
// A long row wraps short of it, so the column stays clear on every line,
// and a pane too narrow for both says the age after the text instead.
func TestListedAgesStandInAColumnAtTheRightEdge(t *testing.T) {
	h := two(t) // Two is waiting
	h.clock = h.clock.Add(120 * 24 * time.Hour)
	h.press("G", "o", `"Ask the landlord about the heater and the boiler and the spare keys and the meter $w"`, "esc")
	h.clock = h.clock.Add(5 * 24 * time.Hour)
	h.press("f", `"waiting"`, "enter")
	pane := h.m.width - sideCols
	lines := strings.Split(h.rows(), "\n")
	if lines[0] != h.edge("▌ ◔ Two · Alpha", "125d") || len(lines) < 3 {
		t.Fatalf("screen:\n%s", h.rows())
	}
	if !strings.HasPrefix(lines[1], "  ◔ Ask the landlord") || !strings.HasSuffix(lines[1], strings.Repeat(" ", 4)+"5d") || ansi.StringWidth(lines[1]) != pane {
		t.Fatalf("a shorter age is not right-aligned in the column:\n%s", h.rows())
	}
	for _, l := range lines[2:] {
		if ansi.StringWidth(l) > pane-len("  125d") {
			t.Fatalf("a wrapped line runs into the column:\n%s", h.rows())
		}
	}
	h.m.Update(tea.WindowSizeMsg{Width: 24, Height: 16})
	if rows := h.rows(); !strings.Contains(rows, "Two · 125d · Alpha") {
		t.Fatalf("too narrow for a column, the age follows the text:\n%s", rows)
	}
}

// Review says each project's tasks left and its age in two columns at the
// right edge. One parked during the review has no columns left.
func TestReviewSaysTasksAndAgeInColumns(t *testing.T) {
	h := two(t)
	h.clock = h.clock.Add(15 * 24 * time.Hour)
	h.press(":", `"review"`, "enter")
	h.want(
		h.edge("▌ · Beta", "∅  15d"),
		h.edge("  · Alpha", "2 to do  15d"),
	)
	h.press("$", `"parked"`, "enter")
	h.want(
		"▌ ◌ Beta",
		h.edge("  · Alpha", "2 to do  15d"),
	)
}
