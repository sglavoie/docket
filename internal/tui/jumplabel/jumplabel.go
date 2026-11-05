// Package jumplabel is the Outliner's jump mode with nothing of the Outliner in
// it: which labels a set of on-screen rows gets, what a keystroke inside the
// mode means, and how a labelled row is painted. The Outliner owns the rows, the
// window, and the cursor; this package owns only the vocabulary, so the label a
// render paints and the label a key resolves are one function of the same
// inputs and cannot disagree.
package jumplabel

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// Alphabet is the label alphabet in the order labels are handed out: the home
// row first, because the nearest rows are the likeliest destinations and the
// home row is where the fingers already rest, then the top row, which is one
// reach up, then the bottom row, which is the awkward one. Every letter is
// lowercase so a label never asks for Shift, and no digit or symbol is used, so
// no label collides with the keys the mode itself reads (Esc, Backspace).
const Alphabet = "asdfghjklqwertyuiopzxcvbnm"

// Capacity is the most targets one frame can label: every two-letter pair.
const Capacity = len(Alphabet) * len(Alphabet)

// State is the mode's whole memory: whether it is open and the first letter of
// a two-letter label already typed. The targets are never stored; they are the
// rows the last frame drew, read again on every render and every key.
type State struct {
	Active bool
	Prefix string
}

// Open enters the mode with nothing typed.
func (s *State) Open() { *s = State{Active: true} }

// Cancel leaves the mode, writing nothing else.
func (s *State) Cancel() { *s = State{} }

// Order sorts the candidate row indices nearest the cursor first, so the
// easiest labels land on the shortest trips. A tie — one row above and one
// below at the same distance — goes to the row below, the direction the outline
// is read in. The cursor's own row is never a target: jumping to it moves
// nothing.
func Order(candidates []int, cursor int) []int {
	ordered := make([]int, 0, len(candidates))
	for _, row := range candidates {
		if row != cursor {
			ordered = append(ordered, row)
		}
	}
	distance := func(row int) int {
		if row < cursor {
			return 2*(cursor-row) + 1
		}
		return 2 * (row - cursor)
	}
	// Insertion sort: a window is a screenful of rows, and stability keeps the
	// order a pure function of the distances.
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && distance(ordered[j]) < distance(ordered[j-1]); j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	return ordered
}

// Labels returns n labels, all one letter when n fits the alphabet and all two
// letters otherwise. Lengths are never mixed, so a first letter is never both a
// whole label and the start of a longer one. Two-letter labels vary the second
// letter fastest, so the nearest rows share the home-row first letter and the
// first keystroke already narrows to a small, close group. Targets past
// Capacity get no label.
func Labels(n int) []string {
	if n <= 0 {
		return nil
	}
	n = min(n, Capacity)
	labels := make([]string, n)
	if n <= len(Alphabet) {
		for i := range labels {
			labels[i] = Alphabet[i : i+1]
		}
		return labels
	}
	for i := range labels {
		first, second := i/len(Alphabet), i%len(Alphabet)
		labels[i] = string([]byte{Alphabet[first], Alphabet[second]})
	}
	return labels
}

// Assignment maps each target row to its label.
type Assignment map[int]string

// Assign labels the candidate rows for a cursor: Order, then Labels.
func Assign(candidates []int, cursor int) Assignment {
	ordered := Order(candidates, cursor)
	labels := Labels(len(ordered))
	assignment := make(Assignment, len(labels))
	for i, label := range labels {
		assignment[ordered[i]] = label
	}
	return assignment
}

// Outcome is what one key did inside the mode.
type Outcome uint8

const (
	// Pending keeps the mode open: a first letter was typed or taken back.
	Pending Outcome = iota
	// Selected names the row to move to; the mode is closed.
	Selected
	// Canceled closes the mode and moves nothing.
	Canceled
	// Missed closes the mode and moves nothing, because a label letter named no
	// row on screen; the caller says so rather than failing silently.
	Missed
)

// Press applies one key. Esc cancels; Backspace takes back a typed first letter
// and otherwise cancels; a label letter selects or narrows; every other key
// cancels and is consumed, never handed on to the grammar underneath.
func (s *State) Press(key string, assignment Assignment) (Outcome, int) {
	switch key {
	case "backspace":
		if s.Prefix != "" {
			s.Prefix = ""
			return Pending, 0
		}
		s.Cancel()
		return Canceled, 0
	case "esc":
		s.Cancel()
		return Canceled, 0
	}
	if len(key) != 1 || !strings.Contains(Alphabet, key) {
		s.Cancel()
		return Canceled, 0
	}
	typed := s.Prefix + key
	narrows := false
	for row, label := range assignment {
		if label == typed {
			s.Cancel()
			return Selected, row
		}
		if strings.HasPrefix(label, typed) {
			narrows = true
		}
	}
	if narrows {
		s.Prefix = typed
		return Pending, 0
	}
	s.Cancel()
	return Missed, 0
}

// Paint draws label over the first cells of an already rendered row and dims
// the rest of it, keeping the row exactly as wide as it was. An empty label
// dims the whole row. A label that no longer matches the typed prefix is drawn
// dimmed too, so after the first letter only the rows still reachable stand out.
func Paint(line, label, prefix string) string {
	width := lipgloss.Width(line)
	plain := ansi.Strip(line)
	if label == "" || width < len(label) {
		return theme.DimStyle.Render(plain)
	}
	style := theme.HighlightStyle
	if !strings.HasPrefix(label, prefix) {
		style = theme.DimStyle
	}
	rest := dropLeadingCells(plain, len(label))
	if pad := width - len(label) - lipgloss.Width(rest); pad > 0 {
		rest += strings.Repeat(" ", pad)
	}
	return style.Render(label) + theme.DimStyle.Render(rest)
}

// dropLeadingCells removes the first n cells of unstyled text. A wide rune the
// cut lands inside is dropped whole and its surviving half becomes a space, so
// the remainder always starts exactly n cells in.
func dropLeadingCells(plain string, n int) string {
	dropped := 0
	for i, r := range plain {
		if dropped >= n {
			return strings.Repeat(" ", dropped-n) + plain[i:]
		}
		dropped += ansi.StringWidth(string(r))
	}
	return strings.Repeat(" ", max(dropped-n, 0))
}
