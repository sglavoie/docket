package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

func TestParseTaskContent(t *testing.T) {
	t.Run("parses title and description", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title")
		task.Description = "Original description."

		content := "# New Title\n\n<!-- Edit the title above and description below. -->\n\nNew description."
		title, desc := parseTaskContent(content, task)

		if title != "New Title" || desc != "New description." {
			t.Errorf("got title=%q desc=%q", title, desc)
		}
	})

	t.Run("empty title preserves original", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title")
		content := "# \n\n<!-- Edit the title above and description below. -->\n\nSome text."
		title, desc := parseTaskContent(content, task)

		if title != "Original Title" || desc != "Some text." {
			t.Errorf("got title=%q desc=%q", title, desc)
		}
	})

	t.Run("preserves HTML comments in description", func(t *testing.T) {
		task := testutil.NewTestTask("Title")
		task.Description = "<!-- user note -->\nSome text."

		content := "# Title\n\n<!-- Edit the title above and description below. -->\n\n<!-- user note -->\nSome text."
		title, desc := parseTaskContent(content, task)

		if title != "Title" {
			t.Errorf("got title %q, want %q", title, "Title")
		}
		expected := "<!-- user note -->\nSome text."
		if desc != expected {
			t.Errorf("got desc:\n%q\nwant:\n%q", desc, expected)
		}
	})

	t.Run("preserves multiple HTML comments in description", func(t *testing.T) {
		task := testutil.NewTestTask("Title")

		content := "# Title\n\n<!-- Edit the title above and description below. -->\n\nBefore.\n\n<!-- hidden note -->\n\nMiddle.\n\n<!-- another comment -->\n\nAfter."
		title, desc := parseTaskContent(content, task)

		if title != "Title" {
			t.Errorf("got title %q, want %q", title, "Title")
		}
		expected := "Before.\n\n<!-- hidden note -->\n\nMiddle.\n\n<!-- another comment -->\n\nAfter."
		if desc != expected {
			t.Errorf("got desc:\n%q\nwant:\n%q", desc, expected)
		}
	})

	t.Run("H1 heading after comment is description not title", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title")

		content := "# My Title\n\n<!-- Edit the title above and description below. -->\n\n# Description Heading\n\nSome text."
		title, desc := parseTaskContent(content, task)

		if title != "My Title" {
			t.Errorf("got title %q, want %q", title, "My Title")
		}
		expected := "# Description Heading\n\nSome text."
		if desc != expected {
			t.Errorf("got desc: %q, want: %q", desc, expected)
		}
	})

	t.Run("trailing empty lines trimmed", func(t *testing.T) {
		task := testutil.NewTestTask("Title")

		content := "# Title\n\n<!-- Edit the title above and description below. -->\n\nContent here.\n\n\n"
		_, desc := parseTaskContent(content, task)

		if desc != "Content here." {
			t.Errorf("got desc %q, want %q", desc, "Content here.")
		}
	})

	t.Run("handles content with leading empty lines after comment", func(t *testing.T) {
		task := testutil.NewTestTask("Title")

		content := "# Title\n\n<!-- Edit the title above and description below. -->\n\n\n\nActual content."
		_, desc := parseTaskContent(content, task)

		if desc != "Actual content." {
			t.Errorf("got desc %q, want %q", desc, "Actual content.")
		}
	})

	t.Run("missing comment marker preserves content as description", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title")

		content := "# New Title\n\nNo comment here.\nSome text."
		title, desc := parseTaskContent(content, task)

		if title != "New Title" || desc != "No comment here.\nSome text." {
			t.Errorf("got title=%q desc=%q", title, desc)
		}
	})

	t.Run("missing comment marker with empty description", func(t *testing.T) {
		task := testutil.NewTestTask("Original Title")

		content := "# New Title"
		title, desc := parseTaskContent(content, task)

		if title != "New Title" || desc != "" {
			t.Errorf("got title=%q desc=%q", title, desc)
		}
	})

	t.Run("multiline description preserved", func(t *testing.T) {
		task := testutil.NewTestTask("Title")

		content := "# Title\n\n<!-- Edit the title above and description below. -->\n\nParagraph one.\n\nParagraph two.\n\n- Bullet one\n- Bullet two"
		_, desc := parseTaskContent(content, task)

		expected := "Paragraph one.\n\nParagraph two.\n\n- Bullet one\n- Bullet two"
		if desc != expected {
			t.Errorf("got desc:\n%s\nwant:\n%s", desc, expected)
		}
	})
}
