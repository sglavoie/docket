package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestTypeAttrs_RoundTrip(t *testing.T) { //nolint:gocyclo
	tests := []struct {
		name string
		task *models.Task
	}{
		{
			name: "bug with all attrs",
			task: &models.Task{
				ID:       "bug-round",
				Type:     models.TaskTypeBug,
				Status:   models.StatusTodo,
				Priority: 2,
				BugAttrs: &models.BugAttrs{
					Reproducible: models.BugReproducibleSometimes,
					Severity:     models.BugSeverityMinor,
					Workaround:   "Workaround with\nmultiple lines",
				},
			},
		},
		{
			name: "idea with all attrs",
			task: &models.Task{
				ID:       "idea-round",
				Type:     models.TaskTypeIdea,
				Status:   models.StatusTodo,
				Priority: 2,
				IdeaAttrs: &models.IdeaAttrs{
					Category:  models.IdeaCategoryResearch,
					Effort:    models.EffortLarge,
					Value:     models.IdeaValueMedium,
					Validated: true,
				},
			},
		},
		{
			name: "task with all attrs",
			task: &models.Task{
				ID:       "task-round",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				TaskAttrs: &models.TaskAttrs{
					Effort:   models.EffortMedium,
					WorkType: models.WorkTypeTesting,
				},
			},
		},
		{
			name: "story with all attrs",
			task: &models.Task{
				ID:       "story-round",
				Type:     models.TaskTypeStory,
				Status:   models.StatusTodo,
				Priority: 2,
				StoryAttrs: &models.StoryAttrs{
					StoryPoints: 8,
					Persona:     "As a user\nI want to\nSo that",
				},
			},
		},
		{
			name: "epic with all attrs",
			task: &models.Task{
				ID:       "epic-round",
				Type:     models.TaskTypeEpic,
				Status:   models.StatusTodo,
				Priority: 2,
				EpicAttrs: &models.EpicAttrs{
					Scope:          models.EpicScopeEvolving,
					Milestone:      "Q2 2025",
					SuccessMetrics: "Metric 1\nMetric 2",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metaStr := WriteMetadata(tc.task)

			if len(metaStr) < 8 || metaStr[:4] != "<!--" || metaStr[len(metaStr)-3:] != "-->" {
				t.Fatalf("Invalid metadata format: %q", metaStr)
			}
			content := metaStr[5 : len(metaStr)-4]

			parsed := &models.Task{Type: tc.task.Type}
			ParseMetadata(parsed, content)

			switch tc.task.Type {
			case models.TaskTypeBug:
				if tc.task.BugAttrs != nil {
					if parsed.BugAttrs == nil {
						t.Fatal("BugAttrs is nil after roundtrip")
					}
					if parsed.BugAttrs.Reproducible != tc.task.BugAttrs.Reproducible {
						t.Errorf("Reproducible = %q, want %q", parsed.BugAttrs.Reproducible, tc.task.BugAttrs.Reproducible)
					}
					if parsed.BugAttrs.Severity != tc.task.BugAttrs.Severity {
						t.Errorf("Severity = %q, want %q", parsed.BugAttrs.Severity, tc.task.BugAttrs.Severity)
					}
					if parsed.BugAttrs.Workaround != tc.task.BugAttrs.Workaround {
						t.Errorf("Workaround = %q, want %q", parsed.BugAttrs.Workaround, tc.task.BugAttrs.Workaround)
					}
				}
			case models.TaskTypeIdea:
				if tc.task.IdeaAttrs != nil {
					if parsed.IdeaAttrs == nil {
						t.Fatal("IdeaAttrs is nil after roundtrip")
					}
					if parsed.IdeaAttrs.Category != tc.task.IdeaAttrs.Category {
						t.Errorf("Category = %q, want %q", parsed.IdeaAttrs.Category, tc.task.IdeaAttrs.Category)
					}
					if parsed.IdeaAttrs.Effort != tc.task.IdeaAttrs.Effort {
						t.Errorf("Effort = %q, want %q", parsed.IdeaAttrs.Effort, tc.task.IdeaAttrs.Effort)
					}
					if parsed.IdeaAttrs.Value != tc.task.IdeaAttrs.Value {
						t.Errorf("Value = %q, want %q", parsed.IdeaAttrs.Value, tc.task.IdeaAttrs.Value)
					}
					if parsed.IdeaAttrs.Validated != tc.task.IdeaAttrs.Validated {
						t.Errorf("Validated = %v, want %v", parsed.IdeaAttrs.Validated, tc.task.IdeaAttrs.Validated)
					}
				}
			case models.TaskTypeTask:
				if tc.task.TaskAttrs != nil {
					if parsed.TaskAttrs == nil {
						t.Fatal("TaskAttrs is nil after roundtrip")
					}
					if parsed.TaskAttrs.Effort != tc.task.TaskAttrs.Effort {
						t.Errorf("Effort = %q, want %q", parsed.TaskAttrs.Effort, tc.task.TaskAttrs.Effort)
					}
					if parsed.TaskAttrs.WorkType != tc.task.TaskAttrs.WorkType {
						t.Errorf("WorkType = %q, want %q", parsed.TaskAttrs.WorkType, tc.task.TaskAttrs.WorkType)
					}
				}
			case models.TaskTypeStory:
				if tc.task.StoryAttrs != nil {
					if parsed.StoryAttrs == nil {
						t.Fatal("StoryAttrs is nil after roundtrip")
					}
					if parsed.StoryAttrs.StoryPoints != tc.task.StoryAttrs.StoryPoints {
						t.Errorf("StoryPoints = %d, want %d", parsed.StoryAttrs.StoryPoints, tc.task.StoryAttrs.StoryPoints)
					}
					if parsed.StoryAttrs.Persona != tc.task.StoryAttrs.Persona {
						t.Errorf("Persona = %q, want %q", parsed.StoryAttrs.Persona, tc.task.StoryAttrs.Persona)
					}
				}
			case models.TaskTypeEpic:
				if tc.task.EpicAttrs != nil {
					if parsed.EpicAttrs == nil {
						t.Fatal("EpicAttrs is nil after roundtrip")
					}
					if parsed.EpicAttrs.Scope != tc.task.EpicAttrs.Scope {
						t.Errorf("Scope = %q, want %q", parsed.EpicAttrs.Scope, tc.task.EpicAttrs.Scope)
					}
					if parsed.EpicAttrs.Milestone != tc.task.EpicAttrs.Milestone {
						t.Errorf("Milestone = %q, want %q", parsed.EpicAttrs.Milestone, tc.task.EpicAttrs.Milestone)
					}
					if parsed.EpicAttrs.SuccessMetrics != tc.task.EpicAttrs.SuccessMetrics {
						t.Errorf("SuccessMetrics = %q, want %q", parsed.EpicAttrs.SuccessMetrics, tc.task.EpicAttrs.SuccessMetrics)
					}
				}
			}
		})
	}
}

// TestEpicMilestone_NewlinesEscaped verifies that a newline in
// EpicAttrs.Milestone is escaped on write and restored on read, instead of
// truncating the metadata comment mid-line. Without the fix, a newline
// terminates the HTML comment payload, corrupting every metadata key that
// follows milestone (or even silently losing the milestone itself when the
// re-parser cannot match the now-multi-line comment).
func TestEpicMilestone_NewlinesEscaped(t *testing.T) {
	original := &models.Task{
		ID:       "epic-newline",
		Type:     models.TaskTypeEpic,
		Status:   models.StatusTodo,
		Priority: 2,
		EpicAttrs: &models.EpicAttrs{
			Scope:     models.EpicScopeDefined,
			Milestone: "Line one\nLine two\nLine three",
		},
	}

	metaStr := WriteMetadata(original)

	// Sanity: the entire metadata string must remain a single line so the
	// surrounding parser (which scans line-by-line) does not break apart.
	for i := 0; i < len(metaStr); i++ {
		if metaStr[i] == '\n' {
			t.Fatalf("metadata contains raw newline at byte %d: %q", i, metaStr)
		}
	}

	if len(metaStr) < 8 {
		t.Fatalf("invalid metadata: %q", metaStr)
	}
	content := metaStr[5 : len(metaStr)-4]

	parsed := &models.Task{Type: models.TaskTypeEpic}
	ParseMetadata(parsed, content)

	if parsed.EpicAttrs == nil {
		t.Fatal("EpicAttrs is nil after roundtrip")
	}
	if parsed.EpicAttrs.Milestone != original.EpicAttrs.Milestone {
		t.Errorf("Milestone = %q, want %q", parsed.EpicAttrs.Milestone, original.EpicAttrs.Milestone)
	}
}
