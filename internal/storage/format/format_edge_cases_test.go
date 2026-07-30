package format

import (
	"strings"
	"testing"
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
