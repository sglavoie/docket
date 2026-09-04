package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/help"
)

func TestCaptureDependencyAutocompleteOffersOpenWorkAcrossProjects(t *testing.T) {
	home, routed := dependencyWorkspace()
	q := draftedCaptureInput(t, "New work ~", home, []*models.Project{home, routed})
	if q.AC.Mode != grammar.ACModeDependency {
		t.Fatalf("autocomplete mode = %d, want ACModeDependency", q.AC.Mode)
	}

	values := suggestionValues(q.AC.Items)
	// Same-project targets insert a local alias and cross-project ones a
	// qualified alias, so an accepted row always parses back to the row it named.
	for _, want := range []string{"1", "2", "4", "routed:1", "routed:2"} {
		if !slices.Contains(values, want) {
			t.Errorf("dependency suggestions omitted %q: %v", want, values)
		}
	}
	// #3 is done, and capture has no set to clear.
	for _, unwanted := range []string{"3", grammar.DependencyClearValue} {
		if slices.Contains(values, unwanted) {
			t.Errorf("dependency suggestions offered %q: %v", unwanted, values)
		}
	}
	displays := suggestionDisplays(q.AC.Items)
	if !slices.Contains(displays, "~2 "+models.StateBacklog.Icon()+" Home #2 Ship the API") {
		t.Fatalf("dependency displays = %v, want a project, state, short ID, and title row", displays)
	}
}

func TestCaptureDependencyAutocompleteFollowsTheDraftedRouteAndDropsUsedTargets(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}

	q := draftedCaptureInput(t, "New work %routed ~", home, projects)
	values := suggestionValues(q.AC.Items)
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
		q := draftedCaptureInput(t, draft, home, projects)
		if slices.Contains(suggestionValues(q.AC.Items), alias) {
			t.Errorf("draft %q re-offered %q, a target it already names: %v", draft, alias, suggestionValues(q.AC.Items))
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
	if addition.AC.Mode != grammar.ACModeDependency {
		t.Fatalf("addition mode = %d, want ACModeDependency", addition.AC.Mode)
	}
	values := suggestionValues(addition.AC.Items)
	if len(values) == 0 || values[0] != grammar.DependencyClearValue {
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
	if removal.AC.Mode != grammar.ACModeRemoveDependency {
		t.Fatalf("removal mode = %d, want ACModeRemoveDependency", removal.AC.Mode)
	}
	removalValues := suggestionValues(removal.AC.Items)
	// A removal reaches exactly the stored set, closed and unloaded targets
	// included, and never offers a clear.
	if !slices.Equal(removalValues, []string{"3", "gone-project:gone-9"}) {
		t.Fatalf("removal suggestions = %v, want the stored set in stored order", removalValues)
	}
	if !slices.Contains(suggestionDisplays(removal.AC.Items), "-~gone-project:gone-9 (unavailable)") {
		t.Fatalf("removal displays = %v, want the unavailable reference named by locator", suggestionDisplays(removal.AC.Items))
	}
}

// TestRowEditorDependencyCompletionAcceptsMidLine keeps the reclaimed sigil
// inside the shared mid-cursor contract: the accepted token replaces the partial
// under the cursor and the tail survives.
func TestRowEditorDependencyCompletionAcceptsMidLine(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]

	q := grammar.NewQuickEditComponent()
	q.EnterWithContext(task, home, projects, false)
	q.Input.SetValue("Edited task ~Shipp #tag")
	q.Input.SetCursor(len([]rune("Edited task ~Shipp")))
	q.RefreshDraft()

	if q.AC.Mode != grammar.ACModeDependency {
		t.Fatalf("autocomplete mode = %d, want ACModeDependency", q.AC.Mode)
	}
	// BuildAcceptedInputAt always closes the accepted token with a space and keeps
	// everything past the cursor verbatim, so the tail keeps its own separator.
	q.AcceptSuggestion()
	if got, want := q.Input.Value(), "Edited task ~4  #tag"; got != want {
		t.Fatalf("accepted input = %q, want %q", got, want)
	}
	result, err := grammar.ParseQuickEdit(q.Input.Value(), task, home, projects)
	if err != nil {
		t.Fatalf("parseQuickEdit after acceptance: %v", err)
	}
	if got := dependencyLocatorStrings(result.DependencyPatch().Add); !slices.Equal(got, []string{"home-project:home-4"}) {
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
	if !slices.Contains(suggestionValues(q.AC.Items), "4") {
		t.Fatalf("suggestions = %v, want the open #4", suggestionValues(q.AC.Items))
	}

	home.Tasks[3].WorkflowState = models.StateDone
	task.Dependencies = []models.TaskReference{{ProjectID: "routed-project", TaskID: "routed-1"}}
	q.RefreshDraft()

	values := suggestionValues(q.AC.Items)
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
	composer.Input.SetValue("First ~2\nSecond ~routed:1 ~4")

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

	composer.Input.SetValue("First ~2\nSecond ~999")
	requests, ok = composer.parseCompactLines()
	if ok || requests != nil {
		t.Fatalf("parseCompactLines = %+v, %v; want the whole batch refused", requests, ok)
	}
	if len(composer.errors) != 1 || composer.errors[0].Line != 2 {
		t.Fatalf("errors = %+v, want one on line 2", composer.errors)
	}
	if !strings.Contains(composer.errors[0].Message, "dependency not found") {
		t.Fatalf("error = %q, want a dependency-reference diagnostic", composer.errors[0].Message)
	}
}

// TestDependencyGrammarReachesBothHelpSurfaces checks the two registry-derived
// teaching surfaces: the composer's syntax guide, which is capture-only, and the
// `?` grammar section, which unions every mutation surface and so must also
// carry the edit-only removal and clear forms.
func TestDependencyGrammarReachesBothHelpSurfaces(t *testing.T) {
	home, routed := dependencyWorkspace()
	q := draftedCaptureInput(t, "", home, []*models.Project{home, routed})

	var guideForms []string
	for _, group := range q.SyntaxGuideGroups() {
		for _, entry := range group.Entries {
			guideForms = append(guideForms, entry.Forms)
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
	for _, item := range help.GrammarHelpItems() {
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
