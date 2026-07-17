package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestCLIReferenceFocusContract(t *testing.T) {
	reference, err := os.ReadFile("cli-reference.md")
	if err != nil {
		t.Fatalf("read CLI reference: %v", err)
	}

	if !strings.Contains(string(reference), "## tt focus queue") {
		t.Fatal("CLI reference is missing tt focus queue")
	}

	const heading = "## tt focus\n"
	start := strings.Index(string(reference), heading)
	if start < 0 {
		t.Fatal("CLI reference is missing tt focus section")
	}
	focusSection := string(reference)[start+len(heading):]
	if end := strings.Index(focusSection, "\n## "); end >= 0 {
		focusSection = focusSection[:end]
	}
	if strings.Contains(focusSection, "--active") {
		t.Fatalf("CLI reference still exposes tt focus --active:\n%s", focusSection)
	}

	const queueHeading = "## tt focus queue\n"
	queueStart := strings.Index(string(reference), queueHeading)
	if queueStart < 0 {
		t.Fatal("CLI reference is missing tt focus queue section")
	}
	queueSection := string(reference)[queueStart+len(queueHeading):]
	if end := strings.Index(queueSection, "\n## "); end >= 0 {
		queueSection = queueSection[:end]
	}
	if !strings.Contains(queueSection, "'q' to toggle Queue membership") ||
		!strings.Contains(queueSection, "dismisses a suggestion without changing existing Queue items") {
		t.Fatalf("CLI reference does not describe explicit Queue controls:\n%s", queueSection)
	}
	if strings.Contains(queueSection, "'x' to remove") {
		t.Fatalf("CLI reference still describes suggestion dismissal as Queue removal:\n%s", queueSection)
	}
}
