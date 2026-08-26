package tui

import (
	"fmt"
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

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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

// TestSearchAutocompleteCardinalityFollowsTheGrammarDefinition walks every search
// field and checks the suppression rule against the definition rather than a
// hand-written expectation, so a cardinality change in the registry moves this
// behavior without touching the module.
func TestSearchAutocompleteCardinalityFollowsTheGrammarDefinition(t *testing.T) {
	task := searchACTask("t1", []string{"infra", "docs"}, "office")
	second := searchACTask("t2", []string{"api"}, "home")
	outliner := searchACOutliner(primaryRowsProject("alpha", task, second))

	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceSearch) {
		// Take the first value the pane actually offers, so the settled prefix is a
		// query the grammar parses back into this field.
		typeSearchAC(outliner, definition.Trigger)
		if !outliner.searchAutocomplete.active() {
			t.Fatalf("%s offered nothing to settle with", definition.Field)
		}
		accepted := outliner.searchAutocomplete.ac.Items[0].Value

		typeSearchAC(outliner, definition.Trigger+accepted+" "+definition.Trigger)
		switch definition.Cardinality {
		case textgrammar.CardinalitySingleton:
			if outliner.searchAutocomplete.active() {
				t.Errorf("%s is a singleton but offered %v after being accepted",
					definition.Field, searchACValues(outliner))
			}
		case textgrammar.CardinalityRepeatable:
			if slices.Contains(searchACValues(outliner), accepted) {
				t.Errorf("%s re-offered the already-used value %q", definition.Field, accepted)
			}
		}
	}
}

// TestSearchAutocompleteDynamicValuesReadTheLiveCandidates covers the whole
// dynamic half at once: the values are the scope's own, normalized the way the
// predicate compares them, and a mutation is reflected on the next refresh
// because nothing is cached.
func TestSearchAutocompleteDynamicValuesReadTheLiveCandidates(t *testing.T) {
	first := searchACTask("t1", []string{"Source:GitHub", "infra"}, "Deep Work")
	second := searchACTask("t2", []string{"infra"}, "office")
	project := primaryRowsProject("alpha", first, second)
	outliner := searchACOutliner(project)

	typeSearchAC(outliner, "#")
	if got := searchACValues(outliner); !slices.Equal(got, []string{"source-github", "infra"}) {
		t.Fatalf("tag values = %v, want the normalized scope tags in encounter order", got)
	}
	// The remove-tag sigil completes the same live population: it asks which tag a
	// result must not carry, and a tag no candidate carries cannot narrow anything.
	typeSearchAC(outliner, "-#")
	if got := searchACValues(outliner); !slices.Equal(got, []string{"source-github", "infra"}) {
		t.Fatalf("remove-tag values = %v, want the same normalized scope tags", got)
	}

	typeSearchAC(outliner, "##")
	if got := searchACValues(outliner); !slices.Equal(got, []string{"deep-work", "office"}) {
		t.Fatalf("context values = %v, want the normalized scope contexts", got)
	}

	// A mutation between keystrokes: the next refresh reads the tasks, not a
	// snapshot taken when the pane opened.
	second.Tags = append(second.Tags, "release")
	typeSearchAC(outliner, "#")
	if got := searchACValues(outliner); !slices.Contains(got, "release") {
		t.Fatalf("tag values after mutation = %v, want the newly added tag", got)
	}
}

// TestSearchAutocompleteEmptyCandidateSetOffersNothing is the honest-silence
// case: a scope with no tags at all must close the pane rather than show an
// empty box.
func TestSearchAutocompleteEmptyCandidateSetOffersNothing(t *testing.T) {
	outliner := searchACOutliner(primaryRowsProject("alpha", searchACTask("t1", nil, "")))

	typeSearchAC(outliner, "#")
	if outliner.searchAutocomplete.active() {
		t.Fatalf("tag pane opened over an empty vocabulary: %v", searchACValues(outliner))
	}
	if outliner.searchAutocomplete.ac.Mode != acModeNone || outliner.searchAutocomplete.triggerText != "" {
		t.Error("the retained mode and trigger outlived the list they described")
	}
	typeSearchAC(outliner, "##")
	if outliner.searchAutocomplete.active() {
		t.Fatalf("context pane opened over an empty vocabulary: %v", searchACValues(outliner))
	}
}

