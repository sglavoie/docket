package storage

import (
	"strings"
	"testing"
)

// TestParse_SectionMetadataFiltersNonGTDSpecs verifies that stale custom
// section names, including names with the legacy color separator, are ignored.
func TestParse_SectionMetadataFiltersNonGTDSpecs(t *testing.T) {
	content := "# Project: Test\n<!-- sections: Next;With:colon:name;Inbox -->\n\n## Task: T1\n<!-- id: t1, status: todo, priority: 3 -->\n"

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Sections) != 2 {
		t.Fatalf("expected 2 GTD sections, got %d: %v", len(result.Sections), result.Sections)
	}
	if result.Sections[0] != "Next" || result.Sections[1] != "Inbox" {
		t.Errorf("sections = %v, want [Next, Inbox]", result.Sections)
	}
}

// TestParse_SectionsAfterFirstTaskIgnored verifies that a sections metadata
// comment appearing after the first task heading is ignored (treated as
// regular description content).
func TestParse_SectionsAfterFirstTaskIgnored(t *testing.T) {
	content := "# Project: Test\n<!-- sections: Next;Inbox -->\n\n## Task: First\n<!-- id: t1, status: todo, priority: 3 -->\n\n<!-- sections: Fake;Sections -->\n\n## Task: Second\n<!-- id: t2, status: todo, priority: 3 -->\n"

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d: %v", len(result.Sections), result.Sections)
	}
	if result.Sections[0] != "Next" || result.Sections[1] != "Inbox" {
		t.Errorf("sections = %v, want [Next, Inbox]", result.Sections)
	}

	if !strings.Contains(result.Tasks[0].Description, "<!-- sections: Fake;Sections -->") {
		t.Error("second sections comment should be part of first task's description")
	}
}
