package taskdetail

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/textfit"
)

func commentPreviewFor(t *testing.T, text string) (string, int) {
	t.Helper()
	var preview strings.Builder
	renderCommentPreview(&preview, text, 60)
	return ansi.Strip(preview.String()), len(strings.Split(textfit.RenderMarkdown(text, 60), "\n"))
}

// TestCommentPreviewShowsAThirdLineInsteadOfCountingIt keeps the preview's third
// row for the comment's own text when that text is all there is left.
func TestCommentPreviewShowsAThirdLineInsteadOfCountingIt(t *testing.T) {
	preview, rendered := commentPreviewFor(t, "- alpha\n- bravo")
	if rendered != 3 {
		t.Fatalf("fixture renders %d lines, want 3", rendered)
	}
	if strings.Contains(preview, "more") || !strings.Contains(preview, "bravo") {
		t.Fatalf("three-line preview:\n%s", preview)
	}
}

// TestCommentPreviewCountsTheRemainderInAgreement names the hidden lines with a
// correctly pluralized count.
func TestCommentPreviewCountsTheRemainderInAgreement(t *testing.T) {
	preview, rendered := commentPreviewFor(t, "- alpha\n- bravo\n- charlie")
	if rendered != 4 {
		t.Fatalf("fixture renders %d lines, want 4", rendered)
	}
	if !strings.Contains(preview, "(2 more lines, press Enter to expand)") || strings.Contains(preview, "bravo") {
		t.Fatalf("four-line preview:\n%s", preview)
	}
}
