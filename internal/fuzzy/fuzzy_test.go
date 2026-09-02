package fuzzy

import (
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
		want    bool
	}{
		// Basic subsequence matching (within density threshold)
		{"exact match", "need", "need", true},
		{"tight subsequence", "implement", "impl", true},
		{"word gap match", "need a way", "need way", true},
		{"partial match", "hello world", "hlo", true},
		{"full subsequence", "abcdef", "ace", true},

		// Scattered matches (rejected by density threshold: span > 3x pattern length)
		{"very scattered rejected", "Some huge old random text", "short", false},
		{"scattered across long text", "start middle content finish end", "smcfe", false},

		// Case insensitivity
		{"case insensitive upper pattern", "need", "NEED", true},
		{"case insensitive upper text", "NEED", "need", true},
		{"mixed case", "NeeD a WaY", "need way", true},

		// Order matters
		{"wrong order", "cab", "abc", false},
		{"partial wrong order", "dcba", "abcd", false},

		// Edge cases
		{"empty pattern", "anything", "", true},
		{"empty text", "", "pattern", false},
		{"both empty", "", "", true},
		{"pattern longer than text", "ab", "abc", false},

		// Real-world examples
		{"task title match", "Implement user authentication", "impl auth", true},
		{"project name match", "my-awesome-project", "awesome", true},
		{"tag match", "backend-api", "bend", true},
		{"description match", "This is a very long description", "long desc", true},

		// Should not match
		{"no common chars", "hello", "xyz", false},
		{"missing char", "hello", "hellox", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(tt.text, tt.pattern) > 0
			if got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.text, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		pattern  string
		wantZero bool // true if score should be 0 (no match)
	}{
		{"exact match scores", "need", "need", false},
		{"tight subsequence scores", "implement", "impl", false},
		{"moderate subsequence allowed", "need a way", "ndway", false}, // span 10, pattern 5, 10 <= 15
		{"scattered subsequence rejected", "start middle content finish end", "smcfe", true},
		{"no match scores zero", "hello", "xyz", true},
		{"empty pattern scores", "text", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(tt.text, tt.pattern)
			if tt.wantZero && got != 0 {
				t.Errorf("Score(%q, %q) = %d, want 0", tt.text, tt.pattern, got)
			}
			if !tt.wantZero && got == 0 {
				t.Errorf("Score(%q, %q) = 0, want > 0", tt.text, tt.pattern)
			}
		})
	}
}

func TestScoreRanking(t *testing.T) {
	// Test that better matches get higher scores
	tests := []struct {
		name        string
		text1       string
		text2       string
		pattern     string
		want1Higher bool // true if text1 should score higher
	}{
		{
			name:        "exact match beats partial",
			text1:       "need",
			text2:       "I need help",
			pattern:     "need",
			want1Higher: true,
		},
		{
			name:        "consecutive beats word-boundary",
			text1:       "hello",
			text2:       "he-llo",
			pattern:     "hello",
			want1Higher: true,
		},
		{
			name:        "word boundary beats middle",
			text1:       "user-auth",
			text2:       "xuseryauth",
			pattern:     "user",
			want1Higher: true,
		},
		{
			name:        "start match beats middle",
			text1:       "authentication",
			text2:       "user authentication",
			pattern:     "auth",
			want1Higher: true,
		},
		{
			name:        "punctuation boundary beats middle",
			text1:       "task:auth",
			text2:       "taskxauth",
			pattern:     "auth",
			want1Higher: true,
		},
		{
			name:        "symbol boundary beats middle",
			text1:       "price$auth",
			text2:       "pricexauth",
			pattern:     "auth",
			want1Higher: true,
		},
		{
			name:        "camelCase boundary beats middle",
			text1:       "fooBar",
			text2:       "fooxbar",
			pattern:     "fb",
			want1Higher: true,
		},
		{
			name:        "prefix beats longer-text interior run",
			text1:       "Analytics",
			text2:       "Katana",
			pattern:     "ana",
			want1Higher: true,
		},
		{
			name:        "prefix beats short interior run",
			text1:       "Analytics",
			text2:       "Xana",
			pattern:     "ana",
			want1Higher: true,
		},
		{
			name:        "exact length beats longer prefix",
			text1:       "auth",
			text2:       "authentication",
			pattern:     "auth",
			want1Higher: true,
		},
		{
			name:        "shorter equal-quality prefix wins",
			text1:       "impl",
			text2:       "implx",
			pattern:     "impl",
			want1Higher: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score1 := Score(tt.text1, tt.pattern)
			score2 := Score(tt.text2, tt.pattern)

			if tt.want1Higher && score1 <= score2 {
				t.Errorf("Expected Score(%q, %q)=%d > Score(%q, %q)=%d",
					tt.text1, tt.pattern, score1, tt.text2, tt.pattern, score2)
			}
			if !tt.want1Higher && score1 >= score2 {
				t.Errorf("Expected Score(%q, %q)=%d < Score(%q, %q)=%d",
					tt.text1, tt.pattern, score1, tt.text2, tt.pattern, score2)
			}
		})
	}
}

