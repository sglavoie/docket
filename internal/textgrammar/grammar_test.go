package textgrammar

import (
	"reflect"
	"strings"
	"testing"
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

func TestSharedParserAdvertisesExactlySevenCaptureSigils(t *testing.T) {
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
	if len(defs) != 7 {
		t.Fatalf("capture definitions = %d, want 7: %+v", len(defs), defs)
	}
	wantForms := []string{"!1-3", "#tag", "@date", "$destination", "%project", ">parent", "<child"}
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
				FieldPriority: {"!1-3"}, FieldTag: {"#tag"}, FieldDue: {"@date"},
				FieldDestination: {"$destination"}, FieldProject: {"%project"}, FieldParent: {">parent"},
				FieldChild: {"<child"},
			},
		},
		{
			name: "edit", surface: SurfaceEdit,
			want: map[Field][]string{
				FieldPriority: {"!1-3", "!0"}, FieldTag: {"#tag"}, FieldRemoveTag: {"-#tag"},
				FieldDue: {"@date", "@0"}, FieldDestination: {"$destination"},
				FieldProject: {"%project"}, FieldParent: {">parent", ">0"},
				FieldChild: {"<child"},
			},
		},
		{
			name: "outline", surface: SurfaceOutline,
			want: map[Field][]string{
				FieldPriority: {"!1-3"}, FieldTag: {"#tag"}, FieldDue: {"@date"},
				FieldDestination: {"$destination"},
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
	const input = "Task !1 #add -#old @tomorrow $next %core >1 <2"
	tests := []struct {
		name     string
		surface  Surface
		wantText string
		fields   []Field
	}{
		{"capture", SurfaceCapture, "Task -#old", []Field{FieldPriority, FieldTag, FieldDue, FieldDestination, FieldProject, FieldParent, FieldChild}},
		{"edit", SurfaceEdit, "Task", []Field{FieldPriority, FieldTag, FieldRemoveTag, FieldDue, FieldDestination, FieldProject, FieldParent, FieldChild}},
		{"outline", SurfaceOutline, "Task -#old %core >1 <2", []Field{FieldPriority, FieldTag, FieldDue, FieldDestination}},
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
}

func TestDuplicateSingletonDiagnosticsComeFromCardinality(t *testing.T) {
	parsed := Parse(
		"Task !1 !2 #one #two -#old -#older @today @tomorrow $today $next %one %two >1 >2 <3 <4",
		SurfaceEdit,
	)
	want := []Field{FieldPriority, FieldDue, FieldDestination, FieldProject, FieldParent}
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
