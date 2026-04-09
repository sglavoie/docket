package storage

import (
	"strings"
	"testing"
)

// TestRoundTrip_SectionColorsWithColonInName verifies that section names
// containing colons don't get confused with the color separator.
func TestRoundTrip_SectionColorsWithColonInName(t *testing.T) {
	content := "# Project: Test\n<!-- sections: Normal;With:colon:name -->\n\n## Task: T1\n<!-- id: t1, status: todo, priority: 3 -->\n"

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, s := range result.Sections {
		if s == "With" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected section 'With' (colon truncates name), got sections: %v", result.Sections)
	}
}

// TestParse_SectionsAfterFirstTaskIgnored verifies that a sections metadata
// comment appearing after the first task heading is ignored (treated as
// regular description content).
func TestParse_SectionsAfterFirstTaskIgnored(t *testing.T) {
	content := "# Project: Test\n<!-- sections: Active;Done -->\n\n## Task: First\n<!-- id: t1, status: todo, priority: 3 -->\n\n<!-- sections: Fake;Sections -->\n\n## Task: Second\n<!-- id: t2, status: todo, priority: 3 -->\n"

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d: %v", len(result.Sections), result.Sections)
	}
	if result.Sections[0] != "Active" || result.Sections[1] != "Done" {
		t.Errorf("sections = %v, want [Active, Done]", result.Sections)
	}

	if !strings.Contains(result.Tasks[0].Description, "<!-- sections: Fake;Sections -->") {
		t.Error("second sections comment should be part of first task's description")
	}
}
