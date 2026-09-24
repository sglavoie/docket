package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

func outlineErrorsFixture(count int) *outliner.OutlineErrorsViewModel {
	problems := make([]grammar.LineError, 0, count)
	for i := range count {
		problems = append(problems, grammar.LineError{
			Line:    i + 1,
			Message: fmt.Sprintf("indent jumps from level 0 to level 2 on problem %d; indent one level at a time", i+1),
		})
	}
	return outliner.NewOutlineErrorsViewModel("Sonar", "/tmp/tt-outline-abc123.md", problems)
}

func TestOutlineErrorsViewNamesEveryProblemAndKeepsTheBuffer(t *testing.T) {
	m := outlineErrorsFixture(3)
	m.SetSize(120, 30)
	rendered := ansi.Strip(m.View().Content)

	for i := 1; i <= 3; i++ {
		if !strings.Contains(rendered, fmt.Sprintf("Line %d: ", i)) {
			t.Errorf("problem %d is missing:\n%s", i, rendered)
		}
	}
	if !strings.Contains(rendered, "Sonar · 3 problems · nothing was changed") {
		t.Errorf("subtitle is missing:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Buffer kept at /tmp/tt-outline-abc123.md") {
		t.Errorf("the buffer path is missing:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Esc close") {
		t.Errorf("the guidance is missing:\n%s", rendered)
	}
}

// TestOutlineErrorsViewFillsExactlyItsHeight is what keeps the buffer path and
// the guidance on screen: the surface is composed above a status bar, so a frame
// even one row too tall silently loses its last rows.
func TestOutlineErrorsViewFillsExactlyItsHeight(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {100, 33}, {60, 18}} {
		for _, count := range []int{1, 5, 40} {
			m := outlineErrorsFixture(count)
			m.SetSize(size[0], size[1])
			height := lipgloss.Height(m.View().Content)
			if height != size[1] {
				t.Errorf("%dx%d with %d problems renders %d rows, want %d", size[0], size[1], count, height, size[1])
			}
		}
	}
}

// TestOutlineErrorsViewScrollsWithoutLosingChrome covers the overflow case: the
// list scrolls, the position is reported, and the path and guidance stay put.
func TestOutlineErrorsViewScrollsWithoutLosingChrome(t *testing.T) {
	m := outlineErrorsFixture(40)
	m.SetSize(120, 20)

	first := ansi.Strip(m.View().Content)
	if !strings.Contains(first, "j/k scroll") {
		t.Fatalf("an overflowing list offers no scroll hint:\n%s", first)
	}
	if !strings.Contains(first, "Line 1: ") {
		t.Fatalf("the list does not start at the top:\n%s", first)
	}

	for range 5 {
		m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	scrolled := ansi.Strip(m.View().Content)
	if strings.Contains(scrolled, "Line 1: ") {
		t.Fatalf("scrolling did not move the window:\n%s", scrolled)
	}
	if !strings.Contains(scrolled, "Buffer kept at /tmp/tt-outline-abc123.md") {
		t.Fatalf("scrolling lost the buffer path:\n%s", scrolled)
	}

	m.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	bottom := ansi.Strip(m.View().Content)
	if !strings.Contains(bottom, "Line 40: ") {
		t.Fatalf("G did not reach the last problem:\n%s", bottom)
	}
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if ansi.Strip(m.View().Content) != bottom {
		t.Fatal("the list scrolled past its last problem")
	}
}

// TestOutlineErrorsSubtitleDegradesInsteadOfClipping keeps the count honest at
// narrow widths: a clipped number reads as a different number.
func TestOutlineErrorsSubtitleDegradesInsteadOfClipping(t *testing.T) {
	m := outlineErrorsFixture(12)
	tests := []struct {
		budget int
		want   string
	}{
		{budget: 80, want: "Sonar · 12 problems · nothing was changed"},
		{budget: 25, want: "Sonar · 12 problems"},
		{budget: 12, want: "12 problems"},
	}
	for _, tt := range tests {
		if got := m.Subtitle(tt.budget); got != tt.want {
			t.Errorf("subtitle(%d) = %q, want %q", tt.budget, got, tt.want)
		}
	}
}

func TestOutlineErrorsSurfaceDoesNotCaptureEsc(t *testing.T) {
	m := outlineErrorsFixture(1)
	if m.ShouldCaptureEsc() {
		t.Fatal("the refusal surface holds no draft, so Esc must unwind the overlay")
	}
}