// TestSearchAutocompleteValuesStayInsideFocus holds completion to the same
// boundary the projection honors: a Focus that hides a task must also hide the
// tag only that task carries, or the pane would offer a query with no results.
func TestSearchAutocompleteValuesStayInsideFocus(t *testing.T) {
	parent := searchACTask("parent", []string{"parent-tag"}, "parent-ctx")
	child := searchACTask("child", []string{"child-tag"}, "child-ctx")
	child.ParentID = &parent.ID
	outside := searchACTask("outside", []string{"outside-tag"}, "outside-ctx")
	alpha := primaryRowsProject("alpha", parent, child, outside)

	betaTask := searchACTask("beta-task", []string{"beta-tag"}, "beta-ctx")
	beta := primaryRowsProject("beta", betaTask)

	outliner := searchACOutliner(alpha, beta)
	typeSearchAC(outliner, "#")
	if got := searchACValues(outliner); !slices.Contains(got, "outside-tag") || !slices.Contains(got, "beta-tag") {
		t.Fatalf("scope-wide tag values = %v, want every project's tags", got)
	}

	outliner.pushFocusFrame(TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}, 0, 0)
	typeSearchAC(outliner, "#")
	if got := searchACValues(outliner); !slices.Equal(got, []string{"child-tag"}) {
		t.Fatalf("focused tag values = %v, want the focused branch alone", got)
	}
	typeSearchAC(outliner, "##")
	if got := searchACValues(outliner); !slices.Equal(got, []string{"child-ctx"}) {
		t.Fatalf("focused context values = %v, want the focused branch alone", got)
	}
}

// TestSearchAutocompleteProjectsComeFromTheLoadedScope proves completion never
// widens the catalog to have something to offer: the projects listed are exactly
// the ones the query in front of it could match.
func TestSearchAutocompleteProjectsComeFromTheLoadedScope(t *testing.T) {
	alpha := primaryRowsProject("alpha", searchACTask("t1", nil, ""))
	beta := primaryRowsProject("beta", searchACTask("t2", nil, ""))
	outliner := searchACOutliner(alpha, beta)

	typeSearchAC(outliner, "%")
	want := make([]string, 0, 2)
	for _, project := range outliner.projectsInCatalogOrder(outliner.workModeScopeProjects()) {
		want = append(want, project.Slug)
	}
	if got := searchACValues(outliner); !slices.Equal(got, want) {
		t.Fatalf("project values = %v, want the loaded scope %v", got, want)
	}

	// Narrowing the loaded scope narrows the offer in the same pass.
	outliner.loadedProjects = []*models.Project{alpha}
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	typeSearchAC(outliner, "%")
	if got := searchACValues(outliner); slices.Contains(got, beta.Slug) {
		t.Fatalf("project values = %v, want no project outside the loaded scope", got)
	}
}

