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
	}
}

func TestSemanticTaskTagForTagsAppliesPrecedenceRegardlessOfSliceOrder(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want SemanticTaskTagRole
	}{
		{name: "idea then bug", tags: []string{"idea", "bug"}, want: SemanticTaskTagBug},
		{name: "bug then idea", tags: []string{"bug", "idea"}, want: SemanticTaskTagBug},
		{name: "story then epic", tags: []string{"story", "epic"}, want: SemanticTaskTagEpic},
		{name: "epic then story", tags: []string{"epic", "story"}, want: SemanticTaskTagEpic},
		{name: "ascending", tags: []string{"idea", "feature", "story", "epic", "bug"}, want: SemanticTaskTagBug},
		{name: "descending", tags: []string{"bug", "epic", "story", "feature", "idea"}, want: SemanticTaskTagBug},
		{name: "ordinary tags interleaved", tags: []string{"urgent", "feature", "backend", "story"}, want: SemanticTaskTagStory},
		{name: "lowest alone", tags: []string{"chore", "idea"}, want: SemanticTaskTagIdea},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, ok := SemanticTaskTagForTags(tt.tags)
			if !ok {
				t.Fatalf("SemanticTaskTagForTags(%v) reported no semantic tag", tt.tags)
			}
			if entry.Role != tt.want {
				t.Fatalf("role = %v, want %v", entry.Role, tt.want)
			}
		})
	}
}

func TestSemanticTaskTagForTagsIgnoresOrdinaryTags(t *testing.T) {
	for _, tags := range [][]string{
		nil,
		{},
		{"type:bug"},
		{"type:epic"},
		{"bugfix"},
		{"storytelling"},
		{"feature:ui"},
		{" idea "},
		{" epic "},
		{"chore", "spike", "urgent", "blocked", "parked"},
	} {
		entry, ok := SemanticTaskTagForTags(tags)
		if ok {
			t.Errorf("SemanticTaskTagForTags(%v) = %q, want no match", tags, entry.Name)
		}
		if entry.Role != SemanticTaskTagNone {
			t.Errorf("SemanticTaskTagForTags(%v) role = %v, want None", tags, entry.Role)
		}
	}
}

func TestSemanticTaskTagByNameMatchesExactlyAndCaseInsensitively(t *testing.T) {
	matches := map[string]SemanticTaskTagRole{
		"bug":     SemanticTaskTagBug,
		"BUG":     SemanticTaskTagBug,
		"Bug":     SemanticTaskTagBug,
		"EPIC":    SemanticTaskTagEpic,
		"Story":   SemanticTaskTagStory,
		"FeAtUrE": SemanticTaskTagFeature,
		"iDeA":    SemanticTaskTagIdea,
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
