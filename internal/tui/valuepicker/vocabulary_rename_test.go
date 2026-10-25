package valuepicker

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
)

// renameKey spells one key the rename workflow reads. It differs from the
// package's keyPress in the one way this workflow needs: a printable key
// carries its text, because the name step is a real text input and a rune with
// no Text is a key the input receives and types nothing from.
func renameKey(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	default:
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
}

// newRenameReportPickerForTest opens the filter-tag picker over one tag and
// retains a report of the given length on it, which is the state a committed
// rename leaves the rebuilt picker in.
func newRenameReportPickerForTest(t *testing.T, lines int) *OperationValuePickerModel {
	t.Helper()
	picker := newFilterTagPickerForTest(t, []query.TagInfo{{Name: "backend", TaskCount: 1}})
	body := make([]string, 0, lines)
	for i := range lines {
		body = append(body, fmt.Sprintf("line-%d", i))
	}
	picker.AdoptVocabularyRenameOutcome(VocabularyRenameOutcome{Heading: "Rename Tag · done", Lines: body})
	return picker
}

// pickerHints is the footer the picker asks for in its current state, which is
// where a step advertises the keys it added and the browse grammar advertises
// the rename.
func pickerHints(t *testing.T, picker *OperationValuePickerModel) string {
	t.Helper()
	spec, ok := picker.GuidanceSpec()
	if !ok {
		t.Fatalf("the picker declares no guidance")
	}
	parts := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		parts = append(parts, strings.TrimSpace(hint.Key+" "+hint.Label))
	}
	return strings.Join(parts, " · ")
}

func renameReportView(t *testing.T, picker *OperationValuePickerModel) string {
	t.Helper()
	return ansi.Strip(picker.RenderWithin(70, 24))
}

func TestVocabularyRenameReportWindowsALongReportAndScrollsIt(t *testing.T) {
	total := vocabularyRenameReportLines + 3
	picker := newRenameReportPickerForTest(t, total)

	view := renameReportView(t, picker)
	if !strings.Contains(view, "Rename Tag · done") {
		t.Fatalf("report does not name the act it reports:\n%s", view)
	}
	for i := range vocabularyRenameReportLines {
		if !strings.Contains(view, fmt.Sprintf("line-%d", i)) {
			t.Fatalf("report omits windowed line %d:\n%s", i, view)
		}
	}
	if strings.Contains(view, fmt.Sprintf("line-%d", vocabularyRenameReportLines)) {
		t.Fatalf("report shows more than %d lines at once:\n%s", vocabularyRenameReportLines, view)
	}
	if !strings.Contains(view, "… 3 more") {
		t.Fatalf("report does not name what it is holding back:\n%s", view)
	}
	if hints := pickerHints(t, picker); !strings.Contains(hints, "j/k scroll") {
		t.Fatalf("a scrollable report does not advertise the scroll: %q", hints)
	}

	for range 3 {
		if _, dismissed, msg := picker.HandleKey(renameKey("j")); dismissed || msg != nil {
			t.Fatalf("scrolling the report left it: dismissed = %v, msg = %#v", dismissed, msg)
		}
	}
	view = renameReportView(t, picker)
	if !strings.Contains(view, fmt.Sprintf("line-%d", total-1)) {
		t.Fatalf("scrolling to the end does not reach the last line:\n%s", view)
	}
	if strings.Contains(view, "line-0") || strings.Contains(view, "more") {
		t.Fatalf("the end of the report still holds something back:\n%s", view)
	}

	// The window is clamped at both ends, so a scroll nobody can take is simply
	// nothing rather than a report scrolled off itself.
	picker.HandleKey(renameKey("j"))
	if view := renameReportView(t, picker); !strings.Contains(view, fmt.Sprintf("line-%d", total-1)) {
		t.Fatalf("scrolling past the end moved the window:\n%s", view)
	}
	for range total {
		picker.HandleKey(renameKey("k"))
	}
	if view := renameReportView(t, picker); !strings.Contains(view, "line-0") {
		t.Fatalf("scrolling back does not return to the top:\n%s", view)
	}
}

