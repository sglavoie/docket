package textgrammar

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/util"
)

func TestParseUsesSharedSurfaceGrammar(t *testing.T) {
	parsed := Parse(
		"Build API ^bug !1 #backend @tomorrow ~weekly >1 <2 %core $backlog",
		SurfaceCapture,
	)
	if parsed.Text != "Build API" {
		t.Fatalf("Text = %q, want Build API", parsed.Text)
	}
	for _, field := range []Field{
		FieldTaskType, FieldPriority, FieldTag, FieldDue, FieldRecurrence, FieldParent,
		FieldChild, FieldProject, FieldDestination,
	} {
		if len(parsed.TokensFor(field)) == 0 {
			t.Errorf("missing %s token in %+v", field, parsed.Tokens)
		}
	}
}

func TestSharedParserAdvertisesExactlyTwelveCaptureSigils(t *testing.T) {
	parsed := Parse(
		"Keep type:bug priority:1 pri:2 tag:backend due:tomorrow recur:weekly project:core proj:core p:core parent:1 child:1 children:1 state:ready has:due no:tag is:pinned",
		SurfaceCapture|SurfaceEdit,
	)
	if len(parsed.Tokens) != 0 {
		t.Fatalf("legacy forms produced tokens: %+v", parsed.Tokens)
	}
	if parsed.Text == "" {
		t.Fatal("legacy forms did not remain literal text")
	}

	defs := Definitions(SurfaceCapture)
	if len(defs) != 12 {
		t.Fatalf("capture definitions = %d, want 12: %+v", len(defs), defs)
	}
	// Parse order, not teach order: ;comment; leads because nothing else may peel
	// a sigil out of a comment's span, and &link sits ahead of #tag because the
	// tag pattern would otherwise hand it a whitespace-preceded &.
	wantForms := []string{";comment;", "&link", "!1-3", "#tag", "##context", "@date", "@@defer", "$destination", "*pin", "%project", ">parent", "<child"}
	gotForms := make([]string, len(defs))
	for i, def := range defs {
		if len(def.Forms) != 1 {
			t.Fatalf("definition %s forms = %v, want exactly one canonical form", def.Field, def.Forms)
		}
		gotForms[i] = def.Forms[0]
	}
	if !reflect.DeepEqual(gotForms, wantForms) {
		t.Fatalf("capture forms = %v, want %v", gotForms, wantForms)
	}
}

func TestRetiredLifecycleFormsRemainLiteralText(t *testing.T) {
	parsed := Parse("Build API /next status:todo", SurfaceCapture)
	if parsed.Text != "Build API /next status:todo" || len(parsed.Tokens) != 0 {
		t.Fatalf("retired forms were parsed: %+v", parsed)
	}
}

func TestParseNamespacedTagsAcrossSupportedSurfaces(t *testing.T) {
	tests := []struct {
		name    string
		surface Surface
		input   string
		field   Field
		want    []string
	}{
		{
			name:    "capture",
			surface: SurfaceCapture,
			input:   "Fix parser #type:bug #source:github",
			field:   FieldTag,
			want:    []string{"type:bug", "source:github"},
		},
		{
			name:    "edit add",
			surface: SurfaceEdit,
			input:   "#source:github",
			field:   FieldTag,
			want:    []string{"source:github"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := Parse(tt.input, tt.surface)
			tokens := parsed.TokensFor(tt.field)
			got := make([]string, len(tokens))
			for i := range tokens {
				got[i] = tokens[i].Value
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("tag values = %v, want %v (parsed=%+v)", got, tt.want, parsed)
			}
			if parsed.Text != "" && tt.input != "Fix parser #type:bug #source:github" {
				t.Fatalf("Text = %q, want empty", parsed.Text)
			}
			if tt.input == "Fix parser #type:bug #source:github" && parsed.Text != "Fix parser" {
				t.Fatalf("Text = %q, want Fix parser", parsed.Text)
			}
		})
	}
}

func TestParseContextDoesNotCollideWithTagTrigger(t *testing.T) {
	parsed := Parse("##frontend Fix bug #ui", SurfaceCapture)
	if parsed.Text != "Fix bug" {
		t.Fatalf("Text = %q, want Fix bug", parsed.Text)
	}
	contexts := parsed.TokensFor(FieldContext)
	tags := parsed.TokensFor(FieldTag)
	if len(contexts) != 1 || contexts[0].Value != "frontend" {
		t.Fatalf("context tokens = %+v, want frontend", contexts)
	}
	if len(tags) != 1 || tags[0].Value != "ui" {
		t.Fatalf("tag tokens = %+v, want ui", tags)
	}

	parsed = Parse("Keep a##b and ## bare", SurfaceCapture)
	if len(parsed.TokensFor(FieldContext)) != 0 || parsed.Text != "Keep a##b and ## bare" {
		t.Fatalf("non-token context forms were consumed: %+v", parsed)
	}
}

