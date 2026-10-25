package query

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/grammar"
)

func TestBuildProjectTagListUsesTaskCounts(t *testing.T) {
	project := &models.Project{
		Tasks: []*models.Task{
			{Tags: []string{"backend", "frontend"}},
			{Tags: []string{"backend", "api", "api"}},
			{WorkflowState: models.StateDone, Tags: []string{"done-only"}},
		},
	}

	got := summarizeTagInfos(BuildProjectTagList(project, false))
	want := []string{"api:1", "backend:2", "done-only:1", "frontend:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildProjectTagList() = %v, want %v", got, want)
	}

	got = summarizeTagInfos(BuildProjectTagList(project, true))
	want = []string{"api:1", "backend:2", "frontend:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildProjectTagList(excludeCompleted) = %v, want %v", got, want)
	}
}

func TestBuildProjectsTagListMergesTaskCounts(t *testing.T) {
	alpha := &models.Project{
		ID: "alpha",
		Tasks: []*models.Task{
			{Tags: []string{"shared", "alpha-task"}},
		},
	}
	beta := &models.Project{
		ID: "beta",
		Tasks: []*models.Task{
			{Tags: []string{"shared"}},
		},
	}

	got := summarizeTagInfos(BuildProjectsTagList([]*models.Project{alpha, beta, alpha}, false))
	want := []string{"alpha-task:1", "shared:2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildProjectsTagList() = %v, want %v", got, want)
	}
}

func TestBuildTagFacetListUnionsTagsAndContexts(t *testing.T) {
	project := &models.Project{
		Tasks: []*models.Task{
			{Tags: []string{"shared", "tag-only"}, Context: "shared"},
			{Context: "context-only"},
			{WorkflowState: models.StateDone, Context: "done-context"},
		},
	}

	got := summarizeTagInfos(BuildTagFacetList([]*models.Project{project}, false))
	want := []string{"context-only:1", "done-context:1", "shared:2", "tag-only:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildTagFacetList() = %v, want %v", got, want)
	}

	got = summarizeTagInfos(BuildTagFacetList([]*models.Project{project}, true))
	want = []string{"context-only:1", "shared:2", "tag-only:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildTagFacetList(excludeCompleted) = %v, want %v", got, want)
	}

	if got := summarizeTagInfos(BuildProjectTagList(project, false)); !reflect.DeepEqual(got, []string{"shared:1", "tag-only:1"}) {
		t.Fatalf("BuildProjectTagList() leaked contexts into the tag editor vocabulary: %v", got)
	}

	contextCounts := make(map[string]int)
	for _, tag := range BuildTagFacetList([]*models.Project{project}, false) {
		contextCounts[tag.Name] = tag.ContextCount
	}
	wantContexts := map[string]int{"context-only": 1, "done-context": 1, "shared": 1, "tag-only": 0}
	if !reflect.DeepEqual(contextCounts, wantContexts) {
		t.Fatalf("BuildTagFacetList() context counts = %v, want %v", contextCounts, wantContexts)
	}

	for _, tag := range BuildProjectTagList(project, false) {
		if tag.ContextCount != 0 {
			t.Fatalf("BuildProjectTagList() stamped a context count on %q", tag.Name)
		}
	}
}

// TestTagAndContextFacetsAreDistinctAxes pins the de-conflation: the tag facet
// reads Tags alone and the context facet reads Context alone, so a value carried
// on one axis can never be filtered out by a facet naming the other. Contexts
// compose with OR, because a task carries at most one.
func TestTagAndContextFacetsAreDistinctAxes(t *testing.T) {
	now := time.Now()
	tagged := &models.Task{Tags: []string{"bug"}}
	contextual := &models.Task{Context: "platform"}
	bare := &models.Task{}

	tagFacet := NewCommonFilterState()
	tagFacet.Tags = map[string]bool{"platform": true}
	for _, testCase := range []struct {
		name string
		task *models.Task
		want bool
	}{
		{name: "tag", task: tagged, want: false},
		{name: "context", task: contextual, want: false},
	} {
		if got := passesCommonFiltersAt(testCase.task, &tagFacet, now); got != testCase.want {
			t.Errorf("tag facet #platform admits the %s task = %v, want %v", testCase.name, got, testCase.want)
		}
	}

	contextFacet := NewCommonFilterState()
	contextFacet.Contexts = map[string]bool{"bug": true, "platform": true}
	if passesCommonFiltersAt(tagged, &contextFacet, now) {
		t.Error("context facet ##bug admits a task carrying bug as a tag")
	}
	if !passesCommonFiltersAt(contextual, &contextFacet, now) {
		t.Error("context facet does not admit ##platform under OR semantics")
	}

	noContext := NewCommonFilterState()
	noContext.NoContext = true
	if !passesCommonFiltersAt(bare, &noContext, now) {
		t.Error("no-context facet rejects a task with no context")
	}
	if passesCommonFiltersAt(contextual, &noContext, now) {
		t.Error("no-context facet admits a task carrying a context")
	}
}

