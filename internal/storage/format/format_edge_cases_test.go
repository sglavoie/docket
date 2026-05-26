package format

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestMetadataRegex_CRLFLineEndings guards against a tasks.md file saved with
// CRLF line endings (common on Windows editors). bufio.Scanner strips the
// trailing \n but not the \r, so the metadata regex must tolerate trailing
// whitespace after the closing --> or the entire metadata line silently
// fails to match and the task loads with default values.
func TestMetadataRegex_CRLFLineEndings(t *testing.T) {
	line := "<!-- id: t1, status: todo, priority: 2 -->\r"
	re := MetadataRegex()
	matches := re.FindStringSubmatch(line)
	if matches == nil {
		t.Fatalf("regex did not match line with trailing CR; got nil")
	}
	if !strings.Contains(matches[1], "id: t1") {
		t.Errorf("captured payload = %q, want it to contain %q", matches[1], "id: t1")
	}
}

// TestParseMetadata_ScoreWithInvalidDimensionIDDoesNotMisSplit ensures that a
// "score.<bad-id>:" pattern inside a free-text value does not get treated as
// a real score key by findScoreKeyIndex (which validates the dim ID).
func TestParseMetadata_ScoreWithInvalidDimensionIDDoesNotMisSplit(t *testing.T) {
	task := &models.Task{}
	// "score.bad-id: 5" has a dash, so isValidDimensionID rejects it. The
	// splitter must NOT split here — the URL should stay intact.
	metaStr := "id: t1, status: todo, priority: 2, url: http://x.com/?score.bad-id: 5"
	ParseMetadata(task, metaStr)

	if task.ID != "t1" {
		t.Errorf("ID = %q, want %q", task.ID, "t1")
	}
	if !strings.Contains(task.URL, "score.bad-id: 5") {
		t.Errorf("url = %q, want it to contain %q", task.URL, "score.bad-id: 5")
	}
}
