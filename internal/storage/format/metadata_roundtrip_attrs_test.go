package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestRoundTrip_BugAttrs(t *testing.T) {
	task := &models.Task{
		ID:       "bug-1",
		Type:     models.TaskTypeBug,
		Status:   models.StatusTodo,
		Priority: 1,
		BugAttrs: &models.BugAttrs{
			Reproducible: models.BugReproducibleYes,
			Severity:     models.BugSeverityCritical,
			Workaround:   "restart the service",
		},
	}

	parsed := &models.Task{Type: models.TaskTypeBug}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.BugAttrs == nil {
		t.Fatal("BugAttrs is nil after round-trip")
	}
	if parsed.BugAttrs.Reproducible != models.BugReproducibleYes {
		t.Errorf("Reproducible = %q, want %q", parsed.BugAttrs.Reproducible, models.BugReproducibleYes)
	}
	if parsed.BugAttrs.Severity != models.BugSeverityCritical {
		t.Errorf("Severity = %q, want %q", parsed.BugAttrs.Severity, models.BugSeverityCritical)
	}
	if parsed.BugAttrs.Workaround != "restart the service" {
		t.Errorf("Workaround = %q, want %q", parsed.BugAttrs.Workaround, "restart the service")
	}
}

func TestRoundTrip_IdeaAttrs(t *testing.T) {
	task := &models.Task{
		ID:       "idea-1",
		Type:     models.TaskTypeIdea,
		Status:   models.StatusTodo,
		Priority: 3,
		IdeaAttrs: &models.IdeaAttrs{
			Category:  models.IdeaCategoryFeature,
			Effort:    models.EffortMedium,
			Value:     models.IdeaValueHigh,
			Validated: true,
		},
	}

	parsed := &models.Task{Type: models.TaskTypeIdea}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.IdeaAttrs == nil {
		t.Fatal("IdeaAttrs is nil after round-trip")
	}
	if parsed.IdeaAttrs.Category != models.IdeaCategoryFeature {
		t.Errorf("Category = %q, want %q", parsed.IdeaAttrs.Category, models.IdeaCategoryFeature)
	}
	if parsed.IdeaAttrs.Effort != models.EffortMedium {
		t.Errorf("Effort = %q, want %q", parsed.IdeaAttrs.Effort, models.EffortMedium)
	}
	if parsed.IdeaAttrs.Value != models.IdeaValueHigh {
		t.Errorf("Value = %q, want %q", parsed.IdeaAttrs.Value, models.IdeaValueHigh)
	}
	if !parsed.IdeaAttrs.Validated {
		t.Errorf("Validated = false, want true")
	}
}

func TestRoundTrip_SourceAndURL(t *testing.T) {
	task := &models.Task{
		ID:       "src-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
		Source:   "cli",
		URL:      "https://example.com",
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.Source != "cli" {
		t.Errorf("Source = %q, want %q", parsed.Source, "cli")
	}
	if parsed.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", parsed.URL, "https://example.com")
	}
}

