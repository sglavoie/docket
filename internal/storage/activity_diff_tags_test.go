package storage

import "testing"

// TestDiffTagsDeduplicatesEntries verifies that diffTags collapses duplicate
// added/removed entries when either input slice contains the same tag more
// than once. Without dedup, a task whose stored Tags slice has been corrupted
// with duplicates would produce repeated "+tag +tag" markers in the activity
// log, polluting the diff output.
func TestDiffTagsDeduplicatesEntries(t *testing.T) {
	cases := []struct {
		name     string
		oldTags  []string
		newTags  []string
		expected string
	}{
		{
			name:     "duplicate added tag collapses",
			oldTags:  []string{"a"},
			newTags:  []string{"a", "b", "b"},
			expected: "+b",
		},
		{
			name:     "duplicate removed tag collapses",
			oldTags:  []string{"a", "b", "b"},
			newTags:  []string{"a"},
			expected: "-b",
		},
		{
			name:     "duplicates on both sides cancel and dedup the rest",
			oldTags:  []string{"a", "a", "c"},
			newTags:  []string{"a", "a", "d", "d"},
			expected: "+d -c",
		},
		{
			name:     "no changes returns empty string",
			oldTags:  []string{"a", "b"},
			newTags:  []string{"a", "b"},
			expected: "",
		},
		{
			name:     "added entries are sorted alphabetically",
			oldTags:  nil,
			newTags:  []string{"zeta", "alpha", "mu"},
			expected: "+alpha +mu +zeta",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := diffTags(tc.oldTags, tc.newTags)
			if got != tc.expected {
				t.Errorf("diffTags(%v, %v) = %q, want %q", tc.oldTags, tc.newTags, got, tc.expected)
			}
		})
	}
}
