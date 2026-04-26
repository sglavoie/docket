package format

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestExtractDescriptionBlock_NoMarkers(t *testing.T) {
	// Old-format description with no DESCRIPTION markers must be left unchanged.
	original := "Some description\nwith multiple lines"
	task := &models.Task{Description: original}

	warnings := ExtractDescriptionBlock(task)

	if task.Description != original {
		t.Errorf("description changed: got %q, want %q", task.Description, original)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

func TestExtractDescriptionBlock_WithMarkers(t *testing.T) {
	task := &models.Task{
		Description: "<!-- DESCRIPTION -->\nSome content\n<!-- /DESCRIPTION -->",
	}

	warnings := ExtractDescriptionBlock(task)

	if task.Description != "Some content" {
		t.Errorf("description = %q, want %q", task.Description, "Some content")
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

func TestExtractDescriptionBlock_EmptyBlock(t *testing.T) {
	// Markers present but no content inside → empty description, no warnings.
	task := &models.Task{
		Description: "<!-- DESCRIPTION -->\n<!-- /DESCRIPTION -->",
	}

	warnings := ExtractDescriptionBlock(task)

	if task.Description != "" {
		t.Errorf("description = %q, want empty string", task.Description)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

func TestExtractDescriptionBlock_EmptyDescription(t *testing.T) {
	// Empty description → no-op, nil warnings.
	task := &models.Task{Description: ""}

	warnings := ExtractDescriptionBlock(task)

	if warnings != nil {
		t.Errorf("expected nil warnings, got %v", warnings)
	}
	if task.Description != "" {
		t.Errorf("description changed from empty: got %q", task.Description)
	}
}

func TestExtractDescriptionBlock_OrphanedContent(t *testing.T) {
	// Text outside the DESCRIPTION block is orphaned: a warning is issued and
	// the orphaned text is appended to the description to prevent data loss.
	task := &models.Task{
		ID:          "t1",
		Description: "<!-- DESCRIPTION -->\nIn-block content\n<!-- /DESCRIPTION -->\nOrphaned text",
	}

	warnings := ExtractDescriptionBlock(task)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].Type != WarningValidation {
		t.Errorf("warning type = %v, want WarningValidation", warnings[0].Type)
	}
	if warnings[0].TaskID != "t1" {
		t.Errorf("warning TaskID = %q, want %q", warnings[0].TaskID, "t1")
	}
	if !strings.Contains(task.Description, "In-block content") {
		t.Errorf("description missing in-block content: %q", task.Description)
	}
	if !strings.Contains(task.Description, "Orphaned text") {
		t.Errorf("description missing orphaned content: %q", task.Description)
	}
}

func TestExtractDescriptionBlock_MultilineContent(t *testing.T) {
	task := &models.Task{
		Description: "<!-- DESCRIPTION -->\nLine one\nLine two\nLine three\n<!-- /DESCRIPTION -->",
	}

	warnings := ExtractDescriptionBlock(task)

	want := "Line one\nLine two\nLine three"
	if task.Description != want {
		t.Errorf("description = %q, want %q", task.Description, want)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

func TestExtractDescriptionBlock_RoundTrip(t *testing.T) {
	// Simulate what the writer produces, then verify ExtractDescriptionBlock
	// recovers the original description.
	originals := []string{
		"Simple description",
		"Multi-line\ndescription\nwith three lines",
		"Description with ## Task: heading-like line",
		"Description with <!-- fake comment --> inside",
	}

	for _, original := range originals {
		t.Run(original[:min(len(original), 30)], func(t *testing.T) {
			// Reproduce what markdown_write.go writes
			written := "<!-- DESCRIPTION -->\n" + original + "\n<!-- /DESCRIPTION -->"
			task := &models.Task{Description: written}

			warnings := ExtractDescriptionBlock(task)

			if task.Description != original {
				t.Errorf("round-trip failed:\n  got:  %q\n  want: %q", task.Description, original)
			}
			if len(warnings) != 0 {
				t.Errorf("unexpected warnings: %v", warnings)
			}
		})
	}
}