func TestVocabularyRenameShortReportNeitherWindowsNorAdvertisesAScroll(t *testing.T) {
	picker := newRenameReportPickerForTest(t, vocabularyRenameReportLines)

	view := renameReportView(t, picker)
	if strings.Contains(view, "more") {
		t.Fatalf("a report that fits holds nothing back:\n%s", view)
	}
	if hints := pickerHints(t, picker); strings.Contains(hints, "scroll") {
		t.Fatalf("a report that fits advertises a scroll: %q", hints)
	}
	picker.HandleKey(renameKey("j"))
	if view := renameReportView(t, picker); !strings.Contains(view, "line-0") {
		t.Fatalf("a report that fits scrolled anyway:\n%s", view)
	}
}

// newRenameConfirmationPickerForTest opens the name step and mounts a frozen
// plan naming the given number of lenses, which is the state the shell leaves
// the picker in when a rename is waiting to be accepted.
func newRenameConfirmationPickerForTest(t *testing.T, lenses int) *OperationValuePickerModel {
	t.Helper()
	picker := newFilterTagPickerForTest(t, []query.TagInfo{{Name: "backend", TaskCount: 1}})
	picker.HandleKey(renameKey("e"))
	rows := make([]string, 0, lenses)
	for i := range lenses {
		rows = append(rows, fmt.Sprintf("  Lens %d · #backend", i))
	}
	picker.ShowVocabularyRenamePlan(VocabularyRenamePlan{
		Heading: "Rename Tag",
		Summary: []string{
			"#backend → #platform-api",
			"4 tasks in 2 projects",
			"Scope: every unarchived project, not the filtered view",
			"",
			fmt.Sprintf("%d saved lenses may select differently:", lenses),
		},
		Lines: rows,
	})
	return picker
}

// renameStepView paints one step inside a terminal of the given height, which
// is the only place the window height is decided.
func renameStepView(t *testing.T, picker *OperationValuePickerModel, height int) string {
	t.Helper()
	return ansi.Strip(picker.RenderWithin(chrome.MinTerminalWidth, height))
}

// A confirmation names one row per affected lens, which is unbounded, and it is
// read before anything is written. Every one of those rows therefore has to be
// reachable inside the smallest supported terminal, and the sentences the
// reader is actually answering — the destination, the counts, the scope, and
// how many lenses are named — have to stay on screen while they are reached.
func TestVocabularyRenameConfirmationReachesEveryLensInsideTheSmallestTerminal(t *testing.T) {
	const lenses = 20
	picker := newRenameConfirmationPickerForTest(t, lenses)

	view := renameStepView(t, picker, chrome.MinTerminalHeight)
	if height := lipgloss.Height(view); height > chrome.MinTerminalHeight-1 {
		t.Fatalf("the confirmation draws %d rows inside %d:\n%s",
			height, chrome.MinTerminalHeight-1, view)
	}
	if strings.Contains(view, "Lens 19 ·") {
		t.Fatalf("a %d-lens plan fits a %d-row terminal whole, so nothing here is windowed:\n%s",
			lenses, chrome.MinTerminalHeight, view)
	}
	if !strings.Contains(view, "more") {
		t.Fatalf("the confirmation does not name what it is holding back:\n%s", view)
	}
	if hints := pickerHints(t, picker); !strings.Contains(hints, "j/k scroll") ||
		!strings.Contains(hints, "y/Enter apply") || !strings.Contains(hints, "n/Esc back") {
		t.Fatalf("the confirmation drops an answer or the scroll: %q", hints)
	}

	// More presses than the window can take, so the clamp is what stops it.
	for range lenses + 5 {
		if _, dismissed, msg := picker.HandleKey(renameKey("j")); dismissed || msg != nil {
			t.Fatalf("scrolling the confirmation acted: dismissed = %v, msg = %#v", dismissed, msg)
		}
	}
	if !picker.VocabularyRenameConfirming() {
		t.Fatal("scrolling the confirmation left the step it was scrolling")
	}
	view = renameStepView(t, picker, chrome.MinTerminalHeight)
	if !strings.Contains(view, fmt.Sprintf("Lens %d ·", lenses-1)) {
		t.Fatalf("scrolling to the end does not reach the last affected lens:\n%s", view)
	}
	for _, pinned := range []string{
		"#backend → #platform-api", "4 tasks in 2 projects", "Scope: every unarchived",
		fmt.Sprintf("%d saved lenses", lenses),
	} {
		if !strings.Contains(view, pinned) {
			t.Fatalf("scrolling scrolled away %q, which the reader is answering for:\n%s", pinned, view)
		}
	}

	for range lenses + 5 {
		picker.HandleKey(renameKey("k"))
	}
	if view := renameStepView(t, picker, chrome.MinTerminalHeight); !strings.Contains(view, "Lens 0 ·") {
		t.Fatalf("scrolling back does not return to the first affected lens:\n%s", view)
	}
}

