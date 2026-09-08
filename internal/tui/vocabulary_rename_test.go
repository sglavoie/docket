package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func TestVocabularyRenameAxisCoversTheTwoFilterFacetsAndNothingElse(t *testing.T) {
	tags, ok := vocabularyRenameAxisFor(operation.ValueProviderFilterTags)
	if !ok || tags.sigil != "#" || tags.noun != "tag" ||
		tags.namespace != app.TaskTagNamespace || tags.field != models.QueryFieldTag {
		t.Fatalf("tag axis = %#v, ok = %v", tags, ok)
	}
	contexts, ok := vocabularyRenameAxisFor(operation.ValueProviderFilterContexts)
	if !ok || contexts.sigil != "##" || contexts.noun != "context" ||
		contexts.namespace != app.TaskContextNamespace || contexts.field != models.QueryFieldContext {
		t.Fatalf("context axis = %#v, ok = %v", contexts, ok)
	}
	for _, provider := range []operation.ValueProviderID{
		operation.ValueProviderTags,
		operation.ValueProviderProjectTags,
		operation.ValueProviderFilterProjectTags,
		operation.ValueProviderPriority,
	} {
		if _, ok := vocabularyRenameAxisFor(provider); ok {
			t.Fatalf("provider %q has a rename axis", provider)
		}
	}
}

// TestVocabularyRenameHeadingNamesTheSameActTheLedgerDoes pins the four titles
// against the undo actions the application layer writes, which is what makes
// the confirmation, the report, and the entry `u` takes back one act rather
// than three wordings of it.
func TestVocabularyRenameHeadingNamesTheSameActTheLedgerDoes(t *testing.T) {
	tags, _ := vocabularyRenameAxisFor(operation.ValueProviderFilterTags)
	contexts, _ := vocabularyRenameAxisFor(operation.ValueProviderFilterContexts)
	cases := []struct {
		axis  vocabularyRenameAxis
		merge bool
		want  string
	}{
		{tags, false, "Rename Tag"},
		{tags, true, "Merge Tags"},
		{contexts, false, "Rename Context"},
		{contexts, true, "Merge Contexts"},
	}
	for _, tc := range cases {
		if got := vocabularyRenameHeading(tc.axis, tc.merge); got != tc.want {
			t.Fatalf("heading(%s, merge=%v) = %q, want %q", tc.axis.noun, tc.merge, got, tc.want)
		}
	}
}

func TestVocabularyRenameRefusalKeepsTheReasonAndDropsTheLayer(t *testing.T) {
	invalid := &app.InvalidInputError{Field: "tag", Reason: `"backend" already is the destination name`}
	if got := operationRefusalReason(invalid); got != `"backend" already is the destination name` {
		t.Fatalf("refusal = %q, want the reason alone", got)
	}
	if got := operationRefusalReason(fmt.Errorf("wrapped: %w", invalid)); got != invalid.Reason {
		t.Fatalf("wrapped refusal = %q, want the reason alone", got)
	}
	if got := operationRefusalReason(errors.New("the workspace is read-only")); got != "the workspace is read-only" {
		t.Fatalf("plain refusal = %q, want the error itself", got)
	}
}

// TestVocabularyRenameApplyRefusalNamesAnExpiredPlanAsRecoverable holds the one
// distinction the apply step adds: a plan the workspace moved under wrote
// nothing and can simply be reviewed again, and nothing else earns that
// sentence.
func TestVocabularyRenameApplyRefusalNamesAnExpiredPlanAsRecoverable(t *testing.T) {
	const recoverable = "The workspace changed while this was open; press Enter to review the rename again"
	for _, err := range []error{
		storage.ErrStaleRevision,
		storage.ErrStaleArtifact,
		fmt.Errorf("apply: %w", storage.ErrStaleRevision),
	} {
		if got := vocabularyRenameApplyRefusal(err); got != recoverable {
			t.Fatalf("refusal for %v = %q, want the review-again sentence", err, got)
		}
	}
	if got := vocabularyRenameApplyRefusal(errors.New("disk full")); got != "disk full" {
		t.Fatalf("an ordinary failure = %q, want the error itself", got)
	}
}

