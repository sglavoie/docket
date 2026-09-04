package outliner

import (
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// TestSearchAutocompleteCardinalityFollowsTheGrammarDefinition walks every search
// field and checks the suppression rule against the definition rather than a
// hand-written expectation, so a cardinality change in the registry moves this
// behavior without touching the module.
func TestSearchAutocompleteCardinalityFollowsTheGrammarDefinition(t *testing.T) {
	task := searchACTask("t1", []string{"infra", "docs"}, "office")
	second := searchACTask("t2", []string{"api"}, "home")
	// The dependency vocabulary is built from stored references, so the fixture has
	// to carry some. Short IDs are set because the offered alias is one, and the
	// reciprocal pair keeps two distinct targets in the vocabulary.
	task.ShortID, second.ShortID = 1, 2
	task.Dependencies = []models.TaskReference{{ProjectID: "alpha", TaskID: second.ID}}
	second.Dependencies = []models.TaskReference{{ProjectID: "alpha", TaskID: task.ID}}
	outliner := searchACOutliner(primaryRowsProject("alpha", task, second))

	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceSearch) {
		// Take the first value the pane actually offers, so the settled prefix is a
		// query the grammar parses back into this field.
		typeSearchAC(outliner, definition.Trigger)
		if !outliner.Search.Autocomplete.Active() {
			t.Fatalf("%s offered nothing to settle with", definition.Field)
		}
		accepted := outliner.Search.Autocomplete.Ac.Items[0].Value

		typeSearchAC(outliner, definition.Trigger+accepted+" "+definition.Trigger)
		switch definition.Cardinality {
		case textgrammar.CardinalitySingleton:
			if outliner.Search.Autocomplete.Active() {
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
	if outliner.Search.Autocomplete.Active() {
		t.Fatalf("tag pane opened over an empty vocabulary: %v", searchACValues(outliner))
	}
	if outliner.Search.Autocomplete.Ac.Mode != grammar.ACModeNone || outliner.Search.Autocomplete.triggerText != "" {
		t.Error("the retained mode and trigger outlived the list they described")
	}
	typeSearchAC(outliner, "##")
	if outliner.Search.Autocomplete.Active() {
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

	outliner.pushFocusFrame(taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}, 0, 0)
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
	for _, project := range outliner.ProjectsInCatalogOrder(outliner.WorkModeScopeProjects()) {
		want = append(want, project.Slug)
	}
	if got := searchACValues(outliner); !slices.Equal(got, want) {
		t.Fatalf("project values = %v, want the loaded scope %v", got, want)
	}

	// Narrowing the loaded scope narrows the offer in the same pass.
	outliner.Population.Projects = []*models.Project{alpha}
	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	typeSearchAC(outliner, "%")
	if got := searchACValues(outliner); slices.Contains(got, beta.Slug) {
		t.Fatalf("project values = %v, want no project outside the loaded scope", got)
	}
}

// TestSearchAutocompleteProjectsRankTypedPartial proves the partial is what
// brings the intended project into the bounded dropdown. The scope holds more
// projects than the pane has rows, so catalog order alone leaves the target
// unreachable; typing either half of its identity must rank it first without the
// scope growing by a single project.
func TestSearchAutocompleteProjectsRankTypedPartial(t *testing.T) {
	slugs := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "zulu"}
	projects := make([]*models.Project, 0, len(slugs))
	for _, slug := range slugs {
		projects = append(projects, primaryRowsProject(slug, searchACTask("t-"+slug, nil, "")))
	}
	target := projects[len(projects)-1]
	target.Name = "Quarterly Roadmap"
	outliner := searchACOutliner(projects...)

	// Without a partial the pane still offers the whole loaded scope in catalog
	// order, and the target sits past the last row the dropdown can draw.
	typeSearchAC(outliner, "%")
	if got := searchACValues(outliner); !slices.Equal(got, slugs) {
		t.Fatalf("unfiltered project values = %v, want the loaded scope %v", got, slugs)
	}
	if got := slices.Index(searchACValues(outliner), target.Slug); got < outlinerSearchSuggestionLimit {
		t.Fatalf("target index without a partial = %d, want it outside the %d-row pane",
			got, outlinerSearchSuggestionLimit)
	}

	// The accepted value ranks it, and every nonmatch is gone rather than pushed
	// down past the pane.
	typeSearchAC(outliner, "%zul")
	got := searchACValues(outliner)
	if !slices.Equal(got, []string{target.Slug}) {
		t.Fatalf("slug-filtered project values = %v, want only %q", got, target.Slug)
	}

	// So does the human-readable name, which the accepted value never contains.
	typeSearchAC(outliner, "%roadmap")
	got = searchACValues(outliner)
	if !slices.Equal(got, []string{target.Slug}) {
		t.Fatalf("name-filtered project values = %v, want only %q", got, target.Slug)
	}
	if displays := searchACDisplays(outliner); len(displays) != 1 ||
		displays[0] != "%"+target.Slug+" "+target.Name {
		t.Fatalf("name-filtered project displays = %v, want the slug and name row", displays)
	}

	// Ranking never reaches for a project the query in front of it could not
	// match: dropping the target from the loaded scope drops it from the offer.
	outliner.Population.Projects = projects[:len(projects)-1]
	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	typeSearchAC(outliner, "%roadmap")
	if got := searchACValues(outliner); len(got) != 0 {
		t.Fatalf("project values outside the loaded scope = %v, want none", got)
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
	if !outliner.Search.Autocomplete.Active() {
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
	if outliner.Search.Autocomplete.Active() {
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