// TestSearchAutocompleteRanksDynamicValuesStably checks both halves of the
// ordering rule: the fuzzy score decides, and an exact tie keeps the order the
// candidates were encountered in so the list does not shuffle under the cursor.
func TestSearchAutocompleteRanksDynamicValuesStably(t *testing.T) {
	tasks := []*models.Task{
		searchACTask("t1", []string{"zebra"}, ""),
		searchACTask("t2", []string{"apex"}, ""),
		searchACTask("t3", []string{"apricot"}, ""),
		searchACTask("t4", []string{"ap"}, ""),
	}
	outliner := searchACOutliner(primaryRowsProject("alpha", tasks...))

	typeSearchAC(outliner, "#")
	if got := searchACValues(outliner); !slices.Equal(got, []string{"zebra", "apex", "apricot", "ap"}) {
		t.Fatalf("unfiltered tag order = %v, want source order when every score ties", got)
	}

	typeSearchAC(outliner, "#ap")
	got := searchACValues(outliner)
	if slices.Contains(got, "zebra") {
		t.Fatalf("filtered tag values = %v, want the non-matching tag dropped", got)
	}
	if len(got) != 3 {
		t.Fatalf("filtered tag values = %v, want the three ap-matching tags", got)
	}
	// Whatever the scorer prefers, two values it scores identically must appear in
	// the order they were collected.
	for i := 1; i < len(got); i++ {
		if got[i-1] == got[i] {
			t.Fatalf("filtered tag values = %v, want no duplicate", got)
		}
	}
	if got[0] != "ap" {
		t.Errorf("filtered tag values = %v, want the exact match first", got)
	}
}

// TestSearchAutocompleteAcceptsMidQueryWithoutLosingTrailingTokens is the
// cursor-aware acceptance contract, exercised over multi-byte text on both sides
// of the caret so a byte/rune confusion cannot pass.
func TestSearchAutocompleteAcceptsMidQueryWithoutLosingTrailingTokens(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))

	query := "café #inf naïve $done"
	caret := utf8.RuneCountInString("café #inf")
	typeSearchACAt(outliner, query, caret)
	if !outliner.searchAutocomplete.active() {
		t.Fatalf("mid-query token opened no suggestions")
	}
	if got := searchACValues(outliner); !slices.Equal(got, []string{"infra"}) {
		t.Fatalf("mid-query values = %v, want the matching tag", got)
	}

	// The inserted token always ends in its own space and the preserved suffix
	// keeps the one it already had, exactly as the shared primitive does on the
	// quick-edit surface; the tokens after the caret survive intact, which is what
	// the acceptance owes them.
	outliner.acceptSearchSuggestion()
	if got, want := outliner.SearchInputValue(), "café #infra  naïve $done"; got != want {
		t.Fatalf("accepted query = %q, want %q", got, want)
	}
	if got, want := outliner.SearchInputCursor(), utf8.RuneCountInString("café #infra "); got != want {
		t.Errorf("caret = %d, want %d (after the inserted token and its space)", got, want)
	}
	if outliner.searchAutocomplete.active() {
		t.Error("the pane stayed open over the trailing space it just inserted")
	}
	if got, want := outliner.SearchQuery(), "café #infra  naïve $done"; got != want {
		t.Errorf("applied query = %q, want the accepted text %q", got, want)
	}
}

// TestSearchAutocompleteAcceptAtEndOfQueryAppends is the ordinary case beside the
// mid-query one: nothing follows the caret, so nothing is preserved after it.
func TestSearchAutocompleteAcceptAtEndOfQueryAppends(t *testing.T) {
	outliner := searchACOutliner(primaryRowsProject("alpha", searchACTask("t1", nil, "")))

	typeSearchAC(outliner, "auth !")
	outliner.acceptSearchSuggestion()
	if got, want := outliner.SearchInputValue(), "auth !1 "; got != want {
		t.Fatalf("accepted query = %q, want %q", got, want)
	}
}

