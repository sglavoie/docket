package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
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
	if len(edit.Clears) != 1 || edit.Clears[0].Value != dependencyClearValue {
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
			result, err := parseQuickCapture(tt.input, home, projects, false)
			if err != nil {
				t.Fatalf("parseQuickCapture(%q): %v", tt.input, err)
			}
			if got := dependencyLocatorStrings(result.dependencyLocators()); !slices.Equal(got, tt.want) {
				t.Fatalf("dependencies = %v, want %v", got, tt.want)
			}
			if result.title != "New work" {
				t.Fatalf("title = %q, want %q", result.title, "New work")
			}
		})
	}
}

// TestCaptureResolvesBareAliasesAgainstTheRoutedProject fixes the reading a bare
// alias gets: the project the line ends in, not the one the composer opened on.
func TestCaptureResolvesBareAliasesAgainstTheRoutedProject(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}

	result, err := parseQuickCapture("New work %routed ~1", home, projects, false)
	if err != nil {
		t.Fatalf("parseQuickCapture: %v", err)
	}
	if got := dependencyLocatorStrings(result.dependencyLocators()); !slices.Equal(got, []string{"routed-project:routed-1"}) {
		t.Fatalf("dependencies = %v, want the routed project's #1", got)
	}

	// A qualified alias is unaffected by the route, and reaches back home.
	result, err = parseQuickCapture("New work %routed ~home:2", home, projects, false)
	if err != nil {
		t.Fatalf("parseQuickCapture: %v", err)
	}
	if got := dependencyLocatorStrings(result.dependencyLocators()); !slices.Equal(got, []string{"home-project:home-2"}) {
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
			_, err := parseQuickCapture(tt.input, home, projects, false)
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
	result, err := parseQuickCapture("Path to~daily file", home, []*models.Project{home, routed}, false)
	if err != nil {
		t.Fatalf("parseQuickCapture: %v", err)
	}
	if result.title != "Path to~daily file" {
		t.Fatalf("title = %q, want the tilde preserved in prose", result.title)
	}
	if len(result.dependencyLocators()) != 0 {
		t.Fatalf("prose minted dependencies: %v", dependencyLocatorStrings(result.dependencyLocators()))
	}
}

func TestQuickEditDependencyPatchHalves(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]
	task.Dependencies = []models.TaskReference{
		{ProjectID: "home-project", TaskID: "home-3"},
		{ProjectID: "gone-project", TaskID: "gone-9"},
	}

	tests := []struct {
		name       string
		input      string
		wantAdd    []string
		wantRemove []string
		wantClear  bool
	}{
		{name: "no token preserves the stored set", input: "Edited task"},
		{name: "addition", input: "Edited task ~2", wantAdd: []string{"home-project:home-2"}},
		{
			name:       "removal of a closed target",
			input:      "Edited task -~3",
			wantRemove: []string{"home-project:home-3"},
		},
		{
			name:       "removal of an unavailable target by stable locator",
			input:      "Edited task -~gone-project:gone-9",
			wantRemove: []string{"gone-project:gone-9"},
		},
		{
			name:       "one line adds and removes",
			input:      "Edited task -~3 ~routed:1",
			wantAdd:    []string{"routed-project:routed-1"},
			wantRemove: []string{"home-project:home-3"},
		},
		{name: "clear", input: "Edited task ~0", wantClear: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseQuickEdit(tt.input, task, home, projects)
			if err != nil {
				t.Fatalf("parseQuickEdit(%q): %v", tt.input, err)
			}
			patch := result.dependencyPatch()
			if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, tt.wantAdd) {
				t.Errorf("add = %v, want %v", got, tt.wantAdd)
			}
			if got := dependencyLocatorStrings(patch.Remove); !slices.Equal(got, tt.wantRemove) {
				t.Errorf("remove = %v, want %v", got, tt.wantRemove)
			}
			if patch.Clear != tt.wantClear {
				t.Errorf("clear = %v, want %v", patch.Clear, tt.wantClear)
			}
			if tt.wantAdd == nil && tt.wantRemove == nil && !tt.wantClear && !patch.IsZero() {
				t.Errorf("a line naming no dependency produced %+v, want a zero patch", patch)
			}
		})
	}
}

