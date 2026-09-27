package util

import "testing"

func TestCountPluralAgreesAtOne(t *testing.T) {
	if got := CountPlural(1, "match", "matches"); got != "1 match" {
		t.Fatalf("CountPlural(1) = %q", got)
	}
	if got := CountPlural(3, "match", "matches"); got != "3 matches" {
		t.Fatalf("CountPlural(3) = %q", got)
	}
	if got := Count(1, "filter"); got != "1 filter" {
		t.Fatalf("Count(1) = %q", got)
	}
}