// TestSearchAutocompleteKeysClaimOnlyTheDropdownControls is the key contract. Tab
// and the four navigation keys belong to the pane; every other key, printable
// letters among them, still reaches the search input, so completion never makes a
// character untypeable.
func TestSearchAutocompleteKeysClaimOnlyTheDropdownControls(t *testing.T) {
	tasks := []*models.Task{
		searchACTask("t1", []string{"alpha"}, ""),
		searchACTask("t2", []string{"beta"}, ""),
		searchACTask("t3", []string{"gamma"}, ""),
	}
	outliner := searchACOutliner(primaryRowsProject("alpha", tasks...))
	typeSearchAC(outliner, "#")

	for _, key := range []string{"down", "ctrl+n"} {
		before := outliner.searchAutocomplete.ac.Cursor
		if !outliner.handleSearchAutocompleteKey(key) {
			t.Fatalf("%s was not claimed by the open pane", key)
		}
		if got := outliner.searchAutocomplete.ac.Cursor; got != before+1 {
			t.Errorf("%s moved the cursor to %d, want %d", key, got, before+1)
		}
	}
	for _, key := range []string{"up", "ctrl+p"} {
		before := outliner.searchAutocomplete.ac.Cursor
		if !outliner.handleSearchAutocompleteKey(key) {
			t.Fatalf("%s was not claimed by the open pane", key)
		}
		if got := outliner.searchAutocomplete.ac.Cursor; got != before-1 {
			t.Errorf("%s moved the cursor to %d, want %d", key, got, before-1)
		}
	}

	// Enter belongs to the search, not to the pane: it reaches the existing
	// confirm-and-land path unchanged.
	if outliner.handleSearchAutocompleteKey("enter") {
		t.Error("the pane claimed Enter, which must stay the result-landing key")
	}
	// j and k belong to the results while the pane covers them, and to the query
	// again the moment it closes.
	for _, key := range []string{"j", "k"} {
		if !outliner.handleSearchAutocompleteKey(key) {
			t.Errorf("the open pane did not claim %q for result navigation", key)
		}
	}
	// Every other printable key still types, so completion makes no letter
	// permanently unreachable.
	for _, key := range []string{"a", "z", "1"} {
		if outliner.handleSearchAutocompleteKey(key) {
			t.Errorf("the pane claimed %q, which must stay typable in a query", key)
		}
	}

	if !outliner.handleSearchAutocompleteKey("tab") {
		t.Fatal("Tab was not claimed by the open pane")
	}
	if got := outliner.SearchInputValue(); !strings.HasPrefix(got, "#") || !strings.HasSuffix(got, " ") {
		t.Errorf("Tab produced %q, want an accepted token with its trailing space", got)
	}

	// A closed pane claims nothing at all.
	if outliner.handleSearchAutocompleteKey("tab") {
		t.Error("a closed pane claimed Tab")
	}
}

// TestSearchAutocompleteJKNavigateResultsWhilePaneOpen is the two-axis contract:
// while the pane covers the results, the arrow family walks the suggestions and
// j/k walk the results underneath them, and neither axis disturbs the other or
// the query. It drives real key presses so the whole path — the pane's claim, the
// shared list navigation, and the search input it must not reach — is exercised.
func TestSearchAutocompleteJKNavigateResultsWhilePaneOpen(t *testing.T) {
	tasks := []*models.Task{
		searchACTask("task-one", []string{"alpha"}, ""),
		searchACTask("task-two", []string{"beta"}, ""),
		searchACTask("task-three", []string{"gamma"}, ""),
	}
	outliner := searchACOutliner(primaryRowsProject("alpha", tasks...))
	for _, key := range []string{"t", "a", "s", "k", "-", " ", "#"} {
		outliner.handleKeyMsg(syntheticKeyStr(key))
	}
	if !outliner.searchAutocomplete.active() {
		t.Fatal("the tag sigil opened no pane for j/k to navigate under")
	}
	if got, want := outliner.SearchInputValue(), "task- #"; got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
	if len(outliner.rows) < 3 {
		t.Fatalf("search projection produced %d rows, want the three matching tasks", len(outliner.rows))
	}

	// The suggestion cursor is moved off its first row first, so a j that leaked
	// into the dropdown would be visible as a suggestion move rather than hidden
	// behind an already-clamped cursor.
	outliner.handleKeyMsg(syntheticKeyStr("down"))
	suggestionCursor := outliner.searchAutocomplete.ac.Cursor
	if suggestionCursor != 1 {
		t.Fatalf("suggestion cursor = %d, want 1 after one Down", suggestionCursor)
	}
	resultCursor := outliner.cursor

	outliner.handleKeyMsg(syntheticKeyStr("j"))
	if got, want := outliner.cursor, resultCursor+1; got != want {
		t.Errorf("j left the result cursor at %d, want %d", got, want)
	}
	outliner.handleKeyMsg(syntheticKeyStr("j"))
	if got, want := outliner.cursor, resultCursor+2; got != want {
		t.Errorf("a second j left the result cursor at %d, want %d", got, want)
	}
	outliner.handleKeyMsg(syntheticKeyStr("k"))
	if got, want := outliner.cursor, resultCursor+1; got != want {
		t.Errorf("k left the result cursor at %d, want %d", got, want)
	}

	// Neither letter typed, and neither moved the suggestion the pane is offering.
	if got, want := outliner.SearchInputValue(), "task- #"; got != want {
		t.Errorf("query after j/k = %q, want it untouched at %q", got, want)
	}
	if got := outliner.searchAutocomplete.ac.Cursor; got != suggestionCursor {
		t.Errorf("j/k moved the suggestion cursor to %d, want it left at %d", got, suggestionCursor)
	}
	if !outliner.searchAutocomplete.active() {
		t.Error("result navigation closed the pane it was supposed to navigate under")
	}

	// Closing the pane gives both letters back to the query, which is what keeps
	// the claim honest: no character is permanently unreachable.
	outliner.handleKeyMsg(syntheticKeyStr("esc"))
	if outliner.searchAutocomplete.active() {
		t.Fatal("Esc left the pane open")
	}
	outliner.handleKeyMsg(syntheticKeyStr("j"))
	if got, want := outliner.SearchInputValue(), "task- #j"; got != want {
		t.Errorf("query after j with the pane closed = %q, want %q", got, want)
	}
}