func TestParseRejectsMalformedNamespacedTagsWithoutPartialConsumption(t *testing.T) {
	for _, input := range []string{"Fix #type:", "Fix #type::bug"} {
		parsed := Parse(input, SurfaceCapture)
		if len(parsed.Tokens) != 0 || parsed.Text != input {
			t.Errorf("Parse(%q) = %+v, want literal text", input, parsed)
		}
	}
}

func TestParseNamespacedTagsPreservesExistingPunctuationBoundary(t *testing.T) {
	parsed := Parse("Fix #type:bug, soon", SurfaceCapture)
	tokens := parsed.TokensFor(FieldTag)
	if len(tokens) != 1 || tokens[0].Value != "type:bug" || parsed.Text != "Fix , soon" {
		t.Fatalf("Parse punctuation boundary = %+v", parsed)
	}
}

func TestParsePriority(t *testing.T) {
	if got, ok := ParsePriority("high", false); ok || got != nil {
		t.Fatalf("ParsePriority(high) = %v, %v; want rejection", got, ok)
	}
	if got, ok := ParsePriority("3-1", true); !ok || !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("ParsePriority(3-1) = %v, %v", got, ok)
	}
	for _, retired := range []string{"4", "5", "1-4"} {
		if got, ok := ParsePriority(retired, true); ok || got != nil {
			t.Errorf("ParsePriority(%q) = %v, %v; want rejection", retired, got, ok)
		}
	}
}

func TestPriorityTokenSurfaceBoundary(t *testing.T) {
	capture := Parse("Task !0 !1 !4", SurfaceCapture)
	if got := capture.TokensFor(FieldPriority); len(got) != 2 || got[0].Value != "1" || got[1].Value != "4" {
		t.Fatalf("capture priority tokens = %+v, want !1 plus retired !4 for error guidance", got)
	}
	if capture.Text != "Task !0" {
		t.Fatalf("capture text = %q, want mutation-only !0 preserved", capture.Text)
	}

	edit := Parse("Task !0 !3 !4", SurfaceEdit)
	if got := edit.TokensFor(FieldPriority); len(got) != 3 || got[0].Value != "0" || got[1].Value != "3" || got[2].Value != "4" {
		t.Fatalf("edit priority tokens = %+v, want !0 and !3 plus retired !4", got)
	}
	if edit.Text != "Task" {
		t.Fatalf("edit text = %q, want priority tokens extracted", edit.Text)
	}

	search := Parse("Task !0 !1 !9 !important", SurfaceSearch)
	if got := search.TokensFor(FieldPriority); len(got) != 2 || got[0].Value != "1" || got[1].Value != "9" {
		t.Fatalf("search priority tokens = %+v, want !1 plus numeric !9 for guidance", got)
	}
	if search.Text != "Task !0 !important" {
		t.Fatalf("search text = %q, want the clear form and the prose word left literal", search.Text)
	}
}

func TestCaptureDestinationNamesAreExactAndOrdered(t *testing.T) {
	want := []string{"inbox", "backlog", "today", "next", "doing", "waiting", "paused", "someday", "done", "canceled"}
	definition, ok := DefinitionFor(SurfaceCapture, FieldDestination)
	if !ok {
		t.Fatal("capture destination definition is missing")
	}
	if got := definition.Values; !reflect.DeepEqual(got, want) {
		t.Fatalf("capture destination values = %v, want %v", got, want)
	}
}

