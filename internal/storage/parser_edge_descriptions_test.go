package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestRoundTrip_DescriptionWithHeadingLikeLines verifies that descriptions
// containing lines that resemble task headings survive write→parse without
// creating phantom tasks or corrupting data.
func TestRoundTrip_DescriptionWithHeadingLikeLines(t *testing.T) {
	tests := []struct {
		name string
		desc string
	}{
		{
			name: "task heading in description",
			desc: "See the following example:\n## Task: Example heading in description\nEnd of example",
		},
		{
			name: "epic heading in description",
			desc: "Reference:\n## Epic: Some epic name\nMore text",
		},
		{
			name: "project header in description",
			desc: "Previous project:\n# Project: Old Name\nWas renamed",
		},
		{
			name: "multiple heading-like lines",
			desc: "## Task: First\n### Story: Second\n#### Bug: Third",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tasks := []*models.Task{
				{
					ID:          "t1",
					ShortID:     1,
					Type:        models.TaskTypeTask,
					Title:       "Test Task",
					Description: tc.desc,
					Status:      models.StatusTodo,
					Priority:    3,
					Tags:        []string{},
					DependsOn:   []string{},
					RelatedTo:   []string{},
					CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			}

			content := WriteTasksMarkdown("Test", tasks)
			_, parsed, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(parsed) != 1 {
				t.Fatalf("expected 1 task (not %d) — heading-like line in description created phantom task", len(parsed))
			}

			if parsed[0].Description != tc.desc {
				t.Errorf("description mismatch:\n  got:  %q\n  want: %q", parsed[0].Description, tc.desc)
			}
		})
	}
}

// TestRoundTrip_DescriptionWithLiteralBackslashHash verifies that a description
// starting with literal `\#` does not get incorrectly unescaped on re-parse.
func TestRoundTrip_DescriptionWithLiteralBackslashHash(t *testing.T) {
	// This description has a line starting with \# which is NOT an escaped
	// heading — it's a literal backslash followed by a hashtag. The unescape
	// logic should leave it alone since stripping the backslash does NOT
	// produce a valid heading regex match.
	desc := "Some text\n\\#hashtag is cool\nMore text"

	tasks := []*models.Task{
		{
			ID:          "t1",
			ShortID:     1,
			Type:        models.TaskTypeTask,
			Title:       "Test Task",
			Description: desc,
			Status:      models.StatusTodo,
			Priority:    3,
			Tags:        []string{},
			DependsOn:   []string{},
			RelatedTo:   []string{},
			CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Test", tasks)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}

	if parsed[0].Description != desc {
		t.Errorf("description mismatch:\n  got:  %q\n  want: %q", parsed[0].Description, desc)
	}
}

// TestRoundTrip_DescriptionEmptyAndWhitespace verifies that descriptions
// that are empty or whitespace-only round-trip correctly as empty.
func TestRoundTrip_DescriptionEmptyAndWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		wantDesc string
	}{
		{"empty", "", ""},
		{"spaces only", "   ", ""},
		{"newlines only", "\n\n\n", ""},
		{"single newline", "\n", ""},
		{"whitespace mixed", "  \n  \n  ", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tasks := []*models.Task{
				{
					ID:          "t1",
					ShortID:     1,
					Type:        models.TaskTypeTask,
					Title:       "Test Task",
					Description: tc.desc,
					Status:      models.StatusTodo,
					Priority:    3,
					Tags:        []string{},
					DependsOn:   []string{},
					RelatedTo:   []string{},
					CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			}

			content := WriteTasksMarkdown("Test", tasks)
			_, parsed, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(parsed) != 1 {
				t.Fatalf("expected 1 task, got %d", len(parsed))
			}

			if parsed[0].Description != tc.wantDesc {
				t.Errorf("description = %q, want %q", parsed[0].Description, tc.wantDesc)
			}
		})
	}
}

// TestRoundTrip_DescriptionWithHTMLCommentLikeMetadata verifies that
// descriptions containing lines that look like metadata comments don't
// get incorrectly parsed as task metadata.
func TestRoundTrip_DescriptionWithHTMLCommentLikeMetadata(t *testing.T) {
	desc := "Here is a code example:\n<!-- id: fake-id, status: done, priority: 1 -->\nThis is not metadata."

	tasks := []*models.Task{
		{
			ID:          "t1",
			ShortID:     1,
			Type:        models.TaskTypeTask,
			Title:       "Test Task",
			Description: desc,
			Status:      models.StatusTodo,
			Priority:    3,
			Tags:        []string{},
			DependsOn:   []string{},
			RelatedTo:   []string{},
			CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Test", tasks)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}

	if parsed[0].ID != "t1" {
		t.Errorf("task ID = %q, want %q — metadata-like line in description corrupted the ID", parsed[0].ID, "t1")
	}

	if !strings.Contains(parsed[0].Description, "<!-- id: fake-id") {
		t.Error("description should preserve the HTML comment-like line")
	}
}
