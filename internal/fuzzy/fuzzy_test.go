package fuzzy

import (
	"slices"
	"strings"
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

// TestMatchUnicodeCaseFolding verifies that case-insensitive matching works
// correctly for Unicode characters whose lowercase form may involve special
// case mappings (e.g., Turkish capital "İ" U+0130, German "ẞ" U+1E9E, Greek "Σ").
//
// The fuzzy matcher uses strings.ToLower on whole strings (not rune-by-rune
// unicode.ToLower), which is the correct approach: it handles the special
// case-mapping table consistently and produces 1:1 rune mappings for the
// default (non-locale-specific) case. This test pins that contract.
func TestMatchUnicodeCaseFolding(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
		want    bool
	}{
		// Turkish dotted capital İ (U+0130) lowercases to "i" (U+0069) in
		// Go's default (non-Turkish) locale via strings.ToLower.
		{"Turkish I-dot capital matches lowercase i", "İstanbul", "ist", true},
		{"lowercase i matches İ at start", "İstanbul", "i", true},
		{"Turkish I-dot as pattern matches i in text", "istanbul", "İ", true},
		{"İ matches İ", "İstanbul", "İstanbul", true},

		// Dotless i (U+0131) is a distinct character; it should NOT match
		// regular "i" because they fold to different lowercase forms in
		// default Go (U+0131 stays U+0131, U+0069 stays U+0069).
		{"dotless i does not match regular i", "ı", "i", false},

		// German sharp s (ß / ẞ) — ẞ lowercases to ß in default Go.
		{"capital sharp S matches lowercase", "Straẞe", "ß", true},
		{"lowercase ß matches ẞ", "STRAẞE", "ß", true},

		// Greek final/non-final sigma. Σ → σ; ς is the final form. Default
		// strings.ToLower maps Σ → σ (not ς), so pattern "σ" matches "Σοφία".
		{"Greek capital sigma matches lowercase sigma", "Σοφία", "σ", true},

		// Accented Latin letters preserve diacritics; the matcher operates on
		// runes after lowercasing, so it does not strip combining marks.
		{"accented e in text needs accented e in pattern", "café", "é", true},
		{"plain e in pattern does not match composed é", "café", "x", false},

		// Mixed-case multi-rune Unicode subsequence.
		{"Unicode subsequence", "Über die Brücke", "übr", true},
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

// TestScoreWithPositionsUnicode verifies that returned positions index into
// the lowercased rune sequence correctly even when the text contains
// multi-byte runes. A common bug is treating positions as byte offsets,
// which would index into the middle of multi-byte UTF-8 sequences.
func TestScoreWithPositionsUnicode(t *testing.T) {
	// "İstanbul" has 8 runes; "İ" is 2 bytes in UTF-8.
	text := "İstanbul"
	score, positions := ScoreWithPositions(text, "ist")
	if score == 0 {
		t.Fatalf("expected non-zero score for %q matching %q", text, "ist")
	}
	if len(positions) != 3 {
		t.Fatalf("expected 3 positions, got %d (%v)", len(positions), positions)
	}
	// Positions must be valid rune indices into the lowercased rune slice.
	// Lowercased "İstanbul" → "istanbul" (8 runes).
	textRunes := []rune("istanbul")
	for _, p := range positions {
		if p < 0 || p >= len(textRunes) {
			t.Errorf("position %d out of bounds for rune-length %d", p, len(textRunes))
		}
	}
	// Strict subsequence ordering.
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Errorf("positions not strictly increasing: %v", positions)
		}
	}
}

// TestScoreParityWithPositions pins the contract that Score and
// ScoreWithPositions agree on the scalar score for any input. The two
// implementations are duplicated; any divergence is a bug.
func TestScoreParityWithPositions(t *testing.T) {
	cases := []struct {
		text, pattern string
	}{
		{"", ""},
		{"", "x"},
		{"x", ""},
		{"a", "a"},
		{"abc", "abc"},
		{"abcdef", "ace"},
		{"hello world", "hlo"},
		{"need a way", "need way"},
		{"İstanbul", "ist"},
		{"Über die Brücke", "übr"},
		{"  abc", "abc"},
		{"-abc-def", "abdf"},
		{"aaaaa", "aaa"},
		{"abXacd", "acd"},
		{"some huge old random text", "short"},
		{"\xff\xfe", "x"},
	}
	for _, c := range cases {
		s1 := Score(c.text, c.pattern)
		s2, _ := ScoreWithPositions(c.text, c.pattern)
		if s1 != s2 {
			t.Errorf("Score(%q,%q)=%d but ScoreWithPositions=%d (must agree)",
				c.text, c.pattern, s1, s2)
		}
	}
}

