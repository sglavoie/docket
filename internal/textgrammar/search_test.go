package textgrammar

import (
	"reflect"
	"slices"
	"strings"
	"testing"
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
