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
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit, SurfaceSearch} {
		for _, state := range models.AllWorkflowStates() {
			got, ok := ParseState(surface, state.String())
			if !ok || got.Legacy || !reflect.DeepEqual(got.States, []models.WorkflowState{state}) {
				t.Errorf("ParseState(%v, %q) = %+v, %v", surface, state, got, ok)
			}
		}
	}
}

func TestParseStateRejectsLegacyAliases(t *testing.T) {
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit, SurfaceSearch} {
		for _, legacy := range []string{"todo", "start", "blocked", "standby", "hold", "maybe", "reopen"} {
			if got, ok := ParseState(surface, legacy); ok {
				t.Errorf("ParseState(%v, %q) = %+v, true; want rejection", surface, legacy, got)
			}
		}
	}
}

func TestParseUsesSharedSurfaceGrammar(t *testing.T) {
	parsed := Parse(
		"Build API !high #backend @tomorrow ~weekly ^bug >1 %core $ready",
		SurfaceCapture,
	)
	if parsed.Text != "Build API" {
		t.Fatalf("Text = %q, want Build API", parsed.Text)
	}
	for _, field := range []Field{
		FieldPriority, FieldTag, FieldDue, FieldRecurrence, FieldTaskType,
		FieldParent, FieldProject, FieldState,
	} {
		if len(parsed.TokensFor(field)) == 0 {
			t.Errorf("missing %s token in %+v", field, parsed.Tokens)
		}
	}
}

func TestRetiredLifecycleFormsRemainLiteralText(t *testing.T) {
	parsed := Parse("Build API /next status:todo", SurfaceCapture)
	if parsed.Text != "Build API /next status:todo" || len(parsed.Tokens) != 0 {
		t.Fatalf("retired forms were parsed: %+v", parsed)
	}
}

func TestParsePriority(t *testing.T) {
	if got, ok := ParsePriority("high", false); !ok || !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("ParsePriority(high) = %v, %v", got, ok)
	}
	if got, ok := ParsePriority("3-1", true); !ok || !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("ParsePriority(3-1) = %v, %v", got, ok)
	}
}
