package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestRoundTripEdgeCaseTitleChars(t *testing.T) {
	edgeTitles := []string{
		"Use #hashtag in title",
		"Arrow -> points right",
		"Single # sign",
		"Backslash \\ character",
		"Ampersand & symbol",
		"Asterisk * wildcard",
		"[brackets] around text",
		"(parens) around text",
		`Backtick ` + "`code`" + ` reference`,
	}

	for _, title := range edgeTitles {
		t.Run(title, func(t *testing.T) {
			tasks := []*models.Task{
				{
					ID:            "task1",
					ShortID:       1,
					Type:          models.TaskTypeTask,
					Title:         title,
					WorkflowState: models.StateReady,
					Tags:          []string{},
				},
			}

			content := WriteTasksMarkdown("Edge Case Project", tasks)

			_, parsed, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("parse error for title %q: %v", title, err)
			}
			if len(parsed) != 1 {
				t.Fatalf("expected 1 task, got %d", len(parsed))
			}
			if parsed[0].Title != title {
				t.Errorf("title mismatch: got %q, want %q", parsed[0].Title, title)
			}
		})
	}
}

func TestParseDeprecatedNoteType(t *testing.T) {
	content := `# Project: Test

## Note: My Meeting Notes
<!-- id: note-1, status: todo, priority: 3 -->

Some notes here.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	task := result.Tasks[0]
	if task.Type != models.TaskTypeTask {
		t.Errorf("expected type 'task', got %q", task.Type)
	}
	if task.Title != "My Meeting Notes" {
		t.Errorf("expected title 'My Meeting Notes', got %q", task.Title)
	}

	var found bool
	for _, w := range result.Warnings {
		if w.Field == "type" && strings.Contains(w.Message, "deprecated type 'note'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected deprecation warning for 'note' type, got warnings: %v", result.Warnings)
	}
}

func TestParseDeprecatedActionType(t *testing.T) {
	content := `# Project: Test

## Action: Review PR
<!-- id: act-1, status: todo, priority: 2 -->

Review the pull request.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	task := result.Tasks[0]
	if task.Type != models.TaskTypeTask {
		t.Errorf("expected type 'task', got %q", task.Type)
	}
	if task.Title != "Review PR" {
		t.Errorf("expected title 'Review PR', got %q", task.Title)
	}

	var found bool
	for _, w := range result.Warnings {
		if w.Field == "type" && strings.Contains(w.Message, "deprecated type 'action'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected deprecation warning for 'action' type, got warnings: %v", result.Warnings)
	}
}

func TestParseTasksMarkdown_DescriptionBlockWithoutMetadataComment(t *testing.T) {
	content := `# Project: Test

## Task: Manual task
<!-- DESCRIPTION -->
My description
<!-- /DESCRIPTION -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}
	if got := result.Tasks[0].Description; got != "My description" {
		t.Fatalf("description = %q, want %q", got, "My description")
	}
}

func TestParseTasksMarkdown_LeadingNonMetadataCommentPreserved(t *testing.T) {
	content := `# Project: Test

## Task: Manual task
<!-- note: important -->
Body text
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	task := result.Tasks[0]
	if task.Priority != models.PriorityDefault {
		t.Fatalf("priority = %d, want default %d", task.Priority, models.PriorityDefault)
	}
	want := "<!-- note: important -->\nBody text"
	if task.Description != want {
		t.Fatalf("description = %q, want %q", task.Description, want)
	}
}

func TestParseTasksMarkdown_DescriptionStartStopsLaterMetadataParsing(t *testing.T) {
	content := `# Project: Test

## Task: Manual task
Intro line
<!-- priority: 1 -->
Outro line
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	task := result.Tasks[0]
	if task.Priority != models.PriorityDefault {
		t.Fatalf("priority = %d, want default %d", task.Priority, models.PriorityDefault)
	}
	want := "Intro line\n<!-- priority: 1 -->\nOutro line"
	if task.Description != want {
		t.Fatalf("description = %q, want %q", task.Description, want)
	}
}
