package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestWriteMetadata_TypeAttrs(t *testing.T) {
	tests := []struct {
		name     string
		task     *models.Task
		contains []string
	}{
		{
			name: "bug attributes",
			task: &models.Task{
				ID:            "b1",
				Type:          models.TaskTypeBug,
				WorkflowState: models.StateReady,
				Priority:      2,
				BugAttrs: &models.BugAttrs{
					Reproducible: models.BugReproducibleYes,
					Severity:     models.BugSeverityMajor,
					Workaround:   "Use Chrome",
				},
			},
			contains: []string{
				"bug.reproducible: yes",
				"bug.severity: major",
				"bug.workaround: Use Chrome",
			},
		},
		{
			name: "idea attributes",
			task: &models.Task{
				ID:            "i1",
				Type:          models.TaskTypeIdea,
				WorkflowState: models.StateReady,
				Priority:      2,
				IdeaAttrs: &models.IdeaAttrs{
					Category:  models.IdeaCategoryFeature,
					Effort:    models.EffortMedium,
					Value:     models.IdeaValueHigh,
					Validated: true,
				},
			},
			contains: []string{
				"idea.category: feature",
				"idea.effort: medium",
				"idea.value: high",
				"idea.validated: true",
			},
		},
		{
			name: "task attributes",
			task: &models.Task{
				ID:            "t1",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      2,
				TaskAttrs: &models.TaskAttrs{
					Effort:   models.EffortSmall,
					WorkType: models.WorkTypeDevelopment,
				},
			},
			contains: []string{
				"task.effort: small",
				"task.worktype: development",
			},
		},
		{
			name: "story attributes",
			task: &models.Task{
				ID:            "s1",
				Type:          models.TaskTypeStory,
				WorkflowState: models.StateReady,
				Priority:      2,
				StoryAttrs: &models.StoryAttrs{
					StoryPoints: 5,
					Persona:     "Admin user",
				},
			},
			contains: []string{
				"story.points: 5",
				"story.persona: Admin user",
			},
		},
		{
			name: "epic attributes",
			task: &models.Task{
				ID:            "e1",
				Type:          models.TaskTypeEpic,
				WorkflowState: models.StateReady,
				Priority:      2,
				EpicAttrs: &models.EpicAttrs{
					Scope:          models.EpicScopeDefined,
					Milestone:      "Q1 2025",
					SuccessMetrics: "100 active users",
				},
			},
			contains: []string{
				"epic.scope: defined",
				"epic.milestone: Q1 2025",
				"epic.metrics: 100 active users",
			},
		},
		{
			name: "bug workaround with newlines escaped",
			task: &models.Task{
				ID:            "b2",
				Type:          models.TaskTypeBug,
				WorkflowState: models.StateReady,
				Priority:      2,
				BugAttrs: &models.BugAttrs{
					Workaround: "Line 1\nLine 2\nLine 3",
				},
			},
			contains: []string{
				`bug.workaround: Line 1\nLine 2\nLine 3`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WriteMetadataV2(tc.task)
			for _, want := range tc.contains {
				if !containsString(got, want) {
					t.Errorf("WriteMetadataV2() = %q, should contain %q", got, want)
				}
			}
		})
	}
}