func TestRegistryDefinitionsCoverCanonicalSurfaces(t *testing.T) {
	tests := []struct {
		name    string
		surface Surface
		want    map[Field][]string
	}{
		{
			name: "capture", surface: SurfaceCapture,
			want: map[Field][]string{
				FieldPriority: {"!1-3"}, FieldTag: {"#tag"}, FieldContext: {"##context"}, FieldDue: {"@date"},
				FieldDefer:       {"@@defer"},
				FieldDestination: {"$destination"}, FieldPinned: {"*pin"},
				FieldProject: {"%project"}, FieldParent: {">parent"},
				FieldChild: {"<child"}, FieldAttachment: {"&link"}, FieldComment: {";comment;"},
			},
		},
		{
			name: "edit", surface: SurfaceEdit,
			want: map[Field][]string{
				FieldPriority: {"!1-3", "!0"}, FieldTag: {"#tag"}, FieldRemoveTag: {"-#tag"},
				FieldContext: {"##context", "##0"},
				FieldDue:     {"@date", "@0"},
				FieldDefer:   {"@@defer", "@@0"}, FieldDestination: {"$destination"},
				FieldPinned:  {"*pin", "*unpin"},
				FieldProject: {"%project"}, FieldParent: {">parent", ">0"},
				FieldChild: {"<child"},
			},
		},
		{
			name: "outline", surface: SurfaceOutline,
			want: map[Field][]string{
				FieldPriority: {"!1-3"}, FieldTag: {"#tag"}, FieldContext: {"##context"}, FieldDue: {"@date"},
				FieldDestination: {"$destination"}, FieldPinned: {"*pin"},
			},
		},
		{
			name: "search", surface: SurfaceSearch,
			want: map[Field][]string{
				FieldPriority: {"!1-3"}, FieldTag: {"#tag"}, FieldRemoveTag: {"-#tag"},
				FieldContext: {"##context"}, FieldDue: {"@date"}, FieldDefer: {"@@defer"},
				FieldDestination: {"$destination"}, FieldPinned: {"*pin", "*unpin"},
				FieldProject: {"%project"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs := Definitions(tt.surface)
			if len(defs) != len(tt.want) {
				t.Fatalf("definitions = %d, want %d: %+v", len(defs), len(tt.want), defs)
			}
			for _, def := range defs {
				forms, ok := tt.want[def.Field]
				if !ok {
					t.Errorf("unexpected field %s", def.Field)
					continue
				}
				if !reflect.DeepEqual(def.Forms, forms) {
					t.Errorf("%s forms = %v, want %v", def.Field, def.Forms, forms)
				}
				if def.Trigger == "" || def.Description == "" {
					t.Errorf("%s is missing trigger/help metadata: %+v", def.Field, def)
				}
			}
		})
	}
}

func TestParseRecognizesOnlyRegistryFormsForEachSurface(t *testing.T) {
	const input = "Task !1 #add -#old @tomorrow $next *pin %core >1 <2"
	tests := []struct {
		name     string
		surface  Surface
		wantText string
		fields   []Field
	}{
		{"capture", SurfaceCapture, "Task -#old", []Field{FieldPriority, FieldTag, FieldDue, FieldDestination, FieldPinned, FieldProject, FieldParent, FieldChild}},
		{"edit", SurfaceEdit, "Task", []Field{FieldPriority, FieldTag, FieldRemoveTag, FieldDue, FieldDestination, FieldPinned, FieldProject, FieldParent, FieldChild}},
		{"outline", SurfaceOutline, "Task -#old %core >1 <2", []Field{FieldPriority, FieldTag, FieldDue, FieldDestination, FieldPinned}},
		{"search", SurfaceSearch, "Task >1 <2", []Field{FieldPriority, FieldTag, FieldRemoveTag, FieldDue, FieldDestination, FieldPinned, FieldProject}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := Parse(input, tt.surface)
			if parsed.Text != tt.wantText {
				t.Fatalf("Text = %q, want %q", parsed.Text, tt.wantText)
			}
			for _, field := range tt.fields {
				if len(parsed.TokensFor(field)) == 0 {
					t.Errorf("missing %s token in %+v", field, parsed.Tokens)
				}
			}
		})
	}
}

func TestRegistryClearSemanticsAreSurfaceSpecific(t *testing.T) {
	for _, field := range []Field{FieldPriority, FieldDue, FieldParent} {
		capture, ok := DefinitionFor(SurfaceCapture, field)
		if !ok || len(capture.Clears) != 0 {
			t.Errorf("capture %s clears = %+v, want none", field, capture.Clears)
		}
		edit, ok := DefinitionFor(SurfaceEdit, field)
		if !ok || len(edit.Clears) != 1 || edit.Clears[0].Value != "0" {
			t.Errorf("edit %s clears = %+v, want one zero clear", field, edit.Clears)
		}
	}
	for _, field := range []Field{FieldPriority, FieldDue, FieldContext} {
		search, ok := DefinitionFor(SurfaceSearch, field)
		if !ok || len(search.Clears) != 0 {
			t.Errorf("search %s clears = %+v, want none; a clear is a mutation", field, search.Clears)
		}
	}
	if _, ok := DefinitionFor(SurfaceSearch, FieldParent); ok {
		t.Error("search advertises parent; placement is not a search predicate")
	}
	if _, ok := DefinitionFor(SurfaceSearch, FieldChild); ok {
		t.Error("search advertises child; placement is not a search predicate")
	}
}

