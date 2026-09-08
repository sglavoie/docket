package valuepicker

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
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

func TestVocabularyRenameReportDismissesOntoTheBrowseGrammar(t *testing.T) {
	picker := newRenameReportPickerForTest(t, 2)

	if _, dismissed, msg := picker.HandleKey(renameKey("esc")); dismissed || msg != nil {
		t.Fatalf("closing the report left the picker: dismissed = %v, msg = %#v", dismissed, msg)
	}
	if picker.VocabularyRenameReporting() {
		t.Fatal("the report is still retained after Esc")
	}
	view := renameReportView(t, picker)
	if !strings.Contains(view, "backend") {
		t.Fatalf("dismissing the report does not reveal the list:\n%s", view)
	}
	if hints := pickerHints(t, picker); !strings.Contains(hints, "e rename") {
		t.Fatalf("the browse grammar is not back: %q", hints)
	}
}

func TestVocabularyRenameNamingStepTakesEveryPrintableKeyAsInput(t *testing.T) {
	picker := newFilterTagPickerForTest(t, []query.TagInfo{{Name: "backend", TaskCount: 1}})
	picker.HandleKey(renameKey("e"))
	if picker.VocabularyRenameSource() != "backend" {
		t.Fatalf("rename source = %q, want the highlighted row", picker.VocabularyRenameSource())
	}
	if picker.VocabularyRenameDestination() != "backend" {
		t.Fatalf("destination = %q, want the prefilled name", picker.VocabularyRenameDestination())
	}

	// `e` opens the workflow from the browse grammar and is ordinary text inside
	// it; `t` and `c` are the category jumps this very picker binds outside it.
	for _, key := range []string{"e", "t", "c"} {
		if _, dismissed, msg := picker.HandleKey(renameKey(key)); dismissed || msg != nil {
			t.Fatalf("key %q escaped the name step: dismissed = %v, msg = %#v", key, dismissed, msg)
		}
	}
	if got := picker.VocabularyRenameDestination(); got != "backendetc" {
		t.Fatalf("destination = %q, want every key taken literally", got)
	}

	_, _, msg := picker.HandleKey(renameKey("enter"))
	requested, ok := msg.(VocabularyRenameRequestedMsg)
	if !ok {
		t.Fatalf("enter on the name step = %#v, want a rename request", msg)
	}
	if requested.From != "backend" || requested.To != "backendetc" ||
		requested.Provider != operation.ValueProviderFilterTags {
		t.Fatalf("request = %#v, want the picker's own provider and both names", requested)
	}
}

func TestVocabularyRenameStepsPreserveTheBrowseStateUnderneath(t *testing.T) {
	picker := newFilterTagPickerForTest(t, []query.TagInfo{
		{Name: "backend", TaskCount: 1},
		{Name: "platform", TaskCount: 1},
	})
	surface := picker.Surface()
	if err := surface.SetSelectedIDs([]listsurface.ListItemID{"tag:platform"}); err != nil {
		t.Fatalf("SetSelectedIDs: %v", err)
	}
	surface.SetQuery("platform")
	if !surface.SetCursorToID("tag:platform") {
		t.Fatal("the fixture cannot put the cursor on tag:platform")
	}

	picker.HandleKey(renameKey("e"))
	picker.ShowVocabularyRenamePlan(VocabularyRenamePlan{
		Heading: "Rename Tag",
		Lines:   []string{"#platform → #runtime"},
	})
	if view := renameReportView(t, picker); strings.Contains(view, "[x] platform") {
		t.Fatalf("a rename step draws the list underneath it:\n%s", view)
	}
	// n walks back to the name step, esc leaves the workflow entirely.
	picker.HandleKey(renameKey("n"))
	picker.HandleKey(renameKey("esc"))

	if got := surface.Query().Raw; got != "platform" {
		t.Fatalf("query = %q, want the one the reader left", got)
	}
	if got := surface.SelectedIDs(); len(got) != 1 || got[0] != "tag:platform" {
		t.Fatalf("pending choices = %v, want the one the reader left", got)
	}
	if item, ok := surface.CurrentItem(); !ok || item.ID != "tag:platform" {
		t.Fatalf("cursor = %v, want the row the rename started from", item.ID)
	}
}