func TestScoreTerms(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		query    string
		wantZero bool
	}{
		// All terms must match
		{"single term match", "hello world", "hello", false},
		{"multi term match", "need a way to do this", "need way", false},
		{"all terms present", "implement user auth", "impl user auth", false},

		// Any missing term = no match
		{"one term missing", "hello world", "hello xyz", true},
		{"first term missing", "world", "hello world", true},

		// Edge cases
		{"empty query", "anything", "", false},
		{"whitespace query", "anything", "   ", false},
		{"empty text", "", "pattern", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreTerms(tt.text, tt.query)
			if tt.wantZero && got != 0 {
				t.Errorf("ScoreTerms(%q, %q) = %d, want 0", tt.text, tt.query, got)
			}
			if !tt.wantZero && got == 0 {
				t.Errorf("ScoreTerms(%q, %q) = 0, want > 0", tt.text, tt.query)
			}
		})
	}
}

func TestMatchTerms(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		query string
		want  bool
	}{
		{"word gap", "need a way", "need way", true},
		{"multiple words", "implement the feature", "impl feat", true},
		{"missing word", "hello world", "hello missing", false},
		{"case insensitive terms", "Hello World", "hello world", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreTerms(tt.text, tt.query) > 0
			if got != tt.want {
				t.Errorf("MatchTerms(%q, %q) = %v, want %v", tt.text, tt.query, got, tt.want)
			}
		})
	}
}

func TestMatchDensityThreshold(t *testing.T) {
	// Match density threshold: span must be <= 3x pattern length
	tests := []struct {
		name     string
		text     string
		pattern  string
		wantZero bool
	}{
		// Pattern "impl" (4 chars) - max span = 12
		{"exact span 1x", "impl", "impl", false},                // span 4
		{"span 2x allowed", "improve layout", "impl", false},    // span ~8, within 3x
		{"span 3x allowed", "improve my layout", "impl", false}, // span ~11, within 3x

		// Pattern "short" (5 chars) - max span = 15
		{"scattered beyond 3x", "Some huge old random text", "short", true}, // span way > 15

		// Pattern "abc" (3 chars) - max span = 9
		{"tight abc", "abcdef", "abc", false},         // span 3
		{"moderate abc", "a--b--c", "abc", false},     // span 7, within 3x
		{"scattered abc", "a----b----c", "abc", true}, // span 11, exceeds 3x

		// Real-world patterns
		{"auth in authentication", "authentication", "auth", false},
		{"impl in implementation", "implementation", "impl", false},
		{"scattered shortcut", "Sync with remote server tasks across multiple", "shortcut", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(tt.text, tt.pattern)
			if tt.wantZero && got != 0 {
				t.Errorf("Score(%q, %q) = %d, want 0 (too scattered)", tt.text, tt.pattern, got)
			}
			if !tt.wantZero && got == 0 {
				t.Errorf("Score(%q, %q) = 0, want > 0 (should match)", tt.text, tt.pattern)
			}
		})
	}
}

func TestIsWordBoundary(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{' ', true},
		{'-', true},
		{'_', true},
		{'/', true},
		{'.', true},
		{':', true},
		{',', true},
		{'(', true},
		{')', true},
		{'$', true},
		{'+', true},
		{'a', false},
		{'Z', false},
		{'0', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			got := isWordBoundary(tt.r)
			if got != tt.want {
				t.Errorf("isWordBoundary(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestIsCamelBoundary(t *testing.T) {
	tests := []struct {
		name string
		prev rune
		curr rune
		want bool
	}{
		{"lower to upper", 'o', 'B', true},
		{"lower to lower", 'o', 'b', false},
		{"upper to upper", 'O', 'B', false},
		{"punctuation handled separately", '-', 'B', false},
		{"digit to upper is not camel", '2', 'B', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCamelBoundary(tt.prev, tt.curr)
			if got != tt.want {
				t.Errorf("isCamelBoundary(%q, %q) = %v, want %v", tt.prev, tt.curr, got, tt.want)
			}
		})
	}
}