// TestSearchAutocompleteEnterStillLandsOnTheResult holds the one key the pane
// must never intercept: Enter belongs to the search's own landing path, and an
// open pane may not turn it into an acceptance.
func TestSearchAutocompleteEnterStillLandsOnTheResult(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))
	typeSearchAC(outliner, "#inf")
	if !outliner.searchAutocomplete.active() {
		t.Fatal("no pane open for Enter to bypass")
	}

	outliner.handleKeyMsg(syntheticKeyStr("enter"))
	if got := outliner.SearchInputValue(); got != "#inf" {
		t.Errorf("query after Enter = %q, want the typed text rather than an accepted suggestion", got)
	}
	if outliner.IsSearchMode() {
		t.Error("Enter left the input focused instead of landing on the results")
	}
	if got, want := outliner.SearchQuery(), "#inf"; got != want {
		t.Errorf("applied query = %q, want %q", got, want)
	}
	if outliner.searchAutocomplete.active() {
		t.Error("landing on the results left the pane open")
	}
}

// TestSearchAutocompleteEscCascadesSuggestionsThenSearch is the required two-step
// unwind: the first Esc gives back the rows the pane covered, the second gives
// back the projection.
func TestSearchAutocompleteEscCascadesSuggestionsThenSearch(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))
	typeSearchAC(outliner, "#")
	if !outliner.searchAutocomplete.active() {
		t.Fatal("no pane to unwind")
	}

	outliner.handleKeyMsg(syntheticKeyStr("esc"))
	if outliner.searchAutocomplete.active() {
		t.Fatal("the first Esc left the pane open")
	}
	if !outliner.IsSearchMode() {
		t.Fatal("the first Esc closed the search instead of the pane")
	}
	if got := outliner.SearchInputValue(); got != "#" {
		t.Errorf("query after the first Esc = %q, want it untouched", got)
	}

	outliner.handleKeyMsg(syntheticKeyStr("esc"))
	if outliner.IsSearchMode() {
		t.Error("the second Esc left the search active")
	}
}