func TestQuickEditDependencyDiagnostics(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clear with an addition", "Edited task ~0 ~2", "clearing dependencies cannot be combined"},
		{"clear with a removal", "Edited task ~0 -~2", "clearing dependencies cannot be combined"},
		{"adding and removing one target", "Edited task ~2 -~2", "conflicting dependency patch"},
		{"self reference", "Edited task ~1", "a task cannot depend on itself"},
		{"terminal addition", "Edited task ~3", "cannot depend on"},
		{"unresolved addition", "Edited task ~999", `dependency not found: "999"`},
		{"ambiguous addition", "Edited task ~Ship", "ambiguous"},
		{"removal is not a clear", "Edited task -~0", "is not a clear form"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseQuickEdit(tt.input, task, home, projects)
			if err == nil {
				t.Fatalf("parseQuickEdit(%q) accepted the line", tt.input)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

// TestQuickEditDependencyPreviewShowsTheResultingSet checks the preview a
// composer paints: the stored set with the patch already applied.
func TestQuickEditDependencyPreviewShowsTheResultingSet(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]
	task.Dependencies = []models.TaskReference{{ProjectID: "home-project", TaskID: "home-3"}}

	previewLocators := func(input string) []string {
		t.Helper()
		result, err := parseQuickEdit(input, task, home, projects)
		if err != nil {
			t.Fatalf("parseQuickEdit(%q): %v", input, err)
		}
		values := make([]string, len(result.preview.Dependencies))
		for i, reference := range result.preview.Dependencies {
			values[i] = reference.Locator()
		}
		return values
	}

	if got := previewLocators("Edited task"); !slices.Equal(got, []string{"home-project:home-3"}) {
		t.Errorf("untouched preview = %v, want the stored set", got)
	}
	if got := previewLocators("Edited task ~2"); !slices.Equal(got, []string{"home-project:home-3", "home-project:home-2"}) {
		t.Errorf("addition preview = %v, want the addition appended", got)
	}
	if got := previewLocators("Edited task -~3"); len(got) != 0 {
		t.Errorf("removal preview = %v, want an empty set", got)
	}
	if got := previewLocators("Edited task ~0"); len(got) != 0 {
		t.Errorf("clear preview = %v, want an empty set", got)
	}
	if len(task.Dependencies) != 1 {
		t.Fatalf("preview mutated the live task: %v", task.Dependencies)
	}
}

func TestCaptureDependencyPreviewNamesEveryReference(t *testing.T) {
	home, routed := dependencyWorkspace()
	q := draftedCaptureInput(t, "New work ~2 ~routed:1", home, []*models.Project{home, routed}, false)
	q.updatePreview()
	if q.previewErr != "" {
		t.Fatalf("preview error: %s", q.previewErr)
	}
	groups, ok := q.previewGroups()
	if !ok {
		t.Fatal("the composer painted no preview")
	}
	var depends *previewGroup
	for i, group := range groups {
		if group.label == "Depends" {
			depends = &groups[i]
		}
	}
	if depends == nil {
		t.Fatalf("preview groups = %+v, want a Depends group", groups)
	}
	want := []string{"~Home #2 Ship the API", "~Routed #1 Routed alpha"}
	if !slices.Equal(depends.chunks, want) {
		t.Fatalf("Depends chunks = %v, want %v", depends.chunks, want)
	}
}

func TestCaptureDependencyAutocompleteOffersOpenWorkAcrossProjects(t *testing.T) {
	home, routed := dependencyWorkspace()
	q := draftedCaptureInput(t, "New work ~", home, []*models.Project{home, routed}, false)
	if q.ac.Mode != acModeDependency {
		t.Fatalf("autocomplete mode = %d, want acModeDependency", q.ac.Mode)
	}

	values := suggestionValues(q.ac.Items)
	// Same-project targets insert a local alias and cross-project ones a
	// qualified alias, so an accepted row always parses back to the row it named.
	for _, want := range []string{"1", "2", "4", "routed:1", "routed:2"} {
		if !slices.Contains(values, want) {
			t.Errorf("dependency suggestions omitted %q: %v", want, values)
		}
	}
	// #3 is done, and capture has no set to clear.
	for _, unwanted := range []string{"3", dependencyClearValue} {
		if slices.Contains(values, unwanted) {
			t.Errorf("dependency suggestions offered %q: %v", unwanted, values)
		}
	}
	displays := suggestionDisplays(q.ac.Items)
	if !slices.Contains(displays, "~2 "+models.StateBacklog.Icon()+" Home #2 Ship the API") {
		t.Fatalf("dependency displays = %v, want a project, state, short ID, and title row", displays)
	}
}

func TestCaptureDependencyAutocompleteFollowsTheDraftedRouteAndDropsUsedTargets(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}

	q := draftedCaptureInput(t, "New work %routed ~", home, projects, false)
	values := suggestionValues(q.ac.Items)
	if !slices.Contains(values, "1") || !slices.Contains(values, "2") {
		t.Fatalf("routed draft = %v, want the routed project's tasks under local aliases", values)
	}
	if !slices.Contains(values, "home:2") {
		t.Fatalf("routed draft = %v, want home targets qualified", values)
	}

	// A target the line already names is withheld under every spelling of it.
	withheld := map[string]string{
		"New work ~2 ~":      "2",
		"New work ~Shipp ~":  "4",
		"New work ~home:2 ~": "2",
	}
	for draft, alias := range withheld {
		q := draftedCaptureInput(t, draft, home, projects, false)
		if slices.Contains(suggestionValues(q.ac.Items), alias) {
			t.Errorf("draft %q re-offered %q, a target it already names: %v", draft, alias, suggestionValues(q.ac.Items))
		}
	}
}

func TestRowEditorDependencyAutocompleteSeparatesAdditionsFromRemovals(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]
	task.Dependencies = []models.TaskReference{
		{ProjectID: "home-project", TaskID: "home-3"},
		{ProjectID: "gone-project", TaskID: "gone-9"},
	}

	addition := draftedRowEditor(t, "Edited task ~", task, home, projects)
	if addition.ac.Mode != acModeDependency {
		t.Fatalf("addition mode = %d, want acModeDependency", addition.ac.Mode)
	}
	values := suggestionValues(addition.ac.Items)
	if len(values) == 0 || values[0] != dependencyClearValue {
		t.Fatalf("addition suggestions = %v, want the ~0 clear row first", values)
	}
	for _, unwanted := range []string{"1", "3", "gone-project:gone-9"} {
		if slices.Contains(values, unwanted) {
			t.Errorf("addition offered %q: %v", unwanted, values)
		}
	}
	if !slices.Contains(values, "2") || !slices.Contains(values, "routed:1") {
		t.Errorf("addition suggestions = %v, want open local and qualified targets", values)
	}

	removal := draftedRowEditor(t, "Edited task -~", task, home, projects)
	if removal.ac.Mode != acModeRemoveDependency {
		t.Fatalf("removal mode = %d, want acModeRemoveDependency", removal.ac.Mode)
	}
	removalValues := suggestionValues(removal.ac.Items)
	// A removal reaches exactly the stored set, closed and unloaded targets
	// included, and never offers a clear.
	if !slices.Equal(removalValues, []string{"3", "gone-project:gone-9"}) {
		t.Fatalf("removal suggestions = %v, want the stored set in stored order", removalValues)
	}
	if !slices.Contains(suggestionDisplays(removal.ac.Items), "-~gone-project:gone-9 (unavailable)") {
		t.Fatalf("removal displays = %v, want the unavailable reference named by locator", suggestionDisplays(removal.ac.Items))
	}
}

