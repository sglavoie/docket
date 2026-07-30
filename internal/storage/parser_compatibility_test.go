package storage

import (
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

func TestParseTasksMarkdown_DescriptionBlockWithoutMetadataComment(t *testing.T) {
	content := `# Project: Test

## Task: Manual task
<!-- DESCRIPTION -->
My description
<!-- /DESCRIPTION -->
`
	result, err := ParseTasksMarkdownDocument(content)
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
	result, err := ParseTasksMarkdownDocument(content)
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
	result, err := ParseTasksMarkdownDocument(content)
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
