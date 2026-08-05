package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeProjectTagsFoldsDedupesAndSorts(t *testing.T) {
	got, err := NormalizeProjectTags([]string{"Zeta", "#Client-ACME", "client-acme", "billable", "CLIENT-ACME", " Zeta "})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"billable", "client-acme", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
}

func TestNormalizeProjectTagsIsOrderIndependent(t *testing.T) {
	first, err := NormalizeProjectTags([]string{"beta", "Alpha", "gamma"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NormalizeProjectTags([]string{"GAMMA", "alpha", "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same set in different input orders produced %v and %v", first, second)
	}
}

func TestNormalizeProjectTagsRejectsInvalidNames(t *testing.T) {
	tests := []struct {
		name string
		tag  string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"hash only", "#"},
		{"contains comma", "a,b"},
		{"contains semicolon", "a;b"},
		{"contains newline", "a\nb"},
		{"too long", strings.Repeat("a", MaxTagLength+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeProjectTags([]string{"billable", tt.tag})
			if err == nil {
				t.Fatalf("NormalizeProjectTags(%q) = %v, want error", tt.tag, got)
			}
			if got != nil {
				t.Fatalf("rejected set returned %v, want nil", got)
			}
		})
	}
}

func TestNormalizeProjectTagsAllowsEmptySet(t *testing.T) {
	for _, input := range [][]string{nil, {}} {
		got, err := NormalizeProjectTags(input)
		if err != nil {
			t.Fatalf("NormalizeProjectTags(%v) error = %v", input, err)
		}
		if len(got) != 0 {
			t.Fatalf("NormalizeProjectTags(%v) = %v, want empty", input, got)
		}
	}
}

func TestNormalizeProjectTagsCanonicalizesReservedParkedTag(t *testing.T) {
	got, err := NormalizeProjectTags([]string{"PARKED", "Parked", "#parked"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{ParkedProjectTag}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
	if ParkedProjectTag != strings.ToLower(ParkedProjectTag) {
		t.Fatalf("ParkedProjectTag = %q, want a lowercase canonical spelling", ParkedProjectTag)
	}
}

// Project tags fold and sort; task tags must keep their case and insertion
// order, since the two namespaces are separate.
func TestNormalizeProjectTagsLeavesTaskTagRulesUnchanged(t *testing.T) {
	got, err := NormalizeTaskTags([]string{"Zeta", "Client-ACME", "client-acme", "Parked"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Zeta", "Client-ACME", "client-acme", "Parked"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("task tags = %v, want %v", got, want)
	}
}
