package jumplabel

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLabelsAreAllOneLetterUntilTheAlphabetRunsOut(t *testing.T) {
	single := Labels(len(Alphabet))
	if len(single) != len(Alphabet) {
		t.Fatalf("labels = %d, want %d", len(single), len(Alphabet))
	}
	for i, label := range single {
		if label != Alphabet[i:i+1] {
			t.Fatalf("label %d = %q, want %q", i, label, Alphabet[i:i+1])
		}
	}
	if got := strings.Join(single[:9], ""); got != "asdfghjkl" {
		t.Fatalf("first labels = %q, want the home row", got)
	}

	double := Labels(len(Alphabet) + 1)
	seen := map[string]bool{}
	for _, label := range double {
		if len(label) != 2 {
			t.Fatalf("label %q: lengths are mixed past the alphabet", label)
		}
		if seen[label] {
			t.Fatalf("label %q handed out twice", label)
		}
		seen[label] = true
	}
	if double[0] != "aa" || double[1] != "as" || double[len(Alphabet)] != "sa" {
		t.Fatalf("two-letter order = %v, want the second letter varying fastest", double[:3])
	}
	if got := len(Labels(Capacity + 10)); got != Capacity {
		t.Fatalf("labels past capacity = %d, want %d", got, Capacity)
	}
	if Labels(0) != nil {
		t.Fatal("zero targets were labelled")
	}
}

func TestOrderPutsTheNearestRowsFirstAndSkipsTheCursor(t *testing.T) {
	got := Order([]int{0, 1, 2, 3, 4, 5, 6}, 3)
	want := []int{4, 2, 5, 1, 6, 0}
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
	assignment := Assign([]int{0, 1, 2, 3, 4, 5, 6}, 3)
	if _, labelled := assignment[3]; labelled {
		t.Fatal("the cursor row took a label")
	}
	if assignment[4] != "a" || assignment[2] != "s" {
		t.Fatalf("assignment = %v, want the nearest rows on the home row", assignment)
	}
}

func TestPressSelectsNarrowsUndoesAndCancels(t *testing.T) {
	rows := make([]int, 30)
	for i := range rows {
		rows[i] = i + 1
	}
	assignment := Assign(rows, 0)
	target := -1
	for row, label := range assignment {
		if label == "sd" {
			target = row
		}
	}

	var s State
	s.Open()
	if outcome, _ := s.Press("s", assignment); outcome != Pending || s.Prefix != "s" {
		t.Fatalf("first letter: outcome %d prefix %q", outcome, s.Prefix)
	}
	if outcome, _ := s.Press("backspace", assignment); outcome != Pending || s.Prefix != "" || !s.Active {
		t.Fatalf("backspace: outcome %d state %+v, want the first letter taken back", outcome, s)
	}
	s.Press("s", assignment)
	if outcome, row := s.Press("d", assignment); outcome != Selected || row != target || s.Active {
		t.Fatalf("second letter: outcome %d row %d state %+v, want row %d selected", outcome, row, s, target)
	}

	for _, key := range []string{"esc", "Q", "/", "ctrl+a", "enter", "1"} {
		s.Open()
		if outcome, _ := s.Press(key, assignment); outcome != Canceled || s.Active {
			t.Errorf("key %q: outcome %d state %+v, want a silent cancel", key, outcome, s)
		}
	}
	s.Open()
	if outcome, _ := s.Press("backspace", assignment); outcome != Canceled {
		t.Errorf("backspace with nothing typed: outcome %d, want cancel", outcome)
	}
	// Thirty targets use two letters drawn from the first two first letters, so
	// a first letter beyond them names nothing on screen.
	s.Open()
	if outcome, _ := s.Press("m", assignment); outcome != Missed || s.Active {
		t.Errorf("unused first letter: outcome %d, want a miss", outcome)
	}
}

func TestPaintKeepsTheRowWidthExactly(t *testing.T) {
	for _, line := range []string{
		lipgloss.NewStyle().Bold(true).Render("├── ◐ Task 01") + "      ",
		"日本語のタスク  ",
		"x",
		"",
	} {
		width := lipgloss.Width(line)
		for _, label := range []string{"", "a", "as"} {
			painted := Paint(line, label, "")
			if got := lipgloss.Width(painted); got != width {
				t.Errorf("Paint(%q, %q) width = %d, want %d", line, label, got, width)
			}
			if label != "" && width >= len(label) && !strings.HasPrefix(ansi.Strip(painted), label) {
				t.Errorf("Paint(%q, %q) = %q, want the label first", line, label, ansi.Strip(painted))
			}
		}
	}
}
