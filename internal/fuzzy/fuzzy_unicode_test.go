package fuzzy

import (
	"slices"
	"strings"
	"testing"
)

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
	score, positions := NewMatcher(false).ScoreWithPositions(text, "ist")
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
		s2, _ := NewMatcher(false).ScoreWithPositions(c.text, c.pattern)
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
		first, firstPos := NewMatcher(false).ScoreWithPositions(c.text, c.pattern)
		for i := range 20 {
			s, p := NewMatcher(false).ScoreWithPositions(c.text, c.pattern)
			if s != first {
				t.Fatalf("score drift for Score(%q,%q): first=%d run=%d got=%d",
					c.text, c.pattern, first, i, s)
			}
			if !slices.Equal(p, firstPos) {
				t.Fatalf("position drift for (%q,%q): first=%v run=%d got=%v",
					c.text, c.pattern, firstPos, i, p)
			}
		}
		termScore, termPos := NewMatcher(false).ScoreTermsWithPositions(c.text, c.pattern)
		for i := range 20 {
			s, p := NewMatcher(false).ScoreTermsWithPositions(c.text, c.pattern)
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
			_, _ = NewMatcher(false).ScoreWithPositions(in.text, in.pattern)
			_ = ScoreTerms(in.text, in.pattern)
			_, _ = NewMatcher(false).ScoreTermsWithPositions(in.text, in.pattern)
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
		score, positions := NewMatcher(false).ScoreWithPositions(c.text, c.pattern)
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
		if s, p := NewMatcher(false).ScoreWithPositions(text, ""); s != 1 || p != nil {
			t.Errorf("NewMatcher(false).ScoreWithPositions(%q, \"\") = (%d, %v), want (1, nil)", text, s, p)
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
		if s, p := NewMatcher(false).ScoreWithPositions(c.text, c.pattern); s != 0 || p != nil {
			t.Errorf("NewMatcher(false).ScoreWithPositions(%q,%q) = (%d,%v), want (0,nil)",
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
