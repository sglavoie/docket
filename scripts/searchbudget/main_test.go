package main

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func sampleLog(scaleBytes, scaleTime float64, count int) string {
	var log strings.Builder
	for name, limit := range budgets {
		for range count {
			fmt.Fprintf(&log, "%s-8 1 100 ns/op %.0f ns/key %.0f B/op 100 allocs/op\n", name, limit.nsPerKey*scaleTime, limit.bytesPerCycle*scaleBytes)
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
