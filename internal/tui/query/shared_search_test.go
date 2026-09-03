package query

import (
	"slices"
	"testing"
)

func TestSearchSubstringPositionsAreCaseInsensitiveAndContiguous(t *testing.T) {
	got := SearchSubstringPositions("Ship OAuth Login", "OAUTH", false)
	want := []int{5, 6, 7, 8, 9}
	if len(got) != len(want) {
		t.Fatalf("positions = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("positions = %v, want %v", got, want)
		}
	}
	if got := SearchSubstringPositions("Ship OAuth Login", "oa log", false); len(got) != 0 {
		t.Fatalf("non-contiguous query positions = %v, want none", got)
	}
}

func TestSearchSubstringPositionsHonorCaseSensitivity(t *testing.T) {
	if got := SearchSubstringPositions("Ship OAuth Login", "OAUTH", true); len(got) != 0 {
		t.Fatalf("case-sensitive positions = %v, want none", got)
	}
	got := SearchSubstringPositions("Ship OAuth Login", "OAuth", true)
	want := []int{5, 6, 7, 8, 9}
	if !slices.Equal(got, want) {
		t.Fatalf("case-sensitive positions = %v, want %v", got, want)
	}
}
