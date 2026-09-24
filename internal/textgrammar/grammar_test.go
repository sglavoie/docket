package textgrammar

import (
	"reflect"
	"testing"
)

func TestParseUsesSharedSurfaceGrammar(t *testing.T) {
	parsed := Parse(
		"Build API ^bug !1 #backend @tomorrow ~12 >1 <2 %core $backlog",
		SurfaceCapture,
	)
	if parsed.Text != "Build API" {
		t.Fatalf("Text = %q, want Build API", parsed.Text)
	}
	for _, field := range []Field{
		FieldTaskType, FieldPriority, FieldTag, FieldDue, FieldDependency, FieldParent,
		FieldChild, FieldProject, FieldDestination,
	} {
		if len(parsed.TokensFor(field)) == 0 {
			t.Errorf("missing %s token in %+v", field, parsed.Tokens)
		}
	}
}

func TestSharedParserAdvertisesExactlyFourteenCaptureSigils(t *testing.T) {
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
	if len(defs) != 14 {
		t.Fatalf("capture definitions = %d, want 14: %+v", len(defs), defs)
	}
	// Parse order, not teach order: ;comment; leads because nothing else may peel
	// a sigil out of a comment's span, and &link sits ahead of #tag because the
	// tag pattern would otherwise hand it a whitespace-preceded &. Context now
	// precedes tag and defer precedes due, because each search-only negation is
	// registered immediately before its positive twin and the longer trigger
	// first, and -##/##/-#/# is the only layout that satisfies both.
	wantForms := []string{";comment;", "&link", "~task", "!1-3", "##context", "#tag", "@@defer", "@date", "$destination", "*pin", "=review", "%project", ">parent", "<child"}
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
	if got := search.TokensFor(FieldPriority); len(got) != 3 || got[0].Value != "0" || got[1].Value != "1" || got[2].Value != "9" {
		t.Fatalf("search priority tokens = %+v, want the !0 question, !1, and numeric !9 for guidance", got)
	}
	if search.Text != "Task !important" {
		t.Fatalf("search text = %q, want only the prose word left literal", search.Text)
	}
}
