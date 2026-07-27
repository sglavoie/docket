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
	// The queue section must describe the state-driven Outliner/Next model:
	// membership is StateNext and the persisted list is ordering-only.
	for _, want := range []string{"StateNext", "ordering-only", "Outliner", "press Tab", "Press 'n'", "'t'"} {
		if !strings.Contains(queueSection, want) {
			t.Fatalf("CLI reference queue section missing %q:\n%s", want, queueSection)
		}
	}
	// Retired TUI navigation and obsolete state-independent Queue vocabulary
	// must not resurface in the generated reference.
	for _, stale := range []string{
		"'q' to toggle Queue membership",
		"dismisses a suggestion without changing existing Queue items",
		"focus view",
		"'x' to remove",
		"Planner",
		"';q'",
		"';w'",
	} {
		if strings.Contains(queueSection, stale) {
			t.Fatalf("CLI reference queue section still contains obsolete wording %q:\n%s", stale, queueSection)
		}
	}

	const todayHeading = "## tt focus today\n"
	todayStart := strings.Index(string(reference), todayHeading)
	if todayStart < 0 {
		t.Fatal("CLI reference is missing tt focus today section")
	}
	todaySection := string(reference)[todayStart+len(todayHeading):]
	if end := strings.Index(todaySection, "\n## "); end >= 0 {
		todaySection = todaySection[:end]
	}
	for _, want := range []string{"synthetic Today group", "All mode", "Press 't'"} {
		if !strings.Contains(todaySection, want) {
			t.Fatalf("CLI reference today section missing %q:\n%s", want, todaySection)
		}
	}
	if strings.Contains(strings.ToLower(todaySection), "cockpit") {
		t.Fatalf("CLI reference today section still references the retired cockpit:\n%s", todaySection)
	}
}
