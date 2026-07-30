package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestRoundTrip_AdversarialInputs stress-tests the parser/writer pair with
// inputs designed to expose escape, delimiter, and block-marker corner cases.
// Each subtest constructs a single task with a hostile field value, writes it,
// parses it, and asserts the value survived intact.
func TestRoundTrip_AdversarialInputs(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	type scenario struct {
		name      string
		mutate    func(*models.Task)
		assertion func(t *testing.T, got *models.Task)
	}

	scenarios := []scenario{
		{
			name: "title with hash characters and colons",
			mutate: func(task *models.Task) {
				task.Title = "Use #tag-syntax: ok? (issue #42)"
			},
			assertion: func(t *testing.T, got *models.Task) {
				want := "Use #tag-syntax: ok? (issue #42)"
				if got.Title != want {
					t.Errorf("title: got %q, want %q", got.Title, want)
				}
			},
		},
		{
			name: "title with unicode and trailing punctuation",
			mutate: func(task *models.Task) {
				task.Title = "café — naïve résumé: déjà vu? 🚀"
			},
			assertion: func(t *testing.T, got *models.Task) {
				want := "café — naïve résumé: déjà vu? 🚀"
				if got.Title != want {
					t.Errorf("title: got %q, want %q", got.Title, want)
				}
			},
		},
		{
			name: "tags with unicode, dots, dashes",
			mutate: func(task *models.Task) {
				task.Tags = []string{"v1.2.3", "kebab-case", "unicode-Ω", "под_черк"}
			},
			assertion: func(t *testing.T, got *models.Task) {
				want := []string{"v1.2.3", "kebab-case", "unicode-Ω", "под_черк"}
				if len(got.Tags) != len(want) {
					t.Fatalf("tags count: got %d, want %d (%v)", len(got.Tags), len(want), got.Tags)
				}
				for i, tag := range want {
					if got.Tags[i] != tag {
						t.Errorf("tag[%d]: got %q, want %q", i, got.Tags[i], tag)
					}
				}
			},
		},
		{
			name: "description with markdown headings that collide with task headings",
			mutate: func(task *models.Task) {
				task.Description = "Intro paragraph.\n## Task: this is documentation, not a task\n### Story: also not a story\nEnd."
			},
			assertion: func(t *testing.T, got *models.Task) {
				want := "Intro paragraph.\n## Task: this is documentation, not a task\n### Story: also not a story\nEnd."
				if got.Description != want {
					t.Errorf("description mismatch:\n  got:  %q\n  want: %q", got.Description, want)
				}
			},
		},
		{
			name: "description with HTML comments resembling metadata",
			mutate: func(task *models.Task) {
				task.Description = "Reference docs say:\n<!-- id: fake-uuid, status: hacked, priority: 0 -->\n<!-- sections: Pretend;Sections -->"
			},
			assertion: func(t *testing.T, got *models.Task) {
				if !strings.Contains(got.Description, "id: fake-uuid") {
					t.Errorf("description lost metadata-like content: %q", got.Description)
				}
				if !strings.Contains(got.Description, "sections: Pretend") {
					t.Errorf("description lost sections-like content: %q", got.Description)
				}
				if got.ID == "fake-uuid" {
					t.Errorf("parser overwrote task ID from description content")
				}
			},
		},
		{
			name: "description containing literal block markers",
			mutate: func(task *models.Task) {
				task.Description = "Before block.\n<!-- DESCRIPTION -->\n<!-- COMMENTS -->\n<!-- /COMMENTS -->\n<!-- ACTIVITY -->\n<!-- /ACTIVITY -->\n<!-- DECISIONS -->\n<!-- /DECISIONS -->\n<!-- /DESCRIPTION -->\nAfter block."
			},
			assertion: func(t *testing.T, got *models.Task) {
				if !strings.Contains(got.Description, "<!-- COMMENTS -->") {
					t.Errorf("description lost COMMENTS marker: %q", got.Description)
				}
				if !strings.Contains(got.Description, "<!-- /DESCRIPTION -->") {
					t.Errorf("description lost /DESCRIPTION marker: %q", got.Description)
				}
				if !strings.Contains(got.Description, "Before block.") || !strings.Contains(got.Description, "After block.") {
					t.Errorf("description lost surrounding text: %q", got.Description)
				}
				if len(got.Comments) != 0 {
					t.Errorf("comments leaked from description: %v", got.Comments)
				}
			},
		},
		{
			name: "description with leading and trailing whitespace lines",
			mutate: func(task *models.Task) {
				task.Description = "line 1\n\n\nline 2 with    spaces\n\ttabbed line\n"
			},
			assertion: func(t *testing.T, got *models.Task) {
				want := "line 1\n\n\nline 2 with    spaces\n\ttabbed line"
				if got.Description != want {
					t.Errorf("description mismatch:\n  got:  %q\n  want: %q", got.Description, want)
				}
			},
		},
		{
			name: "multi-line comment body with heading-like and marker-like content",
			mutate: func(task *models.Task) {
				task.Comments = []*models.Comment{
					{
						Text:      "First line of comment.\n## Task: not a real task\n<!-- comment: 2099-01-01 00:00 -->\n<!-- COMMENTS -->\nLast line.",
						CreatedAt: now.Add(time.Hour),
					},
				}
			},
			assertion: func(t *testing.T, got *models.Task) {
				if len(got.Comments) != 1 {
					t.Fatalf("expected 1 comment, got %d", len(got.Comments))
				}
				want := "First line of comment.\n## Task: not a real task\n<!-- comment: 2099-01-01 00:00 -->\n<!-- COMMENTS -->\nLast line."
				if got.Comments[0].Text != want {
					t.Errorf("comment text mismatch:\n  got:  %q\n  want: %q", got.Comments[0].Text, want)
				}
			},
		},
		{
			name: "decisions and comments alongside heading-like description",
			mutate: func(task *models.Task) {
				task.Description = "## Task: looks-like-heading"
				task.Comments = []*models.Comment{
					{Text: "Important note.", CreatedAt: now.Add(time.Minute)},
				}
			},
			assertion: func(t *testing.T, got *models.Task) {
				wantDesc := "## Task: looks-like-heading"
				if got.Description != wantDesc {
					t.Errorf("description: got %q, want %q", got.Description, wantDesc)
				}
				if len(got.Comments) != 1 || got.Comments[0].Text != "Important note." {
					t.Errorf("comment: %+v", got.Comments)
				}
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			task, err := models.NewGenericTask("adv-id-1", 1, "Adversarial Task")
			if err != nil {
				t.Fatal(err)
			}
			task.CreatedAt = now
			task.UpdatedAt = now
			sc.mutate(task)

			md := WriteTasksMarkdown("Adversarial Project", []*models.Task{task})
			projectName, parsed, err := ParseTasksMarkdown(md)
			if err != nil {
				t.Fatalf("parse error: %v\n---\n%s\n---", err, md)
			}
			if projectName != "Adversarial Project" {
				t.Errorf("project name: got %q, want %q", projectName, "Adversarial Project")
			}
			if len(parsed) != 1 {
				t.Fatalf("expected 1 task, got %d\n---\n%s\n---", len(parsed), md)
			}
			sc.assertion(t, parsed[0])
		})
	}
}
