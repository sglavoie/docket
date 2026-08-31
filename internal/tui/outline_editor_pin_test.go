package tui

import (
	"strings"
	"testing"
	"time"
)

func TestOutlineDumpEmitsThePinSigilAfterTheDestination(t *testing.T) {
	project := outlineTestProject(t)
	project.Tasks[0].Pinned = true
	pinnedAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	project.Tasks[0].PinnedAt = &pinnedAt

	buffer, _, err := dumpOutlineBuffer(project, nil, "All", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	want := "Ship auth rewrite $doing *pin  [[tt:4f2a9c11]]"
	if !strings.Contains(buffer, want+"\n") {
		t.Fatalf("dump missing %q:\n%s", want, buffer)
	}
	if strings.Contains(buffer, "Sweep dead flags $inbox *pin") {
		t.Errorf("dump pinned an unpinned task:\n%s", buffer)
	}
}

func TestOutlineRoundTripCarriesPinnedOnAndOff(t *testing.T) {
	project := outlineTestProject(t)
	project.Tasks[0].Pinned = true

	buffer, _, err := dumpOutlineBuffer(project, nil, "All", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	lines, problems := parseOutlineBuffer(buffer, project, nil)
	if len(problems) > 0 {
		t.Fatalf("unedited buffer was rejected: %v", problems)
	}
	if !lines[0].Pinned {
		t.Error("round trip lost the pin")
	}
	for i, line := range lines[1:] {
		if line.Pinned {
			t.Errorf("line %d gained a pin nobody declared", i+1)
		}
	}

	// The sigil legend in the header also spells *pin, so the edit targets the
	// task line by its destination rather than the first match in the buffer.
	unpinned := strings.Replace(buffer, "$doing *pin ", "$doing ", 1)
	if unpinned == buffer {
		t.Fatal("buffer carried no pin segment to remove")
	}
	lines, problems = parseOutlineBuffer(unpinned, project, nil)
	if len(problems) > 0 {
		t.Fatalf("edited buffer was rejected: %v", problems)
	}
	if lines[0].Pinned {
		t.Error("removing *pin left the line pinned")
	}
}

func TestOutlineParseAcceptsAnAddedPinSigil(t *testing.T) {
	project := outlineTestProject(t)
	buffer := "Sweep dead flags $inbox *pin  [[tt:71f0b3d5]]"

	lines, problems := parseOutlineBuffer(buffer, project, nil)
	if len(problems) > 0 {
		t.Fatalf("buffer was rejected: %v", problems)
	}
	if len(lines) != 1 || !lines[0].Pinned {
		t.Fatalf("lines = %+v, want one pinned line", lines)
	}
	if lines[0].Title != "Sweep dead flags" {
		t.Errorf("title = %q, want the sigil consumed", lines[0].Title)
	}
}

// TestOutlineParseRefusesUnpin keeps the buffer declarative: omission is the
// only clear, so the edit surface's *unpin must be named as an error here
// rather than quietly folded into the title.
func TestOutlineParseRefusesUnpin(t *testing.T) {
	project := outlineTestProject(t)
	buffer := "Sweep dead flags $inbox *unpin  [[tt:71f0b3d5]]"

	lines, problems := parseOutlineBuffer(buffer, project, nil)
	if len(lines) != 0 {
		t.Fatalf("lines = %+v, want the line refused", lines)
	}
	if !outlineProblemsContain(problems, 1, "omit *pin to unpin here") {
		t.Fatalf("problems = %v, want the outline unpin refusal", problems)
	}
}
