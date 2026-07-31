package textgrammar

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCanonicalStatesComeFromModelOrder(t *testing.T) {
	got := StateNames()
	want := make([]string, 0, len(models.AllWorkflowStates()))
	for _, state := range models.AllWorkflowStates() {
		want = append(want, state.String())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StateNames() = %v, want %v", got, want)
	}
}

func TestParseStateCanonicalParity(t *testing.T) {
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit} {
		for _, state := range models.AllWorkflowStates() {
			got, ok := ParseState(surface, state.String())
			if !ok || got.Legacy || !reflect.DeepEqual(got.States, []models.WorkflowState{state}) {
				t.Errorf("ParseState(%v, %q) = %+v, %v", surface, state, got, ok)
			}
		}
	}
}

func TestParseStateRejectsLegacyAliases(t *testing.T) {
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit} {
		for _, legacy := range []string{"todo", "start", "blocked", "standby", "hold", "review", "maybe", "reopen"} {
			if got, ok := ParseState(surface, legacy); ok {
				t.Errorf("ParseState(%v, %q) = %+v, true; want rejection", surface, legacy, got)
			}
		}
	}
}

func TestInvalidStateErrorExplainsRetiredReviewReplacement(t *testing.T) {
	const want = "state $review was removed (use $ready)"
	if got := InvalidStateError("review").Error(); got != want {
		t.Fatalf("InvalidStateError(Review) = %q, want %q", got, want)
	}
}

func TestParseUsesSharedSurfaceGrammar(t *testing.T) {
	parsed := Parse(
		"Build API ^bug !1 #backend @tomorrow ~weekly >1 %core $ready",
		SurfaceCapture,
	)
	if parsed.Text != "Build API" {
		t.Fatalf("Text = %q, want Build API", parsed.Text)
	}
	for _, field := range []Field{
		FieldTaskType, FieldPriority, FieldTag, FieldDue, FieldRecurrence, FieldParent,
		FieldProject, FieldState,
	} {
		if len(parsed.TokensFor(field)) == 0 {
			t.Errorf("missing %s token in %+v", field, parsed.Tokens)
		}
	}
}

func TestSharedParserRecognizesExactlyEightCanonicalSigils(t *testing.T) {
	parsed := Parse(
		"Keep type:bug priority:1 pri:2 tag:backend due:tomorrow recur:weekly project:core proj:core p:core parent:1 state:ready -#old has:due no:tag is:pinned",
		SurfaceCapture|SurfaceEdit,
	)
	if len(parsed.Tokens) != 0 {
		t.Fatalf("legacy forms produced tokens: %+v", parsed.Tokens)
	}
	if parsed.Text == "" {
		t.Fatal("legacy forms did not remain literal text")
	}

	defs := Definitions(SurfaceCapture)
	if len(defs) != 8 {
		t.Fatalf("capture definitions = %d, want 8: %+v", len(defs), defs)
	}
	wantForms := []string{"^type", "!1-5", "#tag", "@date", "~interval", ">parent", "%project", "$state"}
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

func TestTaskMarkerNamesComeFromModelOrder(t *testing.T) {
	want := []string{"epic", "story", "bug", "idea"}
	if got := TaskMarkerNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("TaskMarkerNames() = %v, want %v", got, want)
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
}