func TestSearchDefinitionsDescribePredicatesRatherThanMutations(t *testing.T) {
	for _, def := range Definitions(SurfaceSearch) {
		mutation, ok := DefinitionFor(SurfaceEdit, def.Field)
		if !ok {
			t.Fatalf("search field %s is missing from edit", def.Field)
		}
		if def.Description == "" || def.Description == mutation.Description {
			t.Errorf("search %s description = %q, want predicate wording", def.Field, def.Description)
		}
	}
	pinned, ok := DefinitionFor(SurfaceSearch, FieldPinned)
	if !ok || len(pinned.Clears) != 1 || pinned.Clears[0].Description != "Match unpinned tasks" {
		t.Fatalf("search pin clears = %+v, want the unpin predicate wording", pinned.Clears)
	}
}

func TestDuplicateSingletonDiagnosticsComeFromCardinality(t *testing.T) {
	parsed := Parse(
		"Task !1 !2 #one #two -#old -#older @today @tomorrow $today $next *pin *unpin %one %two >1 >2 <3 <4",
		SurfaceEdit,
	)
	want := []Field{FieldPriority, FieldDue, FieldDestination, FieldPinned, FieldProject, FieldParent}
	got := make([]Field, len(parsed.Diagnostics))
	for i, diagnostic := range parsed.Diagnostics {
		got[i] = diagnostic.Field
		if diagnostic.Message == "" {
			t.Errorf("diagnostic = %+v", diagnostic)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("duplicate fields = %v, want %v (diagnostics=%+v)", got, want, parsed.Diagnostics)
	}
}

func TestChildTokensRepeatOnCaptureAndEditSurfaces(t *testing.T) {
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit} {
		parsed := Parse("Ship release <12 <Login <12", surface)
		tokens := parsed.TokensFor(FieldChild)
		got := make([]string, len(tokens))
		for i, token := range tokens {
			got[i] = token.Value
		}
		if want := []string{"12", "Login", "12"}; !reflect.DeepEqual(got, want) {
			t.Errorf("child values on surface %d = %v, want %v", surface, got, want)
		}
		if parsed.Text != "Ship release" {
			t.Errorf("Text on surface %d = %q, want Ship release", surface, parsed.Text)
		}
		if len(parsed.Diagnostics) != 0 {
			t.Errorf("repeated children produced diagnostics on surface %d: %+v", surface, parsed.Diagnostics)
		}
	}
}

func TestChildIsAbsentFromOutlineSurface(t *testing.T) {
	const input = "Ship release <12"
	parsed := Parse(input, SurfaceOutline)
	if len(parsed.Tokens) != 0 || parsed.Text != input {
		t.Fatalf("outline parse = %+v, want literal text", parsed)
	}
	if _, ok := DefinitionFor(SurfaceOutline, FieldChild); ok {
		t.Fatal("outline surface advertises the child definition")
	}
}

func TestChildZeroIsAnOrdinaryReferenceNotAClear(t *testing.T) {
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit} {
		definition, ok := DefinitionFor(surface, FieldChild)
		if !ok {
			t.Fatalf("child definition is missing on surface %d", surface)
		}
		if len(definition.Clears) != 0 {
			t.Errorf("child clears on surface %d = %+v, want none", surface, definition.Clears)
		}
		if !reflect.DeepEqual(definition.Forms, []string{"<child"}) {
			t.Errorf("child forms on surface %d = %v, want [<child]", surface, definition.Forms)
		}
		tokens := Parse("Ship release <0", surface).TokensFor(FieldChild)
		if len(tokens) != 1 || tokens[0].Value != "0" {
			t.Errorf("<0 tokens on surface %d = %+v, want one ordinary reference", surface, tokens)
		}
	}
}

func TestEmbeddedComparisonTextStaysLiteral(t *testing.T) {
	for _, input := range []string{"Keep a<b stable", "Keep x < y stable", "Keep 5<3 stable", "Keep <"} {
		parsed := Parse(input, SurfaceCapture)
		if len(parsed.TokensFor(FieldChild)) != 0 || parsed.Text != input {
			t.Errorf("Parse(%q) = %+v, want literal text", input, parsed)
		}
	}
}

