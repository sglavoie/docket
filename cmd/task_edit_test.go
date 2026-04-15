package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestBuildEditContent(t *testing.T) {
	t.Run("formats task with title and description", func(t *testing.T) {
		task := testutil.NewTestTask("My Task Title", models.TaskTypeTask)
		task.Description = "This is the task description."

		content := buildEditContent(task)

		expected := "# My Task Title\n\n<!-- Edit the title above and description below. -->\n\nThis is the task description."
		if content != expected {
			t.Errorf("got:\n%s\nwant:\n%s", content, expected)
		}
	})

	t.Run("handles empty description", func(t *testing.T) {
		task := testutil.NewTestTask("Task Without Description", models.TaskTypeTask)
		task.Description = ""

		content := buildEditContent(task)

		expected := "# Task Without Description\n\n<!-- Edit the title above and description below. -->\n\n"
		if content != expected {
			t.Errorf("got:\n%s\nwant:\n%s", content, expected)
		}
	})

	t.Run("handles multiline description", func(t *testing.T) {
		task := testutil.NewTestTask("Multiline Task", models.TaskTypeTask)
		task.Description = "Line one.\n\nLine two.\n\nLine three."

		content := buildEditContent(task)

		expected := "# Multiline Task\n\n<!-- Edit the title above and description below. -->\n\nLine one.\n\nLine two.\n\nLine three."
		if content != expected {
			t.Errorf("got:\n%s\nwant:\n%s", content, expected)
		}
	})
}

func TestParseEditedContent(t *testing.T) {
	t.Run("parses title change only", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title", models.TaskTypeTask)
		task.Description = "Original description."

		content := "# New Title\n\n<!-- Edit the title above and description below. -->\n\nOriginal description."

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "New Title")
		testutil.AssertEqual(t, desc, "Original description.")
	})

	t.Run("parses description change only", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title", models.TaskTypeTask)
		task.Description = "Original description."

		content := "# Original Title\n\n<!-- Edit the title above and description below. -->\n\nNew description."

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "Original Title")
		testutil.AssertEqual(t, desc, "New description.")
	})

	t.Run("parses both title and description changed", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title", models.TaskTypeTask)
		task.Description = "Original description."

		content := "# Updated Title\n\n<!-- Edit the title above and description below. -->\n\nUpdated description."

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "Updated Title")
		testutil.AssertEqual(t, desc, "Updated description.")
	})

	t.Run("empty title preserves original", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title", models.TaskTypeTask)
		task.Description = "Description."

		content := "# \n\n<!-- Edit the title above and description below. -->\n\nDescription."

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "Original Title")
		testutil.AssertEqual(t, desc, "Description.")
	})

	t.Run("missing HTML comment marker preserves content as description", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title", models.TaskTypeTask)
		task.Description = "Original description."

		content := "# New Title\n\nNo comment marker here.\n\nSome text."

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "New Title")
		testutil.AssertEqual(t, desc, "No comment marker here.\n\nSome text.")
	})

	t.Run("# headers in description section are preserved as content", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title", models.TaskTypeTask)
		task.Description = "Original description."

		content := "# Title\n\n<!-- Edit the title above and description below. -->\n\nSome intro text.\n\n## Subheading\n\nBody text."

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "Title")
		expected := "Some intro text.\n\n## Subheading\n\nBody text."
		if desc != expected {
			t.Errorf("got desc: %q, want: %q", desc, expected)
		}
	})

	t.Run("trailing empty lines trimmed from description", func(t *testing.T) {
		task := testutil.NewTestTask("Title", models.TaskTypeTask)
		task.Description = "Description."

		content := "# Title\n\n<!-- Edit the title above and description below. -->\n\nDescription with trailing.\n\n\n"

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "Title")
		testutil.AssertEqual(t, desc, "Description with trailing.")
	})

	t.Run("multiline description preserved", func(t *testing.T) {
		task := testutil.NewTestTask("Title", models.TaskTypeTask)
		task.Description = ""

		content := "# Title\n\n<!-- Edit the title above and description below. -->\n\nParagraph one.\n\nParagraph two.\n\n- Bullet one\n- Bullet two"

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "Title")
		expected := "Paragraph one.\n\nParagraph two.\n\n- Bullet one\n- Bullet two"
		if desc != expected {
			t.Errorf("got desc:\n%s\nwant:\n%s", desc, expected)
		}
	})

	t.Run("handles content with leading empty lines after comment", func(t *testing.T) {
		task := testutil.NewTestTask("Title", models.TaskTypeTask)
		task.Description = ""

		content := "# Title\n\n<!-- Edit the title above and description below. -->\n\n\n\nActual description starts here."

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "Title")
		testutil.AssertEqual(t, desc, "Actual description starts here.")
	})

	t.Run("missing comment marker with empty description", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title", models.TaskTypeTask)
		task.Description = ""

		content := "# New Title"

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "New Title")
		testutil.AssertEqual(t, desc, "")
	})

	t.Run("H1 heading after comment is description not title", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title", models.TaskTypeTask)
		task.Description = ""

		content := "# My Title\n\n<!-- Edit the title above and description below. -->\n\n# Description Heading\n\nSome text."

		title, desc := parseEditedContent(content, task)

		testutil.AssertEqual(t, title, "My Title")
		expected := "# Description Heading\n\nSome text."
		if desc != expected {
			t.Errorf("got desc: %q, want: %q", desc, expected)
		}
	})
}