// TestSearchAutocompleteClosesWhenTheActiveTokenStopsOffering covers the third
// close reason beside Esc and search ending: the token under the caret simply
// runs out of candidates as more is typed.
func TestSearchAutocompleteClosesWhenTheActiveTokenStopsOffering(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))

	typeSearchAC(outliner, "#in")
	if !outliner.searchAutocomplete.active() {
		t.Fatal("#in offered nothing")
	}
	typeSearchAC(outliner, "#zzz")
	if outliner.searchAutocomplete.active() {
		t.Fatalf("#zzz kept the pane open: %v", searchACValues(outliner))
	}
	// Free text is not a token at all.
	typeSearchAC(outliner, "plain text")
	if outliner.searchAutocomplete.active() {
		t.Fatalf("free text opened a pane: %v", searchACValues(outliner))
	}
	// Leaving search closes it regardless of what was on screen.
	typeSearchAC(outliner, "#")
	outliner.ClearSearch()
	if outliner.searchAutocomplete.active() {
		t.Error("clearing the search left the pane open")
	}
}

// searchACWideOutliner builds a scope with enough tags to overflow the row
// budget in both directions.
func searchACWideOutliner(count int) *OutlinerModel {
	tasks := make([]*models.Task, 0, count)
	for i := range count {
		tasks = append(tasks, searchACTask(fmt.Sprintf("t%d", i), []string{fmt.Sprintf("tag%02d", i)}, ""))
	}
	return searchACOutliner(primaryRowsProject("alpha", tasks...))
}

// TestSearchAutocompleteRendersAtMostFiveRowsIncludingOverflow holds the pane to
// its total budget at the top, the middle, and the bottom of a long list, where
// the number of rows spent on hints differs.
func TestSearchAutocompleteRendersAtMostFiveRowsIncludingOverflow(t *testing.T) {
	outliner := searchACWideOutliner(12)
	typeSearchAC(outliner, "#")

	cases := []struct {
		name       string
		cursor     int
		wantTop    bool
		wantBottom bool
	}{
		{"top", 0, false, true},
		{"middle", 6, true, true},
		{"bottom", 11, true, false},
	}
	for _, tc := range cases {
		outliner.searchAutocomplete.ac.Cursor = tc.cursor
		outliner.searchAutocomplete.ac.ClampCursor()
		rendered := ansi.Strip(outliner.renderSearchAutocompleteDropdown(120))
		lines := strings.Split(rendered, "\n")
		if len(lines) > outlinerSearchSuggestionLimit {
			t.Errorf("%s: rendered %d rows, want at most %d", tc.name, len(lines), outlinerSearchSuggestionLimit)
		}
		if got := strings.Contains(rendered, "↑"); got != tc.wantTop {
			t.Errorf("%s: top overflow hint = %v, want %v:\n%s", tc.name, got, tc.wantTop, rendered)
		}
		if got := strings.Contains(rendered, "↓"); got != tc.wantBottom {
			t.Errorf("%s: bottom overflow hint = %v, want %v:\n%s", tc.name, got, tc.wantBottom, rendered)
		}
		if !strings.Contains(rendered, outliner.searchAutocomplete.ac.Selected().Display) {
			t.Errorf("%s: the selected row scrolled out of its own window:\n%s", tc.name, rendered)
		}
	}
}

// TestSearchAutocompleteViewportSubtractsTheRenderedHeight is the layout
// invariant: the rows the pane occupies are taken from the list, so the cursor
// row and the footer stay on screen.
func TestSearchAutocompleteViewportSubtractsTheRenderedHeight(t *testing.T) {
	outliner := searchACWideOutliner(12)
	outliner.SetSize(120, 24)

	outliner.renderOutlinerTreeContent(120)
	baseline := outliner.lastViewport.visibleRows

	typeSearchAC(outliner, "#")
	if !outliner.searchAutocomplete.active() {
		t.Fatal("no pane to reserve for")
	}
	content := outliner.renderOutlinerTreeContent(120)
	withPane := outliner.lastViewport.visibleRows

	// Measure the pane exactly as the render did: whatever height it drew is the
	// height the list lost.
	dropdownHeight := lipgloss.Height(outliner.searchAutocomplete.ac.RenderDropdownWithLimit(
		outlinerSearchDropdownIndent, outlinerSearchSuggestionLimit))
	if got, want := baseline-withPane, dropdownHeight; got != want {
		t.Fatalf("rows reserved = %d, want the pane's own height %d", got, want)
	}
	if withPane < 1 {
		t.Fatalf("viewport collapsed to %d rows", withPane)
	}
	// The whole frame must still fit the terminal it was sized to.
	if got := lipgloss.Height(content); got > outliner.contentHeight()+1 {
		t.Errorf("frame height = %d, want at most %d", got, outliner.contentHeight()+1)
	}
}