func TestUnpinTokenizesEverywhereButIsAdvertisedOnlyOnEditAndSearch(t *testing.T) {
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit, SurfaceOutline, SurfaceSearch} {
		parsed := Parse("Ship release *unpin", surface)
		tokens := parsed.TokensFor(FieldPinned)
		if len(tokens) != 1 || tokens[0].Value != "unpin" {
			t.Errorf("*unpin tokens on surface %d = %+v, want one unpin token", surface, tokens)
		}
		if parsed.Text != "Ship release" {
			t.Errorf("Text on surface %d = %q, want Ship release", surface, parsed.Text)
		}
		definition, ok := DefinitionFor(surface, FieldPinned)
		if !ok {
			t.Fatalf("pin definition is missing on surface %d", surface)
		}
		want := []string{"*pin"}
		if surface == SurfaceEdit || surface == SurfaceSearch {
			want = []string{"*pin", "*unpin"}
		}
		if !reflect.DeepEqual(definition.Forms, want) {
			t.Errorf("pin forms on surface %d = %v, want %v", surface, definition.Forms, want)
		}
	}
}

func TestPinClearValueIsUnpinRatherThanZero(t *testing.T) {
	capture, ok := DefinitionFor(SurfaceCapture, FieldPinned)
	if !ok || len(capture.Clears) != 0 {
		t.Errorf("capture pin clears = %+v, want none", capture.Clears)
	}
	edit, ok := DefinitionFor(SurfaceEdit, FieldPinned)
	if !ok || len(edit.Clears) != 1 || edit.Clears[0].Value != "unpin" {
		t.Fatalf("edit pin clears = %+v, want one unpin clear", edit.Clears)
	}
	if !reflect.DeepEqual(edit.Values, []string{"pin"}) {
		t.Errorf("edit pin values = %v, want [pin]; the clear value is never advertised", edit.Values)
	}
}

func TestUnrecognizedPinValuesStayLiteral(t *testing.T) {
	for _, input := range []string{"Refactor *args handling", "Keep 2*3 stable", "Keep *", "Keep *pinned stable"} {
		parsed := Parse(input, SurfaceEdit)
		if len(parsed.TokensFor(FieldPinned)) != 0 || parsed.Text != input {
			t.Errorf("Parse(%q) = %+v, want literal text", input, parsed)
		}
	}
}

func TestRegistryInvariantsAndDestinationCoverage(t *testing.T) {
	if err := validateRegistry(); err != nil {
		t.Fatalf("validateRegistry: %v", err)
	}
	want := []string{"inbox", "backlog", "today", "next", "doing", "waiting", "paused", "someday", "done", "canceled"}
	definition, ok := DefinitionFor(SurfaceEdit, FieldDestination)
	if !ok || !reflect.DeepEqual(definition.Values, want) {
		t.Fatalf("destination values = %v, want %v", definition.Values, want)
	}
	edit := definition
	if !ok || !reflect.DeepEqual(edit.Forms, []string{"$destination"}) {
		t.Fatalf("edit destination definition = %+v, %v", edit, ok)
	}
}

func TestDefinitionsReturnDefensiveCopies(t *testing.T) {
	defs := Definitions(SurfaceEdit)
	defs[0].Forms[0] = "mutated"
	defs[0].Values[0] = "mutated"
	defs[0].RetiredValues[0] = "mutated"
	defs[0].Examples[0] = "mutated"
	defs[0].Clears[0].Value = "mutated"

	fresh, ok := DefinitionFor(SurfaceEdit, FieldPriority)
	if !ok || fresh.Forms[0] != "!1-3" || fresh.Values[0] != "1" || fresh.RetiredValues[0] != "4" || fresh.Examples[0] != "!1" || fresh.Clears[0].Value != "0" {
		t.Fatalf("registry projection was mutated: %+v", fresh)
	}
}

func TestInvalidPriorityErrorUsesCanonicalValues(t *testing.T) {
	definition, ok := DefinitionFor(SurfaceCapture, FieldPriority)
	if !ok {
		t.Fatal("priority definition is missing")
	}
	err := InvalidPriorityError("5")
	for _, value := range definition.Values {
		if token := definition.Trigger + value; !strings.Contains(err.Error(), token) {
			t.Errorf("priority guidance %q does not contain %q", err, token)
		}
	}
}