// TestRowEditorDependencyCompletionAcceptsMidLine keeps the reclaimed sigil
// inside the shared mid-cursor contract: the accepted token replaces the partial
// under the cursor and the tail survives.
func TestRowEditorDependencyCompletionAcceptsMidLine(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]

	q := NewQuickEditComponent()
	q.EnterWithContext(task, home, projects, false)
	q.input.SetValue("Edited task ~Shipp #tag")
	q.input.SetCursor(len([]rune("Edited task ~Shipp")))
	q.refreshDraft()

	if q.ac.Mode != acModeDependency {
		t.Fatalf("autocomplete mode = %d, want acModeDependency", q.ac.Mode)
	}
	// BuildAcceptedInputAt always closes the accepted token with a space and keeps
	// everything past the cursor verbatim, so the tail keeps its own separator.
	q.acceptSuggestion()
	if got, want := q.input.Value(), "Edited task ~4  #tag"; got != want {
		t.Fatalf("accepted input = %q, want %q", got, want)
	}
	result, err := parseQuickEdit(q.input.Value(), task, home, projects)
	if err != nil {
		t.Fatalf("parseQuickEdit after acceptance: %v", err)
	}
	if got := dependencyLocatorStrings(result.dependencyPatch().Add); !slices.Equal(got, []string{"home-project:home-4"}) {
		t.Fatalf("accepted addition = %v, want the completed task", got)
	}
}

