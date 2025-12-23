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
		// Basic subsequence matching
		{"exact match", "need", "need", true},
		{"subsequence match", "need a way", "ndway", true},
		{"word gap match", "need a way", "need way", true},
		{"partial match", "hello world", "hlo", true},
		{"full subsequence", "abcdef", "ace", true},

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
			got := Match(tt.text, tt.pattern)
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
		{"subsequence scores", "need a way", "ndway", false},
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
			name:        "consecutive beats scattered",
			text1:       "hello",
			text2:       "h e l l o",
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
			got := MatchTerms(tt.text, tt.query)
			if got != tt.want {
				t.Errorf("MatchTerms(%q, %q) = %v, want %v", tt.text, tt.query, got, tt.want)
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

// Benchmark tests
func BenchmarkMatch(b *testing.B) {
	text := "This is a relatively long text that we want to search through"
	pattern := "long search"

	for i := 0; i < b.N; i++ {
		Match(text, pattern)
	}
}

func BenchmarkScore(b *testing.B) {
	text := "This is a relatively long text that we want to search through"
	pattern := "long search"

	for i := 0; i < b.N; i++ {
		Score(text, pattern)
	}
}

func BenchmarkScoreTerms(b *testing.B) {
	text := "This is a relatively long text that we want to search through"
	query := "long text search"

	for i := 0; i < b.N; i++ {
		ScoreTerms(text, query)
	}
}
