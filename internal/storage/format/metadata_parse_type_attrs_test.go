package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseMetadata_BugAttrs(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    *models.BugAttrs
	}{
		{
			name:    "all bug attributes",
			metaStr: "id: b1, status: todo, priority: 2, bug.reproducible: yes, bug.severity: major, bug.workaround: Use Chrome",
			want: &models.BugAttrs{
				Reproducible: models.BugReproducibleYes,
				Severity:     models.BugSeverityMajor,
				Workaround:   "Use Chrome",
			},
		},
		{
			name:    "partial bug attributes",
			metaStr: "id: b2, status: todo, priority: 2, bug.severity: critical",
			want: &models.BugAttrs{
				Severity: models.BugSeverityCritical,
			},
		},
		{
			name:    "bug workaround with escaped newlines",
			metaStr: `id: b3, status: todo, priority: 2, bug.workaround: Line 1\nLine 2\nLine 3`,
			want: &models.BugAttrs{
				Workaround: "Line 1\nLine 2\nLine 3",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{Type: models.TaskTypeBug}
			ParseMetadata(task, tc.metaStr)

			if task.BugAttrs == nil {
				t.Fatal("BugAttrs is nil")
			}
			if task.BugAttrs.Reproducible != tc.want.Reproducible {
				t.Errorf("Reproducible = %q, want %q", task.BugAttrs.Reproducible, tc.want.Reproducible)
			}
			if task.BugAttrs.Severity != tc.want.Severity {
				t.Errorf("Severity = %q, want %q", task.BugAttrs.Severity, tc.want.Severity)
			}
			if task.BugAttrs.Workaround != tc.want.Workaround {
				t.Errorf("Workaround = %q, want %q", task.BugAttrs.Workaround, tc.want.Workaround)
			}
		})
	}
}

func TestParseMetadata_IdeaAttrs(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    *models.IdeaAttrs
	}{
		{
			name:    "all idea attributes",
			metaStr: "id: i1, status: todo, priority: 2, idea.category: feature, idea.effort: medium, idea.value: high, idea.validated: true",
			want: &models.IdeaAttrs{
				Category:  models.IdeaCategoryFeature,
				Effort:    models.EffortMedium,
				Value:     models.IdeaValueHigh,
				Validated: true,
			},
		},
		{
			name:    "partial idea attributes",
			metaStr: "id: i2, status: todo, priority: 2, idea.category: refactor",
			want: &models.IdeaAttrs{
				Category: models.IdeaCategoryRefactor,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{Type: models.TaskTypeIdea}
			ParseMetadata(task, tc.metaStr)

			if task.IdeaAttrs == nil {
				t.Fatal("IdeaAttrs is nil")
			}
			if task.IdeaAttrs.Category != tc.want.Category {
				t.Errorf("Category = %q, want %q", task.IdeaAttrs.Category, tc.want.Category)
			}
			if task.IdeaAttrs.Effort != tc.want.Effort {
				t.Errorf("Effort = %q, want %q", task.IdeaAttrs.Effort, tc.want.Effort)
			}
			if task.IdeaAttrs.Value != tc.want.Value {
				t.Errorf("Value = %q, want %q", task.IdeaAttrs.Value, tc.want.Value)
			}
			if task.IdeaAttrs.Validated != tc.want.Validated {
				t.Errorf("Validated = %v, want %v", task.IdeaAttrs.Validated, tc.want.Validated)
			}
		})
	}
}

func TestParseMetadata_TaskAttrs(t *testing.T) {
	task := &models.Task{Type: models.TaskTypeTask}
	ParseMetadata(task, "id: t1, status: todo, priority: 2, task.effort: small, task.worktype: development")

	if task.TaskAttrs == nil {
		t.Fatal("TaskAttrs is nil")
	}
	if task.TaskAttrs.Effort != models.EffortSmall {
		t.Errorf("Effort = %q, want %q", task.TaskAttrs.Effort, models.EffortSmall)
	}
	if task.TaskAttrs.WorkType != models.WorkTypeDevelopment {
		t.Errorf("WorkType = %q, want %q", task.TaskAttrs.WorkType, models.WorkTypeDevelopment)
	}
}

