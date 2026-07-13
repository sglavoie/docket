package tui

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCollectTaskTagNames(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []*models.Task
		expected []string
	}{
		{
			name:     "empty task list",
			tasks:    nil,
			expected: []string{},
		},
		{
			name: "multiple tasks with overlapping tags",
			tasks: []*models.Task{
				{Tags: []string{"backend", "api"}},
				{Tags: []string{"api", "frontend"}},
				{Tags: []string{"backend"}},
			},
			expected: []string{"api", "backend", "frontend"},
		},
		{
			name: "tasks with no tags",
			tasks: []*models.Task{
				{Tags: nil},
				{Tags: []string{}},
			},
			expected: []string{},
		},
		{
			name: "single task single tag",
			tasks: []*models.Task{
				{Tags: []string{"urgent"}},
			},
			expected: []string{"urgent"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CollectTaskTagNames(tc.tasks)
			if len(result) != len(tc.expected) {
				t.Fatalf("got %d tags, want %d: %v", len(result), len(tc.expected), result)
			}
			for i, tag := range result {
				if tag != tc.expected[i] {
					t.Errorf("tag[%d] = %q, want %q", i, tag, tc.expected[i])
				}
			}
		})
	}
}

func TestBuildProjectTagListIncludesCatalogTagsWithTaskCounts(t *testing.T) {
	project := &models.Project{
		Tags: []string{"unused", "frontend"},
		Tasks: []*models.Task{
			{Tags: []string{"backend", "frontend"}},
			{Tags: []string{"backend", "api", "api"}},
			{WorkflowState: models.StateDone, Tags: []string{"done-only"}},
		},
	}

	got := summarizeTagInfos(BuildProjectTagList(project, false))
	want := []string{"api:1", "backend:2", "done-only:1", "frontend:1", "unused:0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildProjectTagList() = %v, want %v", got, want)
	}

	got = summarizeTagInfos(BuildProjectTagList(project, true))
	want = []string{"api:1", "backend:2", "frontend:1", "unused:0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildProjectTagList(excludeCompleted) = %v, want %v", got, want)
	}
}

func TestBuildProjectsTagListMergesCatalogTagsAndTaskCounts(t *testing.T) {
	alpha := &models.Project{
		ID:   "alpha",
		Tags: []string{"catalog-only", "shared"},
		Tasks: []*models.Task{
			{Tags: []string{"shared", "alpha-task"}},
		},
	}
	beta := &models.Project{
		ID:   "beta",
		Tags: []string{"beta-catalog"},
		Tasks: []*models.Task{
			{Tags: []string{"shared"}},
		},
	}

	got := summarizeTagInfos(BuildProjectsTagList([]*models.Project{alpha, beta, alpha}, false))
	want := []string{"alpha-task:1", "beta-catalog:0", "catalog-only:0", "shared:2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildProjectsTagList() = %v, want %v", got, want)
	}
}

func TestCollectTagNamesForProjectIncludesCatalogTags(t *testing.T) {
	project := &models.Project{
		Tags: []string{"catalog-only"},
		Tasks: []*models.Task{
			{Tags: []string{"task-used"}},
		},
	}

	got := CollectTagNamesForProject(project)
	want := []string{"catalog-only", "task-used"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectTagNamesForProject() = %v, want %v", got, want)
	}
}

func TestValidateSavedStringSet(t *testing.T) {
	tests := []struct {
		name     string
		saved    []string
		valid    []string
		expected map[string]bool
	}{
		{
			name:     "all valid",
			saved:    []string{"a", "b"},
			valid:    []string{"a", "b", "c"},
			expected: map[string]bool{"a": true, "b": true},
		},
		{
			name:     "some invalid",
			saved:    []string{"a", "b", "x"},
			valid:    []string{"a", "c"},
			expected: map[string]bool{"a": true},
		},
		{
			name:     "all invalid",
			saved:    []string{"x", "y"},
			valid:    []string{"a", "b"},
			expected: map[string]bool{},
		},
		{
			name:     "empty saved",
			saved:    nil,
			valid:    []string{"a", "b"},
			expected: map[string]bool{},
		},
		{
			name:     "empty valid",
			saved:    []string{"a", "b"},
			valid:    nil,
			expected: map[string]bool{},
		},
		{
			name:     "both empty",
			saved:    nil,
			valid:    nil,
			expected: map[string]bool{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := validateSavedStringSet(tc.saved, tc.valid)
			if len(result) != len(tc.expected) {
				t.Fatalf("got %d entries, want %d: %v", len(result), len(tc.expected), result)
			}
			for k := range tc.expected {
				if !result[k] {
					t.Errorf("missing expected key %q", k)
				}
			}
		})
	}
}

func summarizeTagInfos(tags []TagInfo) []string {
	result := make([]string, len(tags))
	for i, tag := range tags {
		result[i] = tag.Name + ":" + strconv.Itoa(tag.TaskCount)
	}
	return result
}
