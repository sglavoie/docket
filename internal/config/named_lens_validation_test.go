package config

import (
	"strings"
	"testing"
)

func TestValidateNamedLensID(t *testing.T) {
	cases := map[string]bool{
		"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01":   true,
		"":                                       false,
		"not-a-uuid":                             false,
		"9F8B8E42-7C3A-4D61-8F2A-1B0C9D7E5A01":   false, // uppercase is not canonical
		"{9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01}": false,
		"urn:uuid:9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01":   false,
		"9f8b8e427c3a4d618f2a1b0c9d7e5a01":                false, // unhyphenated
		"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01-9f8b8e4200": false,
	}
	for id, wantValid := range cases {
		err := ValidateNamedLensID(id)
		if wantValid && err != nil {
			t.Errorf("ValidateNamedLensID(%q) = %v, want nil", id, err)
		}
		if !wantValid && err == nil {
			t.Errorf("ValidateNamedLensID(%q) = nil, want error", id)
		}
	}
}

func TestNormalizeNamedLensName(t *testing.T) {
	got, err := NormalizeNamedLensName("  Review Queue  ")
	if err != nil {
		t.Fatalf("NormalizeNamedLensName: %v", err)
	}
	if got != "Review Queue" {
		t.Errorf("NormalizeNamedLensName trimmed = %q, want %q", got, "Review Queue")
	}

	for _, raw := range []string{"", "   ", "line\nbreak", "carriage\rreturn", "bell\a", strings.Repeat("x", MaxNamedLensNameRunes+1)} {
		if _, err := NormalizeNamedLensName(raw); err == nil {
			t.Errorf("NormalizeNamedLensName(%q) = nil error, want rejection", raw)
		}
	}
	if _, err := NormalizeNamedLensName(strings.Repeat("é", MaxNamedLensNameRunes)); err != nil {
		t.Errorf("a %d-rune multibyte name should fit: %v", MaxNamedLensNameRunes, err)
	}
}

func TestNamedLensValidateRejectsMalformedTokens(t *testing.T) {
	cases := []struct {
		name  string
		mutas func(*NamedLens)
	}{
		{"mode", func(l *NamedLens) { l.Mode = "everything" }},
		{"empty mode", func(l *NamedLens) { l.Mode = "" }},
		{"sort field", func(l *NamedLens) { l.Sort.Field = "color" }},
		{"retired status sort field", func(l *NamedLens) { l.Sort.Field = "status" }},
		{"overlong query", func(l *NamedLens) { l.Query = strings.Repeat("x", MaxNamedLensQueryRunes+1) }},
		{"query newline", func(l *NamedLens) { l.Query = "#one\n#two" }},
		{"query control character", func(l *NamedLens) { l.Query = "#one\a" }},
		{"untrimmed name", func(l *NamedLens) { l.Name = " Review Queue" }},
		{"id", func(l *NamedLens) { l.ID = "lens-1" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lens := validLens()
			tc.mutas(&lens)
			if err := lens.Validate(); err == nil {
				t.Fatalf("%s should be rejected", tc.name)
			}
		})
	}
}

func TestNamedLensValidateAcceptsCanonicalDefinition(t *testing.T) {
	if err := validLens().Validate(); err != nil {
		t.Fatalf("canonical lens should validate: %v", err)
	}
}

func TestNamedLensValidateAcceptsOmittedTokenDefaults(t *testing.T) {
	// omitempty writes "" for every axis left at its default, so loading one
	// back must read as the default rather than as a malformed token.
	lens := NamedLens{
		ID:   "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a02",
		Name: "Bare",
		Mode: LensModeToday,
	}
	if err := lens.Validate(); err != nil {
		t.Fatalf("bare lens should validate: %v", err)
	}
}

func TestNamedLensValidateAcceptsCanonicalUnusedTags(t *testing.T) {
	// A canonical tag no task carries is valid; it simply matches nothing.
	lens := validLens()
	lens.Query = "#nobody-uses-this ##nobody-is-here +#nor-this-one"
	if err := lens.Validate(); err != nil {
		t.Fatalf("canonical unused tags should validate: %v", err)
	}
}

// Validation bounds the query and refuses a control character in it; it
// deliberately does not parse it. A query the vocabulary cannot read keeps the
// ordinary invalid-query guidance at projection time instead of making a whole
// workspace file unloadable.
func TestNamedLensValidateDoesNotParseTheQuery(t *testing.T) {
	for _, query := range []string{
		"unparseable-prose",
		"@+3h",
		"$today",
		strings.Repeat("x", MaxNamedLensQueryRunes),
	} {
		lens := validLens()
		lens.Query = query
		if err := lens.Validate(); err != nil {
			t.Errorf("query %q should validate: %v", query, err)
		}
	}
}

// The context facet is two independent halves: a vocabulary and the gap the
// vocabulary cannot name. Either alone is a legal saved facet, so neither may
// require the other to be present.
func TestNamedLensValidateAcceptsEitherHalfOfTheContextFacet(t *testing.T) {
	gapOnly := validLens()
	gapOnly.Query = "##0"
	if err := gapOnly.Validate(); err != nil {
		t.Fatalf("the context gap alone should validate: %v", err)
	}

	namesOnly := validLens()
	namesOnly.Query = "##deep ##errand"
	if err := namesOnly.Validate(); err != nil {
		t.Fatalf("context names alone should validate: %v", err)
	}
}
