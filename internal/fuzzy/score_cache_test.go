package fuzzy

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestScoreCacheParityEvictionAndConcurrentReaders(t *testing.T) {
	cache := NewScoreCache(12 << 10)
	texts := []string{
		strings.Repeat("Résumé mixedCase 確認 notes. ", 20),
		strings.Repeat("ordinary prose ", 400) + "the Handoff",
		strings.Repeat("界", 4096) + "é",
		strings.Repeat("x", 128) + "İstanbul Σοφία Kelvin",
	}
	var wg sync.WaitGroup
	for _, sensitive := range []bool{false, true} {
		matcher := NewMatcher(sensitive)
		for _, query := range []string{"rés", "確認 notes", "mc", "Handoff", "zzzz", "é", "i σ", "kelvin", strings.Repeat("x", 65)} {
			for _, text := range texts {
				want, _ := matcher.ScoreTermsWithPositions(text, query)
				wg.Go(func() {
					for range 3 {
						if got := matcher.CompileTerms(query).WithCache(cache).Score(text); got != want {
							t.Errorf("case=%v query=%q got %d want %d", sensitive, query, got, want)
						}
					}
				})
			}
		}
	}
	wg.Wait()
	if cache.used > cache.budget || cache.used == 0 {
		t.Fatalf("cache accounting: %d / %d", cache.used, cache.budget)
	}
}

func TestScoreCacheQueryRingAndContentIdentity(t *testing.T) {
	cache := NewScoreCache(1 << 20)
	text := strings.Repeat("boring text ", 30) + "release"
	for i := range 40 {
		query := fmt.Sprintf("probe %d", i)
		if got := NewMatcher(false).CompileTerms(query).WithCache(cache).Score(text); got != 0 {
			t.Fatalf("unexpected probe match: %d", got)
		}
	}
	terms := NewMatcher(false).CompileTerms("release").WithCache(cache)
	if terms.Score(text) == 0 || terms.Score(strings.ReplaceAll(text, "release", "removed")) != 0 || terms.Score(text) == 0 {
		t.Fatal("content edit or restoration reused a stale score")
	}
	// A miss is not a safe incremental candidate boundary: longer patterns
	// increase the density window. Both outcomes must match uncached scoring.
	text = strings.Repeat("x", 140) + "a____bc"
	for _, query := range []string{"ab", "abc", "ab", "abc"} {
		matcher := NewMatcher(false)
		if got, want := matcher.CompileTerms(query).WithCache(cache).Score(text), matcher.ScoreTerms(text, query); got != want {
			t.Fatalf("density query %q: %d != %d", query, got, want)
		}
	}
	if got := NewMatcher(false).CompileTerms("release").WithCache(NewScoreCache(0)).Score(text); got != 0 {
		t.Fatal("zero-budget fallback changed the result")
	}
}
