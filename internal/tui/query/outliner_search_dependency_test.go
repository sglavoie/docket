package query

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// searchDependencyProject builds a project whose short IDs are addressable, so a
// bare ~alias in a search has something local to resolve against.
func searchDependencyProject(id, slug string, tasks ...*models.Task) *models.Project {
	project := &models.Project{ID: id, Name: strings.ToUpper(slug), Slug: slug}
	project.Tasks = append(project.Tasks, tasks...)
	return project
}

func searchDependencyTask(id string, shortID int, title string) *models.Task {
	return &models.Task{
		ID:            id,
		Title:         title,
		ShortID:       shortID,
		WorkflowState: models.StateBacklog,
		UpdatedAt:     time.Now(),
	}
}

// compileDependencySearch compiles through the shared gate with no plan and no
// facets. The reference sigil reads neither, so this is the whole of what the
// Outliner hands the gate for a ~ query.
func compileDependencySearch(text string, projects []*models.Project) *SearchPredicates {
	_, predicates := CompileSearchQuery(text, projects, nil, models.Query{})
	return predicates
}

// TestSearchDependencyMatchesOnlyDirectDependents fixes the one-hop rule: the
// predicate reads the task's own stored set and never walks the graph, so a
// transitive dependent is not a match.
func TestSearchDependencyMatchesOnlyDirectDependents(t *testing.T) {
	target := searchDependencyTask("target", 1, "Rotate keys")
	direct := searchDependencyTask("direct", 2, "Ship auth")
	transitive := searchDependencyTask("transitive", 3, "Announce auth")
	unrelated := searchDependencyTask("unrelated", 4, "Sweep flags")
	direct.Dependencies = []models.TaskReference{{ProjectID: "alpha", TaskID: target.ID}}
	transitive.Dependencies = []models.TaskReference{{ProjectID: "alpha", TaskID: direct.ID}}
	project := searchDependencyProject("alpha", "alpha", target, direct, transitive, unrelated)

	predicates := compileDependencySearch("~1", []*models.Project{project})
	if predicates.guidance != "" {
		t.Fatalf("guidance = %q, want a compiled predicate", predicates.guidance)
	}
	for _, tt := range []struct {
		task *models.Task
		want bool
	}{
		{direct, true},
		{transitive, false},
		{unrelated, false},
		{target, false},
	} {
		if got := predicates.admitsDependencies(tt.task); got != tt.want {
			t.Errorf("admits(%s) = %v, want %v", tt.task.ID, got, tt.want)
		}
	}
}

// TestSearchDependencyPredicatesAND matches every other repeatable sigil: two
// targets narrow rather than widen.
func TestSearchDependencyPredicatesAND(t *testing.T) {
	first := searchDependencyTask("first", 1, "First")
	second := searchDependencyTask("second", 2, "Second")
	both := searchDependencyTask("both", 3, "Both")
	one := searchDependencyTask("one", 4, "One")
	both.Dependencies = []models.TaskReference{
		{ProjectID: "alpha", TaskID: first.ID},
		{ProjectID: "alpha", TaskID: second.ID},
	}
	one.Dependencies = []models.TaskReference{{ProjectID: "alpha", TaskID: first.ID}}
	project := searchDependencyProject("alpha", "alpha", first, second, both, one)

	predicates := compileDependencySearch("~1 ~2", []*models.Project{project})
	if predicates.guidance != "" {
		t.Fatalf("guidance = %q, want two compiled predicates", predicates.guidance)
	}
	if len(predicates.dependencies) != 2 {
		t.Fatalf("dependencies = %v, want two", predicates.dependencies)
	}
	if !predicates.admitsDependencies(both) {
		t.Error("a task carrying both targets was rejected")
	}
	if predicates.admitsDependencies(one) {
		t.Error("a task carrying one of two targets was admitted")
	}
}

// TestSearchDependencyResolvesQualifiedAndScopedAliases fixes the two addressing
// rules a search alias follows.
func TestSearchDependencyResolvesQualifiedAndScopedAliases(t *testing.T) {
	local := searchDependencyTask("local", 1, "Local")
	remote := searchDependencyTask("remote", 1, "Remote")
	alpha := searchDependencyProject("alpha", "alpha", local)
	beta := searchDependencyProject("beta", "beta", remote)

	// A bare alias inside a single-project scope is read locally.
	scoped := compileDependencySearch("~1", []*models.Project{alpha})
	if scoped.guidance != "" || len(scoped.dependencies) != 1 {
		t.Fatalf("scoped bare alias = %+v / %q", scoped.dependencies, scoped.guidance)
	}
	if got, want := scoped.dependencies[0].Locator(), "alpha:local"; got != want {
		t.Errorf("scoped bare alias resolved to %q, want %q", got, want)
	}

	// A qualified alias addresses the loaded population directly, scope or not.
	qualified := compileDependencySearch("~beta:1", []*models.Project{alpha, beta})
	if qualified.guidance != "" || len(qualified.dependencies) != 1 {
		t.Fatalf("qualified alias = %+v / %q", qualified.dependencies, qualified.guidance)
	}
	if got, want := qualified.dependencies[0].Locator(), "beta:remote"; got != want {
		t.Errorf("qualified alias resolved to %q, want %q", got, want)
	}
}

func TestSearchDependencyGuidance(t *testing.T) {
	local := searchDependencyTask("local", 1, "Local")
	remote := searchDependencyTask("remote", 1, "Remote")
	alpha := searchDependencyProject("alpha", "alpha", local)
	beta := searchDependencyProject("beta", "beta", remote)
	global := []*models.Project{alpha, beta}

	cases := []struct {
		name    string
		query   string
		scope   []*models.Project
		want    string
		absent  string
		wantAll []string
	}{
		{
			// Picking a project on the user's behalf would answer a question they
			// did not ask, so an ambiguous bare alias reports rather than guesses.
			name:    "ambiguous bare alias",
			query:   "~1",
			scope:   global,
			wantAll: []string{"matches", "ALPHA #1 Local", "BETA #1 Remote", "~project-slug:short-id"},
		},
		{
			name:  "unknown target",
			query: "~99",
			scope: global,
			want:  "dependency not found",
		},
		{
			// Search holds no stored set, so ~0 is named rather than compiled into a
			// locator that could never match.
			name:  "clear has nothing to clear",
			query: "~0",
			scope: global,
			want:  "search has no set to clear",
		},
		{
			// The negation reads the same locator its positive twin does, so an
			// ambiguous alias is reported the same way rather than resolved by a
			// second, laxer path.
			name:    "negative form resolves like its twin",
			query:   "-~1",
			scope:   global,
			wantAll: []string{"matches", "ALPHA #1 Local", "BETA #1 Remote", "~project-slug:short-id"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			predicates := compileDependencySearch(tt.query, tt.scope)
			if predicates.guidance == "" {
				t.Fatalf("query %q compiled silently: %+v", tt.query, predicates.dependencies)
			}
			if len(predicates.dependencies) != 0 || len(predicates.excludedDeps) != 0 {
				t.Errorf("guided query still compiled %v %v", predicates.dependencies, predicates.excludedDeps)
			}
			wants := tt.wantAll
			if tt.want != "" {
				wants = append(wants, tt.want)
			}
			for _, want := range wants {
				if !strings.Contains(predicates.guidance, want) {
					t.Errorf("guidance = %q, want it to contain %q", predicates.guidance, want)
				}
			}
		})
	}
}
