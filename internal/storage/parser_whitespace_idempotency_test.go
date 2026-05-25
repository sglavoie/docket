package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestWriteIdempotency_TitleWhitespace verifies that titles with surrounding
// whitespace round-trip without the second write differing from the first.
// Before the fix, the writer wrote the raw title but the parser trimmed it,
// so a leading/trailing space on the title would disappear on the next save.
func TestWriteIdempotency_TitleWhitespace(t *testing.T) {
	cases := []string{
		"trailing space  ",
		"  leading space",
		"  both  ",
	}
	for _, title := range cases {
		t.Run(title, func(t *testing.T) {
			tasks := []*models.Task{{
				ID: "t1", ShortID: 1, Type: models.TaskTypeTask,
				Title: title, Status: models.StatusTodo, Priority: 3,
				Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{},
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			}}
			first := WriteTasksMarkdown("Test", tasks)
			_, parsed, err := ParseTasksMarkdown(first)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			second := WriteTasksMarkdown("Test", parsed)
			if first != second {
				t.Errorf("write→parse→write not idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
			}
		})
	}
}

// TestWriteIdempotency_EmptyCommentDropped verifies that comments with
// whitespace-only text are not written. The parser silently drops them on
// flush; emitting them would yield write→parse→write where the second save
// is missing a comment header that the first one had.
func TestWriteIdempotency_EmptyCommentDropped(t *testing.T) {
	ts := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	tasks := []*models.Task{{
		ID: "t1", ShortID: 1, Type: models.TaskTypeTask,
		Title: "T1", Status: models.StatusTodo, Priority: 3,
		Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{},
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Comments: []*models.Comment{
			{Text: "   ", CreatedAt: ts},
			{Text: "real content", CreatedAt: ts},
		},
		Decisions: []*models.Decision{
			{Text: "", Rationale: "abandoned", Category: "design", CreatedAt: ts},
			{Text: "kept", Rationale: "", Category: "design", CreatedAt: ts},
		},
	}}
	first := WriteTasksMarkdown("Test", tasks)
	_, parsed, err := ParseTasksMarkdown(first)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(parsed[0].Comments); got != 1 {
		t.Errorf("expected 1 comment after parse, got %d", got)
	}
	if got := len(parsed[0].Decisions); got != 1 {
		t.Errorf("expected 1 decision after parse, got %d", got)
	}
	second := WriteTasksMarkdown("Test", parsed)
	if first != second {
		t.Errorf("write→parse→write not idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestWriteIdempotency_CommentAndDecisionWhitespace verifies that comment text
// and decision text/rationale fields with surrounding whitespace round-trip
// stably. The parser TrimSpace's accumulated text on flush; the writer must
// match so write→parse→write produces identical bytes.
func TestWriteIdempotency_CommentAndDecisionWhitespace(t *testing.T) {
	ts := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	tasks := []*models.Task{{
		ID: "t1", ShortID: 1, Type: models.TaskTypeTask,
		Title: "T1", Status: models.StatusTodo, Priority: 3,
		Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{},
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Comments: []*models.Comment{
			{Text: "  leading", CreatedAt: ts},
			{Text: "trailing  ", CreatedAt: ts},
		},
		Decisions: []*models.Decision{{
			Text:      "  text with leading",
			Rationale: "rationale with trailing  ",
			Category:  models.DecisionCategory("design"),
			CreatedAt: ts,
		}},
	}}
	first := WriteTasksMarkdown("Test", tasks)
	_, parsed, err := ParseTasksMarkdown(first)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	second := WriteTasksMarkdown("Test", parsed)
	if first != second {
		t.Errorf("write→parse→write not idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestWriteIdempotency_DescriptionWhitespace verifies the same property for
// descriptions. The parser strips leading/trailing whitespace from the
// description block; the writer must also trim so a single round-trip is stable.
func TestWriteIdempotency_DescriptionWhitespace(t *testing.T) {
	cases := []string{
		"trailing newline\n",
		"trailing space ",
		"  leading spaces",
		"\n\nleading blank lines",
		"trailing\n\nblank lines\n\n",
	}
	for _, desc := range cases {
		t.Run(desc[:min(12, len(desc))], func(t *testing.T) {
			tasks := []*models.Task{{
				ID: "t1", ShortID: 1, Type: models.TaskTypeTask,
				Title: "T1", Description: desc, Status: models.StatusTodo, Priority: 3,
				Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{},
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			}}
			first := WriteTasksMarkdown("Test", tasks)
			_, parsed, err := ParseTasksMarkdown(first)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			second := WriteTasksMarkdown("Test", parsed)
			if first != second {
				t.Errorf("write→parse→write not idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
			}
		})
	}
}
