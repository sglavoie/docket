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