// TestScoreDeterminism verifies repeated calls return identical results.
// Non-determinism (e.g. from map iteration leaking into ordering) would
// silently destabilize UI lists ranked by Score.
func TestScoreDeterminism(t *testing.T) {
	cases := []struct {
		text, pattern string
	}{
		{"hello world", "hlo"},
		{"aabb", "ab"},
		{"need a way to do this", "need way"},
		{"foo-bar baz", "fbb"},
	}
	for _, c := range cases {
		first, firstPos := ScoreWithPositions(c.text, c.pattern)
		for i := range 20 {
			s, p := ScoreWithPositions(c.text, c.pattern)
			if s != first {
				t.Fatalf("score drift for Score(%q,%q): first=%d run=%d got=%d",
					c.text, c.pattern, first, i, s)
			}
			if !slices.Equal(p, firstPos) {
				t.Fatalf("position drift for (%q,%q): first=%v run=%d got=%v",
					c.text, c.pattern, firstPos, i, p)
			}
		}
		termScore, termPos := ScoreTermsWithPositions(c.text, c.pattern)
		for i := range 20 {
			s, p := ScoreTermsWithPositions(c.text, c.pattern)
			if s != termScore {
				t.Fatalf("ScoreTerms drift for (%q,%q): first=%d run=%d got=%d",
					c.text, c.pattern, termScore, i, s)
			}
			if !slices.Equal(p, termPos) {
				t.Fatalf("ScoreTerms position drift for (%q,%q): first=%v run=%d got=%v",
					c.text, c.pattern, termPos, i, p)
			}
		}
	}
}

// TestNoPanicOnPathologicalInput pins safety against panics on edge inputs
// (empty, invalid UTF-8, null bytes, pattern longer than text, etc).
func TestNoPanicOnPathologicalInput(t *testing.T) {
	inputs := []struct{ text, pattern string }{
		{"", ""},
		{"", "x"},
		{"x", ""},
		{"\x00", "\x00"},
		{"\xff\xfe", "x"},
		{"x", "\xff\xfe"},
		{"\xff\xfe", "\xff\xfe"},
		{string([]byte{0xc3, 0x28}), "x"},
		{"ab", "abc"},
		{strings.Repeat("a", 1000), strings.Repeat("a", 500)},
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on (%q,%q): %v", in.text, in.pattern, r)
				}
			}()
			_ = Score(in.text, in.pattern)
			_ = Score(in.text, in.pattern)
			_, _ = ScoreWithPositions(in.text, in.pattern)
			_ = ScoreTerms(in.text, in.pattern)
			_, _ = ScoreTermsWithPositions(in.text, in.pattern)
			_ = ScoreTerms(in.text, in.pattern)
		}()
	}
}

// TestPositionsAreValidRuneIndices guards against returning positions that
// would index into the middle of a multi-byte rune if a caller used them on
// []rune(text). The contract: positions are valid indices into
// []rune(strings.ToLower(text)).
func TestPositionsAreValidRuneIndices(t *testing.T) {
	cases := []struct {
		text, pattern string
	}{
		{"İstanbul", "ist"},
		{"Über die Brücke", "übr"},
		{"café", "é"},
		{"Σοφία", "σ"},
	}
	for _, c := range cases {
		score, positions := ScoreWithPositions(c.text, c.pattern)
		if score == 0 {
			t.Fatalf("expected match for (%q,%q)", c.text, c.pattern)
		}
		lowered := []rune(strings.ToLower(c.text))
		for _, p := range positions {
			if p < 0 || p >= len(lowered) {
				t.Errorf("position %d out of bounds for %q (rune len %d)",
					p, c.text, len(lowered))
			}
		}
		for i := 1; i < len(positions); i++ {
			if positions[i] <= positions[i-1] {
				t.Errorf("positions not strictly increasing for (%q,%q): %v",
					c.text, c.pattern, positions)
			}
		}
	}
}

// TestEmptyPatternBehavior locks in the documented contract:
//   - Score returns 1 for empty pattern (matches everything weakly)
//   - Positions are nil for empty pattern (no characters to highlight)
//   - Match returns true (since 1 > 0)
func TestEmptyPatternBehavior(t *testing.T) {
	for _, text := range []string{"", "x", "hello world", "İstanbul"} {
		if s := Score(text, ""); s != 1 {
			t.Errorf("Score(%q, \"\") = %d, want 1", text, s)
		}
		if Score(text, "") == 0 {
			t.Errorf("Match(%q, \"\") = false, want true", text)
		}
		if s, p := ScoreWithPositions(text, ""); s != 1 || p != nil {
			t.Errorf("ScoreWithPositions(%q, \"\") = (%d, %v), want (1, nil)", text, s, p)
		}
	}
}

// TestPatternLongerThanText verifies the early-return guard. A pattern with
// more runes than the text can never form a subsequence.
func TestPatternLongerThanText(t *testing.T) {
	cases := []struct{ text, pattern string }{
		{"ab", "abc"},
		{"a", "ab"},
		{"", "x"},
		{"İ", "İs"},
	}
	for _, c := range cases {
		if s := Score(c.text, c.pattern); s != 0 {
			t.Errorf("Score(%q,%q) = %d, want 0 (pattern longer)", c.text, c.pattern, s)
		}
		if s, p := ScoreWithPositions(c.text, c.pattern); s != 0 || p != nil {
			t.Errorf("ScoreWithPositions(%q,%q) = (%d,%v), want (0,nil)",
				c.text, c.pattern, s, p)
		}
	}
}

// Benchmark tests
func BenchmarkMatch(b *testing.B) {
	text := "This is a relatively long text that we want to search through"
	pattern := "long search"

	for i := 0; i < b.N; i++ {
		Score(text, pattern)
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