func TestRoundTrip_NewlineEscapingWithLiteralBackslash(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"literal backslash-n", `Use \n for newlines`},
		{"actual newline", "line1\nline2"},
		{"both literal and actual", "line1\nuse \\n here"},
		{"trailing backslash", `end with \`},
		{"double backslash", `path\\to\\file`},
		{"backslash then newline", "before\\\nafter"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{
				ID:       "escape-test",
				Type:     models.TaskTypeBug,
				Status:   models.StatusTodo,
				Priority: 2,
				BugAttrs: &models.BugAttrs{
					Workaround: tc.input,
				},
			}

			parsed := &models.Task{Type: models.TaskTypeBug}
			warnings := parseWrittenMetadata(task, parsed)

			if len(warnings) != 0 {
				t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
			}
			if parsed.BugAttrs == nil {
				t.Fatal("BugAttrs is nil after round-trip")
			}
			if parsed.BugAttrs.Workaround != tc.input {
				t.Errorf("Workaround = %q, want %q", parsed.BugAttrs.Workaround, tc.input)
			}
		})
	}
}

func TestRoundTrip_StoryPersonaWithBackslash(t *testing.T) {
	input := `Admin uses \n and \\path`
	task := &models.Task{
		ID:       "story-esc-1",
		Type:     models.TaskTypeStory,
		Status:   models.StatusTodo,
		Priority: 2,
		StoryAttrs: &models.StoryAttrs{
			Persona: input,
		},
	}

	parsed := &models.Task{Type: models.TaskTypeStory}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.StoryAttrs == nil {
		t.Fatal("StoryAttrs is nil after round-trip")
	}
	if parsed.StoryAttrs.Persona != input {
		t.Errorf("Persona = %q, want %q", parsed.StoryAttrs.Persona, input)
	}
}

func TestRoundTrip_EpicMetricsWithBackslash(t *testing.T) {
	input := "metric1\nmetric2 with \\n literal"
	task := &models.Task{
		ID:       "epic-esc-1",
		Type:     models.TaskTypeEpic,
		Status:   models.StatusTodo,
		Priority: 2,
		EpicAttrs: &models.EpicAttrs{
			SuccessMetrics: input,
		},
	}

	parsed := &models.Task{Type: models.TaskTypeEpic}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.EpicAttrs == nil {
		t.Fatal("EpicAttrs is nil after round-trip")
	}
	if parsed.EpicAttrs.SuccessMetrics != input {
		t.Errorf("SuccessMetrics = %q, want %q", parsed.EpicAttrs.SuccessMetrics, input)
	}
}

func TestRoundTrip_FreeTextAttrsWithFixedPointPayloads(t *testing.T) {
	tests := []struct {
		name  string
		task  *models.Task
		check func(*testing.T, *models.Task)
	}{
		{
			name: "bug workaround",
			task: &models.Task{
				ID:       "bug-fixed-point",
				Type:     models.TaskTypeBug,
				Status:   models.StatusTodo,
				Priority: 1,
				BugAttrs: &models.BugAttrs{
					Reproducible: models.BugReproducibleYes,
					Severity:     models.BugSeverityCritical,
					Workaround:   "Retry, status: todo, id: bug-fixed-point\n## Task: workaround heading\n<!-- keep literal -->",
				},
			},
			check: func(t *testing.T, parsed *models.Task) {
				t.Helper()
				if parsed.BugAttrs == nil {
					t.Fatal("BugAttrs is nil after round-trip")
				}
				want := "Retry, status: todo, id: bug-fixed-point\n## Task: workaround heading\n<!-- keep literal -->"
				if parsed.BugAttrs.Workaround != want {
					t.Errorf("Workaround = %q, want %q", parsed.BugAttrs.Workaround, want)
				}
			},
		},
		{
			name: "story persona",
			task: &models.Task{
				ID:       "story-fixed-point",
				Type:     models.TaskTypeStory,
				Status:   models.StatusInProgress,
				Priority: 2,
				StoryAttrs: &models.StoryAttrs{
					StoryPoints: 5,
					Persona:     "As operator, priority: 2\n### Story: keep literal\n<!-- persona marker -->",
				},
			},
			check: func(t *testing.T, parsed *models.Task) {
				t.Helper()
				if parsed.StoryAttrs == nil {
					t.Fatal("StoryAttrs is nil after round-trip")
				}
				want := "As operator, priority: 2\n### Story: keep literal\n<!-- persona marker -->"
				if parsed.StoryAttrs.Persona != want {
					t.Errorf("Persona = %q, want %q", parsed.StoryAttrs.Persona, want)
				}
			},
		},
		{
			name: "epic metrics",
			task: &models.Task{
				ID:       "epic-fixed-point",
				Type:     models.TaskTypeEpic,
				Status:   models.StatusTodo,
				Priority: 1,
				EpicAttrs: &models.EpicAttrs{
					Scope:          models.EpicScopeDefined,
					Milestone:      "Phase 1, status: steady",
					SuccessMetrics: "Ship parser, priority: 1\n# Project: embedded\n<!-- metrics marker -->",
				},
			},
			check: func(t *testing.T, parsed *models.Task) {
				t.Helper()
				if parsed.EpicAttrs == nil {
					t.Fatal("EpicAttrs is nil after round-trip")
				}
				want := "Ship parser, priority: 1\n# Project: embedded\n<!-- metrics marker -->"
				if parsed.EpicAttrs.SuccessMetrics != want {
					t.Errorf("SuccessMetrics = %q, want %q", parsed.EpicAttrs.SuccessMetrics, want)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed := &models.Task{Type: tc.task.Type}
			warnings := parseWrittenMetadata(tc.task, parsed)
			if len(warnings) != 0 {
				t.Fatalf("got %d warnings, want 0: %v", len(warnings), warnings)
			}

			tc.check(t, parsed)

			if got, want := WriteMetadata(parsed), WriteMetadata(tc.task); got != want {
				t.Fatalf("metadata fixed point mismatch:\n got: %s\nwant: %s", got, want)
			}
		})
	}
}