// TestContextFacetCountsAsOneClearableFacet keeps the names and the no-context
// row on the one picker they share, the way the project picker's descriptive
// tags and its Untagged row share theirs.
func TestContextFacetCountsAsOneClearableFacet(t *testing.T) {
	filter := NewCommonFilterState()
	filter.Contexts = map[string]bool{"errand": true, "office": true}
	filter.NoContext = true
	if !HasActiveCommonFilters(&filter) {
		t.Fatal("HasActiveCommonFilters() = false with an active context facet")
	}
	if got := CommonFilterFacetCount(&filter); got != 1 {
		t.Fatalf("CommonFilterFacetCount() = %d, want 1", got)
	}
	want := []string{"##errand", "##office", "##0"}
	if got := CommonFilterParts(&filter); !reflect.DeepEqual(got, want) {
		t.Fatalf("CommonFilterParts() = %v, want %v", got, want)
	}
	clearCommonFilters(&filter)
	if HasActiveCommonFilters(&filter) {
		t.Fatal("clearCommonFilters() left the context facet active")
	}
}

// TestContextFilterClausesSplitTheNoContextToken keeps the pseudo-entry in the
// picker's value channel and the none operator, so the facet still needs no
// second value channel now that it writes clauses.
func TestContextFilterClausesSplitTheNoContextToken(t *testing.T) {
	clauses := ContextFilterClauses([]string{"errand", NoContextFilterKey})
	filter := CommonFilterStateFromQuery(models.Query{Clauses: clauses})
	if !filter.NoContext {
		t.Error("the no-context pseudo-entry did not set NoContext")
	}
	if filter.Contexts[NoContextFilterKey] {
		t.Error("the no-context pseudo-entry survived as a context name")
	}
	if !filter.Contexts["errand"] {
		t.Error("the errand context was dropped")
	}
	if got := len(ContextFilterClauses(nil)); got != 0 {
		t.Errorf("an empty picker result minted %d clause(s), want 0", got)
	}
}

func TestCollectContextNamesForProjectsReadsTheFacetCensus(t *testing.T) {
	alpha := &models.Project{
		ID: "alpha",
		Tasks: []*models.Task{
			{Context: "office", Tags: []string{"errand"}},
			{Context: "errand"},
		},
	}
	beta := &models.Project{
		ID:    "beta",
		Tasks: []*models.Task{{Context: "office"}, {Tags: []string{"desk"}}},
	}
	got := CollectContextNamesForProjects([]*models.Project{alpha, beta, alpha})
	want := []string{"errand", "office"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectContextNamesForProjects() = %v, want %v", got, want)
	}
	if got := CountTasksWithoutContext([]*models.Project{alpha, beta, alpha}); got != 1 {
		t.Fatalf("CountTasksWithoutContext() = %d, want 1", got)
	}
}

func TestCollectTagNamesForProjectUsesTaskTags(t *testing.T) {
	project := &models.Project{
		Tasks: []*models.Task{
			{Tags: []string{"task-used"}},
		},
	}

	got := grammar.CollectTagNamesForProject(project)
	want := []string{"task-used"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("grammar.CollectTagNamesForProject() = %v, want %v", got, want)
	}
}

func summarizeTagInfos(tags []TagInfo) []string {
	result := make([]string, len(tags))
	for i, tag := range tags {
		result[i] = tag.Name + ":" + strconv.Itoa(tag.TaskCount)
	}
	return result
}
