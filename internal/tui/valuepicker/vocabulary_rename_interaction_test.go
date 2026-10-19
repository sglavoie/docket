package valuepicker

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
)

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

// The name step is prefilled with a name the workspace already stores, so its
// bound is the namespace's own bound and nothing narrower: a stricter field
// would truncate a valid source before the reader touched it, turning an
// unedited Enter into a confirmation to rename onto a prefix nobody typed, and
// would leave a valid long destination unspellable for a merge.
func TestVocabularyRenameSpansTheWholeCanonicalNameLength(t *testing.T) {
	source := strings.Repeat("a", models.MaxTagLength)
	picker := newFilterTagPickerForTest(t, []query.TagInfo{{Name: source, TaskCount: 1}})
	picker.HandleKey(renameKey("e"))

	if got := picker.VocabularyRenameSource(); got != source {
		t.Fatalf("rename source is %d characters, want the whole %d-character name", len(got), len(source))
	}
	if got := picker.VocabularyRenameDestination(); got != source {
		t.Fatalf("prefilled destination is %d characters, want the source untruncated", len(got))
	}

	_, _, msg := picker.HandleKey(renameKey("enter"))
	requested, ok := msg.(VocabularyRenameRequestedMsg)
	if !ok {
		t.Fatalf("enter on an unedited name step = %#v, want a rename request", msg)
	}
	if requested.From != source || requested.To != source {
		t.Fatalf("request = %q → %q, want the same name on both sides so it refuses as unchanged",
			requested.From, requested.To)
	}
}

func TestVocabularyRenameTakesAFullLengthDestinationAndStaysInsideItsBox(t *testing.T) {
	picker := newFilterTagPickerForTest(t, []query.TagInfo{{Name: "backend", TaskCount: 1}})
	picker.HandleKey(renameKey("e"))
	// More keys than the bound admits, so the field's own limit is what decides
	// the length rather than the typing.
	for range models.MaxTagLength {
		picker.HandleKey(renameKey("z"))
	}
	if got := picker.VocabularyRenameDestination(); len(got) != models.MaxTagLength {
		t.Fatalf("destination is %d characters, want the canonical bound of %d",
			len(got), models.MaxTagLength)
	}

	// A field that admits more than a narrow box can draw windows over its value
	// rather than wrapping it, so the long name stays inside the border and costs
	// the step no row a short one would not have cost.
	const narrow = 60
	view := ansi.Strip(picker.RenderWithin(narrow, 24))
	for _, line := range strings.Split(view, "\n") {
		if width := ansi.StringWidth(line); width > narrow {
			t.Fatalf("the name step draws %d columns inside %d:\n%s", width, narrow, view)
		}
	}
	short := newFilterTagPickerForTest(t, []query.TagInfo{{Name: "backend", TaskCount: 1}})
	short.HandleKey(renameKey("e"))
	shortView := ansi.Strip(short.RenderWithin(narrow, 24))
	if long, brief := lipgloss.Height(view), lipgloss.Height(shortView); long != brief {
		t.Fatalf("a %d-character name draws %d rows against %d for a short one:\n%s",
			models.MaxTagLength, long, brief, view)
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
	if !ok || string(refusal) != "the No context row is a facet, not a context" {
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
