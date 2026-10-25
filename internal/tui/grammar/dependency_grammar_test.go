package grammar

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
)

// dependencyWorkspace is the two-project fixture every dependency grammar test
// reads. The home project holds the task under edit plus one closed and one
// ambiguous-by-prefix target; the routed project exists so a qualified alias has
// somewhere to point that a bare alias cannot reach.
func dependencyWorkspace() (home, routed *models.Project) {
	home = &models.Project{
		ID: "home-project", Name: "Home", Slug: "home",
		Tasks: []*models.Task{
			{ID: "home-1", ShortID: 1, Title: "Edited task", WorkflowState: models.StateBacklog},
			{ID: "home-2", ShortID: 2, Title: "Ship the API", WorkflowState: models.StateBacklog},
			{ID: "home-3", ShortID: 3, Title: "Closed groundwork", WorkflowState: models.StateDone},
			{ID: "home-4", ShortID: 4, Title: "Shipping receipts", WorkflowState: models.StateBacklog},
		},
	}
	routed = &models.Project{
		ID: "routed-project", Name: "Routed", Slug: "routed",
		Tasks: []*models.Task{
			{ID: "routed-1", ShortID: 1, Title: "Routed alpha", WorkflowState: models.StateBacklog},
			{ID: "routed-2", ShortID: 2, Title: "Routed beta", WorkflowState: models.StateDoing},
		},
	}
	home.BuildTaskTree()
	routed.BuildTaskTree()
	return home, routed
}

func dependencyLocatorStrings(locators []app.TaskLocator) []string {
	values := make([]string, len(locators))
	for i, locator := range locators {
		values[i] = locator.Reference().Locator()
	}
	return values
}

func TestDependencyGrammarIsRegisteredOnBothMutationSurfaces(t *testing.T) {
	addition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDependency)
	if !ok {
		t.Fatal("capture does not advertise the dependency field")
	}
	if addition.Trigger != "~" || addition.Cardinality != textgrammar.CardinalityRepeatable {
		t.Fatalf("dependency addition = trigger %q cardinality %v, want a repeatable ~", addition.Trigger, addition.Cardinality)
	}
	if len(addition.Clears) != 0 {
		t.Fatalf("capture advertises a dependency clear form: %+v", addition.Clears)
	}
	if slices.Contains(addition.Forms, "~0") {
		t.Fatalf("capture advertises ~0: %v", addition.Forms)
	}

	edit, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, textgrammar.FieldDependency)
	if !ok {
		t.Fatal("edit does not advertise the dependency field")
	}
	if len(edit.Clears) != 1 || edit.Clears[0].Value != DependencyClearValue {
		t.Fatalf("edit dependency clears = %+v, want one ~0 form", edit.Clears)
	}
	removal, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, textgrammar.FieldRemoveDependency)
	if !ok {
		t.Fatal("edit does not advertise the dependency removal field")
	}
	if removal.Trigger != "-~" || removal.Cardinality != textgrammar.CardinalityRepeatable {
		t.Fatalf("dependency removal = trigger %q cardinality %v, want a repeatable -~", removal.Trigger, removal.Cardinality)
	}

	// The removal form is tokenized at capture so the refusal can name it, but it
	// is never taught there.
	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceCapture) {
		if definition.Field == textgrammar.FieldRemoveDependency {
			t.Fatalf("capture advertises %s, which it can only refuse", definition.Trigger)
		}
	}
}

// TestDependencyTokensAreParsedBeforeTagRewrites pins the parse order the two
// tilde fields need: -~ before ~ so a removal is never read as an addition of a
// tilde-prefixed alias, and both before #tag so peeling a tag cannot leave a
// whitespace-preceded tilde behind.
func TestDependencyTokensAreParsedBeforeTagRewrites(t *testing.T) {
	fields := make([]textgrammar.Field, 0, 4)
	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceEdit) {
		switch definition.Field {
		case textgrammar.FieldRemoveDependency, textgrammar.FieldDependency, textgrammar.FieldTag:
			fields = append(fields, definition.Field)
		default:
			// Only the three fields whose relative order matters here are collected.
		}
	}
	want := []textgrammar.Field{textgrammar.FieldRemoveDependency, textgrammar.FieldDependency, textgrammar.FieldTag}
	if !slices.Equal(fields, want) {
		t.Fatalf("edit parse order = %v, want %v", fields, want)
	}

	parsed := textgrammar.Parse("Work -~2 ~4 #tag", textgrammar.SurfaceEdit)
	if got := parsed.TokensFor(textgrammar.FieldRemoveDependency); len(got) != 1 || got[0].Value != "2" {
		t.Fatalf("removal tokens = %+v, want one 2", got)
	}
	if got := parsed.TokensFor(textgrammar.FieldDependency); len(got) != 1 || got[0].Value != "4" {
		t.Fatalf("addition tokens = %+v, want one 4", got)
	}
	if parsed.Text != "Work" {
		t.Fatalf("residual title = %q, want %q", parsed.Text, "Work")
	}
}