func TestDueSigilStaysDayOnlyAndRefusesHourOffsets(t *testing.T) {
	hourForm := regexp.MustCompile(`^\+\d+h$`)
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit, SurfaceOutline, SurfaceSearch} {
		for _, def := range Definitions(surface) {
			if def.Field != FieldDue {
				continue
			}
			advertised := slices.Concat(def.Values, def.Examples, def.Forms)
			for _, value := range advertised {
				if hourForm.MatchString(strings.TrimPrefix(value, "@")) {
					t.Errorf("@ advertises the hour form %q; hours are defer-only", value)
				}
			}
		}
	}

	tokens := Parse("Ship it @+1h", SurfaceCapture).TokensFor(FieldDue)
	if len(tokens) != 1 {
		t.Fatalf("@+1h due tokens = %+v, want exactly one", tokens)
	}
	if _, err := util.ParseCalendarDate(tokens[0].Value); err == nil {
		t.Fatalf("@%s resolved to a due date; hours are defer-only", tokens[0].Value)
	}
}

// TestDueSigilCannotSwallowTheDeferSigil pins the structural guard rather than
// registry order: the due pattern's last alternative refuses a leading @, so no
// walk order can let @ consume half of an @@ token.
func TestDueSigilCannotSwallowTheDeferSigil(t *testing.T) {
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit, SurfaceSearch} {
		parsed := Parse("Ship it @@+2h", surface)
		if got := parsed.TokensFor(FieldDue); len(got) != 0 {
			t.Errorf("surface %d produced due tokens %+v from @@+2h", surface, got)
		}
		deferred := parsed.TokensFor(FieldDefer)
		if len(deferred) != 1 || deferred[0].Value != "+2h" {
			t.Errorf("surface %d defer tokens = %+v, want one +2h", surface, deferred)
		}
		if parsed.Text != "Ship it" {
			t.Errorf("surface %d text = %q, want Ship it", surface, parsed.Text)
		}
	}

	both := Parse("Ship it @tomorrow @@+2h", SurfaceEdit)
	due := both.TokensFor(FieldDue)
	if len(due) != 1 || due[0].Value != "tomorrow" {
		t.Fatalf("due tokens = %+v, want one tomorrow", due)
	}
	if got := both.TokensFor(FieldDefer); len(got) != 1 || got[0].Value != "+2h" {
		t.Fatalf("defer tokens = %+v, want one +2h", got)
	}
}

// TestDueFormsSurviveTheDeferTightening re-checks every advertised due value
// against the tightened pattern, since [^\s@]\S* is the one alternative that
// could silently stop recognizing a legal form.
func TestDueFormsSurviveTheDeferTightening(t *testing.T) {
	definition, ok := DefinitionFor(SurfaceEdit, FieldDue)
	if !ok {
		t.Fatal("edit due definition is missing")
	}
	values := slices.Concat(definition.Values, []string{clearValue, "2026-08-18", "+1h"})
	for _, value := range values {
		tokens := Parse("Ship it @"+value, SurfaceEdit).TokensFor(FieldDue)
		if len(tokens) != 1 || tokens[0].Value != value {
			t.Errorf("@%s tokens = %+v, want exactly that value", value, tokens)
		}
	}
}

// TestAttachmentSigilLeavesOrdinaryProseLiteral pins the lexical half of the
// attachment rule. The & character occurs constantly in real titles, so anything
// the application layer could not attach must stay in the text rather than
// vanish from it.
func TestAttachmentSigilLeavesOrdinaryProseLiteral(t *testing.T) {
	for _, input := range []string{
		"R&D pipeline",
		"Q&A with legal",
		"Fix A & B",
		"&10:30 standup",
		"Escape &nbsp; in the footer",
		"Ship &",
		// Quoting widens the span a token may cover; it does not exempt the value
		// from the lexical test, so a quoted phrase naming nothing attachable is
		// still returned to the title with its quotes intact.
		`Ask &"why" before the demo`,
		"Read &'notes' first",
	} {
		parsed := Parse(input, SurfaceCapture)
		if got := parsed.TokensFor(FieldAttachment); len(got) != 0 {
			t.Errorf("%q produced attachment tokens %+v", input, got)
		}
		if parsed.Text != input {
			t.Errorf("%q text = %q, want it unchanged", input, parsed.Text)
		}
	}
}

// TestAttachmentSigilRecognizesEveryAttachableShape covers the three paths the
// application layer classifies into: URL, code reference, and file path.
func TestAttachmentSigilRecognizesEveryAttachableShape(t *testing.T) {
	parsed := Parse("Fix OAuth &https://example.com/i/412 &main.go:42 &~/notes/report.pdf", SurfaceCapture)
	var values []string
	for _, token := range parsed.TokensFor(FieldAttachment) {
		values = append(values, token.Value)
	}
	want := []string{"https://example.com/i/412", "main.go:42", "~/notes/report.pdf"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("attachment values = %v, want %v", values, want)
	}
	if parsed.Text != "Fix OAuth" {
		t.Fatalf("Text = %q, want Fix OAuth", parsed.Text)
	}
}

