package textgrammar

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseSearchTokenizesDueHourWindow(t *testing.T) {
	parsed := ParseSearch("@+4h ship it")
	if parsed.Text != "ship it" {
		t.Fatalf("Text = %q, want the free-text remainder only", parsed.Text)
	}
	if len(parsed.Tokens) != 1 || parsed.Tokens[0].Field != FieldDue || parsed.Tokens[0].Value != "+4h" {
		t.Fatalf("tokens = %+v, want a single @+4h due token", parsed.Tokens)
	}
	if guidance := SearchGuidance(parsed); guidance != "" {
		t.Fatalf("SearchGuidance = %q, want a runnable query", guidance)
	}

	// The hour window needs the leading "+"; a bare "4h" is neither a calendar
	// day nor the offset form, so "@4h" stays prose.
	if bare := ParseSearch("@4h soon"); len(bare.Tokens) != 0 || bare.Text != "@4h soon" {
		t.Errorf("ParseSearch(@4h) = %+v, want the whole input as prose", bare)
	}
}

func TestParseSearchSeparatesPredicatesFromFreeText(t *testing.T) {
	parsed := ParseSearch("  #backend  @today   auth flow  *pin ")
	if parsed.Text != "auth flow" {
		t.Fatalf("Text = %q, want the free-text remainder only", parsed.Text)
	}
	got := make(map[Field]string, len(parsed.Tokens))
	for _, token := range parsed.Tokens {
		got[token.Field] = token.Value
	}
	want := map[Field]string{FieldTag: "backend", FieldDue: "today", FieldPinned: "pin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %+v, want %+v", got, want)
	}
	if guidance := SearchGuidance(parsed); guidance != "" {
		t.Fatalf("SearchGuidance = %q, want a runnable query", guidance)
	}
}

func TestParseSearchLeavesUnrecognizedShapesAsProse(t *testing.T) {
	for _, input := range []string{
		"find @sarah",
		"ship !important work",
		"costs $100 total",
		"read @0 again",
		"pick !0 branch",
	} {
		parsed := ParseSearch(input)
		if len(parsed.Tokens) != 0 || parsed.Text != input {
			t.Errorf("ParseSearch(%q) = %+v, want the whole input as prose", input, parsed)
		}
		if guidance := SearchGuidance(parsed); guidance != "" {
			t.Errorf("ParseSearch(%q) guidance = %q, want none", input, guidance)
		}
	}
}

func TestParseSearchTokenizesRecognizedAndWrongValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
		field Field
		want  string
	}{
		{"numeric priority outside the range", "!9 crash", FieldPriority, "!9"},
		{"retired priority", "!4 crash", FieldPriority, "!4"},
		{"retired destination", "$ready work", FieldDestination, "$backlog"},
		{"reserved context name", "##0 work", FieldContext, "clears a context"},
		{"parent placement", "fix >4", FieldParent, "placement is set with e"},
		{"child placement", "fix <4", FieldChild, "placement is set with e"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseSearch(tt.input)
			guidance := SearchGuidance(parsed)
			if guidance == "" {
				t.Fatalf("ParseSearch(%q) = %+v, want guidance", tt.input, parsed)
			}
			if !strings.Contains(guidance, tt.want) {
				t.Fatalf("guidance = %q, want it to mention %q", guidance, tt.want)
			}
			if !slices.ContainsFunc(parsed.Diagnostics, func(d Diagnostic) bool { return d.Field == tt.field }) {
				t.Fatalf("diagnostics = %+v, want one for %s", parsed.Diagnostics, tt.field)
			}
		})
	}
}

func TestParseSearchDuePresetsAreSearchOnly(t *testing.T) {
	if want := []string{"overdue", "today", "week", "none"}; !reflect.DeepEqual(searchDuePresetNames, want) {
		t.Fatalf("searchDuePresetNames = %v, want %v", searchDuePresetNames, want)
	}

	for _, preset := range searchDuePresetNames {
		parsed := ParseSearch("@" + preset)
		tokens := parsed.TokensFor(FieldDue)
		if len(tokens) != 1 || tokens[0].Value != preset {
			t.Errorf("@%s tokens = %+v, want one due token", preset, tokens)
		}
	}

	capture, ok := DefinitionFor(SurfaceCapture, FieldDue)
	if !ok {
		t.Fatal("capture due definition is missing")
	}
	if slices.Contains(capture.Values, "overdue") {
		t.Error("capture advertises @overdue; the relational presets are search-only")
	}
	if tokens := Parse("@overdue", SurfaceCapture).TokensFor(FieldDue); len(tokens) != 1 {
		t.Fatalf("capture @overdue tokens = %+v, want one so capture can refuse it by name", tokens)
	}
}

