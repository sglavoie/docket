package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// searchACOutliner builds a scoped Outliner already in search mode, which is the
// only state the suggestion pane can exist in.
func searchACOutliner(projects ...*models.Project) *OutlinerModel {
	outliner := scopeTestOutliner(projects...)
	outliner.SetSize(120, 40)
	outliner.ActivateSearch()
	return outliner
}

// typeSearchAC puts a whole query in the box with the caret at its end, which is
// the ordinary typing case.
func typeSearchAC(outliner *OutlinerModel, value string) {
	typeSearchACAt(outliner, value, utf8.RuneCountInString(value))
}

// typeSearchACAt is the mid-query case: the caret lands on a rune index inside a
// settled query rather than after it.
func typeSearchACAt(outliner *OutlinerModel, value string, runeCursor int) {
	outliner.SetSearchInputValue(value)
	outliner.SetSearchInputCursor(runeCursor)
	outliner.refreshSearchAutocomplete()
}

func searchACValues(outliner *OutlinerModel) []string {
	values := make([]string, 0, len(outliner.searchAutocomplete.ac.Items))
	for _, item := range outliner.searchAutocomplete.ac.Items {
		values = append(values, item.Value)
	}
	return values
}

func searchACDisplays(outliner *OutlinerModel) []string {
	displays := make([]string, 0, len(outliner.searchAutocomplete.ac.Items))
	for _, item := range outliner.searchAutocomplete.ac.Items {
		displays = append(displays, item.Display)
	}
	return displays
}

func searchACTask(id string, tags []string, context string) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	task.Title = id
	task.Tags = tags
	task.Context = context
	return task
}

// TestSearchAutocompleteTriggersProjectTheSearchSurfaceOneToOne is the vocabulary
// contract: the pane offers exactly the sigils SurfaceSearch declares, in the
// registry's parse order, and owns no trigger of its own.
func TestSearchAutocompleteTriggersProjectTheSearchSurfaceOneToOne(t *testing.T) {
	definitions := textgrammar.Definitions(textgrammar.SurfaceSearch)
	triggers := outlinerSearchTriggers()
	if len(triggers) != len(definitions) {
		t.Fatalf("search triggers = %d, want %d registry definitions", len(triggers), len(definitions))
	}
	for i, definition := range definitions {
		if got, want := triggers[i].Text, definition.Trigger; got != want {
			t.Errorf("trigger %d = %q, want %q for %s", i, got, want, definition.Field)
		}
		back, ok := outlinerSearchDefinitionForMode(triggers[i].Mode)
		if !ok || back.Field != definition.Field {
			t.Errorf("mode %d does not round-trip to %s", triggers[i].Mode, definition.Field)
		}
	}

	// The placement sigils search refuses by name must never acquire a completion
	// mode, or the pane would teach a query the gate rejects.
	for _, field := range []textgrammar.Field{
		textgrammar.FieldParent,
		textgrammar.FieldChild,
		textgrammar.FieldAttachment,
		textgrammar.FieldComment,
	} {
		if _, ok := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, field); ok {
			t.Errorf("%s reached SurfaceSearch, which the search grammar refuses", field)
		}
	}
}

// TestSearchAutocompleteUsesLongestTriggerAtTheCursor covers every pair that
// shares a prefix: a one-character trigger must never swallow the two-character
// one typed above it.
func TestSearchAutocompleteUsesLongestTriggerAtTheCursor(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "office")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))

	cases := []struct {
		query string
		field textgrammar.Field
	}{
		{"#", textgrammar.FieldTag},
		{"##", textgrammar.FieldContext},
		{"-#", textgrammar.FieldRemoveTag},
		{"@", textgrammar.FieldDue},
		{"@@", textgrammar.FieldDefer},
		{"!", textgrammar.FieldPriority},
		{"$", textgrammar.FieldDestination},
		{"*", textgrammar.FieldPinned},
		{"%", textgrammar.FieldProject},
	}
	for _, tc := range cases {
		typeSearchAC(outliner, tc.query)
		if !outliner.searchAutocomplete.active() {
			t.Fatalf("query %q opened no suggestions", tc.query)
		}
		definition, ok := outlinerSearchDefinitionForMode(outliner.searchAutocomplete.ac.Mode)
		if !ok {
			t.Errorf("query %q opened a mode no search definition claims", tc.query)
		} else if definition.Field != tc.field {
			t.Errorf("query %q completed %s, want %s", tc.query, definition.Field, tc.field)
		}
		if got := outliner.searchAutocomplete.triggerText; got != tc.query {
			t.Errorf("query %q retained trigger %q", tc.query, got)
		}
	}
}