// TestAttachmentSigilSpansAPathWithSpaces pins the shapes a real path arrives
// in. A file manager copies "~/Library/Application Support/…" quoted or with
// backslash-escaped spaces, and a bare \S+ run stopped at the first space, which
// attached half the path and left the other half in the title. Each shape
// normalizes to the same plain path, so the application layer classifies a
// quoted paste exactly as it classifies a typed one.
func TestAttachmentSigilSpansAPathWithSpaces(t *testing.T) {
	const want = "/Users/me/Application Support/shot 2.png"
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"single quoted", `Fix it &'/Users/me/Application Support/shot 2.png'`},
		{"double quoted", `Fix it &"/Users/me/Application Support/shot 2.png"`},
		{"escaped spaces", `Fix it &/Users/me/Application\ Support/shot\ 2.png`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed := Parse(tc.input, SurfaceCapture)
			tokens := parsed.TokensFor(FieldAttachment)
			if len(tokens) != 1 || tokens[0].Value != want {
				t.Fatalf("attachment tokens = %+v, want exactly %q", tokens, want)
			}
			if parsed.Text != "Fix it" {
				t.Fatalf("Text = %q, want the whole span out of the title", parsed.Text)
			}
		})
	}
}

// TestQuotedAttachmentKeepsItsPositionAheadOfTag re-checks registry order over
// the widened span: a sigil inside a quoted path is part of the path, and only
// peeling the attachment first can keep it there.
func TestQuotedAttachmentKeepsItsPositionAheadOfTag(t *testing.T) {
	parsed := Parse(`Ship it &'/Users/me/Release #4 notes.md' #work`, SurfaceCapture)
	attachments := parsed.TokensFor(FieldAttachment)
	if len(attachments) != 1 || attachments[0].Value != "/Users/me/Release #4 notes.md" {
		t.Fatalf("attachment tokens = %+v, want the quoted path whole", attachments)
	}
	tags := parsed.TokensFor(FieldTag)
	if len(tags) != 1 || tags[0].Value != "work" {
		t.Fatalf("tag tokens = %+v, want only the tag written outside the quotes", tags)
	}
	if parsed.Text != "Ship it" {
		t.Fatalf("Text = %q, want Ship it", parsed.Text)
	}
}

// TestAttachmentIsParsedBeforeTag pins the one place registry order carries
// meaning. The tag pattern stops at the &, so peeling tags first would leave a
// whitespace-preceded & that becomes a spurious attachment.
func TestAttachmentIsParsedBeforeTag(t *testing.T) {
	parsed := Parse("Task #work&http://x/y", SurfaceCapture)
	if got := parsed.TokensFor(FieldAttachment); len(got) != 0 {
		t.Fatalf("attachment tokens = %+v, want none; the & is glued to the tag", got)
	}
	tags := parsed.TokensFor(FieldTag)
	if len(tags) != 1 || tags[0].Value != "work" {
		t.Fatalf("tag tokens = %+v, want one work", tags)
	}
	if parsed.Text != "Task &http://x/y" {
		t.Fatalf("Text = %q, want the remainder to stay literal", parsed.Text)
	}
}

// TestCommentSigilLeavesOrdinaryProseLiteral pins the lexical half of the
// comment rule. A semicolon is ordinary punctuation, so only a paired,
// whitespace-preceded delimiter may consume anything.
func TestCommentSigilLeavesOrdinaryProseLiteral(t *testing.T) {
	for _, input := range []string{
		"Fix a; then b",
		"Ship ;unterminated",
		"Escape &nbsp; in the footer",
		"Ship ;",
		"Ship ; ;",
	} {
		parsed := Parse(input, SurfaceCapture)
		if got := parsed.TokensFor(FieldComment); len(got) != 0 {
			t.Errorf("%q produced comment tokens %+v", input, got)
		}
		if parsed.Text != input {
			t.Errorf("%q text = %q, want it unchanged", input, parsed.Text)
		}
	}
}

// TestCommentIsParsedBeforeEveryOtherSigil pins the reason the comment leads the
// registry: RE2 has no lookaround, so peeling any other field first would let it
// reach inside the comment's span and mint metadata out of comment prose.
func TestCommentIsParsedBeforeEveryOtherSigil(t *testing.T) {
	parsed := Parse("Fix login ;check the #auth flow;", SurfaceCapture)
	comments := parsed.TokensFor(FieldComment)
	if len(comments) != 1 || comments[0].Value != "check the #auth flow" {
		t.Fatalf("comment tokens = %+v, want one holding the whole span", comments)
	}
	if got := parsed.TokensFor(FieldTag); len(got) != 0 {
		t.Fatalf("tag tokens = %+v, want none; #auth is comment text", got)
	}
	if parsed.Text != "Fix login" {
		t.Fatalf("Text = %q, want Fix login", parsed.Text)
	}
}

