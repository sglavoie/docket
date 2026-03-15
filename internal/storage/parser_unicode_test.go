package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTasksMarkdown_UnicodeTitle(t *testing.T) {
	content := "# Project: Test\n\n## Task: \u4efb\u52d9\u7ba1\u7406\u30c4\u30fc\u30eb\n<!-- id: t1, status: todo, priority: 2 -->\n\nDescription with unicode: \u00e9\u00e8\u00ea\u00eb\n"
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "\u4efb\u52d9\u7ba1\u7406\u30c4\u30fc\u30eb" {
		t.Errorf("expected unicode title, got %q", tasks[0].Title)
	}
	if !strings.Contains(tasks[0].Description, "\u00e9\u00e8\u00ea\u00eb") {
		t.Error("expected unicode in description")
	}
}

func TestParseTasksMarkdown_EmojiTitle(t *testing.T) {
	content := "# Project: Test\n\n## Bug: \U0001f41b Fix login crash\n<!-- id: b1, status: todo, priority: 1 -->\n\n\U0001f525 Hot fix needed\n"
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if !strings.Contains(tasks[0].Title, "Fix login crash") {
		t.Errorf("expected title with emoji prefix, got %q", tasks[0].Title)
	}
}

func TestParseTasksMarkdown_SpecialCharsInTitle(t *testing.T) {
	content := "# Project: Test\n\n## Task: Fix \"quotes\" & <angle> brackets\n<!-- id: t1, status: todo, priority: 2 -->\n\nDescription with `code` and *emphasis*.\n"
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if !strings.Contains(tasks[0].Title, "\"quotes\"") {
		t.Errorf("expected quotes in title, got %q", tasks[0].Title)
	}
	if !strings.Contains(tasks[0].Title, "&") {
		t.Errorf("expected ampersand in title, got %q", tasks[0].Title)
	}
}

func TestRoundTrip_UnicodeTitle(t *testing.T) {
	original := []*models.Task{
		{
			ID:          "t1",
			ShortID:     1,
			Type:        models.TaskTypeTask,
			Title:       "\u4efb\u52d9\u7ba1\u7406 - Task Manager",
			Description: "\u00c9crire du code avec des caract\u00e8res sp\u00e9ciaux: \u00e0\u00e7\u00fc\u00f1",
			Status:      models.StatusTodo,
			Priority:    2,
			CreatedAt:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Test", original)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("roundtrip parse failed: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}
	if parsed[0].Title != original[0].Title {
		t.Errorf("title mismatch: got %q, want %q", parsed[0].Title, original[0].Title)
	}
	if parsed[0].Description != original[0].Description {
		t.Errorf("description mismatch: got %q, want %q", parsed[0].Description, original[0].Description)
	}
}

func TestRoundTrip_EmptyDescription(t *testing.T) {
	original := []*models.Task{
		{
			ID:          "t1",
			ShortID:     1,
			Type:        models.TaskTypeTask,
			Title:       "No Description",
			Description: "",
			Status:      models.StatusTodo,
			Priority:    3,
			CreatedAt:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Test", original)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("roundtrip parse failed: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}
	if parsed[0].Description != "" {
		t.Errorf("expected empty description, got %q", parsed[0].Description)
	}
}

func TestRoundTrip_SpecialCharsInTags(t *testing.T) {
	original := []*models.Task{
		{
			ID:        "t1",
			ShortID:   1,
			Type:      models.TaskTypeTask,
			Title:     "Tagged Task",
			Status:    models.StatusTodo,
			Priority:  2,
			Tags:      []string{"c++", "c#", "front-end", "v2.0"},
			CreatedAt: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Test", original)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("roundtrip parse failed: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}
	if len(parsed[0].Tags) != len(original[0].Tags) {
		t.Fatalf("expected %d tags, got %d", len(original[0].Tags), len(parsed[0].Tags))
	}
	for i, tag := range original[0].Tags {
		if parsed[0].Tags[i] != tag {
			t.Errorf("tag[%d] mismatch: got %q, want %q", i, parsed[0].Tags[i], tag)
		}
	}
}

func TestRoundTrip_ColonInTitle(t *testing.T) {
	// Title with extra colons should be preserved correctly
	original := []*models.Task{
		{
			ID:        "t1",
			ShortID:   1,
			Type:      models.TaskTypeTask,
			Title:     "Fix: Handle edge case: empty input",
			Status:    models.StatusTodo,
			Priority:  2,
			CreatedAt: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Test", original)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("roundtrip parse failed: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}
	if parsed[0].Title != original[0].Title {
		t.Errorf("title mismatch: got %q, want %q", parsed[0].Title, original[0].Title)
	}
}

func TestRoundTrip_DescriptionWithHTMLComment(t *testing.T) {
	// Description containing text that looks like an HTML comment should not
	// confuse the parser on re-read
	original := []*models.Task{
		{
			ID:          "t1",
			ShortID:     1,
			Type:        models.TaskTypeTask,
			Title:       "Tricky Description",
			Description: "Use <!-- comment --> syntax in markdown.",
			Status:      models.StatusTodo,
			Priority:    2,
			CreatedAt:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Test", original)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("roundtrip parse failed: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}
	// The parser should distinguish the description's HTML comment from metadata
	if parsed[0].ID != "t1" {
		t.Errorf("ID mismatch: got %q, want 't1'", parsed[0].ID)
	}
}
