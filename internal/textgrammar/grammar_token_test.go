package textgrammar

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/util"
)

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
	index := slices.IndexFunc(defs, func(def Definition) bool { return def.Field == FieldPriority })
	if index < 0 {
		t.Fatal("edit definitions are missing priority")
	}
	defs[index].Forms[0] = "mutated"
	defs[index].Values[0] = "mutated"
	defs[index].RetiredValues[0] = "mutated"
	defs[index].Examples[0] = "mutated"
	defs[index].Clears[0].Value = "mutated"

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