func TestParseSearchProjectAndTagAxesStaySeparate(t *testing.T) {
	parsed := ParseSearch("%core #backend ##client auth")
	if parsed.Text != "auth" {
		t.Fatalf("Text = %q, want auth", parsed.Text)
	}
	for field, want := range map[Field]string{
		FieldProject: "core",
		FieldTag:     "backend",
		FieldContext: "client",
	} {
		tokens := parsed.TokensFor(field)
		if len(tokens) != 1 || tokens[0].Value != want {
			t.Errorf("%s tokens = %+v, want %q", field, tokens, want)
		}
	}

	qualifier := ParseSearch("alpha#7")
	if len(qualifier.Tokens) != 0 || qualifier.Text != "alpha#7" {
		t.Fatalf("ParseSearch(alpha#7) = %+v, want the exact project qualifier left intact", qualifier)
	}
}

// A repeated project scope in search is a union, the way a repeated context
// is, so the chips the f% facet spells type back into search unrefused.
func TestParseSearchRepeatedProjectIsAUnionNotAConflict(t *testing.T) {
	parsed := ParseSearch("%alpha %beta ship")
	if got := len(parsed.TokensFor(FieldProject)); got != 2 {
		t.Fatalf("project tokens = %d, want 2", got)
	}
	if guidance := SearchGuidance(parsed); guidance != "" {
		t.Fatalf("SearchGuidance = %q, want a runnable union", guidance)
	}
}

// TestParseSearchTokenizesEveryNegation walks the negation set in one pass: each
// spelling must peel whole, land on its own field, and leave the free text
// alone. The two-and three-rune triggers are the point — -@@ must not be read as
// -@ with an @ value, and -## must not be read as -# with a #-prefixed tag.
func TestParseSearchTokenizesEveryNegation(t *testing.T) {
	tests := []struct {
		input string
		field Field
		want  string
	}{
		{"-!2 ship", FieldRemovePriority, "2"},
		{"-#backend ship", FieldRemoveTag, "backend"},
		{"-##client ship", FieldRemoveContext, "client"},
		{"-@today ship", FieldRemoveDue, "today"},
		{"-@@tomorrow ship", FieldRemoveDefer, "tomorrow"},
		{"-$done ship", FieldRemoveDestination, "done"},
		{"-*pin ship", FieldRemovePinned, "pin"},
		{"-%core ship", FieldRemoveProject, "core"},
		{"-~7 ship", FieldRemoveDependency, "7"},
		{"-?stale ship", FieldRemoveDerived, "stale"},
	}
	for _, tt := range tests {
		t.Run(string(tt.field), func(t *testing.T) {
			parsed := ParseSearch(tt.input)
			if parsed.Text != "ship" {
				t.Errorf("Text = %q, want the free-text remainder only", parsed.Text)
			}
			tokens := parsed.TokensFor(tt.field)
			if len(tokens) != 1 || tokens[0].Value != tt.want {
				t.Fatalf("tokens = %+v, want one %s token valued %q", parsed.Tokens, tt.field, tt.want)
			}
			if guidance := SearchGuidance(parsed); guidance != "" {
				t.Errorf("SearchGuidance = %q, want a runnable query", guidance)
			}
		})
	}
}

// TestNegationsReachNoOtherSurface is the surface half of the same contract: a
// negation that subtracts from a result set must not become a mutation a task
// can carry. The two that do subtract from a task, -# and -~, are the exception
// and are named here rather than skipped by a rule.
func TestNegationsReachNoOtherSurface(t *testing.T) {
	mutating := map[Field]bool{FieldRemoveTag: true, FieldRemoveDependency: true}
	for _, spec := range registry {
		if spec.negates == "" || mutating[spec.Field] {
			continue
		}
		if spec.Surfaces != SurfaceSearch {
			t.Errorf("%s reaches %v, want SurfaceSearch alone", spec.Field, spec.Surfaces)
		}
		if tokens := Parse(spec.Trigger+"1", SurfaceCapture).TokensFor(spec.Field); len(tokens) != 0 {
			t.Errorf("%s tokenized while capturing: %+v", spec.Field, tokens)
		}
	}
}