// TestRowEditorDependencySuggestionsFollowLiveMutations checks that the offered
// set is derived per keystroke rather than cached at entry.
func TestRowEditorDependencySuggestionsFollowLiveMutations(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]

	q := draftedRowEditor(t, "Edited task ~", task, home, projects)
	if !slices.Contains(suggestionValues(q.ac.Items), "4") {
		t.Fatalf("suggestions = %v, want the open #4", suggestionValues(q.ac.Items))
	}

	home.Tasks[3].WorkflowState = models.StateDone
	task.Dependencies = []models.TaskReference{{ProjectID: "routed-project", TaskID: "routed-1"}}
	q.refreshDraft()

	values := suggestionValues(q.ac.Items)
	if slices.Contains(values, "4") {
		t.Errorf("a task closed since entry stayed on offer: %v", values)
	}
	if slices.Contains(values, "routed:1") {
		t.Errorf("a reference stored since entry stayed on offer: %v", values)
	}
}

// TestMultilineCaptureCarriesDependenciesPerLine checks the composer's batch
// wiring: each line's references reach that line's request, and one bad
// reference refuses the whole batch before anything is committed.
func TestMultilineCaptureCarriesDependenciesPerLine(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:        ComposerProject,
		Project:     home,
		AllProjects: projects,
	}, nil)
	composer.input.SetValue("First ~2\nSecond ~routed:1 ~4")

	requests, ok := composer.parseCompactLines()
	if !ok || len(requests) != 2 {
		t.Fatalf("parseCompactLines = %+v, %v", requests, ok)
	}
	if got := dependencyLocatorStrings(requests[0].Dependencies); !slices.Equal(got, []string{"home-project:home-2"}) {
		t.Fatalf("first request dependencies = %v", got)
	}
	want := []string{"routed-project:routed-1", "home-project:home-4"}
	if got := dependencyLocatorStrings(requests[1].Dependencies); !slices.Equal(got, want) {
		t.Fatalf("second request dependencies = %v, want %v", got, want)
	}

	composer.input.SetValue("First ~2\nSecond ~999")
	requests, ok = composer.parseCompactLines()
	if ok || requests != nil {
		t.Fatalf("parseCompactLines = %+v, %v; want the whole batch refused", requests, ok)
	}
	if len(composer.errors) != 1 || composer.errors[0].line != 2 {
		t.Fatalf("errors = %+v, want one on line 2", composer.errors)
	}
	if !strings.Contains(composer.errors[0].message, "dependency not found") {
		t.Fatalf("error = %q, want a dependency-reference diagnostic", composer.errors[0].message)
	}
}

// TestDependencyGrammarReachesBothHelpSurfaces checks the two registry-derived
// teaching surfaces: the composer's syntax guide, which is capture-only, and the
// `?` grammar section, which unions every mutation surface and so must also
// carry the edit-only removal and clear forms.
func TestDependencyGrammarReachesBothHelpSurfaces(t *testing.T) {
	home, routed := dependencyWorkspace()
	q := draftedCaptureInput(t, "", home, []*models.Project{home, routed}, false)

	var guideForms []string
	for _, group := range q.syntaxGuideGroups() {
		for _, entry := range group.entries {
			guideForms = append(guideForms, entry.forms)
		}
	}
	if !slices.Contains(guideForms, "~task") {
		t.Fatalf("capture syntax guide = %v, want a ~task row", guideForms)
	}
	for _, form := range guideForms {
		if strings.Contains(form, "-~") || strings.Contains(form, "~0") {
			t.Fatalf("capture syntax guide teaches an edit-only form: %q", form)
		}
	}

	var helpKeys, helpDescriptions []string
	for _, item := range grammarHelpItems() {
		helpKeys = append(helpKeys, item.Key)
		helpDescriptions = append(helpDescriptions, item.Description)
	}
	if !slices.Contains(helpKeys, "~task ~0") {
		t.Fatalf("? grammar keys = %v, want the dependency row with its clear form", helpKeys)
	}
	if !slices.Contains(helpKeys, "-~task") {
		t.Fatalf("? grammar keys = %v, want the removal row", helpKeys)
	}
	if !slices.ContainsFunc(helpDescriptions, func(description string) bool {
		return strings.Contains(description, "Clear dependencies")
	}) {
		t.Fatalf("? grammar descriptions = %v, want the clear operation named", helpDescriptions)
	}
}
