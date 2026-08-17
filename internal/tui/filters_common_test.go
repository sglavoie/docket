package tui

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/sglavoie/tt/internal/models"
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

func TestTaskMatchesTagFacetMatchesEitherAxis(t *testing.T) {
	task := &models.Task{Tags: []string{"bug"}, Context: "platform"}
	for _, value := range []string{"bug", "platform"} {
		if !taskMatchesTagFacet(task, value) {
			t.Errorf("taskMatchesTagFacet(%q) = false, want true", value)
		}
	}
	if taskMatchesTagFacet(task, "other") {
		t.Error("taskMatchesTagFacet(other) = true, want false")
	}
}

func TestCollectTagNamesForProjectUsesTaskTags(t *testing.T) {
	project := &models.Project{
		Tasks: []*models.Task{
			{Tags: []string{"task-used"}},
		},
	}

	got := CollectTagNamesForProject(project)
	want := []string{"task-used"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectTagNamesForProject() = %v, want %v", got, want)
	}
}

func summarizeTagInfos(tags []TagInfo) []string {
	result := make([]string, len(tags))
	for i, tag := range tags {
		result[i] = tag.Name + ":" + strconv.Itoa(tag.TaskCount)
	}
	return result
}
