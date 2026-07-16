package models

import (
	"strings"
	"testing"
)

func TestValidateProjectSlug(t *testing.T) {
	valid := []struct {
		name    string
		slug    string
		sandbox bool
	}{
		{name: "single component", slug: "alpha"},
		{name: "hyphens and digits", slug: "alpha-2"},
		{name: "numeric", slug: "123"},
		{name: "maximum length", slug: strings.Repeat("a", MaxProjectSlugLength)},
		{name: "reserved sandbox component", slug: "_sandbox", sandbox: true},
	}
	for _, tc := range valid {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			if err := ValidateProjectSlug(tc.slug, tc.sandbox); err != nil {
				t.Fatalf("ValidateProjectSlug(%q, %v): %v", tc.slug, tc.sandbox, err)
			}
		})
	}

	invalid := []struct {
		name    string
		slug    string
		sandbox bool
	}{
		{name: "empty", slug: ""},
		{name: "dot traversal", slug: "."},
		{name: "parent traversal", slug: ".."},
		{name: "slash separator", slug: "alpha/beta"},
		{name: "backslash separator", slug: `alpha\beta`},
		{name: "nested traversal", slug: "../escape"},
		{name: "uppercase normalization", slug: "Alpha"},
		{name: "leading hyphen normalization", slug: "-alpha"},
		{name: "repeated hyphen normalization", slug: "alpha--beta"},
		{name: "underscore normalization", slug: "alpha_beta"},
		{name: "unicode normalization", slug: "café"},
		{name: "overlong", slug: strings.Repeat("a", MaxProjectSlugLength+1)},
		{name: "reserved command", slug: "new"},
		{name: "reserved directory", slug: "projects"},
		{name: "sandbox component on ordinary project", slug: "_sandbox"},
	}
	for _, tc := range invalid {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			if err := ValidateProjectSlug(tc.slug, tc.sandbox); err == nil {
				t.Fatalf("ValidateProjectSlug(%q, %v) unexpectedly succeeded", tc.slug, tc.sandbox)
			}
		})
	}
}