// TestSearchAutocompleteRenderFitsNarrowAndMinimumLayouts covers the two ends of
// the size range: a width the rows must be cut to, and a height with room for a
// single row and no hint at all.
func TestSearchAutocompleteRenderFitsNarrowAndMinimumLayouts(t *testing.T) {
	outliner := searchACWideOutliner(12)

	outliner.SetSize(24, 40)
	typeSearchAC(outliner, "#")
	rendered := ansi.Strip(outliner.renderSearchAutocompleteDropdown(14))
	for _, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > 14 {
			t.Errorf("row %q is %d cells, want at most 14", line, got)
		}
	}

	// contentHeight()-2 is the pane's ceiling, so a three-row terminal leaves room
	// for exactly one row and it must be the selected one rather than a hint.
	outliner.SetSize(120, 4)
	outliner.searchAutocomplete.ac.Cursor = 6
	outliner.searchAutocomplete.ac.ClampCursor()
	minimum := ansi.Strip(outliner.renderSearchAutocompleteDropdown(120))
	lines := strings.Split(minimum, "\n")
	if len(lines) != 1 {
		t.Fatalf("minimum-height pane rendered %d rows, want 1:\n%s", len(lines), minimum)
	}
	if !strings.Contains(minimum, outliner.searchAutocomplete.ac.Selected().Display) {
		t.Errorf("minimum-height pane dropped the selection for a hint: %q", minimum)
	}

	// Even at the smallest usable terminal the list keeps a row.
	outliner.SetSize(40, 3)
	outliner.renderOutlinerTreeContent(40)
	if got := outliner.lastViewport.visibleRows; got < 1 {
		t.Errorf("minimum-height viewport = %d rows, want at least 1", got)
	}
}

// TestSearchAutocompleteGuidanceSwapsOnlyWhileTheDropdownIsOpen keeps the footer
// honest about which Esc the next press is.
func TestSearchAutocompleteGuidanceSwapsOnlyWhileTheDropdownIsOpen(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))

	typeSearchAC(outliner, "auth")
	spec, ok := outliner.ContextualGuidance()
	if !ok {
		t.Fatal("search mode published no guidance")
	}
	if !strings.Contains(searchACGuidanceText(spec), "Esc clear search") {
		t.Errorf("plain search guidance = %q, want the ordinary search hints", searchACGuidanceText(spec))
	}

	typeSearchAC(outliner, "#")
	spec, ok = outliner.ContextualGuidance()
	if !ok {
		t.Fatal("an open pane published no guidance")
	}
	text := searchACGuidanceText(spec)
	for _, want := range []string{"Tab accept", "Esc close suggestions"} {
		if !strings.Contains(text, want) {
			t.Errorf("dropdown guidance = %q, want it to name %q", text, want)
		}
	}
	if strings.Contains(text, "Esc clear search") {
		t.Errorf("dropdown guidance = %q, want it to stop promising Esc clears the search", text)
	}
	if !strings.Contains(text, "Enter apply search") {
		t.Errorf("dropdown guidance = %q, want the search's own key still named", text)
	}
}

func searchACGuidanceText(spec GuidanceSpec) string {
	parts := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		parts = append(parts, guidanceHintText(hint))
	}
	return strings.Join(parts, " | ")
}