func TestVocabularyRenameRefusesThePseudoRowAndOffersNoWorkflowElsewhere(t *testing.T) {
	contexts := newFilterContextPickerForTest(t)
	if item, ok := contexts.Surface().CurrentItem(); !ok ||
		string(item.ID) != "filter-context:"+query.NoContextFilterKey {
		t.Fatalf("cursor = %v, want the No context row first", item.ID)
	}
	_, dismissed, msg := contexts.HandleKey(renameKey("e"))
	if dismissed {
		t.Fatal("a refused rename dismissed the picker")
	}
	refusal, ok := msg.(status.StatusMsg)
	if !ok || string(refusal) != "No context is a facet, not a context" {
		t.Fatalf("refusal = %#v, want the pseudo-row refusal", msg)
	}
	if contexts.VocabularyRenameSource() != "" {
		t.Fatalf("a refused rename opened the workflow on %q", contexts.VocabularyRenameSource())
	}

	// The two filter facets are the whole set, so `e` on any other picker is an
	// unbound key rather than a disabled one.
	priorities, err := NewOperationValuePickerModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskSetPriority},
		operation.ValueProviderPriority,
		"Set Priority",
		"Task",
		listsurface.ListSourceFromPriorities([]chrome.PriorityInfo{{Priority: 1, Description: "High"}}),
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("NewOperationValuePickerModel: %v", err)
	}
	if _, dismissed, msg := priorities.HandleKey(renameKey("e")); dismissed || msg != nil {
		t.Fatalf("`e` acted on a picker with no vocabulary: dismissed = %v, msg = %#v", dismissed, msg)
	}
	if hints := pickerHints(t, priorities); strings.Contains(hints, "rename") {
		t.Fatalf("a picker with no vocabulary advertises a rename: %q", hints)
	}
}

func TestVocabularyRenameOffersNothingOnAnEmptyVocabulary(t *testing.T) {
	picker := newFilterTagPickerForTest(t, nil)

	if hints := pickerHints(t, picker); strings.Contains(hints, "rename") {
		t.Fatalf("an empty vocabulary advertises a rename: %q", hints)
	}
	_, dismissed, msg := picker.HandleKey(renameKey("e"))
	if dismissed {
		t.Fatal("`e` on an empty vocabulary dismissed the picker")
	}
	if _, ok := msg.(status.StatusMsg); !ok {
		t.Fatalf("`e` on an empty vocabulary = %#v, want a refusal", msg)
	}
	if picker.VocabularyRenameSource() != "" {
		t.Fatalf("an empty vocabulary opened a rename on %q", picker.VocabularyRenameSource())
	}
}

// newFilterContextPickerForTest opens the context facet over one stored
// context, behind the No context pseudo-row the production source always leads
// with.
func newFilterContextPickerForTest(t *testing.T) *OperationValuePickerModel {
	t.Helper()
	picker, err := NewOperationValuePickerModel(
		operation.OperationExecutionRequest{},
		operation.ValueProviderFilterContexts,
		"Filter by Context",
		"keep matching tasks",
		listsurface.ListSource{Items: []listsurface.ListItem{
			{
				ID:            listsurface.ListItemID("filter-context:" + query.NoContextFilterKey),
				PrimaryText:   "No context",
				SecondaryText: "1 task",
				Data:          query.NoContextFilterKey,
			},
			{
				ID:            "filter-context:platform",
				PrimaryText:   "##platform",
				SecondaryText: "1 task",
				Data:          "platform",
			},
		}},
		true,
		nil,
	)
	if err != nil {
		t.Fatalf("NewOperationValuePickerModel: %v", err)
	}
	return picker
}