// The window is the render's answer, so growing the terminal has to reseat an
// offset the taller window can no longer hold rather than leave the list
// scrolled off its own end.
func TestVocabularyRenameConfirmationReseatsItsWindowOnAResize(t *testing.T) {
	const lenses = 20
	picker := newRenameConfirmationPickerForTest(t, lenses)
	renameStepView(t, picker, chrome.MinTerminalHeight)
	for range lenses + 5 {
		picker.HandleKey(renameKey("j"))
	}

	tall := renameStepView(t, picker, 40)
	if !strings.Contains(tall, fmt.Sprintf("Lens %d ·", lenses-1)) {
		t.Fatalf("a taller terminal lost the end the reader had scrolled to:\n%s", tall)
	}
	if strings.Contains(tall, "more") {
		t.Fatalf("a terminal tall enough for every lens still holds one back:\n%s", tall)
	}
	if hints := pickerHints(t, picker); strings.Contains(hints, "scroll") {
		t.Fatalf("a plan that fits advertises a scroll: %q", hints)
	}
	if short := renameStepView(t, picker, chrome.MinTerminalHeight); !strings.Contains(short, "more") {
		t.Fatalf("shrinking back does not window the plan again:\n%s", short)
	}
}

// Scrolling is not a second route to accepting: the two answers are unchanged
// by it, and neither is reachable from a scroll key.
func TestVocabularyRenameConfirmationScrollDecidesNothing(t *testing.T) {
	picker := newRenameConfirmationPickerForTest(t, 12)
	renameStepView(t, picker, chrome.MinTerminalHeight)
	for range 5 {
		picker.HandleKey(renameKey("j"))
	}
	if view := renameStepView(t, picker, chrome.MinTerminalHeight); strings.Contains(view, "Lens 0 ·") {
		t.Fatalf("j did not move the confirmation's window:\n%s", view)
	}

	_, _, msg := picker.HandleKey(renameKey("y"))
	if _, ok := msg.(VocabularyRenameConfirmedMsg); !ok {
		t.Fatalf("y after a scroll = %#v, want the apply the confirmation promised", msg)
	}

	// And the way back is still the way back, from wherever the reader scrolled to.
	scrolled := newRenameConfirmationPickerForTest(t, 12)
	renameStepView(t, scrolled, chrome.MinTerminalHeight)
	for range 5 {
		scrolled.HandleKey(renameKey("j"))
	}
	_, _, msg = scrolled.HandleKey(renameKey("n"))
	if _, ok := msg.(VocabularyRenameAbandonedMsg); !ok {
		t.Fatalf("n after a scroll = %#v, want the plan released", msg)
	}
	if scrolled.VocabularyRenameConfirming() {
		t.Fatal("the confirmation is still open after stepping back")
	}
	if got := scrolled.VocabularyRenameDestination(); got != "backend" {
		t.Fatalf("the name step returned holding %q, want the name the reader typed", got)
	}
}