func TestParseMetadata_StoryAttrs(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    *models.StoryAttrs
	}{
		{
			name:    "all story attributes",
			metaStr: "id: s1, status: todo, priority: 2, story.points: 5, story.persona: Admin user",
			want: &models.StoryAttrs{
				StoryPoints: 5,
				Persona:     "Admin user",
			},
		},
		{
			name:    "story persona with escaped newlines",
			metaStr: `id: s2, status: todo, priority: 2, story.persona: As an admin\nI want to manage users\nSo that I can control access`,
			want: &models.StoryAttrs{
				Persona: "As an admin\nI want to manage users\nSo that I can control access",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{Type: models.TaskTypeStory}
			ParseMetadata(task, tc.metaStr)

			if task.StoryAttrs == nil {
				t.Fatal("StoryAttrs is nil")
			}
			if task.StoryAttrs.StoryPoints != tc.want.StoryPoints {
				t.Errorf("StoryPoints = %d, want %d", task.StoryAttrs.StoryPoints, tc.want.StoryPoints)
			}
			if task.StoryAttrs.Persona != tc.want.Persona {
				t.Errorf("Persona = %q, want %q", task.StoryAttrs.Persona, tc.want.Persona)
			}
		})
	}
}

func TestParseMetadata_EpicAttrs(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    *models.EpicAttrs
	}{
		{
			name:    "all epic attributes",
			metaStr: "id: e1, status: todo, priority: 2, epic.scope: defined, epic.milestone: Q1 2025, epic.metrics: 100 active users",
			want: &models.EpicAttrs{
				Scope:          models.EpicScopeDefined,
				Milestone:      "Q1 2025",
				SuccessMetrics: "100 active users",
			},
		},
		{
			name:    "epic metrics with escaped newlines",
			metaStr: `id: e2, status: todo, priority: 2, epic.metrics: 100 users\n50% retention\n5 star rating`,
			want: &models.EpicAttrs{
				SuccessMetrics: "100 users\n50% retention\n5 star rating",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{Type: models.TaskTypeEpic}
			ParseMetadata(task, tc.metaStr)

			if task.EpicAttrs == nil {
				t.Fatal("EpicAttrs is nil")
			}
			if task.EpicAttrs.Scope != tc.want.Scope {
				t.Errorf("Scope = %q, want %q", task.EpicAttrs.Scope, tc.want.Scope)
			}
			if task.EpicAttrs.Milestone != tc.want.Milestone {
				t.Errorf("Milestone = %q, want %q", task.EpicAttrs.Milestone, tc.want.Milestone)
			}
			if task.EpicAttrs.SuccessMetrics != tc.want.SuccessMetrics {
				t.Errorf("SuccessMetrics = %q, want %q", task.EpicAttrs.SuccessMetrics, tc.want.SuccessMetrics)
			}
		})
	}
}

func TestParseMetadata_CrossTypeAttrWarning(t *testing.T) {
	tests := []struct {
		name      string
		taskType  models.TaskType
		metaStr   string
		warnField string
	}{
		{
			name:      "bug attr on story task",
			taskType:  models.TaskTypeStory,
			metaStr:   "id: x1, status: todo, priority: 2, bug.severity: major",
			warnField: "bug.severity",
		},
		{
			name:      "idea attr on bug task",
			taskType:  models.TaskTypeBug,
			metaStr:   "id: x2, status: todo, priority: 2, idea.category: feature",
			warnField: "idea.category",
		},
		{
			name:      "task attr on epic task",
			taskType:  models.TaskTypeEpic,
			metaStr:   "id: x3, status: todo, priority: 2, task.effort: small",
			warnField: "task.effort",
		},
		{
			name:      "story attr on task task",
			taskType:  models.TaskTypeTask,
			metaStr:   "id: x4, status: todo, priority: 2, story.points: 5",
			warnField: "story.points",
		},
		{
			name:      "epic attr on idea task",
			taskType:  models.TaskTypeIdea,
			metaStr:   "id: x5, status: todo, priority: 2, epic.scope: defined",
			warnField: "epic.scope",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{Type: tc.taskType}
			warnings := ParseMetadata(task, tc.metaStr)

			found := false
			for _, w := range warnings {
				if w.Type == WarningValidation && w.Field == tc.warnField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected WarningValidation for field %q, got warnings: %v", tc.warnField, warnings)
			}

			if task.BugAttrs != nil {
				t.Error("BugAttrs should be nil for cross-type attr")
			}
			if task.IdeaAttrs != nil {
				t.Error("IdeaAttrs should be nil for cross-type attr")
			}
			if task.TaskAttrs != nil {
				t.Error("TaskAttrs should be nil for cross-type attr")
			}
			if task.StoryAttrs != nil {
				t.Error("StoryAttrs should be nil for cross-type attr")
			}
			if task.EpicAttrs != nil {
				t.Error("EpicAttrs should be nil for cross-type attr")
			}
		})
	}
}
