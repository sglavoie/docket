package fuzzy

import (
	"math/rand"
	"strings"
	"testing"
)

func TestCompiledTermsPreserveScores(t *testing.T) {
	texts := []string{"", "ReleaseNotes review", "Über die Brücke", "İstanbul Straße Σοφία", "確認 é 👩‍💻", strings.Repeat("ab ", 150)}
	queries := []string{"", "  ", "release review", "rn", "übr", "i straße", "確認", "é", "ab", strings.Repeat("ab", 40)}
	random := rand.New(rand.NewSource(42))
	for range 100 {
		var text strings.Builder
		for range 80 {
			text.WriteRune([]rune("abc ABC_é界 ")[random.Intn(11)])
		}
		texts = append(texts, text.String())
		queries = append(queries, string([]rune(text.String())[10:15]))
	}
	for _, sensitive := range []bool{false, true} {
		matcher := NewMatcher(sensitive)
		for _, query := range queries {
			compiled := matcher.CompileTerms(query)
			for _, text := range texts {
				// The positions path is an independent public scorer and also
				// exercises the long-pattern heap fallback and Unicode boundaries.
				want, _ := matcher.ScoreTermsWithPositions(text, query)
				if got := compiled.Score(text); got != want {
					t.Fatalf("case=%t text=%q query=%q score=%d want=%d", sensitive, text, query, got, want)
				}
			}
		}
	}
}