// TestSearchAutocompleteStaticValuesComeFromTheSearchGrammar proves every fixed
// value is a projection: the pane reads Values, Clears, and the two search-only
// projections, and invents nothing.
func TestSearchAutocompleteStaticValuesComeFromTheSearchGrammar(t *testing.T) {
	outliner := searchACOutliner(primaryRowsProject("alpha", searchACTask("t1", nil, "")))

	priority, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldPriority)
	typeSearchAC(outliner, "!")
	if got := searchACValues(outliner); !slices.Equal(got, priority.Values) {
		t.Errorf("priority values = %v, want registry %v", got, priority.Values)
	}
	// The name beside each level comes from the shared priority parse, not from a
	// second table of level names.
	for _, display := range searchACDisplays(outliner) {
		if !strings.Contains(display, models.PriorityName(1)) &&
			!strings.Contains(display, models.PriorityName(2)) &&
			!strings.Contains(display, models.PriorityName(3)) {
			t.Errorf("priority row %q names no shared priority", display)
		}
	}

	destination, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldDestination)
	typeSearchAC(outliner, "$")
	if got := searchACValues(outliner); !slices.Equal(got, destination.Values) {
		t.Errorf("destination values = %v, want registry %v", got, destination.Values)
	}
	if !slices.Contains(searchACValues(outliner), "today") || !slices.Contains(searchACValues(outliner), "next") {
		t.Errorf("destination values = %v, want the two planning lists", searchACValues(outliner))
	}

	pinned, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldPinned)
	typeSearchAC(outliner, "*")
	wantPin := slices.Clone(pinned.Values)
	for _, clear := range pinned.Clears {
		wantPin = append(wantPin, clear.Value)
	}
	if got := searchACValues(outliner); !slices.Equal(got, wantPin) {
		t.Errorf("pin values = %v, want registry values plus clears %v", got, wantPin)
	}

	typeSearchAC(outliner, "@")
	wantDue := append(textgrammar.SearchPresetValues(textgrammar.FieldDue),
		textgrammar.SearchExampleValues(textgrammar.FieldDue)...)
	if got := searchACValues(outliner); !slices.Equal(got, wantDue) {
		t.Errorf("due values = %v, want search presets plus the rolling-hour example %v", got, wantDue)
	}

	deferDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldDefer)
	typeSearchAC(outliner, "@@")
	deferValues := searchACValues(outliner)
	for _, want := range textgrammar.SearchPresetValues(textgrammar.FieldDefer) {
		if !slices.Contains(deferValues, want) {
			t.Errorf("defer values = %v, missing search preset %q", deferValues, want)
		}
	}
	for _, want := range deferDefinition.Values {
		if !slices.Contains(deferValues, want) {
			t.Errorf("defer values = %v, missing definition value %q", deferValues, want)
		}
	}
	for _, example := range deferDefinition.Examples {
		want := strings.TrimPrefix(example, deferDefinition.Trigger)
		if !slices.Contains(deferValues, want) {
			t.Errorf("defer values = %v, missing definition example %q", deferValues, want)
		}
	}
	// An example repeats a value it illustrates, and a duplicated row would let
	// the same completion be selected twice.
	seen := make(map[string]bool)
	for _, value := range deferValues {
		if seen[value] {
			t.Errorf("defer values = %v, want no duplicate %q", deferValues, value)
		}
		seen[value] = true
	}
}

// TestSearchAutocompleteOwnsNoParallelTriggerOrValueRegistry is the consumer
// contract for the new module, matching the one the capture consumers already
// answer to: a second static list here would be a second grammar.
func TestSearchAutocompleteOwnsNoParallelTriggerOrValueRegistry(t *testing.T) {
	name := "outliner_search_autocomplete.go"
	triggerSet := make(map[string]bool)
	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceSearch) {
		triggerSet[definition.Trigger] = true
	}
	destinationSet := make(map[string]bool)
	destination, ok := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldDestination)
	if !ok {
		t.Fatal("search destination grammar is missing")
	}
	for _, value := range destination.Values {
		destinationSet[value] = true
	}

	source, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filepath.Clean(name), source, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CompositeLit:
			triggers, destinations := grammarLiteralsInNode(value, triggerSet, destinationSet)
			if len(triggers) >= 2 {
				t.Errorf("%s:%d owns a parallel trigger list %v", name, fileSet.Position(value.Pos()).Line, mapKeys(triggers))
			}
			if len(destinations) >= 2 {
				t.Errorf("%s:%d owns a parallel destination list %v", name, fileSet.Position(value.Pos()).Line, mapKeys(destinations))
			}
		case *ast.BasicLit:
			if value.Kind != token.STRING {
				return true
			}
			literal, err := strconv.Unquote(value.Value)
			if err == nil && inlineFormTriggerCount(literal, triggerSet) >= 2 {
				t.Errorf("%s:%d owns a parallel inline-form string %q", name, fileSet.Position(value.Pos()).Line, literal)
			}
		}
		return true
	})

	// Cardinality is the other thing a parallel list could smuggle in: it must be
	// read off the definition, so the capture-surface singleton table stays
	// unreferenced here.
	if strings.Contains(string(source), "autocompleteSingletonField") {
		t.Error("search completion read the capture surface's parallel singleton list instead of Definition.Cardinality")
	}
}
