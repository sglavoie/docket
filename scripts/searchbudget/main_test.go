package main

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
)

func sampleLog(scaleBytes, scaleTime float64, count int) string {
	var log strings.Builder
	for name, limit := range budgets {
		for range count {
			fmt.Fprintf(&log, "%s-8 1 100 ns/op %.0f ns/key %.0f p95-ns/key %.0f max-ns/key %.0f escape-ns %.0f B/op 100 allocs/op\n", name, limit.nsPerKey*scaleTime, limit.p95*scaleTime, limit.maximum*scaleTime, limit.escape*scaleTime, limit.bytesPerCycle*scaleBytes)
		}
	}
	return log.String()
}

func TestBudgetRejectsRegressionsAndIncompleteEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, log     string
		strict, fails bool
	}{
		{"within budget", sampleLog(.8, .8, 3), true, false},
		{"allocation regression", sampleLog(1.2, .8, 3), false, true},
		{"timing report", sampleLog(.8, 1.2, 3), false, false},
		{"timing regression", sampleLog(.8, 1.2, 3), true, true},
		{"missing sample", sampleLog(.8, .8, 2), false, true},
		{"empty output", "PASS\n", false, true},
		{"missing metric", strings.ReplaceAll(sampleLog(.8, .8, 3), "ns/key", "ns/op"), false, true},
		{"invalid number", strings.ReplaceAll(sampleLog(.8, .8, 3), "100 ns/op", "NaN ns/op"), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := check(strings.NewReader(tc.log), io.Discard, tc.strict); (err != nil) != tc.fails {
				t.Fatalf("check error = %v, want failure=%t", err, tc.fails)
			}
		})
	}
}

func TestTailAndEscapeRegressionsCannotHideBehindAFastAverage(t *testing.T) {
	for _, metric := range []string{"p95-ns/key", "max-ns/key", "escape-ns"} {
		t.Run(metric, func(t *testing.T) {
			pattern := regexp.MustCompile(`[0-9]+ ` + regexp.QuoteMeta(metric))
			log := pattern.ReplaceAllString(sampleLog(.8, .8, 3), "9000000000 "+metric)
			if err := check(strings.NewReader(log), io.Discard, true); err == nil {
				t.Fatal("a fast average concealed a slow " + metric)
			}
			if err := check(strings.NewReader(log), io.Discard, false); err != nil {
				t.Fatalf("ordinary CI must report timing without depending on CPU share: %v", err)
			}
			missing := pattern.ReplaceAllString(sampleLog(.8, .8, 3), "")
			if err := check(strings.NewReader(missing), io.Discard, false); err == nil {
				t.Fatal("missing tail evidence was accepted")
			}
		})
	}
}