func TestCaptureResolvesDependencyAliasesInEveryForm(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"short id", "New work ~2", []string{"home-project:home-2"}},
		{"unique title prefix", "New work ~Shipp", []string{"home-project:home-4"}},
		{"case insensitive prefix", "New work ~shIPp", []string{"home-project:home-4"}},
		{"qualified slug alias", "New work ~routed:1", []string{"routed-project:routed-1"}},
		{"stable project and task id", "New work ~routed-project:routed-2", []string{"routed-project:routed-2"}},
		{"repeated references keep input order", "New work ~4 ~2", []string{"home-project:home-4", "home-project:home-2"}},
		{"aliases naming one task collapse to the first", "New work ~4 ~Shipp", []string{"home-project:home-4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := capturetext.ParseQuickCapture(tt.input, home, projects, false)
			if err != nil {
				t.Fatalf("parseQuickCapture(%q): %v", tt.input, err)
			}
			if got := dependencyLocatorStrings(result.DependencyLocators()); !slices.Equal(got, tt.want) {
				t.Fatalf("dependencies = %v, want %v", got, tt.want)
			}
			if result.Title != "New work" {
				t.Fatalf("title = %q, want %q", result.Title, "New work")
			}
		})
	}
}

// TestCaptureResolvesBareAliasesAgainstTheRoutedProject fixes the reading a bare
// alias gets: the project the line ends in, not the one the composer opened on.
func TestCaptureResolvesBareAliasesAgainstTheRoutedProject(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}

	result, err := capturetext.ParseQuickCapture("New work %routed ~1", home, projects, false)
	if err != nil {
		t.Fatalf("parseQuickCapture: %v", err)
	}
	if got := dependencyLocatorStrings(result.DependencyLocators()); !slices.Equal(got, []string{"routed-project:routed-1"}) {
		t.Fatalf("dependencies = %v, want the routed project's #1", got)
	}

	// A qualified alias is unaffected by the route, and reaches back home.
	result, err = capturetext.ParseQuickCapture("New work %routed ~home:2", home, projects, false)
	if err != nil {
		t.Fatalf("parseQuickCapture: %v", err)
	}
	if got := dependencyLocatorStrings(result.DependencyLocators()); !slices.Equal(got, []string{"home-project:home-2"}) {
		t.Fatalf("dependencies = %v, want the home project's #2", got)
	}
}

func TestCaptureDependencyDiagnostics(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"removal", "New work -~2", "-~task is not available when capturing"},
		{"clear", "New work ~0", "~0 is not available when capturing"},
		{"unresolved alias", "New work ~999", `dependency not found: "999"`},
		{"unresolved former recurrence value", "New work ~weekly", `dependency not found: "weekly"`},
		{"ambiguous title prefix", "New work ~Ship", "ambiguous"},
		{"unknown project qualifier", "New work ~nowhere:1", `dependency not found: "nowhere:1"`},
		{"terminal target", "New work ~3", "cannot depend on"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := capturetext.ParseQuickCapture(tt.input, home, projects, false)
			if err == nil {
				t.Fatalf("parseQuickCapture(%q) accepted the line", tt.input)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

// TestCaptureLeavesProseTildesAlone keeps the reclaimed sigil from eating text: a
// tilde only opens a reference where a sigil may start.
func TestCaptureLeavesProseTildesAlone(t *testing.T) {
	home, routed := dependencyWorkspace()
	result, err := capturetext.ParseQuickCapture("Path to~daily file", home, []*models.Project{home, routed}, false)
	if err != nil {
		t.Fatalf("parseQuickCapture: %v", err)
	}
	if result.Title != "Path to~daily file" {
		t.Fatalf("title = %q, want the tilde preserved in prose", result.Title)
	}
	if len(result.DependencyLocators()) != 0 {
		t.Fatalf("prose minted dependencies: %v", dependencyLocatorStrings(result.DependencyLocators()))
	}
}

// TestSuggestionCandidatesWithholdHiddenWork keeps hidden work out of the
// completion and picker offers for `>`, `<`, and `~`, stored or inherited, while
// a typed alias still resolves to it.
func TestSuggestionCandidatesWithholdHiddenWork(t *testing.T) {
	home, routed := dependencyWorkspace()
	parentID := "home-2"
	home.Tasks[1].Hidden = true
	home.Tasks = append(home.Tasks, &models.Task{
		ID: "home-5", ShortID: 5, Title: "Inherits hiding", WorkflowState: models.StateBacklog, ParentID: &parentID,
	})
	routed.Tasks[0].Hidden = true
	home.BuildTaskTree()
	routed.BuildTaskTree()

	var structural []string
	for _, task := range StructuralCandidates(home) {
		structural = append(structural, task.ID)
	}
	if want := []string{"home-1", "home-4"}; !slices.Equal(structural, want) {
		t.Fatalf("structural candidates = %v, want %v", structural, want)
	}
	var dependency []string
	for _, candidate := range DependencyAdditionCandidates([]*models.Project{home, routed}, "home-1", nil) {
		dependency = append(dependency, candidate.Task.ID)
	}
	if want := []string{"home-4", "routed-2"}; !slices.Equal(dependency, want) {
		t.Fatalf("dependency candidates = %v, want %v", dependency, want)
	}
	if task, err := capturetext.ResolveParent("2", home); err != nil || task == nil || task.ID != "home-2" {
		t.Fatalf("typed >2 resolved to %v (%v), want the hidden task", task, err)
	}
}