// TestVocabularyRenameAffectedClausesReportsWhatSelectsDifferently is the whole
// lens-reporting rule: a clause is named when it would admit exactly one of the
// two spellings. Nothing is rewritten, so a clause that admits both or neither
// is silent.
func TestVocabularyRenameAffectedClausesReportsWhatSelectsDifferently(t *testing.T) {
	cases := []struct {
		name  string
		query string
		field models.QueryField
		from  string
		to    string
		want  []string
	}{
		{
			name:  "a full name is answerable",
			query: "#security",
			field: models.QueryFieldTag,
			from:  "security", to: "platform",
			want: []string{"#security"},
		},
		{
			name:  "and so is a prefix that keeps only one of them",
			query: "#sec*",
			field: models.QueryFieldTag,
			from:  "security", to: "platform",
			want: []string{"#sec*"},
		},
		{
			name:  "a negation owns no vocabulary, so it is reported like its twin",
			query: "-#security",
			field: models.QueryFieldTag,
			from:  "security", to: "platform",
			want: []string{"-#security"},
		},
		{
			name:  "a prefix both names satisfy selects them the same way",
			query: "#se*",
			field: models.QueryFieldTag,
			from:  "security", to: "server",
			want: nil,
		},
		{
			name:  "and so does a clause neither name satisfies",
			query: "#backend",
			field: models.QueryFieldTag,
			from:  "security", to: "platform",
			want: nil,
		},
		{
			name:  "a context clause is not a tag clause",
			query: "##security",
			field: models.QueryFieldTag,
			from:  "security", to: "platform",
			want: nil,
		},
		{
			name:  "the context axis reads the same query the other way",
			query: "##security #security",
			field: models.QueryFieldContext,
			from:  "security", to: "platform",
			want: []string{"##security"},
		},
		{
			name:  "one clause is named once however often it is spelled",
			query: "#security #security",
			field: models.QueryFieldTag,
			from:  "security", to: "platform",
			want: []string{"#security"},
		},
		{
			name:  "free text is not a predicate this can answer",
			query: "security",
			field: models.QueryFieldTag,
			from:  "security", to: "platform",
			want: nil,
		},
		{
			name:  "a clause naming only the destination now admits more",
			query: "#platform",
			field: models.QueryFieldTag,
			from:  "security", to: "platform",
			want: []string{"#platform"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := vocabularyRenameAffectedClauses(
				config.NamedLens{Name: "Lens", Query: tc.query}, tc.field, tc.from, tc.to,
			)
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Fatalf("clauses = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestVocabularyRenameLensLinesNamesEveryImpactOrSaysThereIsNone(t *testing.T) {
	if got := vocabularyRenameLensLines(nil); len(got) != 2 || got[0] != "" ||
		got[1] != "No saved lens names it." {
		t.Fatalf("empty report = %#v, want the one-line reassurance", got)
	}
	got := vocabularyRenameLensLines([]vocabularyRenameLensImpact{
		{name: "Security Sweep", clauses: []string{"#security"}},
	})
	want := []string{
		"",
		"1 saved lens may select differently, and it is left unchanged:",
		"  Security Sweep · #security",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("one impact = %#v, want %#v", got, want)
	}
	got = vocabularyRenameLensLines([]vocabularyRenameLensImpact{
		{name: "Security Sweep", clauses: []string{"#security"}},
		{name: "Security Family", clauses: []string{"#sec*", "-#security"}},
	})
	want = []string{
		"",
		"2 saved lenses may select differently, and they are left unchanged:",
		"  Security Sweep · #security",
		"  Security Family · #sec* -#security",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("two impacts = %#v, want %#v", got, want)
	}
}

// The confirmation is split into what stays on screen and what scrolls under
// it, and the split is exactly one cut: every sentence the reader is answering
// for is above it, the unbounded enumeration is below, and the two together
// still read as the report the outcome carries whole.
func TestVocabularyRenamePlanPinsEverySentenceAndScrollsOnlyTheEnumeration(t *testing.T) {
	axis, _ := vocabularyRenameAxisFor(operation.ValueProviderFilterTags)
	impacts := []vocabularyRenameLensImpact{
		{name: "Security Sweep", clauses: []string{"#security"}},
		{name: "Security Family", clauses: []string{"#sec*"}},
	}
	preview := app.RenameTaskNamePreview{
		From:       "security",
		To:         "platform-api",
		Merge:      true,
		Tasks:      []app.TaskLocator{{}, {}},
		ProjectIDs: []string{"alpha"},
	}
	summary := vocabularyRenamePlanSummary(axis, preview, impacts)
	want := []string{
		"#security → #platform-api",
		"2 tasks in 1 project",
		"Scope: every unarchived project, not the filtered view",
		"#platform-api is already in use, so the two become one.",
		"",
		"2 saved lenses may select differently, and they are left unchanged:",
	}
	if strings.Join(summary, "\n") != strings.Join(want, "\n") {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
	rows := vocabularyRenameLensRows(impacts)
	wantRows := []string{"  Security Sweep · #security", "  Security Family · #sec*"}
	if strings.Join(rows, "\n") != strings.Join(wantRows, "\n") {
		t.Fatalf("rows = %#v, want %#v", rows, wantRows)
	}
	// One lens is one row, so the half that scrolls is the half that grows.
	if len(rows) != len(impacts) {
		t.Fatalf("%d impacts produced %d scrollable rows", len(impacts), len(rows))
	}
	joined := append(append([]string{}, vocabularyRenameLensSummary(impacts)...), rows...)
	if strings.Join(joined, "\n") != strings.Join(vocabularyRenameLensLines(impacts), "\n") {
		t.Fatalf("the split halves do not rejoin into the report the outcome carries")
	}
}

// vocabularyRenameSurfaceForTest is a filter-tag list over the given rows, which
// is all the pending-selection remap reads of either picker.
func vocabularyRenameSurfaceForTest(t *testing.T, names ...string) *listsurface.ListSurfaceModel {
	t.Helper()
	items := make([]listsurface.ListItem, 0, len(names))
	for _, name := range names {
		items = append(items, listsurface.ListItem{
			ID:          listsurface.ListItemID("tag:" + name),
			PrimaryText: "#" + name,
			Data:        name,
		})
	}
	return mustListSurface(t, items, nil, listsurface.ListSurfaceConfig{
		SelectionMode: listsurface.ListSelectionMulti,
	})
}

func TestVocabularyRenamePendingSelectionMapsTheSourceOntoTheDestination(t *testing.T) {
	cases := []struct {
		name     string
		before   []string
		after    []string
		selected []listsurface.ListItemID
		want     []listsurface.ListItemID
	}{
		{
			name:     "a pending source becomes the row the rename produced",
			before:   []string{"backend", "urgent"},
			after:    []string{"platform", "urgent"},
			selected: []listsurface.ListItemID{"tag:backend", "tag:urgent"},
			want:     []listsurface.ListItemID{"tag:platform", "tag:urgent"},
		},
		{
			name:     "a merge selecting both names collapses to one row",
			before:   []string{"backend", "platform"},
			after:    []string{"platform"},
			selected: []listsurface.ListItemID{"tag:backend", "tag:platform"},
			want:     []listsurface.ListItemID{"tag:platform"},
		},
		{
			name:     "a row the rename removed is dropped rather than refused",
			before:   []string{"backend", "urgent"},
			after:    []string{"platform"},
			selected: []listsurface.ListItemID{"tag:urgent", "tag:backend"},
			want:     []listsurface.ListItemID{"tag:platform"},
		},
		{
			name:     "nothing pending stays nothing pending",
			before:   []string{"backend"},
			after:    []string{"platform"},
			selected: nil,
			want:     []listsurface.ListItemID{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previous := vocabularyRenameSurfaceForTest(t, tc.before...)
			if len(tc.selected) > 0 {
				if err := previous.SetSelectedIDs(tc.selected); err != nil {
					t.Fatalf("SetSelectedIDs: %v", err)
				}
			}
			got := vocabularyRenamePendingSelection(
				previous, vocabularyRenameSurfaceForTest(t, tc.after...), "tag:backend", "tag:platform",
			)
			if len(got) != len(tc.want) {
				t.Fatalf("pending = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("pending = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