// TestCommentSigilIsRepeatable keeps two comments on one line distinct rather
// than folding them into one span.
func TestCommentSigilIsRepeatable(t *testing.T) {
	parsed := Parse("Call Dana ;ask about SSO; ;then confirm the date;", SurfaceCapture)
	var values []string
	for _, token := range parsed.TokensFor(FieldComment) {
		values = append(values, token.Value)
	}
	want := []string{"ask about SSO", "then confirm the date"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("comment values = %v, want %v", values, want)
	}
	if parsed.Text != "Call Dana" {
		t.Fatalf("Text = %q, want Call Dana", parsed.Text)
	}
}

// TestSearchRefusesTheCommentSigilByName mirrors the attachment refusal: search
// is a fourth surface of the same registry, not a place a comment can be written.
func TestSearchRefusesTheCommentSigilByName(t *testing.T) {
	guidance := SearchGuidance(ParseSearch("dana ;ask about SSO;"))
	if !strings.Contains(guidance, ";comment; is not a search predicate") {
		t.Fatalf("guidance = %q, want the comment refusal", guidance)
	}
	if !strings.Contains(guidance, "Comments panel") {
		t.Fatalf("guidance = %q, want it to name where comments are added", guidance)
	}
	if got := SearchGuidance(ParseSearch("fix a; then b")); got != "" {
		t.Fatalf("guidance = %q, want an ordinary query to run", got)
	}
}

// TestGuideOrderTeachesTheSameFieldsParseOrderRecognizes is the fence that makes
// splitting the two lists safe. Parse order can move a field for
// disambiguation; it may never change what the guide teaches.
func TestGuideOrderTeachesTheSameFieldsParseOrderRecognizes(t *testing.T) {
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit, SurfaceOutline, SurfaceSearch} {
		parseOrder := Definitions(surface)
		guide := GuideDefinitions(surface)
		if len(parseOrder) != len(guide) {
			t.Fatalf("surface %d guide = %d definitions, want %d", surface, len(guide), len(parseOrder))
		}
		parseFields := make([]Field, len(parseOrder))
		for i, def := range parseOrder {
			parseFields[i] = def.Field
		}
		guideFields := make([]Field, len(guide))
		for i, def := range guide {
			guideFields[i] = def.Field
		}
		slices.Sort(parseFields)
		slices.Sort(guideFields)
		if !slices.Equal(parseFields, guideFields) {
			t.Errorf("surface %d guide fields = %v, want %v", surface, guideFields, parseFields)
		}
	}

	// The attachment and comment sigils are the reason the split exists: parsed
	// first and second, taught last.
	capture := Definitions(SurfaceCapture)
	if capture[0].Field != FieldComment {
		t.Errorf("parse order position 0 = %s, want comment ahead of every other field", capture[0].Field)
	}
	if capture[1].Field != FieldAttachment {
		t.Errorf("parse order position 1 = %s, want attachment ahead of tag", capture[1].Field)
	}
	guide := GuideDefinitions(SurfaceCapture)
	if guide[len(guide)-2].Field != FieldAttachment {
		t.Errorf("guide order second-to-last = %s, want attachment", guide[len(guide)-2].Field)
	}
	if guide[len(guide)-1].Field != FieldComment {
		t.Errorf("guide order ends with %s, want comment", guide[len(guide)-1].Field)
	}
}

// TestSearchRefusesTheAttachmentSigilByName keeps the refusal specific: the
// sigil is named only when the value is one capture would have honored, so an
// ordinary query containing an ampersand still runs.
func TestSearchRefusesTheAttachmentSigilByName(t *testing.T) {
	refused := ParseSearch("spec &https://example.com/spec.pdf")
	guidance := SearchGuidance(refused)
	if !strings.Contains(guidance, "&link is not a search predicate") {
		t.Fatalf("guidance = %q, want the attachment refusal", guidance)
	}
	if !strings.Contains(guidance, "Attachments panel") {
		t.Fatalf("guidance = %q, want it to name where attachments are added", guidance)
	}
	if got := SearchGuidance(ParseSearch("escape &nbsp; in the footer")); got != "" {
		t.Fatalf("guidance = %q, want an ordinary query to run", got)
	}
}