// TestParseSearchDerivedValuesAreTheClosedSet holds ? to the canonical property
// list and to the one qualifier that exists. A name outside the set is left as
// prose, the way !important is: the sigil consumes only what it can answer, so
// the values that do block a query are the recognized ones it cannot honor.
func TestParseSearchDerivedValuesAreTheClosedSet(t *testing.T) {
	definition, ok := DefinitionFor(SurfaceSearch, FieldDerived)
	if !ok {
		t.Fatal("search derived definition is missing")
	}
	if !slices.Contains(definition.Values, string(models.DerivedReview)) {
		t.Fatalf("derived completion values = %v, want review attention", definition.Values)
	}
	if !strings.Contains(definition.Description, "current review attention") {
		t.Fatalf("derived help description = %q, want current review attention named", definition.Description)
	}

	for _, name := range derivedNames {
		parsed := ParseSearch("?" + name)
		tokens := parsed.TokensFor(FieldDerived)
		if len(tokens) != 1 || tokens[0].Value != name {
			t.Errorf("?%s tokens = %+v, want one derived token", name, parsed.Tokens)
		}
		if guidance := SearchGuidance(parsed); guidance != "" {
			t.Errorf("?%s guidance = %q, want a runnable query", name, guidance)
		}
	}

	if parsed := ParseSearch("?completed:90d"); SearchGuidance(parsed) != "" {
		t.Errorf("?completed:90d guidance = %q, want a runnable query", SearchGuidance(parsed))
	}
	for input, field := range map[string]Field{
		"?review":  FieldDerived,
		"-?review": FieldRemoveDerived,
	} {
		parsed := ParseSearch(input)
		tokens := parsed.TokensFor(field)
		if len(tokens) != 1 || tokens[0].Value != string(models.DerivedReview) {
			t.Errorf("ParseSearch(%s) = %+v, want review attention on %s", input, parsed, field)
		}
	}
	if prose := ParseSearch("?nonsense"); len(prose.Tokens) != 0 || prose.Text != "?nonsense" {
		t.Errorf("ParseSearch(?nonsense) = %+v, want the whole input as prose", prose)
	}
	for input, want := range map[string]string{
		"?stale:90d":       "?stale takes no value",
		"?completed:90":    "invalid completion age",
		"?completed:0d":    "invalid completion age",
		"-?completed:ever": "invalid completion age",
	} {
		if guidance := SearchGuidance(ParseSearch(input)); !strings.Contains(guidance, want) {
			t.Errorf("%s guidance = %q, want it to mention %q", input, guidance, want)
		}
	}
}

// TestParseSearchPriorityThresholdIsSearchOnly holds the "+" suffix to the one
// surface that can widen a match. Capture and edit assign a priority, and "and
// above" is not a value a task can carry.
func TestParseSearchPriorityThresholdIsSearchOnly(t *testing.T) {
	parsed := ParseSearch("!2+ ship")
	tokens := parsed.TokensFor(FieldPriority)
	if len(tokens) != 1 || tokens[0].Value != "2+" {
		t.Fatalf("tokens = %+v, want one !2+ priority token", parsed.Tokens)
	}
	if guidance := SearchGuidance(parsed); guidance != "" {
		t.Fatalf("SearchGuidance = %q, want a runnable query", guidance)
	}

	if admitted, ok := ParseSearchPriority("2+"); !ok || !reflect.DeepEqual(admitted, []int{1, 2}) {
		t.Errorf("ParseSearchPriority(2+) = %v %v, want [1 2]", admitted, ok)
	}
	if admitted, ok := ParseSearchPriority("2"); !ok || !reflect.DeepEqual(admitted, []int{2}) {
		t.Errorf("ParseSearchPriority(2) = %v %v, want [2]", admitted, ok)
	}
	if _, ok := ParseSearchPriority("9+"); ok {
		t.Error("ParseSearchPriority(9+) succeeded; the threshold does not widen the legal range")
	}

	// Capture keeps the strict reading: a priority is assigned, so "2+" is not a
	// value it can honor, and the token is raised rather than silently accepted.
	if _, ok := ParsePriority("2+", false); ok {
		t.Error("ParsePriority(2+) succeeded; the threshold belongs to search alone")
	}
}
