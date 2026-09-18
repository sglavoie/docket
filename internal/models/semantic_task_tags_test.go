package models

import (
	"strings"
	"testing"
)

func TestSemanticTaskTagsStayInPrecedenceOrder(t *testing.T) {
	want := []struct {
		role SemanticTaskTagRole
		name string
	}{
		{SemanticTaskTagBug, "bug"},
		{SemanticTaskTagQuestion, "question"},
		{SemanticTaskTagEpic, "epic"},
		{SemanticTaskTagStory, "story"},
		{SemanticTaskTagFeature, "feature"},
		{SemanticTaskTagIdea, "idea"},
	}
	if len(SemanticTaskTags) != len(want) {
		t.Fatalf("registry has %d entries, want %d", len(SemanticTaskTags), len(want))
	}
	for i, entry := range SemanticTaskTags {
		if entry.Role != want[i].role || entry.Name != want[i].name {
			t.Fatalf("entry %d = %v/%q, want %v/%q", i, entry.Role, entry.Name, want[i].role, want[i].name)
		}
		if strings.TrimSpace(entry.Summary) == "" {
			t.Fatalf("entry %d (%q) has no summary", i, entry.Name)
		}
		if strings.TrimSpace(entry.RowSummary) == "" {
			t.Fatalf("entry %d (%q) has no row summary", i, entry.Name)
		}
	}
	wantRows := map[string]string{
		"bug":      "A defect to fix. Highest precedence.",
		"question": "An open question to investigate.",
		"epic":     "A large body of related work.",
		"story":    "A user-facing slice of work.",
		"feature":  "New capability to build.",
		"idea":     "An unrefined thought. Lowest precedence.",
	}
	for _, entry := range SemanticTaskTags {
		if got := entry.RowSummary; got != wantRows[entry.Name] {
			t.Fatalf("%s row summary = %q, want %q", entry.Name, got, wantRows[entry.Name])
		}
	}
}

// TestSemanticTaskTagsResolveOneAtATime holds the registry to per-tag lookup.
// Individual tag lookup is independent of the precedence used to resolve a
// task's workflow-state icon accent.
func TestSemanticTaskTagsResolveOneAtATime(t *testing.T) {
	for _, tags := range [][]string{
		{"idea", "bug"},
		{"bug", "idea"},
		{"urgent", "feature", "backend", "story"},
	} {
		for _, tag := range tags {
			entry, ok := SemanticTaskTagByName(tag)
			if _, semantic := map[string]struct{}{
				"bug": {}, "question": {}, "epic": {}, "story": {}, "feature": {}, "idea": {},
			}[tag]; semantic != ok {
				t.Fatalf("SemanticTaskTagByName(%q) ok = %v, want %v", tag, ok, semantic)
			}
			if ok && entry.Name != tag {
				t.Fatalf("SemanticTaskTagByName(%q) = %q", tag, entry.Name)
			}
		}
	}
}

func TestSemanticTaskTagByNameMatchesExactlyAndCaseInsensitively(t *testing.T) {
	matches := map[string]SemanticTaskTagRole{
		"bug":      SemanticTaskTagBug,
		"BUG":      SemanticTaskTagBug,
		"Bug":      SemanticTaskTagBug,
		"question": SemanticTaskTagQuestion,
		"QuEsTiOn": SemanticTaskTagQuestion,
		"EPIC":     SemanticTaskTagEpic,
		"Story":    SemanticTaskTagStory,
		"FeAtUrE":  SemanticTaskTagFeature,
		"iDeA":     SemanticTaskTagIdea,
	}
	for tag, want := range matches {
		entry, ok := SemanticTaskTagByName(tag)
		if !ok {
			t.Fatalf("SemanticTaskTagByName(%q) reported no match", tag)
		}
		if entry.Role != want {
			t.Fatalf("SemanticTaskTagByName(%q) role = %v, want %v", tag, entry.Role, want)
		}
	}

	for _, tag := range []string{"", "type:bug", "Type:Bug", "bugfix", "feature:ui", " idea ", "#bug", "bugs"} {
		if entry, ok := SemanticTaskTagByName(tag); ok {
			t.Errorf("SemanticTaskTagByName(%q) = %q, want no match", tag, entry.Name)
		}
	}
}
